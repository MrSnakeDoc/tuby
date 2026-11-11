<h1>
  <img src="./static/img/logo.svg" alt="Tuby logo" width="36" height="36">
  Tuby
</h1>

**Procrastination, now self-hosted.**

Tuby is a lightweight, self-hosted YouTube subscription tracker. Follow channels without a Google account, find uploads you haven't opened, and save videos for later.

When you want to watch something, Tuby sends you to YouTube rather than playing it inside the app.

Built with Go, server-rendered HTML, HTMX, and SQLite. No user accounts or external database required.

## Features

- **Your channels:** Add YouTube channels by URL and browse their recent uploads.
- **Per-channel content:** Follow long videos, Shorts, or both; long videos remain the preferred source for channel artwork.
- **Missed:** See videos you haven't marked as watched, with a count in the navigation.
- **Watch later:** Bookmark videos to revisit, with its own navigation count.
- **Mark as watched:** Mark individual videos or all videos from a channel as watched.
- **Arrange channels:** Enter edit mode, drag channels into your preferred order, and save it.
- **Automatic synchronization:** Check for new uploads every 15 minutes, with per-channel scheduling and retry backoff.
- **Live synchronization:** Propagate watched, bookmarked, channel, counter, and scheduler updates to every open browser through SSE.
- **Manual refresh:** Refresh all subscriptions or one channel on demand.
- **Changing channel artwork:** Channel cards rotate through video thumbnails every 15 minutes while the page is open.
- **Persistent storage:** Channels, videos, watched status, bookmarks, and ordering are stored in SQLite.
- **Responsive dark UI:** A compact interface for desktop and mobile.

## Quick start with Docker Compose

```yaml
services:
  tuby:
    image: YOUR_TUBY_IMAGE:YOUR_TAG
    ports:
      - "8080:8080"
    volumes:
      - tuby-data:/app/data
    restart: unless-stopped

volumes:
  tuby-data:
```

Replace `YOUR_TUBY_IMAGE:YOUR_TAG` with the image and tag published for your release, then run:

```sh
docker compose up -d
```

Open http://localhost:8080.

**Data permissions:** The provided Dockerfile uses `gcr.io/distroless/static-debian12:nonroot`, whose default user is UID 65532. The mounted volume must be writable by the container's effective user.

If you override the runtime user (for example, to `1000:1000`), make sure the volume permissions match.

## Configuration

| Environment variable | Default | Description                                                         |
| -------------------- | ------- | ------------------------------------------------------------------- |
| `TUBY_PORT`          | `8080`  | HTTP listening port inside the container.                           |
| `TUBY_ALLOWED_CIDRS` | `*`     | Comma-separated client IP addresses or CIDR ranges; `*` allows all. |
| `TUBY_LOG_LEVEL`     | `info`  | `debug`, `info`, `warn`, or `error`.                                |
| `TUBY_PRETTY_LOG`    | `true`  | Enable human-readable logs; set to `false` for non-pretty output.   |

The SQLite database is stored at `data/tuby.sqlite` relative to the process working directory (`/app` in the Docker image).

### Network security

Tuby does not provide user authentication.

Keep it on a trusted network or protect it with an authenticated reverse proxy.

IP filtering is available through `TUBY_ALLOWED_CIDRS`. The server checks the direct peer address rather than trusting forwarded proxy headers.

### Reverse proxy and live updates

Live browser synchronization uses a long-lived Server-Sent Events connection on `GET /api/events`.

If Tuby runs behind a reverse proxy, disable response buffering for this route and allow long-lived connections. The endpoint sends a heartbeat every 25 seconds and includes `X-Accel-Buffering: no`, but proxy-level timeouts and buffering settings still take precedence.

## How it works

Tuby resolves a channel reference, retrieves its public YouTube RSS feed, and stores the available video metadata locally.

### Automatic synchronization

Tuby includes a background scheduler that keeps subscriptions up to date.

- **Startup:** Checks for channels that are due for synchronization.
- **Every 15 minutes:** Checks again for channels requiring an update.
- **Per-channel scheduling:** Uses the last successful synchronization and the next scheduled check to determine when a channel is due.
- **Failure handling:** Applies exponential backoff after failed requests, starting at 15 minutes and capped at 6 hours.
- **Request staggering:** Waits 2 seconds between scheduled channel refresh attempts.
- **Graceful shutdown:** Stops the scheduler when the application shuts down.

The scheduler only refreshes channels that are due. It does not blindly fetch every subscription on every tick.

Manual refresh remains available from the interface, both globally and per channel.

### Video tracking

Tuby stores video metadata and tracks watched and bookmarked states locally.

Opening a video takes you directly to YouTube. Videos can also be marked as watched without opening them.

Watched and Watch later states are independent of your YouTube account.

### Channel artwork

Channel cards use thumbnails from their videos rather than static channel avatars.

The browser rotates these images every 15 minutes, independently of the background RSS synchronization.

## Limitations

- YouTube RSS feeds expose only a limited selection of recent uploads. Tuby cannot recover a channel's complete video history from RSS alone.
- Watched and Watch later states are local to Tuby and are not synchronized with your YouTube account.
- There is no built-in user authentication or multi-user account system.
- Videos older than the application's retention window (90 days) may be removed during feed synchronization.
- Feed availability and metadata depend on YouTube's public endpoints.

## Development

Requires Go 1.27.4 as declared in `go.mod`.

```sh
go run ./cmd/tuby
go test ./...
```

For local execution, the application creates its SQLite database under `./data/`.

---

_TUBY. SERIOUSLY UNSERIOUS SUBSCRIPTIONS._
