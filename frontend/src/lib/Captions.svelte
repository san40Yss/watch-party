<script>
  import {
    caption, persistCaption, resetCaption, resetCaptionPosition,
    rgbaFromHex, FONTS, EDGES, SWATCHES,
  } from './captionStyle.svelte.js'
  import { t } from './i18n.svelte.js'

  let open = $state(false)
  let layer = $state(null) // overlay filling the player area (drag reference)
  let dragging = $state(false)
  let layerH = $state(0)
  let videoH = $state(0) // painted video height, published by Player as --wp-video-h

  function measure() {
    if (layer) layerH = layer.clientHeight
    // Read the actual video height Player measured, so the preview scales off
    // the same reference as the real captions (not the letterboxed container).
    const mp = document.querySelector('media-player')
    if (mp) {
      const v = parseFloat(getComputedStyle(mp).getPropertyValue('--wp-video-h'))
      videoH = v > 0 ? v : 0
    }
  }
  $effect(() => {
    if (!open) return
    measure()
    const on = () => measure()
    window.addEventListener('resize', on)
    // The video height can settle a beat after the panel opens (metadata load).
    const t = setInterval(measure, 500)
    return () => { window.removeEventListener('resize', on); clearInterval(t) }
  })

  // Matches the real cue size: videoHeight * 4.5% * multiplier (falls back to
  // the layer height until the video is measured).
  const previewPx = $derived(Math.max(10, (videoH || layerH) * 0.045 * caption.size))

  function set(key, val) { caption[key] = val; persistCaption() }

  // Free-drag the preview; its position (box center, as % of the video area) is
  // what the real captions use.
  function startDrag(e) {
    e.preventDefault()
    dragging = true
    const rect = layer.getBoundingClientRect()
    const move = (ev) => {
      caption.posX = Math.min(96, Math.max(4, ((ev.clientX - rect.left) / rect.width) * 100))
      caption.posY = Math.min(97, Math.max(6, ((ev.clientY - rect.top) / rect.height) * 100))
    }
    const up = () => {
      dragging = false
      persistCaption()
      window.removeEventListener('pointermove', move)
      window.removeEventListener('pointerup', up)
    }
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', up)
  }

  const fonts = ['sans', 'serif', 'mono', 'rounded']
  const edges = ['none', 'shadow', 'outline', 'raised']
</script>

