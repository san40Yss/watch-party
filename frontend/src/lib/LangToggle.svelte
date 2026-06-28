<script>
  import { i18n, setLang } from './i18n.svelte.js'
</script>

<div class="lang" role="group" aria-label="Language">
  <button class:active={i18n.lang === 'en'} onclick={() => setLang('en')}>EN</button>
  <button class:active={i18n.lang === 'ru'} onclick={() => setLang('ru')}>RU</button>
</div>

<style>
  .lang {
    display: inline-flex;
    gap: 2px;
    background: var(--surface-2);
    border-radius: var(--r-full);
    padding: 3px;
  }
  .lang button {
    background: none;
    border: none;
    color: var(--text-faint);
    font: inherit;
    font-size: 0.7rem;
    font-weight: 700;
    letter-spacing: 0.04em;
    padding: 3px 8px;
    border-radius: var(--r-full);
    cursor: pointer;
    transition: background var(--dur) var(--ease), color var(--dur) var(--ease);
  }
  .lang button:hover { color: var(--text-muted); }
  .lang button.active { background: var(--accent-soft); color: var(--accent); }
</style>
