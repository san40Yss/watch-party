<script>
  import * as api from './api.js'

  let { onClose } = $props()
  let current = $state('')
  let next = $state('')
  let confirm = $state('')
  let error = $state('')
  let busy = $state(false)
  let done = $state(false)

  async function submit(e) {
    e.preventDefault()
    error = ''
    if (next.length < 6) {
      error = 'Новый пароль — минимум 6 символов'
      return
    }
    if (next !== confirm) {
      error = 'Пароли не совпадают'
      return
    }
    busy = true
    try {
      await api.changePassword(current, next)
      done = true
      setTimeout(onClose, 1200)
    } catch (err) {
      error = err.message || 'Не удалось сменить пароль'
    } finally {
      busy = false
    }
  }
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && onClose()} />

<div class="overlay" onclick={(e) => { if (e.target === e.currentTarget) onClose() }} role="presentation">
  <form class="modal" onsubmit={submit}>
    <h2>Сменить пароль</h2>
    {#if done}
      <div class="ok">
        <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m5 12 5 5L20 6" /></svg>
        Пароль изменён
      </div>
    {:else}
      <input class="input" type="password" placeholder="Текущий пароль" bind:value={current} autocomplete="current-password" />
      <input class="input" type="password" placeholder="Новый пароль" bind:value={next} autocomplete="new-password" />
      <input class="input" type="password" placeholder="Повтор нового пароля" bind:value={confirm} autocomplete="new-password" />
      {#if error}<div class="err">{error}</div>{/if}
      <div class="actions">
        <button type="button" class="btn-ghost" onclick={onClose}>Отмена</button>
        <button class="btn" disabled={busy}>{busy ? 'Сохранение…' : 'Сохранить'}</button>
      </div>
    {/if}
  </form>
</div>

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.65);
    backdrop-filter: blur(4px);
    display: grid;
    place-items: center;
    padding: var(--sp-4);
    z-index: var(--z-modal);
    animation: fade var(--dur) var(--ease);
  }
  .modal {
    display: flex;
    flex-direction: column;
    gap: var(--sp-3);
    width: 100%;
    max-width: 320px;
    padding: var(--sp-5);
    background: var(--surface-1);
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
    box-shadow: var(--shadow-3);
    animation: pop var(--dur) var(--ease);
  }
  h2 { font-size: var(--text-lg); font-weight: 700; margin: 0 0 var(--sp-1); }
  .err { color: var(--error); font-size: var(--text-sm); }
  .ok {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--sp-2);
    color: var(--success);
    font-size: var(--text-base);
    font-weight: 600;
    padding: var(--sp-3) 0;
  }
  .actions { display: flex; gap: var(--sp-2); justify-content: flex-end; margin-top: var(--sp-1); }

  @keyframes fade { from { opacity: 0; } }
  @keyframes pop { from { opacity: 0; transform: scale(0.96) translateY(6px); } }
</style>
