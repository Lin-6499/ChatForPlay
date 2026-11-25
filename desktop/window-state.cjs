const fs = require('node:fs')
const path = require('node:path')

const DEFAULT_SIZE = { width: 1280, height: 850 }
const MIN_SIZE = { width: 820, height: 600 }
const validBounds = value => value && ['x', 'y', 'width', 'height'].every(key => Number.isSafeInteger(value[key])) &&
  value.width > 0 && value.height > 0
const boundsOnly = ({ x, y, width, height }) => ({ x, y, width, height })
const clamp = (value, min, max) => Math.min(max, Math.max(min, value))
const overlap = (a, b) => Math.max(0, Math.min(a.x + a.width, b.x + b.width) - Math.max(a.x, b.x)) *
  Math.max(0, Math.min(a.y + a.height, b.y + b.height) - Math.max(a.y, b.y))

function restorePlacement(saved, displays, primary) {
  const valid = saved?.version === 1 && validBounds(saved.bounds)
  let display = primary
  if (valid) {
    // Prefer the original monitor even if its desktop coordinates have changed.
    display = displays.find(item => item.id === saved.display?.id) ||
      displays.reduce((best, item) => overlap(saved.bounds, item.workArea) > overlap(saved.bounds, best.workArea) ? item : best, primary)
  }
  const area = display.workArea
  const minWidth = Math.min(MIN_SIZE.width, area.width), minHeight = Math.min(MIN_SIZE.height, area.height)
  const width = clamp(valid ? saved.bounds.width : DEFAULT_SIZE.width, minWidth, area.width)
  const height = clamp(valid ? saved.bounds.height : DEFAULT_SIZE.height, minHeight, area.height)
  let x = area.x + Math.round((area.width - width) / 2), y = area.y + Math.round((area.height - height) / 2)
  if (valid) {
    x = saved.bounds.x; y = saved.bounds.y
    if (display.id === saved.display?.id && validBounds(saved.display.workArea)) {
      x += area.x - saved.display.workArea.x
      y += area.y - saved.display.workArea.y
    }
  }
  return {
    bounds: { x: clamp(x, area.x, area.x + area.width - width), y: clamp(y, area.y, area.y + area.height - height), width, height },
    maximized: valid && saved.maximized === true,
    display: { id: display.id, workArea: boundsOnly(area) },
    minWidth, minHeight,
  }
}

function createWindowState({ file, screen, delay = 350, warn = error => console.warn('Could not save window state:', error.message) }) {
  let saved
  try { saved = JSON.parse(fs.readFileSync(file, 'utf8')) } catch {}
  const placement = restorePlacement(saved, screen.getAllDisplays(), screen.getPrimaryDisplay())
  let current = { version: 1, bounds: placement.bounds, maximized: placement.maximized, display: placement.display }
  let window, timer, restored = false, lastWritten = '', warned = false

  function capture() {
    // Minimized/fullscreen rectangles must never replace the normal restore bounds.
    if (!restored || !window || window.isDestroyed() || window.isMinimized() || window.isFullScreen()) return
    const bounds = window.getNormalBounds()
    if (!validBounds(bounds)) return
    const display = screen.getDisplayMatching(bounds)
    current = { version: 1, bounds: boundsOnly(bounds), maximized: window.isMaximized(),
      display: { id: display.id, workArea: boundsOnly(display.workArea) } }
  }
  function write() {
    clearTimeout(timer); timer = undefined
    if (!restored) return
    const content = JSON.stringify(current, null, 2) + '\n'
    if (content === lastWritten) return
    const temporary = file + '.tmp'
    try {
      fs.mkdirSync(path.dirname(file), { recursive: true })
      fs.writeFileSync(temporary, content, 'utf8')
      fs.renameSync(temporary, file)
      lastWritten = content; warned = false
    } catch (error) {
      try { fs.unlinkSync(temporary) } catch {}
      if (!warned) { warned = true; warn(error) }
    }
  }
  function changed() {
    capture()
    clearTimeout(timer)
    timer = setTimeout(write, delay)
    timer.unref?.()
  }
  function save() { capture(); write() }
  function bind(win) {
    window = win
    for (const event of ['move', 'resize', 'maximize', 'unmaximize', 'restore', 'leave-full-screen']) window.on(event, changed)
    for (const event of ['minimize', 'hide', 'close']) window.on(event, save)
    window.once('closed', () => { clearTimeout(timer); timer = undefined; window = undefined })
  }
  function restore() {
    if (restored || !window || window.isDestroyed()) return
    restored = true
    // Electron's maximize() can show a hidden window. Call this only from the
    // application's intentional show path, never while constructing the window.
    if (placement.maximized) window.maximize()
    changed()
  }
  return { options: { ...placement.bounds, minWidth: placement.minWidth, minHeight: placement.minHeight }, bind, restore, save }
}

module.exports = { restorePlacement, createWindowState }
