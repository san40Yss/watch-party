<script>
  import { t } from './i18n.svelte.js'

  let { videos, currentId, onSelect, etas = {} } = $props()

  const STATUS_KEY = {
    pending: 'st_pending',
    processing: 'st_processing',
    ready: 'st_ready',
    error: 'st_error',
  }

  function statusText(v) {
    if (v.status === 'processing') {
      const pct = Math.round(v.progress || 0)
      const eta = etas[v.id]
      return eta != null ? t('lib_processing_eta', { pct, min: eta }) : t('lib_processing', { pct })
    }
    return t(STATUS_KEY[v.status]) || v.status
  }
</script>

<div class="section-label lib-label">{t('library')}</div>

{#if videos.length === 0}
  <p class="empty">{t('library_empty')}</p>
{:else}
  <div class="list">
    {#each videos as v (v.id)}
      <button
        class="video-item"
        class:active={v.id === currentId}
        onclick={() => onSelect(v.id)}
      >
        <div class="video-title">{v.title}</div>
        <div class="video-meta">
          <span class="badge {v.status}">
            {#if v.status === 'ready'}
              <svg viewBox="0 0 20 20" width="11" height="11" fill="currentColor" aria-hidden="true"><path d="M7 5v10l8-5z" /></svg>
            {/if}
            {statusText(v)}
          </span>
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
  </div>
{/if}

<style>
  .lib-label { margin-bottom: var(--sp-3); }
  .empty { font-size: var(--text-sm); color: var(--text-faint); margin: 0; }

  .list { display: flex; flex-direction: column; gap: var(--sp-1); }

  .video-item {
    display: block;
    width: 100%;
    text-align: left;
    background: transparent;
    border: 1px solid transparent;
    border-left: 3px solid transparent;
    padding: var(--sp-3);
    border-radius: var(--r-md);
    cursor: pointer;
    color: inherit;
    font: inherit;
    transition: background var(--dur) var(--ease), border-color var(--dur) var(--ease);
  }
  .video-item:hover { background: var(--surface-1); }
  .video-item.active {
    background: var(--surface-2);
    border-left-color: var(--accent);
  }

  .video-title {
    font-size: var(--text-sm);
    color: var(--text);
    font-weight: 500;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .video-meta {
    display: flex;
    align-items: center;
    gap: var(--sp-2);
    margin-top: var(--sp-2);
  }
  .badge {
    display: inline-flex;
    align-items: center;
    gap: 3px;
    padding: 2px 7px;
    border-radius: var(--r-full);
    font-size: var(--text-xs);
    font-weight: 600;
  }
  .badge.pending { background: var(--surface-3); color: var(--text-muted); }
  .badge.processing { background: var(--warn-soft); color: var(--warn); }
  .badge.ready { background: var(--success-soft); color: var(--success); }
  .badge.error { background: var(--error-soft); color: var(--error); }
  .codec {
    font-size: var(--text-xs);
    color: var(--text-faint);
    font-variant-numeric: tabular-nums;
  }

  .progress-track {
    margin-top: var(--sp-2);
    height: 4px;
    border-radius: var(--r-full);
    background: var(--surface-3);
    overflow: hidden;
  }
  .progress-fill {
    height: 100%;
    background: var(--accent);
    border-radius: var(--r-full);
    transition: width 0.3s var(--ease);
  }
</style>
