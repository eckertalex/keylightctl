#include <assert.h>
#include <ctype.h>
#include <errno.h>
#include <fcntl.h>
#include <netdb.h>
#include <poll.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <strings.h>
#include <sys/socket.h>
#include <time.h>
#include <unistd.h>

typedef int32_t s32;
typedef uint32_t u32;
typedef uint8_t u8;
typedef int32_t b32;
typedef size_t usize;

#ifndef KLAIR_VERSION
#define KLAIR_VERSION "v0.0.0"
#endif

#define MAX_LIGHTS 16
#define UNSET (-1)
#define ArrayCount(a) (sizeof(a) / sizeof((a)[0]))

typedef struct {
    s32 on;
    s32 brightness;
    s32 temperature;
} LightDetail;

typedef struct {
    LightDetail light;
    s32 number_of_lights;
} LightStatus;

typedef struct {
    char name[64];
    char ip[64];
} LightConfig;

// --- unit conversion ---
// The Elgato wire protocol speaks mired; the CLI speaks Kelvin. Integer math,
// truncating, matching the original Go implementation exactly.

static s32 round_to_50(s32 n) {
    return (n + 25) / 50 * 50;
}

static s32 mired_to_kelvin(s32 mired) {
    return round_to_50(1000000 / mired);
}

static s32 kelvin_to_mired(s32 kelvin) {
    return 1000000 / kelvin;
}

// --- config parsing ---
// name = ip, one per line. No JSON: this is our own format, so we write the
// few lines of scanning code instead of a general-purpose parser.

static char *trim(char *s) {
    while(isspace((unsigned char)*s)) {
        s++;
    }

    if(*s == '\0') {
        return s;
    }

    char *end = s + strlen(s) - 1;
    while(end > s && isspace((unsigned char)*end)) {
        end--;
    }
    end[1] = '\0';

    return s;
}

static b32 parse_config(char *text, LightConfig *out, s32 max, s32 *count) {
    *count = 0;
    char *line = text;

    while(line && *line) {
        char *newline = strchr(line, '\n');
        if(newline) {
            *newline = '\0';
        }

        char *trimmed = trim(line);
        if(*trimmed == '\0') {
            line = newline ? newline + 1 : NULL;
            continue;
        }

        char *eq = strchr(trimmed, '=');
        if(!eq) {
            return false;
        }
        *eq = '\0';

        char *name = trim(trimmed);
        char *ip = trim(eq + 1);

        if(*count >= max) {
            return false;
        }
        if(strlen(name) >= sizeof(out[0].name) || strlen(ip) >= sizeof(out[0].ip)) {
            return false;
        }

        strcpy(out[*count].name, name);
        strcpy(out[*count].ip, ip);
        (*count)++;

        line = newline ? newline + 1 : NULL;
    }

    return true;
}

static b32 resolve_config_path(char *buf, usize cap) {
    const char *home = getenv("HOME");
    if(!home || *home == '\0') {
        return false;
    }

    s32 len = snprintf(buf, cap, "%s/.config/klair", home);
    return len > 0 && (usize)len < cap;
}

static b32 load_config(const char *path, LightConfig *out, s32 max, s32 *count) {
    FILE *f = fopen(path, "rb");
    if(!f) {
        return false;
    }

    char buf[8192];
    usize n = fread(buf, 1, sizeof(buf) - 1, f);
    fclose(f);
    buf[n] = '\0';

    return parse_config(buf, out, max, count);
}

// --- device response parsing ---
// The wire format here is JSON, but it's a fixed shape we don't control
// (Elgato's), and it's tiny. So instead of writing a general JSON parser we
// pull exactly the three integer fields we need out of the response text.

static b32 find_int_after_key(const char *json, const char *key, s32 *out) {
    const char *p = strstr(json, key);
    if(!p) {
        return false;
    }

    p = strchr(p + strlen(key), ':');
    if(!p) {
        return false;
    }
    p++;

    while(isspace((unsigned char)*p)) {
        p++;
    }

    char *end;
    long value = strtol(p, &end, 10);
    if(end == p) {
        return false;
    }

    *out = (s32)value;
    return true;
}

