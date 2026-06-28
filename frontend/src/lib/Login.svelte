<script>
  import * as api from './api.js'

  let { onLogin } = $props()
  let mode = $state('login') // 'login' | 'register'
  let username = $state('')
  let password = $state('')
  let error = $state('')
  let busy = $state(false)

  const isRegister = $derived(mode === 'register')

  function switchMode(next) {
    mode = next
    error = ''
  }

  async function submit(e) {
    e.preventDefault()
    busy = true
    error = ''
    try {
      const user = isRegister
        ? await api.register(username, password)
        : await api.login(username, password)
      onLogin(user)
    } catch (err) {
      // Backend sends a friendly message for register (e.g. "имя занято");
      // login deliberately returns a generic one.
      error = isRegister
        ? err.message || 'Не удалось зарегистрироваться'
        : 'Неверный логин или пароль'
    } finally {
      busy = false
    }
  }
</script>

<div class="login-wrap">
  <form class="card" onsubmit={submit}>
    <div class="brand">
      <svg class="logo" viewBox="0 0 64 64" aria-hidden="true">
        <circle cx="32" cy="32" r="18" fill="none" stroke="var(--accent)" stroke-width="4" />
        <path d="M27.5 23.5 L43 32 L27.5 40.5 Z" fill="var(--accent)" />
      </svg>
      <h1>Watch Party</h1>
      <p class="tagline">Смотрите свою медиатеку вместе</p>
    </div>

    <div class="tabs">
      <button type="button" class:active={!isRegister} onclick={() => switchMode('login')}>Войти</button>
      <button type="button" class:active={isRegister} onclick={() => switchMode('register')}>Регистрация</button>
    </div>

    <input class="input" placeholder="Логин" bind:value={username} autocomplete="username" />
    <input
      class="input"
      placeholder="Пароль"
      type="password"
      bind:value={password}
      autocomplete={isRegister ? 'new-password' : 'current-password'}
    />
    {#if isRegister}
      <div class="hint">Логин от 3 символов, пароль от 6</div>
    {/if}
    {#if error}<div class="err">{error}</div>{/if}
    <button class="btn submit" disabled={busy}>
      {busy
        ? (isRegister ? 'Создание…' : 'Вход…')
        : (isRegister ? 'Создать аккаунт' : 'Войти')}
    </button>
  </form>
</div>

<style>
  .login-wrap {
    min-height: 100dvh;
    display: grid;
    place-items: center;
    padding: var(--sp-4);
    position: relative;
    z-index: 1;
  }
  .card {
    display: flex;
    flex-direction: column;
    gap: var(--sp-3);
    width: 100%;
    max-width: 320px;
    padding: var(--sp-6);
    background: var(--surface-1);
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
    box-shadow: var(--shadow-3);
  }
  .brand { display: flex; flex-direction: column; align-items: center; gap: var(--sp-2); margin-bottom: var(--sp-2); }
  .logo { width: 44px; height: 44px; filter: drop-shadow(var(--glow-accent)); }
  h1 { font-size: var(--text-lg); font-weight: 700; margin: 0; }
  .tagline { font-size: var(--text-sm); color: var(--text-muted); margin: 0; text-align: center; }

  .tabs {
    display: flex;
    gap: var(--sp-1);
    background: var(--surface-2);
    border-radius: var(--r-md);
    padding: 4px;
  }
  .tabs button {
    flex: 1;
    background: none;
    border: none;
    color: var(--text-muted);
    padding: var(--sp-2);
    border-radius: var(--r-sm);
    font: inherit;
    font-size: var(--text-sm);
    cursor: pointer;
    font-weight: 600;
    transition: background var(--dur) var(--ease), color var(--dur) var(--ease);
  }
  .tabs button.active { background: var(--surface-3); color: var(--text); }

  .hint { font-size: var(--text-xs); color: var(--text-faint); }
  .err { color: var(--error); font-size: var(--text-sm); }
  .submit { width: 100%; min-height: 42px; margin-top: var(--sp-1); }
</style>
