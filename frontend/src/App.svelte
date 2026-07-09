<script>
  import { onMount } from 'svelte'
  import * as api from './lib/api.js'
  import VideoList from './lib/VideoList.svelte'
  import Player from './lib/Player.svelte'
  import Login from './lib/Login.svelte'
  import Upload from './lib/Upload.svelte'
  import ChangePassword from './lib/ChangePassword.svelte'
  import Room from './lib/Room.svelte'
  import LangToggle from './lib/LangToggle.svelte'
  import Captions from './lib/Captions.svelte'
  import { room, setUser, attachController, localAction, join as joinRoom, leave as leaveRoom, resumeSync, switchVideo, storedRoomId } from './lib/room.svelte.js'
  import { t } from './lib/i18n.svelte.js'

  let videos = $state([])
  let currentId = $state(null)
  let user = $state(null)
  let authChecked = $state(false) // avoid flashing the login screen before /me resolves
  let showPassword = $state(false) // change-password modal
  let quality = $state(1440) // target height for HEVC→H.264 transcode
  let pollTimer = null

  // Imperative handle to the player (play/pause/seek/getState). Rooms drive it
  // from incoming events; the player feeds local user actions back via
  // onLocalAction. attachController re-syncs a guest the moment the player mounts.
  let controller = $state(null)

  // A ?room=CODE in the URL (shared link): auto-join once we know who the user is.
  let pendingRoom = new URLSearchParams(location.search).get('room')

  // When the room picks a video (host bound it, or a guest joined), select it so
  // the player loads it. Only while actually in a room, so it never fights the
  // user's manual selection.
  $effect(() => {
    if (room.id && room.videoId != null && room.videoId !== currentId) {
      currentId = room.videoId
    }
  })

  // ETA estimation: average encode rate since we first saw a video processing.
  let firstSample = {} // id -> { p, t }
  let etas = $state({}) // id -> remaining minutes

  function updateEtas(vids) {
    const now = Date.now()
    const next = {}
    for (const v of vids) {
      if (v.status !== 'processing') {
        delete firstSample[v.id]
        continue
      }
      if (!firstSample[v.id]) firstSample[v.id] = { p: v.progress, t: now }
      const s = firstSample[v.id]
      const dp = v.progress - s.p
      const dt = (now - s.t) / 1000
      if (dp > 0.1 && dt > 3) {
        next[v.id] = Math.max(0, Math.round((100 - v.progress) / (dp / dt) / 60))
      }
    }
    etas = next
  }

  const current = $derived(videos.find((v) => v.id === currentId) ?? null)
  // HLS master playlist or MP4 stream depending on the video; empty until ready.
  const streamSrc = $derived(api.playbackUrl(current))

  const placeholder = $derived(
    !current
      ? t('ph_select')
      : current.status === 'processing'
        ? t('ph_processing', { pct: Math.round(current.progress || 0) }) +
          (etas[current.id] != null ? t('ph_eta', { min: etas[current.id] }) : t('ph_dots'))
        : current.status === 'error'
          ? t('ph_error', { msg: current.error || t('ph_error_generic') })
          : t('ph_press_process')
  )

  // Transient toast for failed actions (auto-clears).
  let uiError = $state('')
  let uiErrorTimer = null
  function showError(msg) {
    uiError = msg
    clearTimeout(uiErrorTimer)
    uiErrorTimer = setTimeout(() => (uiError = ''), 4000)
  }

  async function refresh() {
    try {
      videos = await api.listVideos()
    } catch {
      return // transient network/auth hiccup — the next poll or action retries
    }
    updateEtas(videos)
    ensurePolling()
  }

  // Register files dropped/symlinked into the media folder (covers VR files
  // under media/vr too, which land in the VR library).
  let scanning = $state(false)
  let scanMsg = $state('')
  async function scanLibrary() {
    scanning = true
    scanMsg = ''
    try {
      const { added } = await api.scanLibrary()
      scanMsg = t('scan_done', { n: added })
      await refresh()
    } catch {
      scanMsg = t('scan_error')
    } finally {
      scanning = false
      setTimeout(() => (scanMsg = ''), 4000)
    }
  }

  function ensurePolling() {
    const anyProcessing = videos.some((v) => v.status === 'processing')
    if (anyProcessing && !pollTimer) {
      pollTimer = setInterval(refresh, 2000)
    } else if (!anyProcessing && pollTimer) {
      clearInterval(pollTimer)
      pollTimer = null
    }
  }

  async function startProcessing() {
    if (!current) return
    try {
      await api.processVideo(current.id, quality)
    } catch {
      showError(t('err_process'))
      return
    }
    await refresh()
  }

  async function deleteCurrent() {
    if (!current) return
    if (!confirm(t('confirm_delete', { title: current.title }))) return
    try {
      await api.deleteVideo(current.id)
    } catch {
      showError(t('err_delete'))
      return
    }
    currentId = null
    await refresh()
  }

  // Selecting a film: local pick — and if we're hosting a party, the whole
  // room follows (guests' selections stay local-only; the follow-effect above
  // snaps them back to the room's video).
  function selectVideo(id) {
    if (room.id && room.isHost) switchVideo(id)
    currentId = id
  }

  async function logout() {
    await api.logout()
    // Stop everything tied to the session: the room socket would otherwise
    // stay connected, and the poll would hammer 401s.
    leaveRoom()
    if (pollTimer) {
      clearInterval(pollTimer)
      pollTimer = null
    }
    user = null
  }

  const processLabel = $derived(
    !current
      ? ''
      : current.status === 'processing'
        ? t('processing_pct', { pct: Math.round(current.progress || 0) })
        : current.status === 'ready'
          ? t('reprocess')
          : t('process')
  )

  // Shared by initial load and login: identify the user to the room store, load
  // the library, then auto-join a shared-link room if one is pending.
  async function afterAuth() {
    setUser(user.id)
    await refresh()
    // Auto-rejoin: a shared ?room= link takes priority, else the room we were in
    // before a refresh (persisted in localStorage).
    const code = pendingRoom || storedRoomId()
    if (code) {
      pendingRoom = null
      history.replaceState({}, '', location.pathname) // drop ?room= from the URL
      await joinRoom(code).catch(() => {})
    }
  }

  onMount(async () => {
    user = await api.me().catch(() => null)
    authChecked = true
    if (user) await afterAuth()
  })

  async function onLogin(u) {
    user = u
    await afterAuth()
  }