static b32 parse_light_status(const char *json, LightStatus *out) {
    memset(out, 0, sizeof(*out));

    find_int_after_key(json, "\"numberOfLights\"", &out->number_of_lights);

    if(!find_int_after_key(json, "\"on\"", &out->light.on)) {
        return false;
    }

    find_int_after_key(json, "\"brightness\"", &out->light.brightness);
    find_int_after_key(json, "\"temperature\"", &out->light.temperature);

    return true;
}

// --- request body ---

static s32 build_light_body(char *buf, usize cap, LightDetail d) {
    s32 len = snprintf(buf, cap, "{\"lights\":[{\"on\":%d", d.on);

    if(d.brightness != UNSET) {
        len += snprintf(buf + len, cap - (usize)len, ",\"brightness\":%d", d.brightness);
    }
    if(d.temperature != UNSET) {
        len += snprintf(buf + len, cap - (usize)len, ",\"temperature\":%d", d.temperature);
    }

    len += snprintf(buf + len, cap - (usize)len, "}]}");
    return len;
}

// --- retry / backoff ---
// Factored out of the networking code so it's testable without a socket.
// 3 attempts total; on failure wait `backoff_ms`, then double it.

typedef struct {
    s32 attempts_left;
    double backoff_ms;
} RetryState;

static void retry_init(RetryState *r, s32 max_attempts, double initial_backoff_ms) {
    r->attempts_left = max_attempts;
    r->backoff_ms = initial_backoff_ms;
}

static b32 retry_advance(RetryState *r, double *wait_ms) {
    r->attempts_left--;
    if(r->attempts_left <= 0) {
        return false;
    }

    *wait_ms = r->backoff_ms;
    r->backoff_ms *= 2;
    return true;
}

// --- HTTP over sockets + poll() event loop ---
//
// A dead light eating its full connect timeout while other lights wait behind
// it is a real failure mode on flaky wifi gear. Fixing it is an I/O-waiting
// problem, not a compute problem: one thread, an array of non-blocking
// sockets, one poll() call. Each light is a small state machine because
// that's what non-blocking I/O actually is.

typedef enum {
    METHOD_GET,
    METHOD_PUT,
} Method;

typedef enum {
    JOB_RETRY_WAIT,
    JOB_CONNECTING,
    JOB_WRITING,
    JOB_READING,
    JOB_DONE,
    JOB_FAILED,
} JobState;

typedef enum {
    ERR_NONE,
    ERR_TIMEOUT,
    ERR_CLOSED,
    ERR_CONNECT_FAILED,
    ERR_OTHER,
} ErrorKind;

typedef struct {
    const LightConfig *cfg;
    Method method;

    char host[128];
    char port[16];

    char request[512];
    s32 request_len;
    s32 request_sent;

    char response[8192];
    s32 response_len;

    int fd;
    JobState state;

    RetryState retry;
    double wake_at_ms;
    double deadline_ms;

    b32 ok;
    LightStatus result;
    ErrorKind err_kind;
    char err[128];
} Job;

static double now_ms(void) {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return (double)ts.tv_sec * 1000.0 + (double)ts.tv_nsec / 1e6;
}

static b32 split_host_port(const char *ip, char *host, usize host_cap, char *port, usize port_cap) {
    const char *colon = strrchr(ip, ':');
    if(!colon) {
        return false;
    }

    usize host_len = (usize)(colon - ip);
    if(host_len >= host_cap) {
        return false;
    }
    memcpy(host, ip, host_len);
    host[host_len] = '\0';

    usize port_len = strlen(colon + 1);
    if(port_len >= port_cap) {
        return false;
    }
    strcpy(port, colon + 1);

    return true;
}

