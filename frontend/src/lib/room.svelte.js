// Watch-party room: the WebSocket connection + playback sync, wired to the
// player's imperative controller (the seam from the player refactor).
//
// Host-only model: the host's play/pause/seek is broadcast; guests follow and
// can't drive shared playback. Because the controller's commands are imperative
// (they don't emit the player's "request" events), applying a remote state
// never echoes back as a local action — so there's no feedback loop to guard.
import * as api from './api.js'

export const room = $state({
  id: null,
  isHost: false,
  connected: false,
  members: [],
  videoId: null, // which video the room is watching (drives App's selection)
  // True when the browser blocked autoplay for a guest: playback can only
  // start from a user gesture, so the UI shows a "tap to sync" overlay that
  // calls resumeSync().
  needsGesture: false,
})

let ws = null
let controller = null
let userId = null
let lastState = null     // latest { position, paused, receivedAt } from the server
let needsRestore = false // host: apply the next state once (restore position on rejoin)

// Reconnect-with-backoff bookkeeping. A dropped socket (phone locked, Wi-Fi
// blip) must not silently end the party.
let reconnectTimer = null
let reconnectAttempts = 0

// Which room we're in, persisted so a page refresh can rejoin automatically.
const STORE_KEY = 'wp_room'
export const storedRoomId = () => localStorage.getItem(STORE_KEY)

export function setUser(id) {
  userId = id
}

// The player hands its controller here when mounted (and null when torn down).
// Applies the room state once ready: guest follow, or host position restore.
export function attachController(c) {
  controller = c
  if (c && lastState) maybeApply(lastState)
}

// Guests always follow incoming state. The host applies only the first state
// after (re)connecting — that restores its position on refresh, without the host
// then fighting the echoes of its own broadcasts.
function maybeApply(s) {
  if (!controller) return
  if (!room.isHost) {
    applyState(s)
  } else if (needsRestore) {
    applyState(s)
    needsRestore = false
  }
}

function applyState(s) {
  if (!controller) return
  // The anchor was correct at receive time; while playing it keeps advancing,
  // so extrapolate by the local time elapsed since then. (Wall-clock deltas
  // against serverTime would add the server↔client clock skew instead.)
  const target = s.paused ? s.position : s.position + (Date.now() - s.receivedAt) / 1000
  // Only correct when drift is real, to avoid jitter from sub-second nudges.
  if (Math.abs(controller.getState().time - target) > 0.75) controller.seek(target)
  if (s.paused) {
    controller.pause()
  } else {
    // Browsers may reject play() without a user gesture (guest just opened the
    // link). Surface that so the UI can offer a tap-to-sync button.
    controller.play()?.then(
      () => { room.needsGesture = false },
      () => { room.needsGesture = true },
    )
  }
}

// Called from a click on the "tap to sync" overlay — inside a user gesture,
// play() is allowed again.
export function resumeSync() {
  room.needsGesture = false
  if (lastState) applyState(lastState)
}

// Host picked another film from the library: rebind the room. The server
// resets the anchor (paused at 0) and broadcasts it to everyone. room.videoId
// is set immediately so App's follow-effect doesn't revert the host's click
// while the echo is in flight.
export function switchVideo(videoId) {
  if (!room.isHost || videoId == null) return
  room.videoId = videoId
  sendCmd({ type: 'switch', videoId })
}

// Player.onLocalAction (a genuine user action): host broadcasts it; a guest's
// action is bounced back to the room's state (they can't control playback).
export function localAction(action) {
  if (!room.connected) return
  if (room.isHost) {
    let paused = controller?.getState().paused ?? false
    if (action.type === 'play') paused = false
    else if (action.type === 'pause') paused = true
    sendCmd({ type: action.type, time: action.time, paused })
  } else if (lastState) {
    // A guest can't drive shared playback. Re-assert the room state, but deferred:
    // our request-event listener runs before Vidstack applies the guest's own
    // action, so snapping back synchronously would just get overwritten.
    const s = lastState
    setTimeout(() => applyState(s), 80)
  }
}

function sendCmd(cmd) {
  if (ws && ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify(cmd))
}

function onMessage(raw) {
  const msg = JSON.parse(raw)
  if (msg.type === 'presence') {
    room.members = msg.members
  } else if (msg.type === 'state') {
    lastState = { ...msg, receivedAt: Date.now() }
    if (msg.videoId != null) room.videoId = msg.videoId
    maybeApply(lastState)
  }
}

function openSocket(id) {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  ws = new WebSocket(`${proto}://${location.host}/api/rooms/${id}/ws`)
  ws.onmessage = (e) => onMessage(e.data)
  ws.onopen = () => {
    room.connected = true
    reconnectAttempts = 0
  }
  // Closed for any reason while we still consider ourselves in the room →
  // reconnect. leave() clears room.id first, so a voluntary exit doesn't.
  ws.onclose = () => {
    room.connected = false
    if (room.id) scheduleReconnect()
  }
}

function scheduleReconnect() {
  if (reconnectTimer) return
  const delay = Math.min(1000 * 2 ** reconnectAttempts, 15000)
  reconnectAttempts++
  reconnectTimer = setTimeout(async () => {
    reconnectTimer = null
    if (!room.id) return
    try {
      await api.getRoom(room.id) // still exists? (also detects a purged room)
    } catch {
      leave()
      return
    }
    openSocket(room.id)
  }, delay)
}

// A backgrounded tab (locked phone) usually kills the socket; reconnect
// immediately when it becomes visible again instead of waiting out the backoff.
document.addEventListener('visibilitychange', () => {
  if (document.hidden || !room.id || room.connected) return
  clearTimeout(reconnectTimer)
  reconnectTimer = null
  reconnectAttempts = 0
  scheduleReconnect()
})

export async function create(videoId) {
  const r = await api.createRoom(videoId)
  room.id = r.id
  room.isHost = true
  room.videoId = r.video_id ?? videoId
  lastState = null
  needsRestore = true
  localStorage.setItem(STORE_KEY, r.id)
  openSocket(r.id)
  return r.id
}

export async function join(id) {
  let r
  try {
    r = await api.getRoom(id) // throws if it doesn't exist
  } catch (e) {
    localStorage.removeItem(STORE_KEY) // stale link or stored room
    throw e
  }
  room.id = r.id
  room.isHost = r.host_id === userId
  room.videoId = r.video_id
  lastState = null
  needsRestore = true
  localStorage.setItem(STORE_KEY, r.id)
  openSocket(r.id)
}

export function leave() {
  room.id = null // before close(), so onclose doesn't schedule a reconnect
  clearTimeout(reconnectTimer)
  reconnectTimer = null
  reconnectAttempts = 0
  if (ws) {
    ws.close()
    ws = null
  }
  localStorage.removeItem(STORE_KEY)
  room.isHost = false
  room.connected = false
  room.members = []
  room.videoId = null
  room.needsGesture = false
  lastState = null
  needsRestore = false
}

export function shareLink() {
  return room.id ? `${location.origin}/?room=${room.id}` : ''
}
