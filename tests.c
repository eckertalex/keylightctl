#define KLAIR_NO_MAIN
#include "klair.c"

#include <assert.h>
#include <stdio.h>

static void test_round_to_50(void) {
    assert(round_to_50(0) == 0);
    assert(round_to_50(24) == 0);
    assert(round_to_50(25) == 50);
    assert(round_to_50(50) == 50);
    assert(round_to_50(74) == 50);
    assert(round_to_50(75) == 100);
    assert(round_to_50(3003) == 3000);
    assert(round_to_50(3025) == 3050);
}

static void test_mired_to_kelvin(void) {
    assert(mired_to_kelvin(200) == 5000);
    assert(mired_to_kelvin(250) == 4000);
    assert(mired_to_kelvin(333) == 3000);
    assert(mired_to_kelvin(344) == 2900);
}

static void test_kelvin_to_mired(void) {
    assert(kelvin_to_mired(5000) == 200);
    assert(kelvin_to_mired(4000) == 250);
    assert(kelvin_to_mired(3000) == 333);
    assert(kelvin_to_mired(2900) == 344);
}

static void test_kelvin_mired_round_trip(void) {
    s32 kelvins[] = {2900, 3000, 3500, 4000, 4500, 5000, 5500, 6000};
    for(usize i = 0; i < ArrayCount(kelvins); i++) {
        assert(mired_to_kelvin(kelvin_to_mired(kelvins[i])) == kelvins[i]);
    }
}

static void test_parse_config_single(void) {
    char text[] = "left = 192.168.1.1:9123\n";
    LightConfig lights[MAX_LIGHTS];
    s32 count;
    assert(parse_config(text, lights, MAX_LIGHTS, &count));
    assert(count == 1);
    assert(strcmp(lights[0].name, "left") == 0);
    assert(strcmp(lights[0].ip, "192.168.1.1:9123") == 0);
}

static void test_parse_config_multiple(void) {
    char text[] = "left  = 192.168.1.1:9123\nright = 192.168.1.2:9123\n";
    LightConfig lights[MAX_LIGHTS];
    s32 count;
    assert(parse_config(text, lights, MAX_LIGHTS, &count));
    assert(count == 2);
    assert(strcmp(lights[0].name, "left") == 0);
    assert(strcmp(lights[1].name, "right") == 0);
    assert(strcmp(lights[1].ip, "192.168.1.2:9123") == 0);
}

static void test_parse_config_blank_lines_skipped(void) {
    char text[] = "\nleft = 192.168.1.1:9123\n\n\nright = 192.168.1.2:9123\n\n";
    LightConfig lights[MAX_LIGHTS];
    s32 count;
    assert(parse_config(text, lights, MAX_LIGHTS, &count));
    assert(count == 2);
}

static void test_parse_config_empty(void) {
    char text[] = "";
    LightConfig lights[MAX_LIGHTS];
    s32 count;
    assert(parse_config(text, lights, MAX_LIGHTS, &count));
    assert(count == 0);
}

static void test_parse_config_invalid(void) {
    char text[] = "not a valid line\n";
    LightConfig lights[MAX_LIGHTS];
    s32 count;
    assert(!parse_config(text, lights, MAX_LIGHTS, &count));
}

static void test_parse_light_status(void) {
    const char *json = "{\"numberOfLights\":1,\"lights\":[{\"on\":1,\"brightness\":50,\"temperature\":250}]}";
    LightStatus status;
    assert(parse_light_status(json, &status));
    assert(status.number_of_lights == 1);
    assert(status.light.on == 1);
    assert(status.light.brightness == 50);
    assert(status.light.temperature == 250);
}

static void test_parse_light_status_missing_on(void) {
    const char *json = "{\"numberOfLights\":1,\"lights\":[{}]}";
    LightStatus status;
    assert(!parse_light_status(json, &status));
}

static void test_validate_brightness(void) {
    assert(validate_brightness(0));
    assert(validate_brightness(50));
    assert(validate_brightness(100));
    assert(!validate_brightness(-1));
    assert(!validate_brightness(101));
}

static void test_validate_temperature(void) {
    assert(validate_temperature(2900));
    assert(validate_temperature(5000));
    assert(validate_temperature(7000));
    assert(!validate_temperature(2899));
    assert(!validate_temperature(7001));
}