static b32 build_request(Job *job, LightDetail *settings) {
    if(job->method == METHOD_GET) {
        job->request_len = snprintf(job->request, sizeof(job->request),
            "GET /elgato/lights HTTP/1.1\r\n"
            "Host: %s\r\n"
            "Accept: application/json\r\n"
            "Connection: close\r\n"
            "\r\n",
            job->host);
    } else {
        char body[256];
        s32 body_len = build_light_body(body, sizeof(body), *settings);

        job->request_len = snprintf(job->request, sizeof(job->request),
            "PUT /elgato/lights HTTP/1.1\r\n"
            "Host: %s\r\n"
            "Content-Type: application/json\r\n"
            "Accept: application/json\r\n"
            "Content-Length: %d\r\n"
            "Connection: close\r\n"
            "\r\n"
            "%s",
            job->host, body_len, body);
    }

    return job->request_len > 0 && (usize)job->request_len < sizeof(job->request);
}

static void fail_or_retry(Job *job) {
    double wait_ms;
    if(retry_advance(&job->retry, &wait_ms)) {
        job->state = JOB_RETRY_WAIT;
        job->wake_at_ms = now_ms() + wait_ms;
    } else {
        job->state = JOB_FAILED;
    }
}

static void job_init(Job *job, const LightConfig *cfg, Method method, LightDetail *settings) {
    memset(job, 0, sizeof(*job));
    job->cfg = cfg;
    job->method = method;
    job->fd = -1;
    job->err_kind = ERR_NONE;
    retry_init(&job->retry, 3, 100.0);

    if(!split_host_port(cfg->ip, job->host, sizeof(job->host), job->port, sizeof(job->port))) {
        job->err_kind = ERR_OTHER;
        snprintf(job->err, sizeof(job->err), "invalid address: %s", cfg->ip);
        job->state = JOB_FAILED;
        return;
    }

    if(!build_request(job, settings)) {
        job->err_kind = ERR_OTHER;
        snprintf(job->err, sizeof(job->err), "request too large");
        job->state = JOB_FAILED;
        return;
    }

    // Seed the job as an already-elapsed retry wait: the very first attempt
    // and every subsequent retry go through the exact same code path.
    job->state = JOB_RETRY_WAIT;
    job->wake_at_ms = now_ms();
}

static b32 start_connect(Job *job) {
    struct addrinfo hints;
    memset(&hints, 0, sizeof(hints));
    hints.ai_family = AF_UNSPEC;
    hints.ai_socktype = SOCK_STREAM;

    struct addrinfo *result;
    int rc = getaddrinfo(job->host, job->port, &hints, &result);
    if(rc != 0) {
        job->err_kind = ERR_CONNECT_FAILED;
        snprintf(job->err, sizeof(job->err), "resolve failed: %s", gai_strerror(rc));
        return false;
    }

    int fd = socket(result->ai_family, result->ai_socktype, result->ai_protocol);
    if(fd < 0) {
        freeaddrinfo(result);
        job->err_kind = ERR_OTHER;
        snprintf(job->err, sizeof(job->err), "socket: %s", strerror(errno));
        return false;
    }

    int flags = fcntl(fd, F_GETFL, 0);
    fcntl(fd, F_SETFL, flags | O_NONBLOCK);

    int rc2 = connect(fd, result->ai_addr, result->ai_addrlen);
    freeaddrinfo(result);

    if(rc2 < 0 && errno != EINPROGRESS) {
        close(fd);
        job->err_kind = ERR_CONNECT_FAILED;
        snprintf(job->err, sizeof(job->err), "connect: %s", strerror(errno));
        return false;
    }

    job->fd = fd;
    job->state = JOB_CONNECTING;
    job->request_sent = 0;
    job->response_len = 0;
    job->deadline_ms = now_ms() + 3000.0;
    return true;
}

static const char *find_header(const char *headers, const char *header_end, const char *name) {
    usize name_len = strlen(name);
    for(const char *p = headers; p + name_len <= header_end; p++) {
        if(strncasecmp(p, name, name_len) == 0) {
            return p + name_len;
        }
    }
    return NULL;
}

