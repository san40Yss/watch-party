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

<div class="overlay" onclick={onClose} role="presentation">
  <form class="modal" onsubmit={submit} onclick={(e) => e.stopPropagation()}>
    <h2>Сменить пароль</h2>
    {#if done}
      <div class="ok">Пароль изменён ✓</div>
    {:else}
      <input type="password" placeholder="Текущий пароль" bind:value={current} autocomplete="current-password" />
      <input type="password" placeholder="Новый пароль" bind:value={next} autocomplete="new-password" />
      <input type="password" placeholder="Повтор нового пароля" bind:value={confirm} autocomplete="new-password" />
      {#if error}<div class="err">{error}</div>{/if}
      <div class="actions">
        <button type="button" class="ghost" onclick={onClose}>Отмена</button>
        <button disabled={busy}>{busy ? 'Сохранение…' : 'Сохранить'}</button>
      </div>
    {/if}
  </form>
</div>

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.6);
    display: grid;
    place-items: center;
    z-index: 100;
  }
  .modal {
    display: flex;
    flex-direction: column;
    gap: 0.7rem;
    width: 300px;
    padding: 1.5rem;
    background: #141414;
    border: 1px solid #1e1e1e;
    border-radius: 10px;
  }
  h2 { font-size: 1rem; margin: 0 0 0.25rem; }
  input {
    background: #1a1a1a;
    border: 1px solid #2a2a2a;
    color: #e0e0e0;
    padding: 0.6rem 0.75rem;
    border-radius: 6px;
    font-size: 0.9rem;
  }
  .err { color: #e74c3c; font-size: 0.8rem; }
  .ok { color: #2ecc71; font-size: 0.9rem; text-align: center; padding: 0.5rem 0; }
  .actions { display: flex; gap: 0.5rem; justify-content: flex-end; margin-top: 0.25rem; }
  button {
    background: #1f6feb;
    color: #fff;
    border: none;
    padding: 0.5rem 0.9rem;
    border-radius: 6px;
    font-size: 0.85rem;
    cursor: pointer;
    font-weight: 500;
  }
  button:disabled { background: #2a2a2a; color: #666; }
  .ghost { background: #222; }
  .ghost:hover { background: #2c2c2c; }
</style>
