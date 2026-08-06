# matrix-log-chat

> Pipe **syslog → Matrix** with one command. Emoji-decorated severity and facility at a glance, env-only config, no flags, no daemon.

`matrix-log-chat` reads syslog lines from **stdin**, decorates them with emoji that signal severity and facility at a glance, and forwards each line as a chat message to a dedicated **Matrix** room. It is designed to sit at the end of a pipe:

```
journalctl -f -o cat | matrix-log-chat
```

The binary is configuration-light: every setting is read from environment variables, so it drops neatly behind `systemd`'s `EnvironmentFile=` or a container orchestrator without any flag plumbing.

---

## Why

Real-time logs in a chat room are surprisingly useful: a green console gives you nothing a phone notification can't, but a red 🔴 in a quiet Matrix room at 03:00 will. `matrix-log-chat` is the smallest possible bridge that gets you there without a log shipper, a queue, an indexer, or a database — just a process at the end of a pipe.

- **Pipe-friendly.** Reads `stdin`, posts to one room. Composes with `journalctl`, `tail -F`, `docker logs -f`, `kubectl logs -f`, anything that writes lines.
- **Two-format syslog parser.** RFC 3164 (BSD) and RFC 5424 (modern). Anything that does not match is forwarded verbatim — so the bridge doubles as a generic `stdin → Matrix` pipe.
- **Emoji-scannable output.** A reader learns to scan the leading icon triple: severity first, facility second, source third. A noisy room becomes instantly parseable.
- **Env-only config.** No flags. Drop behind `EnvironmentFile=` or a `docker run -e …`.
- **Dry-run mode.** `MATRIX_DRY_RUN=true` prints rendered messages to stderr without ever touching the network — perfect for diffing against a fixture or previewing against a recorded log.
- **One static binary.** `CGO_ENABLED=0 go build`, copy to `/usr/local/bin`, done.

---

## Quick start

```sh
export MATRIX_HOMESERVER=https://matrix.example.com
export MATRIX_USER=@logs:matrix.example.com
export MATRIX_TOKEN=syt_…           # see "Provisioning" below
export MATRIX_ROOM='!logs:matrix.example.com'

journalctl -f -o cat | matrix-log-chat
```

Preview the rendered output without any network access:

```sh
journalctl --no-pager -o cat -n 200 | MATRIX_DRY_RUN=true matrix-log-chat
```

---

## Example output

```
🔴🚨 🐧 <kernel> system halted  @host · 12:00:00
🟥 🔐 <sshd[1234]> connection accepted from 1.2.3.4  @host · 12:00:00
📢 👤 <cron[1423]> job completed  @myhost · 22:14:15
hello raw world
```

The shape is fixed so the eye learns it fast:

```
[severityEmoji] [facilityEmoji] <app[pid]> message  @host · HH:MM:SS
```

Raw (non-syslog) input is forwarded verbatim, so the bridge also works as a generic `stdin → Matrix` pipe.

### Emoji reference

