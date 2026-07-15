# klair — a walkthrough

This is a companion to `klair.c`, meant to be read with the source open beside
it. The source itself stays light on comments; the explanations live here
instead.

`klair` controls Elgato Key Light Air devices over HTTP on the LAN: get status,
turn on/off, set brightness and color temperature. It's a rewrite of an
original Go program, done in C as a learning exercise — specifically to see
what Go's standard library was doing for us, by doing it ourselves.

## 1. Orientation

The whole program is one file, `klair.c`, compiled as a single translation
unit ("unity build"). There's no header to jump to, no build graph to
untangle — you read it top to bottom like a book, and by the time a function
is used, its dependencies have already been read. The sections, in order:

1. Types
2. Unit conversion (Kelvin ↔ mired)
3. Config parsing (our own `name = ip` format)
4. Device response parsing (a slice of JSON)
5. Request body building
6. Retry/backoff bookkeeping
7. HTTP over sockets + the `poll()` event loop
8. Validation, light resolution, output formatting
9. CLI parsing and the three commands
10. `main`

A few conventions worth knowing before you start reading:

- **`s32`/`u32`/`u8`/`b32`/`usize`** are just short aliases for `int32_t`,
  `uint32_t`, `uint8_t`, `int32_t` (used for booleans — C has no real bool
  distinction at the ABI level, `_Bool`/`stdbool.h` aside; being explicit that
  a "boolean" is really an int keeps the size honest), and `size_t`. No
  cleverness, just less to type and a consistent width regardless of platform.
- **No `malloc` anywhere.** Every buffer is a fixed-size array on the stack or
  inside a struct: light configs cap out at `MAX_LIGHTS` (16), responses are
  read into an 8 KiB buffer, and so on. A tool that talks to a handful of
  lights on your LAN has a bounded problem size — there's no reason to pay for
  dynamic allocation, and no heap means nothing to leak or double-free.
- **The sentinel `-1` (`UNSET`)** marks "the user didn't pass this flag," for
  `brightness` and `temperature`. This matters for the request body: `on` is
  always sent, but `brightness`/`temperature` are only included when they were
  actually set — see section 5.
- **Every buffer-writing function is bounds-checked** (`snprintf` with a
  capacity, explicit length checks before `strcpy`). This is the C version of
  what a slice-and-`append` gives you for free in Go: you're doing the bounds
  arithmetic yourself, so the code is more verbose, but nothing here can walk
  off the end of a stack buffer via attacker-controlled or malformed input.

## 2. Domain types & unit math

```c
typedef struct { s32 on, brightness, temperature; } LightDetail;
typedef struct { LightDetail light; s32 number_of_lights; } LightStatus;
typedef struct { char name[64]; char ip[64]; } LightConfig;
```

