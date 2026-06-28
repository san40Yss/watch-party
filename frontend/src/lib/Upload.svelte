<script>
  import * as tus from 'tus-js-client'

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
  + Загрузить видео
</div>

{#each uploads as u (u.name)}
  <div class="up">
    <div class="up-row">
      <span class="up-name">{u.name}</span>
      <span class="up-pct">
        {u.status === 'done' ? '✓' : u.status === 'error' ? '✕' : `${Math.round(u.progress)}%`}
      </span>
    </div>
    <div class="up-track">
      <div class="up-fill" class:err={u.status === 'error'} style="width:{u.progress}%"></div>
    </div>
  </div>
{/each}

<style>
  .dropzone {
    border: 1px dashed #2a2a2a;
    border-radius: 8px;
    padding: 0.75rem;
    text-align: center;
    font-size: 0.8rem;
    color: #888;
    cursor: pointer;
    margin-bottom: 1rem;
    transition: border-color 0.15s, color 0.15s;
  }
  .dropzone:hover, .dropzone.dragging { border-color: #1f6feb; color: #ddd; }

  .up { margin-bottom: 0.6rem; }
  .up-row { display: flex; justify-content: space-between; font-size: 0.72rem; color: #aaa; }
  .up-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 180px; }
  .up-track { height: 4px; background: #1e1e1e; border-radius: 2px; margin-top: 0.3rem; overflow: hidden; }
  .up-fill { height: 100%; background: #1f6feb; transition: width 0.2s; }
  .up-fill.err { background: #e74c3c; }
</style>
