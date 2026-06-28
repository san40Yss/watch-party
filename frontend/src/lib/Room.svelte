<script>
  import { room, create, join, leave, shareLink } from './room.svelte.js'

  let { currentVideoId } = $props()
  let code = $state('')
  let copied = $state(false)
  let err = $state('')
  let busy = $state(false)

  async function startParty() {
    err = ''
    busy = true
    try {
      await create(currentVideoId)
    } catch {
      err = 'Не удалось создать комнату'
    } finally {
      busy = false
    }
  }

  async function joinByCode() {
    err = ''
    busy = true
    try {
      await join(code.trim().toUpperCase())
    } catch {
      err = 'Комната не найдена'
    } finally {
      busy = false
    }
  }

  async function copy() {
    try {
      await navigator.clipboard.writeText(shareLink())
      copied = true
      setTimeout(() => (copied = false), 1500)
    } catch {
      /* clipboard blocked — the code is shown anyway */
    }
  }
</script>

<div class="room-panel">
  <div class="sidebar-label">Вечеринка</div>

  {#if !room.id}
    <button class="primary" onclick={startParty} disabled={!currentVideoId || busy}>
      Начать вечеринку
    </button>
    <div class="join">
      <input placeholder="Код" bind:value={code} maxlength="6" />
      <button onclick={joinByCode} disabled={!code || busy}>Войти</button>
    </div>
    {#if err}<div class="err">{err}</div>{/if}
  {:else}
    <div class="room-head">
      <span class="code">{room.id}</span>
      <span class="dot" class:on={room.connected}></span>
      <button class="link" onclick={leave}>покинуть</button>
    </div>
    <button class="copy" onclick={copy}>
      {copied ? 'Скопировано ✓' : 'Копировать ссылку'}
    </button>
    <div class="members">
      {#each room.members as m (m.username)}
        <div class="member">
          <span>{m.username}</span>
          {#if m.isHost}<span class="badge">хост</span>{/if}
        </div>
      {/each}
    </div>
    {#if !room.isHost}
      <div class="follow-note">▶ Воспроизведение ведёт хост</div>
    {/if}
  {/if}
</div>

<style>
  .room-panel { margin-bottom: 1.25rem; }
  .sidebar-label {
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: #555;
    margin-bottom: 0.6rem;
  }
  button {
    background: #1f6feb;
    color: #fff;
    border: none;
    padding: 0.5rem 0.8rem;
    border-radius: 6px;
    font-size: 0.8rem;
    cursor: pointer;
    font-weight: 500;
  }
  button:hover:not(:disabled) { background: #2d7ff9; }
  button:disabled { background: #2a2a2a; color: #666; cursor: default; }
  .primary { width: 100%; }

  .join { display: flex; gap: 0.4rem; margin-top: 0.5rem; }
  .join input {
    flex: 1;
    min-width: 0;
    background: #1a1a1a;
    border: 1px solid #2a2a2a;
    color: #e0e0e0;
    padding: 0.5rem;
    border-radius: 6px;
    font-size: 0.8rem;
    text-transform: uppercase;
  }
  .err { color: #e74c3c; font-size: 0.75rem; margin-top: 0.4rem; }

  .room-head {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin-bottom: 0.5rem;
  }
  .code {
    font-family: ui-monospace, monospace;
    font-size: 1rem;
    letter-spacing: 0.1em;
    color: #e0e0e0;
  }
  .dot { width: 7px; height: 7px; border-radius: 50%; background: #555; }
  .dot.on { background: #2ecc71; }
  .link {
    margin-left: auto;
    background: none;
    color: #666;
    padding: 0;
    font-size: 0.75rem;
  }
  .link:hover { color: #aaa; background: none; }
  .copy { width: 100%; background: #222; }
  .copy:hover:not(:disabled) { background: #2c2c2c; }

  .members { margin-top: 0.75rem; display: flex; flex-direction: column; gap: 0.3rem; }
  .member {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    font-size: 0.8rem;
    color: #bbb;
  }
  .badge {
    font-size: 0.65rem;
    background: #06331c;
    color: #2ecc71;
    padding: 0.05rem 0.35rem;
    border-radius: 4px;
  }
  .follow-note { margin-top: 0.6rem; font-size: 0.75rem; color: #e0b000; }
</style>
