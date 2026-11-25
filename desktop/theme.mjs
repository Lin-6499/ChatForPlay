// Shared by the Vue client and the packaged Electron connection center.
export const APPEARANCE_KEY = 'linchat-appearance-v1'
export const DEFAULT_APPEARANCE = Object.freeze({
  version: 1, accent: '#a8c7b5', background: '#111615', panel: '#1b2220',
})
export const APPEARANCE_PRESETS = Object.freeze([
  { id: 'ink', name: '墨色', ...DEFAULT_APPEARANCE },
  { id: 'night', name: '夜蓝', version: 1, accent: '#a5bbdf', background: '#12161e', panel: '#1c2330' },
  { id: 'clay', name: '暖灰', version: 1, accent: '#d3b99d', background: '#191715', panel: '#26221f' },
  { id: 'graphite', name: '黑曜', version: 1, accent: '#c5c7cb', background: '#141414', panel: '#222222' },
])

export function normalizeColor(value) {
  if (typeof value !== 'string') return null
  const hex = value.trim().toLowerCase()
  if (/^#[0-9a-f]{6}$/.test(hex)) return hex
  if (/^#[0-9a-f]{3}$/.test(hex)) return '#' + [...hex.slice(1)].map(c => c + c).join('')
  return null
}
export function normalizeAppearance(value) {
  return {
    version: 1,
    ...Object.fromEntries(['accent', 'background', 'panel'].map(key => [
      key, normalizeColor(value?.[key]) || DEFAULT_APPEARANCE[key],
    ])),
  }
}
function channels(color) { return [1, 3, 5].map(i => parseInt(color.slice(i, i + 2), 16)) }
export function mix(a, b, amount) {
  const other = channels(b)
  return '#' + channels(a).map((n, i) => Math.round(n + (other[i] - n) * amount).toString(16).padStart(2, '0')).join('')
}
function luminance(color) {
  const c = channels(color).map(n => {
    const s = n / 255
    return s <= .04045 ? s / 12.92 : ((s + .055) / 1.055) ** 2.4
  })
  return c[0] * .2126 + c[1] * .7152 + c[2] * .0722
}
export function contrast(a, b) {
  const x = luminance(a), y = luminance(b)
  return (Math.max(x, y) + .05) / (Math.min(x, y) + .05)
}
export function readableColor(background, preferred, minimum = 4.5) {
  if (contrast(background, preferred) >= minimum) return preferred
  const target = contrast(background, '#ffffff') > contrast(background, '#000000') ? '#ffffff' : '#000000'
  for (let i = 1; i <= 100; i++) {
    const result = mix(preferred, target, i / 100)
    if (contrast(background, result) >= minimum) return result
  }
  return target
}
function surfaceTokens(background, accent) {
  const dark = contrast(background, '#ffffff') > contrast(background, '#000000')
  const text = readableColor(background, dark ? '#edf2ef' : '#17231c', 7)
  const lift = dark && luminance(background) < .12 ? '#ffffff' : dark ? '#000000' : '#ffffff'
  const raised = mix(background, lift, .035)
  const selected = mix(background, accent, .14)
  const warningBg = mix(background, '#d9b77b', .10)
  const dangerBg = mix(background, '#e5928b', .10)
  return {
    text, muted: readableColor(background, mix(background, text, .62), 5.5),
    line: mix(background, text, .12),
    border: readableColor(background, mix(background, text, .30), 2.8),
    hover: mix(background, text, .07), raised,
    'raised-text': readableColor(raised, text, 7),
    field: mix(background, lift, .02),
    'accent-text': readableColor(background, accent, 4.8),
    focus: readableColor(background, accent, 3),
    selected, 'selected-text': readableColor(selected, text, 7),
    'selected-muted': readableColor(selected, mix(selected, text, .7), 4.8),
    'selected-accent': readableColor(selected, accent, 4.8),
    'selected-line': mix(background, accent, .38),
    warning: readableColor(warningBg, '#d9b77b', 4.8),
    'warning-bg': warningBg, 'warning-line': mix(background, '#d9b77b', .34),
    danger: readableColor(dangerBg, '#e5928b', 4.8),
    'danger-bg': dangerBg, 'danger-line': mix(background, '#e5928b', .34),
    success: readableColor(background, '#9cc8ac', 4.8),
    'color-scheme': dark ? 'dark' : 'light',
    ...Object.fromEntries(Object.entries({ blue: '#829ec2', peach: '#cfa383', lilac: '#b2a2c9', sand: '#c6b579' }).flatMap(([name, color]) => {
      const fill = mix(background, color, .15)
      return [[`avatar-${name}`, fill], [`avatar-${name}-text`, readableColor(fill, text, 7)]]
    })),
  }
}
export function appearanceTokens(value) {
  const { accent, background, panel } = normalizeAppearance(value)
  const base = surfaceTokens(background, accent)
  const panels = surfaceTokens(panel, accent)
  const rail = mix(background, '#000000', .18)
  return {
    '--bg': background, '--panel': panel, '--accent': accent,
    '--on-accent': readableColor(accent, '#14251b', 7),
    '--accent-hover': mix(accent, readableColor(accent, '#ffffff', 3), .08),
    '--rail': rail,
    ...Object.fromEntries(Object.entries(base).map(([key, val]) => [`--${key}`, val])),
    ...Object.fromEntries(Object.entries(panels).map(([key, val]) => [`--panel-${key}`, val])),
    ...Object.fromEntries(Object.entries(surfaceTokens(rail, accent)).map(([key, val]) => [`--rail-${key}`, val])),
  }
}
let current = { ...DEFAULT_APPEARANCE }
let desktopBridge
let started = false
const listeners = new Set()
export function getAppearance() { return { ...current } }
export function onAppearanceChange(listener) {
  listeners.add(listener)
  return () => listeners.delete(listener)
}
export function applyAppearance(value) {
  current = normalizeAppearance(value)
  if (typeof document !== 'undefined') {
    const tokens = appearanceTokens(current)
    for (const [key, val] of Object.entries(tokens)) document.documentElement.style.setProperty(key, val)
    document.documentElement.style.colorScheme = tokens['--color-scheme']
    document.querySelector('meta[name="theme-color"]')?.setAttribute('content', current.background)
  }
  for (const listener of listeners) listener(getAppearance())
  return getAppearance()
}
function localAppearance() {
  try { return JSON.parse(localStorage.getItem(APPEARANCE_KEY) || 'null') } catch { return null }
}
export async function initializeAppearance(bridge) {
  desktopBridge = bridge
  applyAppearance(localAppearance())
  if (!started && typeof window !== 'undefined') {
    started = true
    window.addEventListener('storage', event => {
      if (event.key === APPEARANCE_KEY || event.key === null) applyAppearance(localAppearance())
    })
  }
  if (bridge?.getAppearance) {
    try {
      const stored = await bridge.getAppearance()
      if (stored) {
        applyAppearance(stored)
        try { localStorage.setItem(APPEARANCE_KEY, JSON.stringify(current)) } catch {}
      }
    } catch { /* Older desktop clients can keep using browser-local preferences. */ }
  }
}
export async function saveAppearance(value) {
  const normalized = normalizeAppearance(value)
  let saved = false
  if (desktopBridge?.setAppearance) {
    await desktopBridge.setAppearance(normalized)
    saved = true
  }
  try { localStorage.setItem(APPEARANCE_KEY, JSON.stringify(normalized)); saved = true } catch {}
  if (!saved) throw new Error('配色未保存：本地存储不可用，请允许网站存储后重试。')
  return applyAppearance(normalized)
}
