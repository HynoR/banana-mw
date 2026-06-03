# Repository Guidelines

## What This Project Is

`banana-mw` is a Go reverse proxy for OTA-style API paths. It enforces path/token guards, optional Redis secure sessions, in-memory response caching, 48-hour Redis stats, and an embedded admin dashboard. One process runs **two** `http.Server`s: the reverse proxy on `port` (always) and, when `is_admin=1`, the admin dashboard on a separate `admin_port`.

## Repository Layout

```
cmd/banana-mw/          # main entry (signal handling → server.Run)
internal/
  config/                 # YAML/JSON load, defaults, log_level, duration parsing
  server/                 # app bootstrap, ProxyRouter / AdminRouter, http.Server
  cache/                  # memory 200/4xx caches + cache middleware
  middleware/             # recovery, guards, debug request trace (log_level=debug)
  secure/                 # Redis session middleware, /active handler
  stats/                  # async stats workers, cleaner, admin/legacy API handlers
  admin/                  # embedded UI + health handler; registers chi routes
  adminauth/              # Bearer token check for admin JSON APIs
  redisstore/             # shared Redis client (secure + stats)
  requtil/                # token/IP/JSON helpers, StatusCaptureWriter
config.example.yaml       # documented template (committed)
data/                     # runtime config (gitignored except data/README.md)
build/                    # local binaries (gitignored except build/README.md)
Dockerfile, docker-compose.yml, makefile, README.md
```

Module path: `hynor/banana-mw`. Go **1.26.4**.

## Runtime Modes (Important)

| `is_admin` | Behavior |
|------------|----------|
| `0` (default) | **Proxy only**: `ProxyRouter` on `port`. Business traffic goes through the guard/cache/proxy chain. No admin routes are exposed anywhere. |
| `1` | **Proxy + admin**: `ProxyRouter` on `port` **and** `AdminRouter` on `admin_port` (separate `http.Server`). Admin routes (`/_gogoadmin/`, `/banana-mw/api/get`) live only on `admin_port`, never on `port`. |

Two `http.Server`s are started by `runHTTPServers` (`internal/server/http.go`) and share a context for graceful shutdown. `admin_port` must differ from `port` when `is_admin=1` (validated at config load). `ProxyRouter` no longer registers admin routes — splitting onto separate ports is what removes the path-conflict / unguarded-admin exposure.

## Proxy Middleware Order

Implemented in `internal/server/router.go` (`proxyChain`, inner → outer call order):

1. `RequestTrace` (only when `log_level=debug`) — one `DEBUG request` log per request
2. `MethodGuard` — GET/POST only
3. `UserAgentGuard` — non-empty UA
4. `PathPrefixGuard` — `allowed_prefixes`
5. `TokenGuard` — query/form/link token shape
6. `SecureMiddleware` — if `secure=1`
7. `StatsMiddleware` — if `stats_enabled=1`
8. `Cache4xx` then `Cache200` + reverse proxy to `upstream`

## Logging

- JSON `slog` to stdout; `time` field formatted as `2006-01-02T15:04:05`.
- `log_level`: `info` (default), `debug`, `warn`, `error` — set in `data/config.yaml`.
- **`info`**: `access` logs for requests that reach the cache layer (hit/miss + upstream status). Guard/secure blocks mostly emit `WARN` only on failures.
- **`debug`**: per-request `request` trace (secure outcome, cache flags, `response_source`, `blocked_by`); suppresses duplicate `access` lines.

Durations in logs use human-readable strings (not raw nanoseconds) where `internal/stats/logattr` or explicit `.String()` is used.

## Build, Test, and Development

| Command | Purpose |
|---------|---------|
| `make run` | `go run ./cmd/banana-mw` (reads `data/config.yaml`) |
| `make build` | Linux amd64 → `build/banana-mw` |
| `make build-local` | Current OS → `build/banana-mw.local` |
| `make test` | `go test ./...` |
| `make fmt` | `go fmt ./...` |
| `make deps` | `go mod download && go mod tidy` |
| `make clean` | Remove `build/` |

Prefer `go test ./...` and `go build ./cmd/banana-mw` for local checks. Run `docker compose build` only when the user asks for Docker.

Docker: mount `./data` → `/app/data` (read-only). Config path inside the container: `/app/data/config.yaml`.

## Coding Conventions

- Standard Go style, `gofmt`, tabs.
- Tests live beside code as `*_test.go`; use `config.Test()` in `internal/config/testcfg.go` for router/handler tests.
- New code should match naming in the target package (e.g. `middleware.go`, `memory.go`), not legacy root-level `*_mw.go` names.
- Keep changes scoped; avoid unrelated refactors.

## Testing Focus

Existing coverage includes: config load/defaults, `LoadDefault` path priority, log level, secure Redis key prefix, router guard order, cache key behavior, admin auth, stats queue overflow/summarize, stats cleaner key typing, debug request trace.

Add focused tests when touching config parsing, middleware order, cache keys, stats worker/cleaner, or admin auth.

## Configuration (`data/config.yaml`)

Load order: `data/config.yaml` → `config.yaml` → `data/config.json` → `config.json`.

Notable fields:

- `upstream` — required; `https://example.com` is rejected at startup.
- `port` — reverse proxy HTTP listen port (default `8080`).
- `admin_port` — admin dashboard HTTP listen port (default `8081`); only used when `is_admin=1`, must differ from `port`.
- `log_level` — see Logging above.
- `allowed_prefixes`, `cache_ttl_*`, `cache_include_query`
- `secure`, `secure_time`, `secure_redis_key_prefix` (default `secure_session::user::`)
- `token_redis_*`, Redis timeouts
- `stats_enabled`, `stats_prefix`, worker/queue/cleaner settings
- `is_admin` (enables the admin server on `admin_port`), `admin_token` — empty `admin_token` disables admin APIs (503)
- HTTP server timeouts: `read_header_timeout`, `read_timeout`, `write_timeout`, `idle_timeout`, `shutdown_timeout`

Copy from `config.example.yaml`; see `data/README.md`.

## Security Notes

- Do not commit `data/config.yaml`, Redis passwords, or `admin_token`.
- Admin JSON APIs require `Authorization: Bearer <admin_token>`.
- Admin HTML/API set security headers (CSP, `X-Frame-Options: DENY`, etc.).
- Admin routes are served only on `admin_port` (when `is_admin=1`), never on the public `port`. Restrict `admin_port` to trusted networks (firewall / internal interface) in production.

## Commit & PR Guidelines

- Short imperative commit subjects; body for behavior changes.
- PRs: summary, `go test ./...` evidence, note any config/docker changes.
