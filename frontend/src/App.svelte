<script>
  import { onMount } from 'svelte'
  import * as api from './lib/api.js'
  import VideoList from './lib/VideoList.svelte'
  import Player from './lib/Player.svelte'
  import Login from './lib/Login.svelte'
  import Upload from './lib/Upload.svelte'
  import ChangePassword from './lib/ChangePassword.svelte'
  import Room from './lib/Room.svelte'
  import { room, setUser, attachController, localAction, join as joinRoom, storedRoomId } from './lib/room.svelte.js'

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
      ? 'Выберите видео'
      : current.status === 'processing'
        ? `Обработка ${Math.round(current.progress || 0)}%` +
          (etas[current.id] != null ? ` · осталось ~${etas[current.id]} мин` : '…')
        : current.status === 'error'
          ? `Ошибка: ${current.error || 'обработки'}`
          : 'Нажмите «Обработать»'
  )

  async function refresh() {
    videos = await api.listVideos()
    updateEtas(videos)
    ensurePolling()
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
    await api.processVideo(current.id, quality)
    await refresh()
  }

  async function deleteCurrent() {
    if (!current) return
    if (!confirm(`Удалить «${current.title}» из библиотеки? Исходный файл останется.`)) return
    await api.deleteVideo(current.id)
    currentId = null
    await refresh()
  }

  async function logout() {
    await api.logout()
    user = null
  }

  const processLabel = $derived(
    !current
      ? ''
      : current.status === 'processing'
        ? `Обработка ${Math.round(current.progress || 0)}%`
        : current.status === 'ready'
          ? 'Переобработать'
          : 'Обработать'
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
  <h1>Watch Party</h1>
  <div class="header-right">
    <span class="user">{user.username}</span>
    <button class="link" onclick={() => (showPassword = true)}>пароль</button>
    <button class="link" onclick={logout}>выйти</button>
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
    {/if}
    <VideoList {videos} {currentId} {etas} onSelect={(id) => (currentId = id)} />
  </aside>

  <main class="player-area">
    {#if current && user.is_admin}
      <div class="controls-bar">
        <select bind:value={quality} disabled={current.status === 'processing'} title="Качество H.264 (для HEVC)">
          <option value={1080}>1080p</option>
          <option value={1440}>1440p · 2K</option>
          <option value={2160}>2160p · 4K</option>
        </select>
        <button onclick={startProcessing} disabled={current.status === 'processing'}>
          {processLabel}
        </button>
        <button class="danger" onclick={deleteCurrent} disabled={current.status === 'processing'} title="Удалить из библиотеки">
          ✕
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
  </main>
</div>
{/if}

<style>
  :global(body) {
    margin: 0;
    background: #0d0d0d;
    color: #e0e0e0;
    font-family: system-ui, -apple-system, sans-serif;
  }
  :global(#app) {
    height: 100vh;
    display: flex;
    flex-direction: column;
  }

  header {
    padding: 0.75rem 1.5rem;
    border-bottom: 1px solid #1e1e1e;
    flex-shrink: 0;
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  header h1 { font-size: 1rem; font-weight: 600; letter-spacing: 0.02em; margin: 0; }
  .header-right { display: flex; align-items: center; gap: 0.75rem; }
  .user { font-size: 0.8rem; color: #666; }
  .link {
    background: none;
    border: none;
    color: #666;
    font-size: 0.8rem;
    cursor: pointer;
    padding: 0;
  }
  .link:hover { color: #aaa; }
  .danger { background: #3a1515; }
  .danger:hover:not(:disabled) { background: #5a1f1f; }

  .layout {
    display: grid;
    grid-template-columns: 300px 1fr;
    flex: 1;
    overflow: hidden;
  }
  .sidebar {
    border-right: 1px solid #1e1e1e;
    overflow-y: auto;
    padding: 1rem;
  }
  .player-area {
    background: #000;
    position: relative;
    overflow: hidden;
  }
  .controls-bar {
    position: absolute;
    top: 1rem;
    right: 1rem;
    z-index: 10;
    display: flex;
    gap: 0.5rem;
  }
  select {
    background: #1a1a1a;
    color: #ddd;
    border: 1px solid #2a2a2a;
    border-radius: 6px;
    padding: 0.5rem;
    font-size: 0.8rem;
  }
  button {
    background: #1f6feb;
    color: #fff;
    border: none;
    padding: 0.5rem 0.9rem;
    border-radius: 6px;
    font-size: 0.8rem;
    cursor: pointer;
    font-weight: 500;
  }
  button:hover { background: #2d7ff9; }
  button:disabled { background: #2a2a2a; color: #666; cursor: default; }
</style>
