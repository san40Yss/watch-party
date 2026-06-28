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
  <form class="login" onsubmit={submit}>
    <h1>Watch Party</h1>

    <div class="tabs">
      <button
        type="button"
        class:active={!isRegister}
        onclick={() => switchMode('login')}
      >Войти</button>
      <button
        type="button"
        class:active={isRegister}
        onclick={() => switchMode('register')}
      >Регистрация</button>
    </div>

    <input
      placeholder="Логин"
      bind:value={username}
      autocomplete="username"
    />
    <input
      placeholder="Пароль"
      type="password"
      bind:value={password}
      autocomplete={isRegister ? 'new-password' : 'current-password'}
    />
    {#if isRegister}
      <div class="hint">Логин от 3 символов, пароль от 6.</div>
    {/if}
    {#if error}<div class="err">{error}</div>{/if}
    <button disabled={busy}>
      {busy
        ? (isRegister ? 'Создание…' : 'Вход…')
        : (isRegister ? 'Создать аккаунт' : 'Войти')}
    </button>
  </form>
</div>

<style>
  .login-wrap {
    height: 100vh;
    display: grid;
    place-items: center;
    background: #0d0d0d;
  }
  .login {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
    width: 280px;
    padding: 2rem;
    background: #141414;
    border: 1px solid #1e1e1e;
    border-radius: 10px;
  }
  h1 { font-size: 1.1rem; margin: 0 0 0.5rem; text-align: center; }

  .tabs {
    display: flex;
    gap: 0.25rem;
    background: #1a1a1a;
    border-radius: 6px;
    padding: 0.2rem;
  }
  .tabs button {
    flex: 1;
    background: none;
    border: none;
    color: #888;
    padding: 0.4rem;
    border-radius: 4px;
    font-size: 0.8rem;
    cursor: pointer;
    font-weight: 500;
  }
  .tabs button.active { background: #2a2a2a; color: #e0e0e0; }

  input {
    background: #1a1a1a;
    border: 1px solid #2a2a2a;
    color: #e0e0e0;
    padding: 0.6rem 0.75rem;
    border-radius: 6px;
    font-size: 0.9rem;
  }
  .hint { font-size: 0.72rem; color: #555; }
  .err { color: #e74c3c; font-size: 0.8rem; }
  button {
    background: #1f6feb;
    color: #fff;
    border: none;
    padding: 0.6rem;
    border-radius: 6px;
    font-size: 0.9rem;
    cursor: pointer;
    font-weight: 500;
  }
  button:disabled { background: #2a2a2a; color: #666; }
</style>
