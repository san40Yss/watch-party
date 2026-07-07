<script>
  // Vidstack web components + default video layout (controls, menus for audio
  // tracks and subtitles, quality, etc).
  import 'vidstack/player'
  import 'vidstack/player/layouts/default'
  import 'vidstack/player/ui'
  import 'vidstack/player/styles/default/theme.css'
  import 'vidstack/player/styles/default/layouts/video.css'
  import HLS from 'hls.js'
  import { caption, captionVars } from './captionStyle.svelte.js'

  let {
    src,
    title,
    placeholder = '', // App always passes a translated value; this is just a fallback
    // Fired on a genuine USER action (clicking play/pause, dragging the
    // scrubber): { type: 'play' | 'pause' | 'seek', time }. Step 5 (rooms)
    // broadcasts these to the room. Null = nobody's listening (local-only).
    onLocalAction = null,
    // Handed an imperative controller when the player is mounted, and null when
    // it tears down. Step 5 drives play/pause/seek from incoming room events
    // through this handle.
    onReady = null,
  } = $props()

  let player = $state(null)

  // Configure the HLS provider when it attaches:
  //  - library = bundled hls.js (Vidstack defaults to a jsdelivr CDN, which may
  //    be unreachable — bundling removes that dependency entirely).
  //  - renderTextTracksNatively off so subtitles don't show up twice.
  $effect(() => {
    if (!player) return
    const el = player
    const onProvider = (e) => {
      const p = e.detail
      if (p?.type === 'hls') {
        p.library = HLS
        // startPosition 0: a still-processing video plays as a growing EVENT
        // playlist, and hls.js would otherwise start at the live edge (the
        // encode frontier) instead of the beginning of the film.
        p.config = { ...p.config, renderTextTracksNatively: false, startPosition: 0 }
      }
    }
    // When the film changes, drop the previous source's subtitle tracks so the
    // menu doesn't accumulate every film opened this session. The player element
    // stays put (no teardown churn — which froze switching after a few cycles).
    const onSourceChange = () => {
      for (const tr of [...el.textTracks]) {
        if (tr.kind === 'subtitles' || tr.kind === 'captions') el.textTracks.remove(tr)
      }
    }
    el.addEventListener('provider-change', onProvider)
    el.addEventListener('source-change', onSourceChange)
    return () => {
      el.removeEventListener('provider-change', onProvider)
      el.removeEventListener('source-change', onSourceChange)
    }
  })

  // Control seam. Two halves, deliberately kept apart:
  //
  //  - OUTBOUND: Vidstack's "request" events fire only on user intent (play
  //    button, scrubber, keyboard, gestures) — never on programmatic commands.
  //    So they're exactly "the user did something" → surface via onLocalAction.
  //  - INBOUND: the controller drives the element imperatively. Those commands
  //    do NOT emit request events, so applying a remote command never echoes
  //    back as a local action — no feedback loop, no suppression flags needed.
  //
  // Local-only playback (today) just leaves both ends unwired; behavior is
  // identical to before. Rooms (Step 5) connect onLocalAction → broadcast and
  // room events → controller.
  $effect(() => {
    if (!player) return
    const el = player

    const controller = {
      // Returns the play() promise so callers can detect an autoplay block
      // (a guest joining a playing room without a user gesture).
      play: () => (el.paused ? el.play() : Promise.resolve()),
      pause: () => { if (!el.paused) el.pause().catch(() => {}) },
      seek: (t) => { el.currentTime = t },
      // Snapshot for syncing a late joiner to where the room already is.
      getState: () => ({ time: el.currentTime, paused: el.paused, duration: el.duration }),
    }

    const onPlayReq = () => onLocalAction?.({ type: 'play', time: el.currentTime })
    const onPauseReq = () => onLocalAction?.({ type: 'pause', time: el.currentTime })
    // The seek-request detail is the committed target time (seconds).
    const onSeekReq = (e) => onLocalAction?.({ type: 'seek', time: e.detail })

    el.addEventListener('media-play-request', onPlayReq)
    el.addEventListener('media-pause-request', onPauseReq)
    el.addEventListener('media-seek-request', onSeekReq)

    onReady?.(controller)

    return () => {
      el.removeEventListener('media-play-request', onPlayReq)
      el.removeEventListener('media-pause-request', onPauseReq)
      el.removeEventListener('media-seek-request', onSeekReq)
      onReady?.(null)
    }
  })
</script>

{#if src}
  <!-- captionVars feeds Vidstack's --media-user-* caption styling plus our
       --cap-* position vars — all per-viewer, from the caption store. -->
  <media-player bind:this={player} {title} {src} style={captionVars(caption)}>
    <media-provider></media-provider>
    <media-video-layout></media-video-layout>
  </media-player>
{:else}
  <div class="placeholder">
    <svg class="ph-logo" viewBox="0 0 64 64" aria-hidden="true">
      <circle cx="32" cy="32" r="18" fill="none" stroke="currentColor" stroke-width="3" />
      <path d="M27.5 23.5 L43 32 L27.5 40.5 Z" fill="currentColor" />
    </svg>
    <span class="ph-text">{placeholder}</span>
  </div>
{/if}

<style>
  media-player {
    width: 100%;
    height: 100%;
    --media-border-radius: 0;
    /* Theme Vidstack's default controls to match the amber accent. */
    --media-brand: var(--accent);
    --media-focus-ring-color: var(--accent);
  }
  .placeholder {
    height: 100%;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--sp-4);
    color: var(--text-faint);
    padding: var(--sp-5);
    text-align: center;
  }
  .ph-logo { width: 54px; height: 54px; opacity: 0.3; }
  .ph-text { font-size: var(--text-sm); max-width: 32ch; line-height: 1.5; }

  /* Per-viewer caption styling. We apply our own --wp-cc-* / --cap-* vars to the
     cues directly (Vidstack's built-in caption-settings owns the --media-user-*
     vars and strips them at init, so we can't ride those). Vidstack styles cues
     with :where() (zero specificity); !important also beats its per-cue inline
     styles. Size is a multiplier on the standard 4.5%-of-height cue size, using
     Vidstack's own --player-height (kept current across resize/fullscreen). */
  :global(.vds-captions) {
    font-size: calc(var(--player-height, 400px) / 100 * 4.5 * var(--wp-cc-size, 1)) !important;
    font-family: var(--wp-cc-font, sans-serif) !important;
  }
  :global(.vds-captions [data-part="cue-display"]) {
    top: var(--cap-y, 90%) !important;
    left: var(--cap-x, 50%) !important;
    right: auto !important;
    bottom: auto !important;
    transform: translate(-50%, -50%) !important;
    text-align: center !important;
    width: max-content !important;
    max-width: 84% !important;
  }
  :global(.vds-captions [data-part="cue"]) {
    color: var(--wp-cc-color, #fff) !important;
    background-color: var(--wp-cc-bg, rgba(0, 0, 0, 0.75)) !important;
    text-shadow: var(--wp-cc-shadow, 1px 1px 2px rgba(0, 0, 0, 0.95)) !important;
    backdrop-filter: none !important;
  }
</style>
