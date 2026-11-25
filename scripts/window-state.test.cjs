const { test } = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const { EventEmitter } = require('node:events')
const { restorePlacement, createWindowState } = require('../desktop/window-state.cjs')

const primary = { id: 1, workArea: { x: 0, y: 0, width: 1920, height: 1040 } }
const left = { id: 2, workArea: { x: -1920, y: -200, width: 1920, height: 1040 } }
const original = { x: -1720, y: -100, width: 1100, height: 760 }
const saved = (bounds = original, maximized = false) => ({ version: 1, bounds, maximized, display: left })

test('remembers exact normal bounds on a negative-coordinate secondary display', () => {
  const state = restorePlacement(saved(), [primary, left], primary)
  assert.deepEqual(state.bounds, original)
  assert.equal(state.maximized, false)
})

test('moves with the same monitor when desktop arrangement changes', () => {
  const moved = { ...left, workArea: { ...left.workArea, x: 1920, y: 50 } }
  assert.deepEqual(restorePlacement(saved(), [primary, moved], primary).bounds,
    { x: 2120, y: 150, width: 1100, height: 760 })
})

test('unplugged monitor, changed DPI/resolution and small work areas keep the window reachable', () => {
  const small = { id: 1, workArea: { x: 0, y: 30, width: 800, height: 550 } }
  const state = restorePlacement(saved(original, true), [small], small)
  assert.deepEqual(state.bounds, small.workArea)
  assert.equal(state.minWidth, 800)
  assert.equal(state.minHeight, 550)
  assert.equal(state.maximized, true)
  const resized = { ...left, workArea: { x: -1024, y: 0, width: 1024, height: 728 } }
  assert.deepEqual(restorePlacement(saved(), [primary, resized], primary).bounds,
    { x: -1024, y: 0, width: 1024, height: 728 })
})

test('missing, corrupt and unsupported preferences use centered screen-sized defaults', () => {
  for (const value of [null, {}, { version: 2, bounds: original }, saved({ ...original, width: -1 }), saved({ ...original, x: Infinity })]) {
    assert.deepEqual(restorePlacement(value, [primary], primary).bounds, { x: 320, y: 95, width: 1280, height: 850 })
  }
})

class Window extends EventEmitter {
  constructor(bounds) { super(); this.bounds = bounds; this.maximized = false; this.minimized = false; this.fullscreen = false; this.maximizeCalls = 0 }
  isDestroyed() { return false }
  isMinimized() { return this.minimized }
  isFullScreen() { return this.fullscreen }
  isMaximized() { return this.maximized }
  getNormalBounds() { return this.bounds }
  maximize() { this.maximizeCalls++; this.maximized = true; this.emit('maximize') }
}
function fixture(t) {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'linchat-window-state-'))
  const file = path.join(directory, 'window-state.json')
  const screen = { getAllDisplays: () => [primary, left], getPrimaryDisplay: () => primary,
    getDisplayMatching: bounds => bounds.x < 0 ? left : primary }
  const create = options => createWindowState({ file, screen, ...options })
  t.after(() => fs.rmSync(directory, { recursive: true, force: true }))
  return { file, directory, create, read: () => JSON.parse(fs.readFileSync(file, 'utf8')) }
}

test('resize/move followed immediately by minimize or close persists the latest normal placement', t => {
  const f = fixture(t), state = f.create(), win = new Window({ x: 120, y: 100, width: 1000, height: 700 })
  state.bind(win); state.restore()
  win.bounds = original; win.emit('move'); win.emit('resize')
  win.minimized = true; win.bounds = { x: -32000, y: -32000, width: 160, height: 28 }; win.emit('minimize')
  assert.deepEqual(f.read().bounds, original)
  win.emit('close')
  assert.deepEqual(f.read().bounds, original)
  assert.deepEqual(f.create().options, { ...original, minWidth: 820, minHeight: 600 })
  win.emit('closed')
})

test('maximized launches retain normal restore bounds and only maximize on intentional presentation', t => {
  const f = fixture(t)
  fs.writeFileSync(f.file, JSON.stringify(saved(original, true)))
  const state = f.create(), win = new Window(original)
  state.bind(win)
  win.emit('resize'); state.save()
  assert.equal(win.maximizeCalls, 0)
  assert.equal(f.read().maximized, true)
  state.restore(); state.restore(); state.save()
  assert.equal(win.maximizeCalls, 1)
  assert.deepEqual(f.read().bounds, original)
  assert.equal(f.read().maximized, true)
  win.fullscreen = true; win.bounds = { x: 0, y: 0, width: 1920, height: 1080 }; win.emit('resize'); state.save()
  assert.deepEqual(f.read().bounds, original)
  win.fullscreen = false; win.bounds = original; win.maximized = false; win.emit('unmaximize'); state.save()
  assert.equal(f.read().maximized, false)
  win.emit('closed')
})

test('debounced writes are atomic, and corrupted files do not block startup', async t => {
  const f = fixture(t)
  fs.writeFileSync(f.file, '{partial')
  const state = f.create({ delay: 5 }), win = new Window({ x: 100, y: 70, width: 1200, height: 800 })
  state.bind(win); state.restore()
  for (let i = 0; i < 10; i++) { win.bounds = { ...win.bounds, x: 100 + i }; win.emit('move') }
  await new Promise(resolve => setTimeout(resolve, 20))
  assert.equal(f.read().bounds.x, 109)
  assert.equal(fs.existsSync(f.file + '.tmp'), false)
  win.emit('closed')
})

test('unwritable preferences do not prevent closing or destroy the previous file', t => {
  const f = fixture(t), warnings = []
  fs.writeFileSync(f.file, JSON.stringify(saved()))
  fs.mkdirSync(f.file + '.tmp')
  const state = f.create({ warn: error => warnings.push(error) }), win = new Window(original)
  state.bind(win); state.restore(); state.save(); state.save()
  assert.equal(warnings.length, 1)
  assert.deepEqual(f.read().bounds, original)
  win.emit('closed')
})
