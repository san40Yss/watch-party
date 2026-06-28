<script>
  import * as tus from 'tus-js-client'
  import { t } from './i18n.svelte.js'

  let { onDone } = $props()
  let dragging = $state(false)
  // Active uploads: { name, progress, status: 'uploading'|'done'|'error' }
  let uploads = $state([])

  function pick() {
    const input = document.createElement('input')
    input.type = 'file'
    input.accept = 'video/*,.mkv'
    input.multiple = true
    input.onchange = () => startAll([...input.files])
    input.click()
  }

  function startAll(files) {
    for (const f of files) startOne(f)
  }

  function startOne(file) {
    const entry = $state({ name: file.name, progress: 0, status: 'uploading' })
    uploads = [...uploads, entry]

    const upload = new tus.Upload(file, {
      endpoint: '/api/upload/',
      // 50 MB chunks — resumable across drops without huge single requests.
      chunkSize: 50 * 1024 * 1024,
      retryDelays: [0, 1000, 3000, 5000, 10000],
      metadata: { filename: file.name, filetype: file.type },
      onError() {
        entry.status = 'error'
      },
      onProgress(sent, total) {
        entry.progress = (sent / total) * 100
      },
      onSuccess() {
        entry.status = 'done'
        onDone?.()
      },
    })
    upload.start()
  }

  function onDrop(e) {
    e.preventDefault()
    dragging = false
    startAll([...e.dataTransfer.files])
  }
</script>

<div
  class="dropzone"
  class:dragging
  role="button"
  tabindex="0"
  onclick={pick}
  onkeydown={(e) => e.key === 'Enter' && pick()}
  ondragover={(e) => { e.preventDefault(); dragging = true }}
  ondragleave={() => (dragging = false)}
  ondrop={onDrop}
>
  <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <path d="M12 16V4m0 0L7 9m5-5 5 5M5 18v1a1 1 0 0 0 1 1h12a1 1 0 0 0 1-1v-1" />
  </svg>
  <span class="dz-title">{t('upload_video')}</span>
  <span class="dz-sub">{t('upload_drop')}</span>
</div>

{#if uploads.length}
  <div class="uploads">
    {#each uploads as u (u.name)}
      <div class="up">
        <div class="up-row">
          <span class="up-name">{u.name}</span>
          <span class="up-pct tabular" class:done={u.status === 'done'} class:err={u.status === 'error'}>
            {u.status === 'done' ? t('up_done') : u.status === 'error' ? t('up_error') : `${Math.round(u.progress)}%`}
          </span>
        </div>
        <div class="up-track">
          <div class="up-fill" class:err={u.status === 'error'} style="width:{u.progress}%"></div>
        </div>
      </div>
    {/each}
  </div>
{/if}

<style>
  .dropzone {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--sp-1);
    border: 1px dashed var(--border-strong);
    border-radius: var(--r-lg);
    padding: var(--sp-5) var(--sp-3);
    text-align: center;
    color: var(--text-muted);
    cursor: pointer;
    transition: border-color var(--dur) var(--ease), color var(--dur) var(--ease), background var(--dur) var(--ease);
  }
  .dropzone:hover,
  .dropzone.dragging {
    border-color: var(--accent);
    color: var(--text);
    background: var(--accent-soft);
  }
  .dz-title { font-size: var(--text-sm); font-weight: 600; }
  .dz-sub { font-size: var(--text-xs); color: var(--text-faint); }

  .uploads { display: flex; flex-direction: column; gap: var(--sp-3); margin-top: var(--sp-3); }
  .up-row { display: flex; justify-content: space-between; gap: var(--sp-2); font-size: var(--text-xs); color: var(--text-muted); }
  .up-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .up-pct { flex-shrink: 0; }
  .up-pct.done { color: var(--success); }
  .up-pct.err { color: var(--error); }
  .up-track { height: 5px; background: var(--surface-3); border-radius: var(--r-full); margin-top: var(--sp-1); overflow: hidden; }
  .up-fill { height: 100%; background: var(--accent); border-radius: var(--r-full); transition: width 0.2s var(--ease); }
  .up-fill.err { background: var(--error); }
</style>