// Have we received a complete HTTP response? Uses Content-Length when present;
// otherwise the caller falls back to "connection closed = response complete".
static b32 response_complete(Job *job, s32 *out_status, char **out_body, s32 *out_body_len) {
    char *header_end = strstr(job->response, "\r\n\r\n");
    if(!header_end) {
        return false;
    }

    s32 status = 0;
    if(sscanf(job->response, "HTTP/%*d.%*d %d", &status) != 1) {
        return false;
    }

    char *body = header_end + 4;
    s32 header_len = (s32)(body - job->response);
    s32 body_len = job->response_len - header_len;

    const char *cl = find_header(job->response, header_end, "Content-Length:");
    if(cl) {
        s32 content_length = (s32)strtol(cl, NULL, 10);
        if(body_len < content_length) {
            return false;
        }
    }

    *out_status = status;
    *out_body = body;
    *out_body_len = body_len;
    return true;
}

static void finish_with_status(Job *job, s32 status, char *body, s32 body_len) {
    (void)body_len;

    // Only updateLight (PUT) checks the status code; getLight (GET) parses
    // whatever body comes back regardless, matching the original Go asymmetry.
    if(job->method == METHOD_PUT && status != 200) {
        job->err_kind = ERR_OTHER;
        snprintf(job->err, sizeof(job->err), "unexpected status %d: %s", status, body);
        fail_or_retry(job);
        return;
    }

    if(!parse_light_status(body, &job->result)) {
        job->err_kind = ERR_OTHER;
        snprintf(job->err, sizeof(job->err), "parsing response failed");
        fail_or_retry(job);
        return;
    }

    job->ok = true;
    job->state = JOB_DONE;
}

static void service_job(Job *job) {
    if(job->state == JOB_CONNECTING) {
        int err = 0;
        socklen_t len = sizeof(err);
        if(getsockopt(job->fd, SOL_SOCKET, SO_ERROR, &err, &len) < 0) {
            err = errno;
        }

        if(err != 0) {
            close(job->fd);
            job->fd = -1;
            job->err_kind = ERR_CONNECT_FAILED;
            snprintf(job->err, sizeof(job->err), "connect: %s", strerror(err));
            fail_or_retry(job);
            return;
        }

        job->state = JOB_WRITING;
        return;
    }

    if(job->state == JOB_WRITING) {
        ssize_t n = write(job->fd, job->request + job->request_sent,
            (usize)(job->request_len - job->request_sent));

        if(n < 0) {
            if(errno == EAGAIN || errno == EWOULDBLOCK || errno == EINTR) {
                return;
            }
            close(job->fd);
            job->fd = -1;
            job->err_kind = ERR_OTHER;
            snprintf(job->err, sizeof(job->err), "write: %s", strerror(errno));
            fail_or_retry(job);
            return;
        }

        job->request_sent += (s32)n;
        if(job->request_sent >= job->request_len) {
            job->state = JOB_READING;
        }
        return;
    }

    if(job->state == JOB_READING) {
        if(job->response_len >= (s32)sizeof(job->response) - 1) {
            close(job->fd);
            job->fd = -1;
            job->err_kind = ERR_OTHER;
            snprintf(job->err, sizeof(job->err), "response too large");
            fail_or_retry(job);
            return;
        }

        ssize_t n = read(job->fd, job->response + job->response_len,
            sizeof(job->response) - 1 - (usize)job->response_len);

        if(n < 0) {
            if(errno == EAGAIN || errno == EWOULDBLOCK || errno == EINTR) {
                return;
            }
            close(job->fd);
            job->fd = -1;
            job->err_kind = ERR_OTHER;
            snprintf(job->err, sizeof(job->err), "read: %s", strerror(errno));
            fail_or_retry(job);
            return;
        }

        if(n == 0) {
            close(job->fd);
            job->fd = -1;
            job->response[job->response_len] = '\0';

            s32 status, body_len;
            char *body;
            if(response_complete(job, &status, &body, &body_len)) {
                finish_with_status(job, status, body, body_len);
            } else {
                job->err_kind = ERR_CLOSED;
                snprintf(job->err, sizeof(job->err), "connection closed before response completed");
                fail_or_retry(job);
            }
            return;
        }

        job->response_len += (s32)n;
        job->response[job->response_len] = '\0';

        s32 status, body_len;
        char *body;
        if(response_complete(job, &status, &body, &body_len)) {
            close(job->fd);
            job->fd = -1;
            finish_with_status(job, status, body, body_len);
        }
        return;
    }
}

