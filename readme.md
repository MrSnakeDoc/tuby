<h1>
  <img src="./internal/web/static/img/favicon.svg" alt="" width="36" height="36">
  Tuby
</h1>

**Procrastination, now self-hosted.**

Tuby is a lightweight, single-user YouTube subscription tracker. It follows public channel feeds without a Google account, keeps an independent Missed and Watch later library, and opens videos on YouTube when you decide to watch them.

The application is a single Go binary with embedded server-rendered HTML, HTMX, JavaScript and CSS. SQLite is the only database. There is no frontend build step and no external service is required unless optional [Navire notifications](#navire-notifications) are enabled.

## Features

- Add a channel from its YouTube URL or `UC…` identifier.
- Follow long-form videos, Shorts, or both independently for each channel.
- Refresh due feeds automatically every 15 minutes, or refresh globally/per channel on demand.
- Track Missed, Watch later and watched states locally; opening an unseen video marks it watched immediately.
- Organize channels into collapsible, ordered categories with drag-and-drop and keyboard controls.
- Search channels instantly in the browser, without a server request.
- Rotate channel artwork through recent long-form thumbnails, falling back to Shorts for Shorts-only channels.
- Fetch a video's duration on demand without slowing down normal RSS synchronization.
- Synchronize views, counters, preferences and content changes between open browsers with Server-Sent Events (SSE).
- Reload stale browser clients automatically after a new Tuby build is deployed.
- Optionally notify Navire when eligible videos are discovered, with a priority configured per channel.
- Run as a non-root, multi-architecture container with persistent SQLite storage.

## Quick start

### Docker Compose

The published image listens on port `8090` and runs as UID/GID `65532`.

```yaml
services:
  tuby:
    image: git.nexacloud.dev/mrsnakedoc/tuby:latest
    ports:
      - "8090:8090"
    environment:
      TUBY_PORT: "8090"
      TUBY_ALLOWED_CIDRS: "*"
    volumes:
      - ./data:/app/data
    restart: unless-stopped
```

Prepare the bind mount for the image's non-root user, start the service, then open <http://localhost:8090>:

```sh
mkdir -p data
sudo chown 65532:65532 data
docker compose up -d
```

If Compose overrides the container user, make `./data` writable by that UID/GID instead. Never bake `tuby.sqlite` into the image.

### Local Go process

Tuby requires Go 1.27.4, matching [`go.mod`](./go.mod).

```sh
go run ./cmd/tuby
```

Open <http://localhost:8090>. The database is created at `data/tuby.sqlite` relative to the process working directory.

## Configuration

All configuration is read from environment variables at startup.

| Variable | Default | Description |
| --- | --- | --- |
| `TUBY_PORT` | `8090` | HTTP listening port. |
| `TUBY_ALLOWED_CIDRS` | `*` | Comma-separated direct client IPs/CIDRs. `*` allows every peer. |
| `TUBY_LOG_LEVEL` | `info` | `debug`, `info`, `warn`, or `error`. |
| `TUBY_PRETTY_LOG` | `true` | Human-readable logs. Set to `false` for structured JSON logs. |
| `TUBY_NAVIRE_ENABLED` | `false` | Expose notification controls and start the Navire delivery worker. |
| `TUBY_NAVIRE_URL` | empty | Navire base URL. Required only when Navire is enabled. |
| `TUBY_NAVIRE_REQUEST_TIMEOUT` | `5s` | Timeout for one Navire HTTP request. |
| `TUBY_NAVIRE_MAX_ATTEMPTS` | `4` | Total delivery attempts before an event is permanently dropped. |
| `TUBY_NAVIRE_RETRY_INITIAL_WAIT` | `2s` | Initial retry delay. |
| `TUBY_NAVIRE_RETRY_MAX_WAIT` | `30s` | Maximum exponential-backoff delay. |
| `TUBY_NOTIFICATION_MESSAGES` | empty | Optional JSON array of humorous messages. No message field is sent when absent, empty, or `[]`. |

Durations use Go syntax such as `500ms`, `5s`, or `1m`. Retry counts and durations must be positive, and the initial retry wait cannot exceed the maximum wait.

`TUBY_ALLOWED_CIDRS` checks the direct TCP peer. Tuby deliberately does not trust forwarded headers. If it is behind a reverse proxy, allow the proxy's address and enforce end-user authentication/network policy at the proxy.

## Navire notifications

Navire integration is optional and disabled by default. When enabled, every channel gains one of four notification levels:

| Tuby level | Navire priority | Behavior |
| --- | --- | --- |
| `off` | — | No events; this is the default. |
| `low` | `low` | Low-priority notification. |
| `medium` | `normal` | Normal-priority notification. |
| `critical` | `critical` | Critical-priority notification. |

Enabling notifications is not retroactive. Only videos published after a channel's first notification activation and newly inserted by a later RSS synchronization are eligible. Long/Shorts preferences still apply, and the initial channel import never emits notifications.

### 1. Install the Navire template

Copy [`deploy/navire/templates/tuby_video.yml`](./deploy/navire/templates/tuby_video.yml) into the directory mounted as `NAVIRE_TEMPLATES_DIR` by Navire. Its `gotify.endpoints.prod` entry is a placeholder: rename it or adjust it to an enabled endpoint in your Navire configuration.

The template contains the two variants Tuby can publish:

- `new_video` when no optional message is configured;
- `new_video_with_message` when a message was selected.

### 2. Enable the integration

```yaml
services:
  tuby:
    environment:
      TUBY_NAVIRE_ENABLED: "true"
      TUBY_NAVIRE_URL: "https://navire.example.com"
      TUBY_NAVIRE_REQUEST_TIMEOUT: "5s"
      TUBY_NAVIRE_MAX_ATTEMPTS: "4"
      TUBY_NAVIRE_RETRY_INITIAL_WAIT: "2s"
      TUBY_NAVIRE_RETRY_MAX_WAIT: "30s"
      TUBY_NOTIFICATION_MESSAGES: >-
        ["First optional message.", "Another optional message."]
```

Tuby posts template key `tuby_video` to `POST /api/v1/events`. Discovery and delivery are separated by a transactional SQLite outbox: saving a new video and queuing its event happen in one transaction, while a background worker delivers it independently. Network errors and every non-2xx response are retried with bounded exponential backoff. After the configured attempt limit, the event is logged and permanently removed.

Disabling Navire hides the selectors and stops the worker without deleting pending outbox entries. They are resumed if the integration is enabled again.

## How synchronization works

Tuby resolves a public channel reference, reads its YouTube RSS feed, and stores the available metadata locally. ETag and Last-Modified validators avoid unnecessary transfers when YouTube provides them.

The scheduler runs once at startup and every 15 minutes. It refreshes only channels whose next check is due, waits two seconds between channels, prevents concurrent refreshes of the same channel, and records exponential backoff after feed failures. Manual global and per-channel refreshes use the same synchronization service.

Feed synchronization retains watched/bookmarked state across updates. Videos absent from the current feed may be removed only after 90 days when they are neither unseen nor bookmarked. Disabling Long or Shorts removes videos of that type from the local library.

YouTube RSS exposes only a recent window, not a channel's complete history. Tuby cannot recover videos no longer present in that feed.

## Browser behavior

Tuby uses HTMX for navigation and mutations. A long-lived `GET /api/events` SSE connection updates every open client after changes. The stream sends a heartbeat every 25 seconds; reverse proxies must allow long-lived responses and disable buffering for that route. Tuby also sends `X-Accel-Buffering: no`.

Each HTML document and SSE payload carries a build identifier. If an old browser tab sees a different server build, it performs a full reload before applying an incompatible partial update.

Channel search, category collapse state, drag-and-drop interactions and thumbnail rotation run locally in the browser. Category open/closed state is saved in `localStorage`.

### Keyboard shortcuts

| Shortcut | Action |
| --- | --- |
| `g` then `h` | Home |
| `g` then `m` | Missed |
| `g` then `w` | Watch later |
| `a` then `c` | Add channel |
| `a` then `g` | Add category on Home |
| `c` then `a` | Collapse every category |
| `e` then `a` | Expand every category |
| `/` | Focus channel search |
| `?` | Show the in-app shortcut reference |

Two-key sequences expire after one second and are ignored while typing or while a dialog is open.

## Data and operations

- SQLite database: `data/tuby.sqlite`
- SQLite mode: WAL, foreign keys enabled, five-second busy timeout
- Health endpoint: `GET /healthz`
- Live-update endpoint: `GET /api/events`
- Graceful shutdown: `SIGINT` or `SIGTERM` stops HTTP, SSE, the scheduler and the notification worker

Back up the database and its WAL files consistently, or stop Tuby before copying `data/`. Schema migrations run automatically and incrementally at startup; the application never recreates an existing database.

Tuby has no authentication and is designed as one application instance with one shared library. Keep it on a trusted network or behind an authenticated reverse proxy.

## Development and validation

Common commands:

```sh
go run ./cmd/tuby
go build ./...
go test ./...
go test -race ./...
go vet ./...
golangci-lint run --config .golangci.yml
govulncheck ./...
```

The release pipeline in [`.woodpecker.yml`](./.woodpecker.yml) runs build, vulnerability, lint and test checks for `v*` tags, then publishes `linux/amd64` and `linux/arm64` images. Build metadata is injected through `VERSION`, `GIT_COMMIT`, and `BUILD_DATE` Docker build arguments and is exposed by `/healthz`.

Tests create isolated temporary SQLite databases and do not modify `data/tuby.sqlite`.

## Limitations

- Tuby depends on YouTube's undocumented public channel pages and RSS feeds; their availability and markup can change without notice.
- Watched and Watch later state is local to Tuby and is not synchronized with a YouTube account.
- Video duration retrieval parses the public watch page on explicit request and can fail for unavailable videos, live streams, consent pages, or upstream markup changes.
- There is no built-in authentication or multi-user separation.
- Navire delivery is at-least-once across transient failures until its bounded retry limit; Navire owns downstream routing and any provider-specific deduplication.

---

_TUBY. SERIOUSLY UNSERIOUS SUBSCRIPTIONS._
