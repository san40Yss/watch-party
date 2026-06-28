<script>
  // Vidstack web components + default video layout (controls, menus for audio
  // tracks and subtitles, quality, etc).
  import 'vidstack/player'
  import 'vidstack/player/layouts/default'
  import 'vidstack/player/ui'
  import 'vidstack/player/styles/default/theme.css'
  import 'vidstack/player/styles/default/layouts/video.css'
  import HLS from 'hls.js'

  let {
    src,
    title,
    placeholder = 'Выберите видео',
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
    const onProvider = (e) => {
      const p = e.detail
      if (p?.type === 'hls') {
        p.library = HLS
        p.config = { ...p.config, renderTextTracksNatively: false }
      }
    }
    player.addEventListener('provider-change', onProvider)
    return () => player.removeEventListener('provider-change', onProvider)
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
      play: () => { if (el.paused) el.play().catch(() => {}) },
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
  <media-player bind:this={player} {title} {src}>
    <media-provider></media-provider>
    <media-video-layout></media-video-layout>
  </media-player>
{:else}
  <div class="placeholder">{placeholder}</div>
{/if}

<style>
  media-player {
    width: 100%;
    height: 100%;
    --media-border-radius: 0;
  }
  .placeholder {
    color: #333;
    font-size: 0.9rem;
    display: grid;
    place-items: center;
    height: 100%;
  }
</style>
