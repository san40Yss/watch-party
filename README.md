# Watch Party

Self-hosted media server for watching your **own** video library together with
friends, in the browser — **synchronized** playback, with each viewer free to
pick their **own audio track and subtitles**. No re-uploading to a chat app, no
screen-share compression: the server prepares each title once and every viewer
streams it directly.

Think of it as Jellyfin-style media handling plus a synchronized "watch party"
room layer.

## Features

- **Browser playback of MKV/MP4 libraries** via HLS. The server prepares a title
  once; every viewer streams H.264 that plays in any modern browser.
- **Probe-and-branch pipeline** — already-browser-ready H.264 video is *copied*
  (no re-encode, no quality loss); HEVC / VC-1 / etc. are transcoded to H.264
  only when necessary.
- **Per-viewer tracks** — every viewer independently selects their own audio
  track and subtitles; only the *playback position* is synchronized.
- **Synchronized rooms** — a host controls play / pause / seek over WebSocket;
  guests follow with drift correction. Join by shareable link or room code.
- **Accounts & roles** — cookie sessions, registration / login, password change,
  and admin-only library management (processing, deletion, uploads).
- **Resumable uploads** (tus) with live processing progress and ETA.

## How it works

Browsers only reliably decode **H.264 video + stereo AAC audio** over HLS/MSE.
So the pipeline is:

```
ffprobe  →  decide  →  ffmpeg  →  HLS package  →  nginx (X-Accel-Redirect)  →  hls.js
           per stream            video: copy H.264 / transcode to H.264
                                 audio: each track → stereo AAC rendition
                                 subs:  text tracks → WebVTT renditions
```

- **Go authorizes, nginx serves the bytes.** The Go backend handles auth and
  returns an `X-Accel-Redirect` header; nginx streams the actual media via
  `sendfile` (zero-copy) from an internal location.
- **Rooms** store playback state as an *anchor* (`position` + `updated_at`) and
  extrapolate the live position; guests correct only on real drift, so there's
  no feedback loop between the host's commands and their echoes.

## Tech stack

- **Backend:** Go — [chi](https://github.com/go-chi/chi) router,
  [pgx](https://github.com/jackc/pgx) (Postgres),
  [coder/websocket](https://github.com/coder/websocket),
  [tusd](https://github.com/tus/tusd) resumable uploads, `ffmpeg` / `ffprobe`.
- **Frontend:** Svelte 5 (runes) + Vite, [Vidstack](https://vidstack.io/) player
  with bundled [hls.js](https://github.com/video-dev/hls.js).
- **Infra:** Postgres, Nginx (SPA + media serving), Docker Compose.

## Quick start

Requires Docker.

```bash
cp .env.example .env          # set MEDIA_DIR / PROCESSED_DIR to your host paths
docker compose up --build
```

Then open <http://localhost>, register an account, and process a title from your
library to start watching. To invite friends over your network, open the app via
your machine's LAN/VPN address (so the room links resolve for them too).

## Configuration

| Variable         | Purpose                                                        |
| ---------------- | ------------------------------------------------------------- |
| `MEDIA_DIR`      | Host path to your source video library (mounted read-write).  |
| `PROCESSED_DIR`  | Host path where browser-ready HLS output is written.          |
| `DATABASE_URL`   | Postgres connection (only for running the app outside Docker).  |
| `DEV_AUTO_LOGIN` | `false` to require login; unset/`true` only for local dev.     |

The default database credentials (`wp:wp`) and the seeded `host` / `changeme`
account are **development defaults** — change them before exposing the app.

## Status

Personal project, work in progress. Designed for home/LAN use behind a real
account system; not hardened for untrusted public exposure.

## License

MIT — see [LICENSE](LICENSE).
