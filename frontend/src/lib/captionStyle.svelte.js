// Per-viewer subtitle appearance + position, persisted to localStorage. Every
// viewer customizes their own captions (font, size, color, opacity, background,
// edge, and free position) — like YouTube's caption settings. This pairs with
// the per-viewer audio/subtitle-track model: nothing here is shared with the
// room, it's purely local presentation.

const KEY = 'wp_captions'

export const defaults = {
  size: 1, // font-size multiplier (Vidstack --media-user-font-size; 1 = default)
  color: '#ffffff',
  textOpacity: 1,
  bg: '#000000',
  bgOpacity: 0.75,
  font: 'sans', // sans | serif | mono | rounded
  edge: 'shadow', // none | shadow | outline | raised
  posX: 50, // % of the video area, box center (50 = horizontally centered)
  posY: 90, // % of the video area (90 = near the bottom)
}

function load() {
  try {
    const saved = JSON.parse(localStorage.getItem(KEY))
    if (saved && typeof saved === 'object') return { ...defaults, ...saved }
  } catch {
    /* corrupt/absent — fall through to defaults */
  }
  return { ...defaults }
}

export const caption = $state(load())

export function persistCaption() {
  localStorage.setItem(KEY, JSON.stringify({ ...caption }))
}

export function resetCaption() {
  Object.assign(caption, defaults)
  persistCaption()
}

export function resetCaptionPosition() {
  caption.posX = defaults.posX
  caption.posY = defaults.posY
  persistCaption()
}

export const FONTS = {
  sans: 'system-ui, -apple-system, "Segoe UI", Roboto, sans-serif',
  serif: 'Georgia, "Times New Roman", serif',
  mono: '"SF Mono", "Roboto Mono", ui-monospace, monospace',
  rounded: '"Trebuchet MS", "Comic Sans MS", "Chalkboard SE", cursive',
}

export const EDGES = {
  none: 'none',
  shadow: '1px 1px 2px rgba(0,0,0,0.95), 0 0 4px rgba(0,0,0,0.6)',
  outline:
    '-1px -1px 0 #000, 1px -1px 0 #000, -1px 1px 0 #000, 1px 1px 0 #000, 0 0 3px rgba(0,0,0,0.9)',
  raised: '1px 1px 0 rgba(0,0,0,0.6), 2px 2px 0 rgba(0,0,0,0.5), 3px 3px 4px rgba(0,0,0,0.7)',
}

// Preset swatches (YouTube's caption colors).
export const SWATCHES = ['#ffffff', '#faff00', '#00ff00', '#00ffff', '#3ea6ff', '#ff4fd8', '#ff3b30', '#000000']

// rgbaFromHex applies an opacity (0..1) to a #rrggbb color.
export function rgbaFromHex(hex, a) {
  const n = parseInt(hex.slice(1), 16)
  const r = (n >> 16) & 255
  const g = (n >> 8) & 255
  const b = n & 255
  return `rgba(${r}, ${g}, ${b}, ${a})`
}

// captionVars maps the store to inline CSS custom properties. These use our own
// --wp-cc-* / --cap-* namespace on purpose: Vidstack's built-in caption-settings
// system owns (and strips at init) its --media-user-* vars, so we don't touch
// those — Player.svelte applies our vars to the cues directly. size is a
// multiplier on the base cue size (see Player's font-size override).
export function captionVars(c) {
  return [
    `--wp-cc-size:${c.size}`,
    `--wp-cc-color:${rgbaFromHex(c.color, c.textOpacity)}`,
    `--wp-cc-bg:${rgbaFromHex(c.bg, c.bgOpacity)}`,
    `--wp-cc-font:${FONTS[c.font] ?? FONTS.sans}`,
    `--wp-cc-shadow:${EDGES[c.edge] ?? EDGES.shadow}`,
    `--cap-x:${c.posX}%`,
    `--cap-y:${c.posY}%`,
  ].join(';')
}