</script>

{#if !authChecked}
  <!-- brief blank while we check the session -->
{:else if !user}
  <Login {onLogin} />
{:else}
<header>
  <div class="brand">
    <svg class="logo" viewBox="0 0 64 64" aria-hidden="true">
      <circle cx="32" cy="32" r="18" fill="none" stroke="var(--accent)" stroke-width="4" />
      <path d="M27.5 23.5 L43 32 L27.5 40.5 Z" fill="var(--accent)" />
    </svg>
    <h1>Watch&nbsp;Party</h1>
  </div>
  <div class="header-right">
    <LangToggle />
    <span class="user">{user.username}{#if user.is_admin}<span class="admin-tag">admin</span>{/if}</span>
    <button class="link" onclick={() => (showPassword = true)}>{t('hdr_password')}</button>
    <button class="link" onclick={logout}>{t('hdr_logout')}</button>
  </div>
</header>

{#if showPassword}
  <ChangePassword onClose={() => (showPassword = false)} />
{/if}

<div class="layout">
  <aside class="sidebar">
    <Room currentVideoId={currentId} />
    {#if user.is_admin}
      <Upload onDone={refresh} />
      <div class="scan">
        <button class="btn-ghost scan-btn" onclick={scanLibrary} disabled={scanning}>
          {scanning ? t('scanning') : t('scan_library')}
        </button>
        {#if scanMsg}<div class="scan-msg">{scanMsg}</div>{/if}
      </div>
    {/if}
    <VideoList {videos} {currentId} {etas} onSelect={selectVideo} />
  </aside>

  <main class="player-area">
    {#if current && user.is_admin}
      <div class="controls-bar">
        <select class="quality" bind:value={quality} disabled={current.status === 'processing'} aria-label={t('quality_aria')}>
          <option value={1080}>1080p</option>
          <option value={1440}>1440p · 2K</option>
          <option value={2160}>2160p · 4K</option>
        </select>
        <button class="btn" onclick={startProcessing} disabled={current.status === 'processing'}>
          {processLabel}
        </button>
        <button class="btn-icon danger" onclick={deleteCurrent} disabled={current.status === 'processing'} aria-label={t('delete_video')} title={t('delete_video')}>
          <svg viewBox="0 0 20 20" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" aria-hidden="true">
            <path d="M3 5h14M8 5V3.5h4V5m-6 0 .6 11h6.8L16 5" />
          </svg>
        </button>
      </div>
    {/if}
    <Player
      src={streamSrc}
      title={current?.title ?? ''}
      {placeholder}
      onLocalAction={localAction}
      onReady={(c) => { controller = c; attachController(c) }}
    />
    {#if current?.status === 'ready' && current.playback_type === 'hls'}
      <!-- Per-viewer subtitle customization (only HLS videos carry subtitles). -->
      <Captions />
    {/if}
    {#if room.needsGesture}
      <!-- Autoplay was blocked for a guest; playback needs one user gesture. -->
      <button class="sync-overlay" onclick={resumeSync}>
        <svg viewBox="0 0 24 24" width="22" height="22" fill="currentColor" aria-hidden="true">
          <path d="M8 5v14l11-7z" />
        </svg>
        {t('tap_to_sync')}
      </button>
    {/if}
  </main>
</div>

{#if uiError}
  <div class="toast" role="alert">{uiError}</div>
{/if}
{/if}

<style>
  header {
    position: relative;
    z-index: 1;
    padding: var(--sp-3) var(--sp-5);
    border-bottom: 1px solid var(--border);
    flex-shrink: 0;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--sp-3);
  }
  .scan { display: flex; flex-direction: column; gap: var(--sp-2); }
  .scan-btn { width: 100%; }
  .scan-msg { font-size: var(--text-xs); color: var(--text-muted); text-align: center; }

  .brand { display: flex; align-items: center; gap: var(--sp-2); }
  .logo { width: 26px; height: 26px; filter: drop-shadow(var(--glow-accent)); }
  .brand h1 {
    font-size: var(--text-base);
    font-weight: 700;
    letter-spacing: 0.01em;
    margin: 0;
  }
  .header-right { display: flex; align-items: center; gap: var(--sp-4); }
  .user {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-2);
    font-size: var(--text-sm);
    color: var(--text-muted);
  }
  .admin-tag {
    font-size: 0.62rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--accent);
    background: var(--accent-soft);
    padding: 1px 6px;
    border-radius: var(--r-full);
    font-weight: 600;
  }

  .layout {
    display: grid;
    grid-template-columns: 320px 1fr;
    flex: 1;
    min-height: 0;
    overflow: hidden;
    position: relative;
    z-index: 1;
  }
  .sidebar {
    border-right: 1px solid var(--border);
    overflow-y: auto;
    padding: var(--sp-4);
    display: flex;
    flex-direction: column;
    gap: var(--sp-5);
  }
  .player-area {
    background: #000;
    position: relative;
    overflow: hidden;
  }

  .controls-bar {
    position: absolute;
    top: var(--sp-3);
    right: var(--sp-3);
    z-index: 10;
    display: flex;
    gap: var(--sp-2);
    padding: var(--sp-2);
    background: rgba(10, 10, 12, 0.6);
    backdrop-filter: blur(8px);
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
  }
  .quality {
    appearance: none;
    background: var(--surface-2);
    color: var(--text);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    padding: 0 var(--sp-3);
    height: 38px;
    font: inherit;
    font-size: var(--text-sm);
    cursor: pointer;
  }
  .quality:focus { outline: none; border-color: var(--accent); }
  .danger { color: var(--error); }
  .danger:hover:not(:disabled) { background: var(--error-soft); color: var(--error); }

  .sync-overlay {
    position: absolute;
    inset: 0;
    z-index: 8;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--sp-3);
    width: 100%;
    background: rgba(10, 10, 12, 0.72);
    border: none;
    color: var(--accent);
    font: inherit;
    font-size: var(--text-base);
    font-weight: 600;
    cursor: pointer;
  }

  .toast {
    position: fixed;
    bottom: var(--sp-5);
    left: 50%;
    transform: translateX(-50%);
    z-index: var(--z-modal);
    padding: var(--sp-3) var(--sp-4);
    background: var(--surface-2);
    border: 1px solid var(--error);
    border-radius: var(--r-md);
    color: var(--error);
    font-size: var(--text-sm);
    box-shadow: var(--shadow-3);
    animation: fade var(--dur) var(--ease);
  }
  @keyframes fade { from { opacity: 0; } }

  /* Mobile: stack the player on top (sticky) with the panel scrolling below. */
  @media (max-width: 720px) {
    header { padding: var(--sp-3) var(--sp-4); }
    .header-right { gap: var(--sp-3); }
    .layout {
      display: flex;
      flex-direction: column;
      overflow-y: auto;
    }
    .player-area {
      order: -1; /* show the player above the panel on phones */
      aspect-ratio: 16 / 9;
      flex-shrink: 0;
      position: sticky;
      top: 0;
      z-index: 5;
    }
    .sidebar {
      border-right: none;
      overflow-y: visible;
    }
  }
</style>
