<script>
  let { videos, currentId, onSelect, etas = {} } = $props()

  const STATUS_LABEL = {
    pending: 'не обработано',
    processing: 'обработка',
    ready: 'готово',
    error: 'ошибка',
  }

  function statusText(v) {
    if (v.status === 'processing') {
      const pct = Math.round(v.progress || 0)
      const eta = etas[v.id]
      return eta != null ? `обработка ${pct}% · ~${eta} мин` : `обработка ${pct}%`
    }
    return STATUS_LABEL[v.status] || v.status
  }
</script>

<div class="sidebar-label">Библиотека</div>

{#if videos.length === 0}
  <span class="hint">Нет видео в библиотеке</span>
{:else}
  {#each videos as v (v.id)}
    <button
      class="video-item"
      class:active={v.id === currentId}
      onclick={() => onSelect(v.id)}
    >
      <div class="video-title">{v.title}</div>
      <div class="video-meta">
        <span class="badge {v.status}">{statusText(v)}</span>
        {#if v.video_codec}
          <span class="codec">{v.video_codec}{v.hdr ? ' · HDR' : ''}</span>
        {/if}
      </div>
      {#if v.status === 'processing'}
        <div class="progress-track">
          <div class="progress-fill" style="width:{v.progress || 0}%"></div>
        </div>
      {/if}
    </button>
  {/each}
{/if}

<style>
  .sidebar-label {
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: #555;
    margin-bottom: 0.75rem;
  }
  .hint { font-size: 0.8rem; color: #444; }

  .video-item {
    display: block;
    width: 100%;
    text-align: left;
    background: none;
    border: none;
    padding: 0.6rem 0.75rem;
    border-radius: 6px;
    cursor: pointer;
    margin-bottom: 0.25rem;
    color: inherit;
    font: inherit;
  }
  .video-item:hover { background: #1a1a1a; }
  .video-item.active { background: #1f1f1f; }

  .video-title { font-size: 0.875rem; color: #ddd; }

  .video-meta {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    margin-top: 0.3rem;
    font-size: 0.7rem;
  }
  .badge {
    padding: 0.1rem 0.4rem;
    border-radius: 4px;
    font-weight: 600;
  }
  .badge.pending { background: #2a2a2a; color: #888; }
  .badge.processing { background: #3a2e00; color: #e0b000; }
  .badge.ready { background: #06331c; color: #2ecc71; }
  .badge.error { background: #3a0d0d; color: #e74c3c; }
  .codec { color: #555; }

  .progress-track {
    margin-top: 0.4rem;
    height: 4px;
    border-radius: 2px;
    background: #1e1e1e;
    overflow: hidden;
  }
  .progress-fill {
    height: 100%;
    background: #e0b000;
    border-radius: 2px;
    transition: width 0.3s ease;
  }
</style>
