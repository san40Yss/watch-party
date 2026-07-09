// Thin API client. All calls go to the Go backend via nginx (same origin in
// prod; proxied in dev). Cookies (the session) ride along automatically.

async function json(res) {
  if (!res.ok) {
    // Surface the backend's {error} message when it sends one (login/register).
    let msg = `${res.status} ${res.statusText}`
    try {
      const body = await res.json()
      if (body?.error) msg = body.error
    } catch { /* non-JSON body */ }
    throw new Error(msg)
  }
  return res.json()
}

export const listVideos = () => fetch('/api/videos').then(json)

export const getVideo = (id) => fetch(`/api/videos/${id}`).then(json)

export const processVideo = (id, height) =>
  fetch(`/api/videos/${id}/process?height=${height}`, { method: 'POST' }).then(json)

export const deleteVideo = async (id) => {
  const res = await fetch(`/api/videos/${id}`, { method: 'DELETE' })
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
}

// scanLibrary registers video files dropped/symlinked into the media folder
// (shared library + VR under media/vr). Returns { added }.
export const scanLibrary = () =>
  fetch('/api/library/scan', { method: 'POST' }).then(json)

export const me = () => fetch('/api/me').then(json)

export const login = (username, password) =>
  fetch('/api/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  }).then(json)

export const register = (username, password) =>
  fetch('/api/register', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  }).then(json)

export const logout = () => fetch('/api/logout', { method: 'POST' })

// changePassword returns nothing on success (204); throws the backend's {error}
// message on failure. Doesn't use json() because there's no success body.
export const changePassword = async (current, newPassword) => {
  const res = await fetch('/api/password', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ current, new: newPassword }),
  })
  if (!res.ok) {
    let msg = `${res.status} ${res.statusText}`
    try {
      const body = await res.json()
      if (body?.error) msg = body.error
    } catch { /* non-JSON body */ }
    throw new Error(msg)
  }
}

// --- Rooms ---

export const createRoom = (videoId) =>
  fetch('/api/rooms', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ video_id: videoId }),
  }).then(json)

export const getRoom = (id) => fetch(`/api/rooms/${id}`).then(json)

// playbackUrl returns the right source URL for the player. HLS videos point at
// the master playlist (Vidstack drives it via hls.js / native HLS); everything
// else is the single-file MP4 stream. The URL keeps a recognizable extension so
// Vidstack picks the right provider.
//
// A still-processing video becomes playable as soon as the backend publishes
// its master (processed_path set while status is 'processing') — the EVENT
// playlist keeps growing under the player while ffmpeg works.
export function playbackUrl(v) {
  if (!v) return ''
  const watchable = v.status === 'ready' || (v.status === 'processing' && v.processed_path)
  if (!watchable) return ''
  return v.playback_type === 'hls'
    ? `/api/videos/${v.id}/hls/master.m3u8`
    : `/api/videos/${v.id}/stream`
}