<div class="cap-layer" bind:this={layer}>
  <button class="cap-toggle" class:on={open} onclick={() => (open = !open)}
    aria-label={t('cc_settings')} title={t('cc_settings')}>
    <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
      <rect x="3" y="5" width="18" height="14" rx="2.5" />
      <path d="M9 10.5a2 2 0 0 0-3 1.5 2 2 0 0 0 3 1.5M15.5 10.5a2 2 0 0 0-3 1.5 2 2 0 0 0 3 1.5" />
    </svg>
  </button>

  {#if open}
    <!-- Draggable live preview: what you see is where the subtitles land. -->
    <div
      class="cap-preview"
      class:dragging
      style="left:{caption.posX}%; top:{caption.posY}%; font-size:{previewPx}px; font-family:{FONTS[caption.font]};"
      onpointerdown={startDrag}
      role="button"
      tabindex="0"
      aria-label={t('cc_drag_hint')}
    >
      <span style="color:{rgbaFromHex(caption.color, caption.textOpacity)}; background:{rgbaFromHex(caption.bg, caption.bgOpacity)}; text-shadow:{EDGES[caption.edge]};">
        {t('cc_sample')}
      </span>
    </div>

    <div class="cap-panel">
      <div class="cap-head">
        <span>{t('cc_settings')}</span>
        <button class="cap-x" onclick={() => (open = false)} aria-label={t('cc_close')}>
          <svg viewBox="0 0 20 20" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" aria-hidden="true"><path d="m5 5 10 10M15 5 5 15" /></svg>
        </button>
      </div>

      <p class="cap-hint">{t('cc_drag_hint')}</p>

      <label class="fld">
        <span>{t('cc_size')} <b class="tabular">{Math.round(caption.size * 100)}%</b></span>
        <input type="range" min="0.5" max="3" step="0.05" value={caption.size} oninput={(e) => set('size', +e.target.value)} />
      </label>

      <div class="fld">
        <span>{t('cc_text_color')}</span>
        <div class="swatches">
          {#each SWATCHES as sw}
            <button class="sw" class:sel={caption.color === sw} style="background:{sw}" onclick={() => set('color', sw)} aria-label={sw}></button>
          {/each}
        </div>
      </div>

      <label class="fld">
        <span>{t('cc_text_opacity')} <b class="tabular">{Math.round(caption.textOpacity * 100)}%</b></span>
        <input type="range" min="0.2" max="1" step="0.05" value={caption.textOpacity} oninput={(e) => set('textOpacity', +e.target.value)} />
      </label>

      <div class="fld">
        <span>{t('cc_bg_color')}</span>
        <div class="swatches">
          {#each SWATCHES as sw}
            <button class="sw" class:sel={caption.bg === sw} style="background:{sw}" onclick={() => set('bg', sw)} aria-label={sw}></button>
          {/each}
        </div>
      </div>

      <label class="fld">
        <span>{t('cc_bg_opacity')} <b class="tabular">{Math.round(caption.bgOpacity * 100)}%</b></span>
        <input type="range" min="0" max="1" step="0.05" value={caption.bgOpacity} oninput={(e) => set('bgOpacity', +e.target.value)} />
      </label>

      <div class="fld">
        <span>{t('cc_font')}</span>
        <div class="seg">
          {#each fonts as f}
            <button class:sel={caption.font === f} style="font-family:{FONTS[f]}" onclick={() => set('font', f)}>{t('font_' + f)}</button>
          {/each}
        </div>
      </div>

      <div class="fld">
        <span>{t('cc_edge')}</span>
        <div class="seg">
          {#each edges as ed}
            <button class:sel={caption.edge === ed} onclick={() => set('edge', ed)}>{t('edge_' + ed)}</button>
          {/each}
        </div>
      </div>

      <div class="cap-actions">
        <button class="cap-reset" onclick={resetCaptionPosition}>{t('cc_reset_pos')}</button>
        <button class="cap-reset" onclick={resetCaption}>{t('cc_reset')}</button>
      </div>
    </div>
  {/if}
</div>

<style>
  .cap-layer {
    position: absolute;
    inset: 0;
    pointer-events: none; /* only interactive children capture input */
    z-index: 12;
  }

  .cap-toggle {
    position: absolute;
    top: var(--sp-3);
    left: var(--sp-3);
    pointer-events: auto;
    display: grid;
    place-items: center;
    width: 38px;
    height: 38px;
    border-radius: var(--r-md);
    border: 1px solid var(--border);
    background: rgba(10, 10, 12, 0.6);
    backdrop-filter: blur(8px);
    color: var(--text);
    cursor: pointer;
    transition: color var(--dur) var(--ease), border-color var(--dur) var(--ease);
  }
  .cap-toggle:hover, .cap-toggle.on { color: var(--accent); border-color: var(--accent); }

  .cap-preview {
    position: absolute;
    transform: translate(-50%, -50%);
    pointer-events: auto;
    cursor: grab;
    max-width: 84%;
    text-align: center;
    line-height: 1.25;
    white-space: pre-line;
    touch-action: none;
  }
  .cap-preview.dragging { cursor: grabbing; }
  .cap-preview span { padding: 0.15em 0.4em; border-radius: 3px; box-decoration-break: clone; -webkit-box-decoration-break: clone; }
  .cap-preview::after {
    content: "";
    position: absolute;
    inset: -10px;
    border: 1.5px dashed var(--accent);
    border-radius: var(--r-md);
    opacity: 0.6;
    pointer-events: none;
  }

  .cap-panel {
    position: absolute;
    top: var(--sp-3);
    left: calc(38px + var(--sp-3) * 2);
    width: 280px;
    max-width: calc(100% - 38px - var(--sp-3) * 3);
    max-height: calc(100% - var(--sp-3) * 2);
    overflow-y: auto;
    pointer-events: auto;
    background: rgba(14, 14, 17, 0.94);
    backdrop-filter: blur(16px);
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
    box-shadow: var(--shadow-3);
    padding: var(--sp-4);
    display: flex;
    flex-direction: column;
    gap: var(--sp-3);
  }
  .cap-head { display: flex; align-items: center; justify-content: space-between; font-weight: 600; font-size: var(--text-sm); }
  .cap-x { background: none; border: none; color: var(--text-muted); cursor: pointer; padding: 4px; display: grid; place-items: center; }
  .cap-x:hover { color: var(--text); }
  .cap-hint { margin: 0; font-size: var(--text-xs); color: var(--text-faint); line-height: 1.4; }

  .fld { display: flex; flex-direction: column; gap: var(--sp-2); font-size: var(--text-xs); color: var(--text-muted); }
  .fld > span { display: flex; justify-content: space-between; }
  .fld b { color: var(--text); font-weight: 600; }
  input[type="range"] { width: 100%; accent-color: var(--accent); cursor: pointer; }

  .swatches { display: flex; gap: var(--sp-2); flex-wrap: wrap; }
  .sw {
    width: 24px; height: 24px; border-radius: var(--r-full);
    border: 2px solid transparent; cursor: pointer; padding: 0;
    box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.15);
  }
  .sw.sel { border-color: var(--accent); }

  .seg { display: flex; gap: 4px; }
  .seg button {
    flex: 1; padding: var(--sp-2) 4px; font-size: var(--text-xs);
    background: var(--surface-2); border: 1px solid var(--border);
    border-radius: var(--r-sm); color: var(--text-muted); cursor: pointer;
    white-space: nowrap; overflow: hidden; text-overflow: ellipsis;
  }
  .seg button.sel { background: var(--accent-soft); border-color: var(--accent); color: var(--accent); }

  .cap-actions { display: flex; gap: var(--sp-2); margin-top: var(--sp-1); }
  .cap-reset { flex: 1; padding: var(--sp-2); font-size: var(--text-xs); background: var(--surface-2); border: 1px solid var(--border); border-radius: var(--r-sm); color: var(--text-muted); cursor: pointer; }
  .cap-reset:hover { color: var(--text); border-color: var(--text-faint); }

  @media (max-width: 720px) {
    .cap-panel { left: var(--sp-3); right: var(--sp-3); width: auto; max-width: none; }
  }
</style>
