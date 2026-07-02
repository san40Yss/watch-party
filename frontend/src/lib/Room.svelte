<script>
  import { room, create, join, leave, shareLink } from './room.svelte.js'
  import { t } from './i18n.svelte.js'

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
      err = t('err_create_room')
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
      err = t('err_room_not_found')
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
  <div class="section-label">{t('party')}</div>

  {#if !room.id}
    <div class="card">
      <button class="btn start" onclick={startParty} disabled={!currentVideoId || busy}>
        <svg viewBox="0 0 24 24" width="16" height="16" fill="currentColor" aria-hidden="true">
          <path d="M8 5v14l11-7z" />
        </svg>
        {t('start_party')}
      </button>
      {#if !currentVideoId}
        <p class="hint">{t('pick_video')}</p>
      {/if}

      <div class="divider"><span>{t('or')}</span></div>

      <div class="join">
        <input class="input code-input" placeholder={t('code')} bind:value={code} maxlength="6" aria-label={t('room_code_aria')} />
        <button class="btn-ghost" onclick={joinByCode} disabled={!code || busy}>{t('join')}</button>
      </div>
      {#if err}<div class="err">{err}</div>{/if}
    </div>
  {:else}
    <div class="card live">
      <div class="room-head">
        <div class="code-block">
          <span class="code tabular">{room.id}</span>
          <span class="status">
            <span class="dot" class:on={room.connected}></span>
            {room.connected ? t('live') : t('connecting')}
          </span>
        </div>
        <button class="link" onclick={leave}>{t('leave')}</button>
      </div>

      <button class="btn-ghost copy" onclick={copy}>
        {#if copied}
          <svg viewBox="0 0 20 20" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m4 10 4 4 8-9" /></svg>
          {t('copied')}
        {:else}
          <svg viewBox="0 0 20 20" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" aria-hidden="true"><path d="M8 12a3 3 0 0 0 4.2 0l2.3-2.3a3 3 0 0 0-4.2-4.2l-.8.8M12 8a3 3 0 0 0-4.2 0L5.5 10.3a3 3 0 0 0 4.2 4.2l.8-.8" /></svg>
          {t('copy_link')}
        {/if}
      </button>

      <div class="members">
        <!-- Keyed by connection id: the same account in two tabs is two members. -->
        {#each room.members as m (m.connId)}
          <div class="member">
            <span class="avatar" class:host={m.isHost}>{m.username.slice(0, 1).toUpperCase()}</span>
            <span class="mname">{m.username}</span>
            {#if m.isHost}<span class="badge">{t('host')}</span>{/if}
          </div>
        {/each}
      </div>

      {#if !room.isHost}
        <div class="follow-note">
          <svg viewBox="0 0 24 24" width="13" height="13" fill="currentColor" aria-hidden="true"><path d="M8 5v14l11-7z" /></svg>
          {t('host_controls')}
        </div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .section-label { margin-bottom: var(--sp-3); }

  .card {
    background: var(--surface-1);
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
    padding: var(--sp-4);
    display: flex;
    flex-direction: column;
    gap: var(--sp-3);
  }
  .card.live { box-shadow: inset 0 0 0 1px var(--accent-soft), var(--shadow-2); }

  .start { width: 100%; }
  .hint { margin: 0; font-size: var(--text-xs); color: var(--text-faint); text-align: center; }

  .divider {
    display: flex;
    align-items: center;
    gap: var(--sp-3);
    color: var(--text-faint);
    font-size: var(--text-xs);
  }
  .divider::before,
  .divider::after {
    content: "";
    flex: 1;
    height: 1px;
    background: var(--border);
  }

  .join { display: flex; gap: var(--sp-2); }
  .code-input {
    flex: 1;
    min-width: 0;
    text-transform: uppercase;
    letter-spacing: 0.14em;
    text-align: center;
    font-family: var(--font-mono);
  }
  .err { color: var(--error); font-size: var(--text-xs); }

  .room-head { display: flex; align-items: flex-start; gap: var(--sp-3); }
  .code-block { display: flex; flex-direction: column; gap: 2px; }
  .code {
    font-family: var(--font-mono);
    font-size: 1.5rem;
    font-weight: 600;
    letter-spacing: 0.16em;
    color: var(--accent);
    line-height: 1;
  }
  .status {
    display: inline-flex;
    align-items: center;
    gap: var(--sp-2);
    font-size: var(--text-xs);
    color: var(--text-muted);
  }
  .dot {
    width: 7px;
    height: 7px;
    border-radius: var(--r-full);
    background: var(--text-faint);
    transition: background var(--dur) var(--ease);
  }
  .dot.on {
    background: var(--success);
    box-shadow: 0 0 0 3px var(--success-soft);
  }
  .link { margin-left: auto; }
  .copy { width: 100%; }

  .members { display: flex; flex-direction: column; gap: var(--sp-2); }
  .member { display: flex; align-items: center; gap: var(--sp-3); font-size: var(--text-sm); color: var(--text); }
  .avatar {
    display: grid;
    place-items: center;
    width: 28px;
    height: 28px;
    flex-shrink: 0;
    border-radius: var(--r-full);
    background: var(--surface-3);
    color: var(--text-muted);
    font-size: var(--text-xs);
    font-weight: 600;
  }
  .avatar.host { background: var(--accent-soft); color: var(--accent); }
  .mname { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .badge {
    margin-left: auto;
    font-size: 0.62rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    background: var(--accent-soft);
    color: var(--accent);
    padding: 2px 7px;
    border-radius: var(--r-full);
    font-weight: 600;
  }
  .follow-note {
    display: flex;
    align-items: center;
    gap: var(--sp-2);
    font-size: var(--text-xs);
    color: var(--accent);
    padding-top: var(--sp-1);
  }
</style>