static void test_resolve_lights(void) {
    LightConfig lights[2] = {
        {.name = "Left", .ip = "192.168.1.1:9123"},
        {.name = "Right", .ip = "192.168.1.2:9123"},
    };
    const LightConfig *out[MAX_LIGHTS];

    assert(resolve_lights(lights, 2, "", out) == 2);
    assert(resolve_lights(lights, 2, "Left", out) == 1);
    assert(strcmp(out[0]->name, "Left") == 0);
    assert(resolve_lights(lights, 2, "Middle", out) == 0);
}

static void test_build_light_body_on_only(void) {
    char buf[256];
    LightDetail d = {.on = 1, .brightness = UNSET, .temperature = UNSET};
    build_light_body(buf, sizeof(buf), d);
    assert(strcmp(buf, "{\"lights\":[{\"on\":1}]}") == 0);
}

static void test_build_light_body_off(void) {
    char buf[256];
    LightDetail d = {.on = 0, .brightness = UNSET, .temperature = UNSET};
    build_light_body(buf, sizeof(buf), d);
    assert(strcmp(buf, "{\"lights\":[{\"on\":0}]}") == 0);
}

static void test_build_light_body_full(void) {
    char buf[256];
    LightDetail d = {.on = 1, .brightness = 75, .temperature = 250};
    build_light_body(buf, sizeof(buf), d);
    assert(strcmp(buf, "{\"lights\":[{\"on\":1,\"brightness\":75,\"temperature\":250}]}") == 0);
}

static void test_retry_succeeds_on_second_attempt(void) {
    RetryState r;
    retry_init(&r, 3, 100.0);

    double wait_ms;
    assert(retry_advance(&r, &wait_ms));
    assert(wait_ms == 100.0);
    assert(r.attempts_left == 2);
    assert(r.backoff_ms == 200.0);
    // Second attempt succeeds: no further retry_advance calls.
}

static void test_resolve_config_path(void) {
    setenv("HOME", "/tmp/klair-test-home", 1);
    char path[512];
    assert(resolve_config_path(path, sizeof(path)));
    assert(strcmp(path, "/tmp/klair-test-home/.config/klair") == 0);
}

static void test_load_config(void) {
    char path[] = "/tmp/klair-test-config-XXXXXX";
    int fd = mkstemp(path);
    assert(fd >= 0);

    const char *content = "left = 192.168.1.1:9123\nright = 192.168.1.2:9123\n";
    assert(write(fd, content, strlen(content)) == (ssize_t)strlen(content));
    close(fd);

    LightConfig lights[MAX_LIGHTS];
    s32 count;
    assert(load_config(path, lights, MAX_LIGHTS, &count));
    assert(count == 2);
    assert(strcmp(lights[0].name, "left") == 0);
    assert(strcmp(lights[1].name, "right") == 0);

    unlink(path);
}

static void test_load_config_missing_file(void) {
    LightConfig lights[MAX_LIGHTS];
    s32 count;
    assert(!load_config("/tmp/klair-test-does-not-exist", lights, MAX_LIGHTS, &count));
}

static void test_retry_exhausted(void) {
    RetryState r;
    retry_init(&r, 3, 100.0);

    double wait_ms;
    assert(retry_advance(&r, &wait_ms) && wait_ms == 100.0);
    assert(retry_advance(&r, &wait_ms) && wait_ms == 200.0);
    assert(!retry_advance(&r, &wait_ms));
    assert(r.attempts_left == 0);
}

int main(void) {
    test_round_to_50();
    test_mired_to_kelvin();
    test_kelvin_to_mired();
    test_kelvin_mired_round_trip();
    test_parse_config_single();
    test_parse_config_multiple();
    test_parse_config_blank_lines_skipped();
    test_parse_config_empty();
    test_parse_config_invalid();
    test_parse_light_status();
    test_parse_light_status_missing_on();
    test_validate_brightness();
    test_validate_temperature();
    test_resolve_lights();
    test_build_light_body_on_only();
    test_build_light_body_off();
    test_build_light_body_full();
    test_resolve_config_path();
    test_load_config();
    test_load_config_missing_file();
    test_retry_succeeds_on_second_attempt();
    test_retry_exhausted();

    printf("all tests passed\n");
    return 0;
}