`LightConfig` is what you write by hand. `LightDetail`/`LightStatus` mirror
what the device actually speaks: `on` is `0`/`1`, `brightness` is a percentage,
and `temperature`... is not Kelvin. Elgato's API speaks
[mired](https://en.wikipedia.org/wiki/Mired) — reciprocal color temperature,
`10^6 / kelvin`. Camera and lighting gear often use mired because it's roughly
linear with how "warm/cool" a color looks to the eye, unlike Kelvin.

The CLI exposes Kelvin because that's what people think in. So there's a
conversion at the boundary, `round_to_50`/`mired_to_kelvin`/`kelvin_to_mired`,
done in integer (truncating) arithmetic to match the original program exactly.
`round_to_50` is the classic "round to nearest multiple" trick:
`(n + 25) / 50 * 50` works because adding half the step size before truncating
division rounds instead of floors.

One consequence worth knowing: this conversion is lossy in one direction.
`kelvin_to_mired(6500) = 153`, but `mired_to_kelvin(153) = round_to_50(6535) =
6550`, not 6500. Only Kelvin values with a near-round mired equivalent survive
a round trip — see `test_kelvin_mired_round_trip` in `tests.c` for exactly
which ones do.

## 3. Reading the config

The original Go version used JSON for its config file, because Go's standard
library gives you a JSON parser for free. In C, that's not free — writing a
JSON parser is real work, and it would be scaffolding built to parse a format
we invented ourselves and don't need. So the config format changed to
something a few lines of C can parse outright:

```
left  = 192.168.2.164:9123
right = 192.168.2.165:9123
```

`parse_config` (klair.c) walks the buffer a line at a time using `strchr` to
find `'\n'` boundaries, splits each line on the first `=`, and trims
whitespace from both halves with a small `trim` helper (`isspace` from both
ends, in place). Blank lines are skipped; a non-blank line with no `=` is a
parse error.

Notice this mutates the buffer it's given — `*eq = '\0'` and `*newline =
'\0'` chop the text up in place rather than copying substrings around. This is
a very ordinary C parsing idiom: the input buffer becomes scratch space, and
you only copy out the small pieces you actually want to keep (`strcpy` into
`out[*count].name`/`.ip`, which are fixed 64-byte fields).

## 4. Talking HTTP by hand

Section 7 of `klair.c` builds an HTTP/1.1 request and reads a response using
nothing but BSD sockets. Two ideas underpin it.

**What a socket actually is.** `socket()` asks the kernel for an endpoint;
`connect()` asks it to perform the TCP three-way handshake to a remote
address; `write()`/`read()` move bytes across that connection; `close()` tears
it down. None of this is unique to HTTP — HTTP is just a *text convention*
laid on top of a byte stream. There's no special "HTTP socket."

**What HTTP actually looks like on the wire.** An HTTP/1.1 request is just
text:

```
GET /elgato/lights HTTP/1.1\r\n
Host: 192.168.2.164\r\n
Accept: application/json\r\n
Connection: close\r\n
\r\n
```

Request line, headers (each `Name: value\r\n`), then a blank line (`\r\n\r\n`)
marking the end of headers. A PUT adds a body after that blank line, and a
`Content-Length` header telling the reader how many bytes of body to expect.
The response has the same shape, but starts with a status line
(`HTTP/1.1 200 OK`) instead of a request line.

`build_request` in klair.c constructs this text with `snprintf` directly —
there's no "HTTP request object," just a byte buffer. On the reading side,
`response_complete` looks for `"\r\n\r\n"` to find where headers end, reads the
status code out of the first line with `sscanf`, and — if a `Content-Length`
header is present — waits until it has that many body bytes before declaring
the response complete. If there's no `Content-Length`, we fall back to "the
peer closed the connection" as the completion signal (every request sends
`Connection: close`, so the device is expected to hang up once it's done).

This is targeted parsing, not a general HTTP client: it assumes HTTP/1.1,
a single response, no chunked transfer encoding, no redirects. That's a fair
trade for a program that only ever talks to one specific kind of device.

The device's JSON response gets the same treatment as the config file: rather
than writing a JSON parser, `find_int_after_key` just searches for a key like
`"brightness"`, walks past the following `:`, and reads the integer. It works
because the response shape is fixed and small (`parse_light_status` in
klair.c). Also notice the one deliberately preserved quirk from the original
program: a GET (`getLight`) parses whatever body comes back regardless of
status code, while a PUT (`updateLight`) treats a non-200 status as an error.
That asymmetry was true of the original Go implementation and is reproduced
here (`finish_with_status`).

## 5. The `poll()` event loop — the centerpiece

Here's the problem this section solves: these are cheap wifi devices, and
wifi devices fall off the network. If you talk to three lights one at a time
and the first one is dead, you sit through its *entire* connection timeout —
three seconds, times however many retries — before you even attempt the
second light. That's a real, user-facing bug, not a hypothetical one.

The instinct might be "use threads, one per light." That would be a mistake.
There is no computation happening here — no CPU work to parallelize. The only
thing happening is *waiting*: waiting for a TCP handshake, waiting for bytes
to arrive. The kernel already knows how to wait on many things at once; you
don't need a thread per wait, you need to ask the kernel to wait on your
behalf. That's what `poll()` is for.

**`poll()` is a syscall, not a library trick.** The `poll()` you call from C is
a thin libc wrapper (`<poll.h>`) around a kernel trap. You hand it an array of
`struct pollfd { int fd; short events; short revents; }` — "here are the file
descriptors I care about, and for each, whether I want to know when it's
readable (`POLLIN`) or writable (`POLLOUT`)" — plus a timeout. The kernel parks
your thread (genuinely asleep, zero CPU) until one of three things happens: a
socket becomes ready, the timeout elapses, or a signal arrives. It fills in
`revents` for whichever sockets are ready and wakes you up. One thread,
watching N sockets, truly idle while waiting. This is the boring, correct,
POSIX-portable primitive for this problem (Linux's `epoll` and macOS/BSD's
`kqueue` are faster at *thousands* of fds, but for a handful of lights that
would be premature, non-portable machinery).

**Non-blocking sockets and the state machine.** By default, `connect()` blocks
until the handshake finishes (or fails) — exactly what we're trying to avoid.
`fcntl(fd, F_SETFL, O_NONBLOCK)` changes that: `connect()` returns immediately
with `errno == EINPROGRESS`, meaning "handshake started, ask me later." From
that point on, every step of talking to one light is genuinely asynchronous:

```
JOB_RETRY_WAIT → JOB_CONNECTING → JOB_WRITING → JOB_READING → JOB_DONE
                                                             ↘ JOB_FAILED
```

This is what non-blocking I/O actually *is*: a state machine, made explicit,
because there's no blocking call left to hide it inside. Each `Job` (klair.c)
carries its own socket, buffers, and state — `run_jobs` drives an array of
them through one shared `poll()` call per iteration. When `poll()` returns, we
look at which fds are ready and advance exactly those jobs one step
(`service_job`): a connecting socket gets its error status checked
(`getsockopt(SO_ERROR)` — this is how you learn whether an async `connect()`
ultimately succeeded or failed), a writable socket gets more of the request
written to it, a readable socket gets read from and checked for a complete
response.

**One nice simplification**: the very first attempt and every retry attempt
go through the *same* code path. A freshly-initialized job is seeded as
`JOB_RETRY_WAIT` with `wake_at_ms = now_ms()` — i.e., "ready right now." The
loop's retry-expiry check (`t >= job->wake_at_ms`) fires on the very first
iteration and calls `start_connect`, exactly as it would for a real retry
after backoff. "Start" and "retry" are the same operation, so they're the same
code.

**Timing.** Two clocks matter per job: `deadline_ms` (3 seconds from when the
current attempt started — covers connect+write+read as one budget, mirroring
the original's per-attempt HTTP client timeout) and `wake_at_ms` (when a
backoff wait ends). `poll()`'s timeout argument is computed as "how long until
the *soonest* thing that isn't a socket becoming ready" — the nearest deadline
or wake time across all jobs — so the loop never busy-spins and never
oversleeps past a timeout it needs to notice.

**Retry/backoff** is deliberately factored into its own tiny piece,
`RetryState`/`retry_advance` (section 6), independent of any socket. Three
attempts, waiting 100ms then 200ms between them (doubling), matching the
original. Because it's separated from the networking, `tests.c` can exercise
the exact attempt-counting logic without opening a single socket.

**A worked example.** Say light A is reachable and light B is unreachable
(dead port or dropped off wifi). `run_jobs` starts both: A's connect completes
almost immediately (`POLLOUT` fires within milliseconds, `SO_ERROR` is 0), it
moves through writing and reading and hits `JOB_DONE` — probably within the
same `poll()` call or the next one. B's connect never completes; each loop
iteration, B's fd stays in the `pollfd` array wanting `POLLOUT`, contributing
its `deadline_ms` to the timeout calculation, until 3 seconds pass and the
loop's deadline check fires, closing B's socket and moving it to
`JOB_RETRY_WAIT` for a 100ms backoff, then trying again, then again, finally
landing on `JOB_FAILED` after the third attempt (roughly 9 seconds total from
three 3-second deadlines plus the two short backoffs). The entire time, A's
result was ready and waiting; B being slow never blocked A. Two lights, one
unreachable, complete in ~9 seconds total, not ~18 — same time as if B were
the *only* light. That's the whole point of the exercise.

## 6. CLI & main

Argument parsing is a hand-rolled scan (`parse_common_flags`) rather than
anything general — each command only recognizes `-b/--brightness`,
`-t/--temperature`, `-l/--light` as applicable, taking the next token as the
value. `resolve_lights` implements the filtering: no `-l` means all configured
lights, a matching name means just that one, and an unknown name prints the
available names to stderr and resolves to zero lights (the command then does
nothing, matching the original's "not found, ok=false" behavior).

`validate_brightness`/`validate_temperature` are plain range checks (0–100,
2900–7000). `classify_error` translates the `ErrorKind` set on a failed `Job`
into the same three human messages the original program used, plus a raw
fallback for anything else (JSON parse failures, unexpected status codes).

`main` checks for `--version` first (this doesn't need a flags library for one
flag), resolves the config path (`$HOME/.config/klair`, no override, no
`$XDG_CONFIG_HOME` — one fixed location kept things simple), loads it, and
dispatches on the first remaining argument to `cmd_status`/`cmd_on`/`cmd_off`,
or prints usage for anything else. No command at all is treated as `status`.

## 7. Building & testing

`tests.c` is `#include "klair.c"` with `KLAIR_NO_MAIN` defined first, so
`main` is compiled out and every other function in the file becomes directly
callable — no header, no linking, just one more file that pulls the whole
program in as source. It uses plain `assert()` (no test framework) to check
the pure logic: conversions, config parsing, response parsing, validation,
light resolution, request body construction, and the retry/backoff counting.
HTTP-level tests (spinning up a mock server) were intentionally left out —
the retry/backoff logic that would otherwise need one is tested directly
instead, socket-free.

The Makefile mirrors a typical small-C-project layout: `make` builds an
optimized release binary (`-O2 -DNDEBUG`), `make debug` builds one with
AddressSanitizer and UndefinedBehaviorSanitizer enabled (useful if you're
poking at the socket code — it will loudly complain about buffer overruns or
undefined behavior instead of silently corrupting memory), and `make test`
compiles and runs `tests.c` — deliberately *without* `-DNDEBUG`, since that
flag would compile every `assert()` into a no-op and the test binary would
"pass" by doing nothing.
