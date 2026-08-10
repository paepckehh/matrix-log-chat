# AGENTS.md

Guidance for AI agents working in this repository.


> ## FIXED REQUIREMENT — EVERY CHANGE, NO EXCEPTIONS
>
> Before a task or change is considered done, all five steps below MUST be completed
> in this exact order. Skipping or reordering any step is a failure.
>
> 1. **Format source code** — run `gofmt -w .` 
> 2. **Build** —  go build
> 3. **Test** — `go test -count=1 ./...` must be green 
> 4. **Commit** — `git add . && git commit -m '<message>'`.
> 5. **Tag** — bump the patch segment only: the result is `v0.0.<N+1>`.
>    Never move, delete, or reuse an existing tag.



## Project

`matrix-log-chat` — a small stdin-to-Matrix bridge. It reads syslog lines from
standard input, parses them (RFC 3164 / RFC 5424), decorates each line with
emoji that convey severity and facility, and posts the result to a Matrix
chat room. All configuration is via environment variables; there are no CLI
flags. Module path: `paepcke.de/matrix-log-chat` (vanity path, not GitHub).


## Architecture

```
stdin ── bufio.Scanner ── syslog.Parse ── transform.ToChat ── matrix.Sender ── Matrix room
                                       (config via env)        (rate-limited, retry)
```

Single `main()` in `main.go` owns the loop; all logic lives in three
`internal/` packages. There is no concurrency except a single goroutine that
closes `os.Stdin` on `SIGINT`/`SIGTERM` to unblock the scanner. Sends are
strictly sequential — the rate limiter is a `time.Sleep` after each send.

### Package layout

- `internal/config` — `Load()` reads and validates `MATRIX_*` env vars.
  Required: `MATRIX_HOMESERVER`, `MATRIX_USER`, `MATRIX_TOKEN`, `MATRIX_ROOM`.
  Optional: `MATRIX_RATE_MS` (300), `MATRIX_MAX_RETRIES` (3),
  `MATRIX_GRACE_MS` (500), `MATRIX_DRY_RUN` (false). Returns descriptive
  error listing all missing vars at once.
- `internal/syslog` — `Parse(line string) Event`. **Never returns an error**:
  unmatched input becomes a raw user/notice `Event` so the caller can forward
  it without branching. Detects RFC 5424 by the version digit after `<PRI>`
  and RFC 3164 by the `Mon DD HH:MM:SS` timestamp. RFC 3164 timestamps carry
  no year — the parser assumes the current year and rolls back one year if
  that lands more than 24h in the future.
- `internal/transform` — `ToChat(Event) string`. Renders the fixed-shape chat
  line: `[severityEmoji] [facilityEmoji] <app[pid]> message  @host · HH:MM:SS`.
  Raw events are forwarded verbatim. `Summary(...)` produces the startup log
  line.
- `internal/matrix` — `Sender`. `New()` builds a `gomatrix.Client` and verifies
  the token via `GetOwnDisplayName()` (the library has **no `Whoami()`** —
  despite what older `main.go` revisions may suggest). In dry-run mode `New()`
  returns a sender with no client and `Send()` writes to stderr without
  sleeping or contacting any server. `Send()` retries up to `maxRetries` with a
  fixed `grace` backoff; on final failure the line is **logged and dropped**
  (the bridge never blocks on a single bad line — a syslog stream must keep
  flowing).

### Control flow

1. `config.Load()` — fatal (`log.Fatalf`) on missing/invalid env.
2. `matrix.New()` — fatal on auth check failure (except in dry-run, which
   skips the network entirely).
3. Goroutine registers `SIGINT`/`SIGTERM` and closes `os.Stdin` on signal;
   this is the only shutdown path for a never-closing pipe producer.
4. `bufio.Scanner` over stdin with a 256 KiB per-line buffer (syslog lines
   can be long; the default 64 KiB would silently truncate).
5. Per line: `syslog.Parse` → `transform.ToChat` → `sender.Send`.
6. On scanner EOF or error, exit.

## Conventions and gotchas

- **No CLI flags.** All configuration is env-only. Adding a flag means
  duplicating the env source; prefer extending `internal/config`.
- **`Parse` never errors** — any new syslog quirk belongs in
  `internal/syslog` as an additional regex/fallback, never as an error path
  that the caller has to handle.
- **`Send` drops lines on persistent failure** — by design. A syslog bridge
  must keep flowing; do not add unbounded queuing without an explicit
  overflow policy.
- **Rate limiting is a flat `time.Sleep` after each send**, not a token bucket.
  It spaces messages apart but does not batch. Adjust via `MATRIX_RATE_MS`.
- **Dry-run skips all network and all pacing** so it can be piped into `head`
  or diffed against a fixture. Do not add side effects to the dry-run path.
- **`GetOwnDisplayName`, not `Whoami`** — `github.com/matrix-org/gomatrix`
  exposes no `Whoami` method. The auth check uses `GetOwnDisplayName()`,
  which returns 401 on a bad token.
- **`tail` dependency was removed** — the original design tailed a file; the
  new design reads stdin. If you reintroduce file tailing, do not bring back
  `nxadm/tail` without revisiting the shutdown path (the current signal
  handler closes `os.Stdin`, which only works for the stdin model).
- **Module path is a vanity path** (`paepcke.de/matrix-log-chat`), not a
  GitHub path. Internal packages import it as `paepcke.de/matrix-log-chat/internal/...`.
- **`Makefile` `deps` target wipes `go.mod`/`go.sum`** — do not invoke
  casually; it re-runs `go mod init` with the project basename as the module
  path, which happens to match here but is destructive in general.
- **No tests exist yet.** When adding tests, follow the table-driven style
  and keep them in `*_test.go` next to the code; `make check` does not run
  them, so use `go test ./...` directly.
- **gopls may show stale "unused dependency" warnings** for
  `nxadm/tail`/`fsnotify`/`golang.org/x/sys`/`tomb.v1` after `go mod tidy`.
  These are stale cache entries; `go.mod` already excludes them. Restart
  gopls if the diagnostics do not clear after a save.