static void run_jobs(Job *jobs, s32 n) {
    for(;;) {
        struct pollfd fds[MAX_LIGHTS];
        s32 fd_job_index[MAX_LIGHTS];
        s32 fd_count = 0;

        b32 all_done = true;
        double next_wake = -1;
        double t = now_ms();

        for(s32 i = 0; i < n; i++) {
            Job *job = &jobs[i];
            if(job->state == JOB_DONE || job->state == JOB_FAILED) {
                continue;
            }
            all_done = false;

            if(job->state == JOB_RETRY_WAIT) {
                if(next_wake < 0 || job->wake_at_ms < next_wake) {
                    next_wake = job->wake_at_ms;
                }
                continue;
            }

            fds[fd_count].fd = job->fd;
            fds[fd_count].events = (job->state == JOB_READING) ? POLLIN : POLLOUT;
            fds[fd_count].revents = 0;
            fd_job_index[fd_count] = i;
            fd_count++;

            if(next_wake < 0 || job->deadline_ms < next_wake) {
                next_wake = job->deadline_ms;
            }
        }

        if(all_done) {
            break;
        }

        double timeout_ms = next_wake < 0 ? 3000.0 : next_wake - t;
        if(timeout_ms < 0) {
            timeout_ms = 0;
        }

        poll(fds, (nfds_t)fd_count, (int)timeout_ms);

        for(s32 k = 0; k < fd_count; k++) {
            if(fds[k].revents != 0) {
                service_job(&jobs[fd_job_index[k]]);
            }
        }

        t = now_ms();
        for(s32 i = 0; i < n; i++) {
            Job *job = &jobs[i];
            if(job->state == JOB_DONE || job->state == JOB_FAILED) {
                continue;
            }

            if(job->state == JOB_RETRY_WAIT) {
                if(t >= job->wake_at_ms && !start_connect(job)) {
                    fail_or_retry(job);
                }
                continue;
            }

            if(t >= job->deadline_ms) {
                close(job->fd);
                job->fd = -1;
                job->err_kind = ERR_TIMEOUT;
                snprintf(job->err, sizeof(job->err), "timed out");
                fail_or_retry(job);
            }
        }
    }
}

typedef struct {
    b32 ok;
    LightStatus status;
    ErrorKind err_kind;
    char err[128];
} LightResult;

static void run_lights(const LightConfig *targets, s32 count, Method method, LightDetail *settings, LightResult *results) {
    Job jobs[MAX_LIGHTS];
    for(s32 i = 0; i < count; i++) {
        job_init(&jobs[i], &targets[i], method, settings);
    }

    run_jobs(jobs, count);

    for(s32 i = 0; i < count; i++) {
        results[i].ok = jobs[i].ok;
        results[i].status = jobs[i].result;
        results[i].err_kind = jobs[i].err_kind;
        strcpy(results[i].err, jobs[i].err);
    }
}

static const char *classify_error(ErrorKind kind, const char *raw) {
    switch(kind) {
        case ERR_TIMEOUT: return "timeout while connecting";
        case ERR_CLOSED: return "connection closed unexpectedly";
        case ERR_CONNECT_FAILED: return "failed to connect";
        default: return raw;
    }
}

// --- validation ---