**Severity** (the eye's first lock):

| Emoji | Severity |
|------|---------|
| 🔴🚨 | emerg |
| 🚨   | alert |
| 🟥   | crit |
| ❌   | err |
| ⚠️   | warning |
| 📢   | notice |
| ℹ️   | info |
| 🐛   | debug |

**Facility** (where the line came from):

| Emoji | Facility |
|------|----------|
| 🐧 | kern |
| 👤 | user |
| 📧 | mail |
| ⚙️ | daemon |
| 🔐 | auth |
| 📜 | syslog |
| 🖨️ | lpr |
| 📰 | news |
| 📦 | uucp |
| 🕐 | clock |
| 🛡️ | authpriv |
| 📁 | ftp |
| 🕒 | ntp |
| 🧾 | log audit |
| 📣 | log alert |
| 🟦…⬜ | local0–local7 |

Unknown severities / facilities fall back to ❔.

---

## Configuration

All configuration is via environment variables. There are no CLI flags.

| Variable             | Required | Default | Description                                    |
|----------------------|----------|---------|------------------------------------------------|
| `MATRIX_HOMESERVER`  | yes      | —       | Homeserver base URL, e.g. `https://matrix.org` (must include `http://` or `https://`) |
| `MATRIX_USER`        | yes      | —       | Bot MXID, e.g. `@logs:matrix.org`              |
| `MATRIX_TOKEN`       | yes      | —       | Access token for the bot user                  |
| `MATRIX_ROOM`        | yes      | —       | Target room ID, e.g. `!abc123:matrix.org` (a room ID, not an alias) |
| `MATRIX_RATE_MS`     | no       | `300`   | Minimum spacing between sends, in milliseconds |
| `MATRIX_MAX_RETRIES` | no       | `3`     | Send attempts per line before giving up         |
| `MATRIX_GRACE_MS`    | no       | `500`   | Backoff between retries, in milliseconds        |
| `MATRIX_DRY_RUN`     | no       | `false` | Print to stderr instead of posting to Matrix    |

`MATRIX_DRY_RUN=true` skips all network access — handy for previewing the rendered messages against a recorded log file:

```
journalctl --no-pager -o cat -n 200 | MATRIX_DRY_RUN=true matrix-log-chat
```

---

## Build

```sh
make build                  # → ./matrix-log-chat
make check                  # gofmt + go vet + go fix + go test
make test                   # go test ./...
```

Or directly:

```sh
CGO_ENABLED=0 go build -o matrix-log-chat .
```

The `Makefile` target `make deps` is **destructive** (it wipes `go.mod`/`go.sum` and re-runs `go mod init`); do not run it casually.

### Requirements

- Go 1.26+ (uses `time` parsing extensions from the modern toolchain).

---

## Usage

### Run directly

```sh
export MATRIX_HOMESERVER=https://matrix.example.com
export MATRIX_USER=@logs:matrix.example.com
export MATRIX_TOKEN=syt_...
export MATRIX_ROOM=!logs:matrix.example.com

journalctl -f -o cat | matrix-log-chat
```

### systemd unit example

```
[Service]
EnvironmentFile=/etc/matrix-log-chat.env
ExecStart=/usr/local/bin/matrix-log-chat
StandardInput=journal        # or pipe from a producer via ExecStart=/bin/sh -c '... | matrix-log-chat'
Restart=on-failure
```

### Common sources

```sh
# Systemd journal
journalctl -f -o cat | matrix-log-chat

# Plain text file
tail -F /var/log/syslog | matrix-log-chat

# Docker container
docker logs -f myapp | matrix-log-chat

# Kubernetes pod
kubectl logs -f deploy/api | matrix-log-chat

# Anything else that prints lines
echo "hello from the bridge" | matrix-log-chat
```

---

## Architecture

```
stdin ── bufio.Scanner ── syslog.Parse ── transform.ToChat ── matrix.Sender ── Matrix room
                                       (config via env)        (rate-limited, retry)
```

Single `main()` in `main.go` owns the loop; all logic lives in three `internal/` packages. There is no concurrency except a single goroutine that closes `os.Stdin` on `SIGINT`/`SIGTERM` to unblock the scanner. Sends are strictly sequential — the rate limiter is a `time.Sleep` after each send.

### Package layout

| Package | Responsibility |
|--------|----------------|
| `internal/config`   | Loads + validates `MATRIX_*` env vars. Reports all missing vars at once. |
| `internal/syslog`   | Parses RFC 3164 / RFC 5424 into a normalized `Event`. Never returns an error — unmatched input becomes a raw `Event`. |
| `internal/transform`| Renders an `Event` as the fixed-shape chat line. |
| `internal/matrix`   | Wraps `gomatrix` with an auth check, rate-limited sends, bounded retry, and dry-run mode. |

### Behavior notes

- **`Parse` never errors.** Any new syslog quirk belongs in `internal/syslog` as an additional regex/fallback, never as an error path the caller has to handle.
- **`Send` drops lines on persistent failure** — by design. A syslog bridge must keep flowing; do not add unbounded queuing without an explicit overflow policy.
- **Rate limiting is a flat `time.Sleep` after each send**, not a token bucket. It spaces messages apart but does not batch. Adjust via `MATRIX_RATE_MS`.
- **Dry-run skips all network and all pacing** so it can be piped into `head` or diffed against a fixture. Do not add side effects to the dry-run path.
- **Auth check uses `GetOwnDisplayName`**, not a hypothetical `Whoami`. `github.com/matrix-org/gomatrix` exposes no `Whoami`; `GetOwnDisplayName()` returns 401 on a bad token, which is far more useful than failing on the first real send.
- **256 KiB per-line buffer.** `bufio.Scanner`'s default 64 KiB would silently truncate multi-kB stack traces, NDJSON, etc.

See `AGENTS.md` for the full contributor notes, gotchas, and rationale.

---

## Provisioning: accounts, tokens, and rooms

The bridge needs (a) a Matrix user with a known access token and (b) a room the bot can post in. The steps below use the Matrix HTTP API directly so they work against any compliant homeserver (Synapse, Dendrite, Conduit). Any Matrix admin client (Element Web, `matrix-commander`, `mjolnir`, …) can perform the same operations through the UI.

### 1. Register the bot user

Create a dedicated, non-interactive account. Pick a localpart that signals its purpose, e.g. `logs`. On a homeserver that allows open registration:

```sh
curl -XPOST "https://matrix.example.com/_matrix/client/v3/register" \
  -H 'Content-Type: application/json' \
  -d '{"username":"logs","password":"CHANGE_ME_long_random","auth":{"type":"m.login.dummy"}}'
```

If your homeserver requires email/registration token, set the corresponding fields in the `auth` block or register via the admin UI instead.

### 2. Issue an access token

The simplest long-lived token is the one returned by a password login:

```sh
curl -XPOST "https://matrix.example.com/_matrix/client/v3/login" \
  -H 'Content-Type: application/json' \
  -d '{
    "type":"m.login.password",
    "identifier":{"type":"m.id.user","user":"logs"},
    "password":"CHANGE_ME_long_random"
  }'
```

The response contains `access_token` — that is what goes into `MATRIX_TOKEN`. Keep it secret; it is a full-credential bearer token for the bot account.

On Synapse you can also mint an **admin** token that does not expire and is independent of any password change:

```sh
# On the Synapse host: produces a bcrypt hash for the user
synctl hash_password
```

Or, for a token that survives password resets, use the admin API (server admin only):

```sh
curl -XPOST "https://matrix.example.com/_synapse/admin/v1/users/@logs:matrix.example.com/access_token" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"valid_until_ms":0}'
```

`valid_until_ms: 0` means "never expire". The response field `access_token` is what you store in `MATRIX_TOKEN`. **Rotate immediately on leak.**

### 3. Create a dedicated room

The room must be a Matrix room ID (`!...:server`) — Matrix aliases (`#...`) work for joining but the bridge posts to `MATRIX_ROOM` directly, so resolve to the ID.

Create a private, non-federated room (recommended for internal logs):

```sh
curl -XPOST "https://matrix.example.com/_matrix/client/v3/createRoom" \
  -H "Authorization: Bearer $BOT_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "name":"syslog",
    "preset":"private_chat",
    "federate": false,
    "topic":"syslog → Matrix bridge"
  }'
```

The response returns `room_id`, e.g. `!abc123:matrix.example.com` — this is `MATRIX_ROOM`. Store it.

### 4. Invite the bot (if the room was not created by it)

If the room already exists and is owned by another user, invite the bot:

```sh
curl -XPOST "https://matrix.example.com/_matrix/client/v3/rooms/$ROOM_ID/invite" \
  -H "Authorization: Bearer $OWNER_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"@logs:matrix.example.com"}'
```

Then, **as the bot**, accept the invite:

```sh
curl -XPOST "https://matrix.example.com/_matrix/client/v3/rooms/$ROOM_ID/join" \
  -H "Authorization: Bearer $BOT_TOKEN"
```

### 5. Give the bot power to post (optional but recommended)

By default any joined member can send `m.room.message`. If your room's power levels require a higher level, raise the bot's `users` entry:

```sh
curl -XPUT "https://matrix.example.com/_matrix/client/v3/rooms/$ROOM_ID/state/m.room.power_levels" \
  -H "Authorization: Bearer $OWNER_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "ban":50,"kick":50,"invite":50,"redact":50,
    "events_default":0,
    "users":{"@owner:matrix.example.com":100,"@logs:matrix.example.com":0}
  }'
```

`events_default: 0` lets any joined member post messages; the bot only needs that level. Do **not** give the bot moderation powers unless you intend to have it redact spam.

### 6. Verify

```sh
echo "hello from the bridge" | matrix-log-chat
```

A `hello from the bridge` message should appear in the room.

### Token rotation / revocation

On Synapse, revoke a compromised bot token with the admin API:

```sh
curl -XPOST "https://matrix.example.com/_synapse/admin/v1/users/@logs:matrix.example.com/access_tokens/delete" \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

Then re-issue a fresh token via step 2 and update `MATRIX_TOKEN`.

---

## Security notes

- `MATRIX_TOKEN` is a full-credential bearer token for the bot account. Treat it like a password.
- Prefer non-federated private rooms for internal logs. Logs leak infrastructure details; do not federate them across servers you do not control.
- Rotate the token immediately on leak. The Synapse admin API above lets you delete specific tokens without resetting the password.
- The bridge does not log the token, the URL, or the message body of dropped lines beyond what the operator sees at startup (`Summary` logs only the homeserver, user, and room).

---

## Contributing

This is a small, focused tool. PRs that keep it small are welcome.

- Run `make check` before pushing — it runs `gofmt`, `go vet`, `go fix`, and the test suite.
- Keep the package layout: `internal/` only, `main.go` owns the loop.
- Do not add CLI flags — all config is env-only by design. Extend `internal/config` instead.
- Do not add a log queue without an explicit overflow policy. `Send` drops on persistent failure by design — a syslog stream must keep flowing.
- New syslog quirks go in `internal/syslog` as an additional regex/fallback; `Parse` must never return an error.

---

## License

BSD 3-Clause — see [LICENSE](LICENSE).