static b32 validate_brightness(s32 n) {
    return n >= 0 && n <= 100;
}

static b32 validate_temperature(s32 n) {
    return n >= 2900 && n <= 7000;
}

// --- light resolution ---

static s32 resolve_lights(const LightConfig *lights, s32 count, const char *name, const LightConfig **out) {
    if(!name || *name == '\0') {
        for(s32 i = 0; i < count; i++) {
            out[i] = &lights[i];
        }
        return count;
    }

    for(s32 i = 0; i < count; i++) {
        if(strcmp(lights[i].name, name) == 0) {
            out[0] = &lights[i];
            return 1;
        }
    }

    fprintf(stderr, "light \"%s\" not found; available: ", name);
    for(s32 i = 0; i < count; i++) {
        fprintf(stderr, "%s%s", i ? ", " : "", lights[i].name);
    }
    fprintf(stderr, "\n");
    return 0;
}

// --- output ---

static const char *format_on_off(s32 on) {
    return on == 1 ? "ON" : "OFF";
}

static void print_light_result(const char *name, const char *verb, LightResult *result) {
    if(!result->ok) {
        printf("%s of light \"%s\": error: %s\n", verb, name, classify_error(result->err_kind, result->err));
        return;
    }

    printf("Status of light \"%s\":\n", name);
    printf("  Power:       %s\n", format_on_off(result->status.light.on));
    printf("  Brightness:  %d%%\n", result->status.light.brightness);
    printf("  Temperature: %dK\n", mired_to_kelvin(result->status.light.temperature));
}

static void print_usage(void) {
    fprintf(stderr,
        "Usage: klair [--version] <command> [flags]\n"
        "\n"
        "Commands:\n"
        "  status  [-l NAME]\n"
        "  on      [-b 0-100] [-t 2900-7000] [-l NAME]\n"
        "  off     [-l NAME]\n"
        "\n"
        "No command: same as status.\n");
}

// --- CLI parsing + commands ---

static b32 match_flag(const char *arg, const char *short_name, const char *long_name) {
    return strcmp(arg, short_name) == 0 || strcmp(arg, long_name) == 0;
}

static b32 parse_common_flags(char **args, s32 count, s32 *brightness, s32 *temperature, char *light_name, usize light_name_cap) {
    for(s32 i = 0; i < count; i++) {
        if(brightness && match_flag(args[i], "-b", "--brightness")) {
            if(i + 1 >= count) {
                return false;
            }
            *brightness = atoi(args[++i]);
        } else if(temperature && match_flag(args[i], "-t", "--temperature")) {
            if(i + 1 >= count) {
                return false;
            }
            *temperature = atoi(args[++i]);
        } else if(match_flag(args[i], "-l", "--light")) {
            if(i + 1 >= count) {
                return false;
            }
            strncpy(light_name, args[++i], light_name_cap - 1);
            light_name[light_name_cap - 1] = '\0';
        } else {
            return false;
        }
    }
    return true;
}

static void cmd_status(const LightConfig *lights, s32 light_count, char **args, s32 arg_count) {
    char light_name[64] = {0};
    if(!parse_common_flags(args, arg_count, NULL, NULL, light_name, sizeof(light_name))) {
        print_usage();
        exit(1);
    }

    const LightConfig *targets[MAX_LIGHTS];
    s32 target_count = resolve_lights(lights, light_count, light_name, targets);
    if(target_count == 0) {
        return;
    }

    LightConfig target_cfgs[MAX_LIGHTS];
    for(s32 i = 0; i < target_count; i++) {
        target_cfgs[i] = *targets[i];
    }

    LightResult results[MAX_LIGHTS];
    run_lights(target_cfgs, target_count, METHOD_GET, NULL, results);

    for(s32 i = 0; i < target_count; i++) {
        print_light_result(target_cfgs[i].name, "Status", &results[i]);
    }
}

static void cmd_on(const LightConfig *lights, s32 light_count, char **args, s32 arg_count) {
    s32 brightness = UNSET;
    s32 temperature = UNSET;
    char light_name[64] = {0};

    if(!parse_common_flags(args, arg_count, &brightness, &temperature, light_name, sizeof(light_name))) {
        print_usage();
        exit(1);
    }

    LightDetail settings = {.on = 1, .brightness = UNSET, .temperature = UNSET};

    if(brightness != UNSET) {
        if(!validate_brightness(brightness)) {
            fprintf(stderr, "invalid brightness: must be between 0 and 100\n");
            return;
        }
        settings.brightness = brightness;
    }

    if(temperature != UNSET) {
        if(!validate_temperature(temperature)) {
            fprintf(stderr, "invalid temperature: must be between 2900K and 7000K\n");
            return;
        }
        settings.temperature = kelvin_to_mired(temperature);
    }

    const LightConfig *targets[MAX_LIGHTS];
    s32 target_count = resolve_lights(lights, light_count, light_name, targets);
    if(target_count == 0) {
        return;
    }

    LightConfig target_cfgs[MAX_LIGHTS];
    for(s32 i = 0; i < target_count; i++) {
        target_cfgs[i] = *targets[i];
    }

    LightResult results[MAX_LIGHTS];
    run_lights(target_cfgs, target_count, METHOD_PUT, &settings, results);

    for(s32 i = 0; i < target_count; i++) {
        print_light_result(target_cfgs[i].name, "Update", &results[i]);
    }
}

static void cmd_off(const LightConfig *lights, s32 light_count, char **args, s32 arg_count) {
    char light_name[64] = {0};
    if(!parse_common_flags(args, arg_count, NULL, NULL, light_name, sizeof(light_name))) {
        print_usage();
        exit(1);
    }

    LightDetail settings = {.on = 0, .brightness = UNSET, .temperature = UNSET};

    const LightConfig *targets[MAX_LIGHTS];
    s32 target_count = resolve_lights(lights, light_count, light_name, targets);
    if(target_count == 0) {
        return;
    }

    LightConfig target_cfgs[MAX_LIGHTS];
    for(s32 i = 0; i < target_count; i++) {
        target_cfgs[i] = *targets[i];
    }

    LightResult results[MAX_LIGHTS];
    run_lights(target_cfgs, target_count, METHOD_PUT, &settings, results);

    for(s32 i = 0; i < target_count; i++) {
        print_light_result(target_cfgs[i].name, "Update", &results[i]);
    }
}

#ifndef KLAIR_NO_MAIN
int main(int argc, char **argv) {
    if(argc >= 2 && (strcmp(argv[1], "--version") == 0 || strcmp(argv[1], "-version") == 0)) {
        printf("%s\n", KLAIR_VERSION);
        return 0;
    }

    char config_path[512];
    if(!resolve_config_path(config_path, sizeof(config_path))) {
        fprintf(stderr, "cannot determine home directory\n");
        return 1;
    }

    LightConfig lights[MAX_LIGHTS];
    s32 light_count = 0;
    if(!load_config(config_path, lights, MAX_LIGHTS, &light_count)) {
        fprintf(stderr, "error loading config: %s\n", config_path);
        return 1;
    }

    if(argc < 2) {
        cmd_status(lights, light_count, NULL, 0);
        return 0;
    }

    char **cmd_args = argv + 2;
    s32 cmd_arg_count = argc - 2;

    if(strcmp(argv[1], "status") == 0) {
        cmd_status(lights, light_count, cmd_args, cmd_arg_count);
    } else if(strcmp(argv[1], "on") == 0) {
        cmd_on(lights, light_count, cmd_args, cmd_arg_count);
    } else if(strcmp(argv[1], "off") == 0) {
        cmd_off(lights, light_count, cmd_args, cmd_arg_count);
    } else {
        fprintf(stderr, "unknown command: %s\n\n", argv[1]);
        print_usage();
        return 1;
    }

    return 0;
}
#endif
