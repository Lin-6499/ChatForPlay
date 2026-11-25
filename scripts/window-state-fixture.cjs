// Loads the real desktop entry with every presentation path intercepted BEFORE
// it creates a window. Native hidden bounds are real; maximize is simulated so
// Electron cannot implicitly show the window during unattended verification.
const electron = require('electron')
const { app, screen, BrowserWindow } = electron
const { EventEmitter } = require('node:events')
const Module = require('node:module')
const fs = require('node:fs')
const path = require('node:path')
const assert = require('node:assert/strict')
const settings = JSON.parse(fs.readFileSync(process.env.LINCHAT_WINDOW_CHECK, 'utf8'))
process.env.LINCHAT_USER_DATA = settings.profile
const file = path.join(settings.profile, 'window-state.json')
let win, initialOptions, maximized = false, maximizeCalls = 0
const presentations = [], violations = []
const oldLoad = Module._load
class QuietTray extends EventEmitter { setToolTip() {} setContextMenu() {} }
class QuietNotification extends EventEmitter { static isSupported() { return false } show() { violations.push('notification') } }
Module._load = function (request, parent, isMain) {
  if (request === 'electron') return { ...electron, Tray: QuietTray, Notification: QuietNotification,
    BrowserWindow: new Proxy(BrowserWindow, { construct(Target, args) {
      assert.equal(args[0].show, false, 'Production window must be constructed hidden')
      initialOptions = args[0]
      return new Target(...args)
    } }),
  }
  return oldLoad.call(this, request, parent, isMain)
}
const ready = new Promise(resolve => app.once('browser-window-created', (_event, window) => {
  win = window
  for (const name of ['show', 'showInactive', 'focus', 'moveTop', 'restore', 'minimize', 'setFullScreen']) {
    window[name] = () => { presentations.push(name) }
  }
  window.maximize = () => { maximizeCalls++; maximized = true; window.emit('maximize') }
  window.isMaximized = () => maximized
  window.on('show', () => violations.push('visible-window'))
  window.on('focus', () => violations.push('focused-window'))
  window.webContents.once('did-finish-load', () => setImmediate(resolve))
}))
const previous = fs.existsSync(file) ? JSON.parse(fs.readFileSync(file, 'utf8')) : null
require(settings.main)

;(async () => {
  await ready
  assert.equal(win.isVisible(), false)
  assert.equal(win.isFocused(), false)
  assert.deepEqual(violations, [])
  const initial = win.getNormalBounds()
  if (settings.phase !== 'save') {
    for (const key of ['x', 'y', 'width', 'height']) assert.equal(initialOptions[key], previous.bounds[key])
    assert.deepEqual(initial, previous.bounds)
  }
  if (settings.phase === 'save') {
    const area = screen.getPrimaryDisplay().workArea
    win.setBounds({ x: area.x + 30, y: area.y + 25, width: Math.min(1000, area.width - 30), height: Math.min(700, area.height - 25) })
    // close is intercepted by production code and hides to tray. It must flush
    // without waiting for the move/resize debounce.
    win.close()
    assert.equal(win.isDestroyed(), false)
    assert.deepEqual(JSON.parse(fs.readFileSync(file, 'utf8')).bounds, win.getNormalBounds())
    win.setPosition(area.x + 40, area.y + 35)
  }
  if (settings.phase === 'maximized') assert.equal(maximizeCalls, 1)
  else assert.equal(maximizeCalls, 0)
  const expected = win.getNormalBounds()
  app.once('will-quit', () => {
    try {
      const stored = JSON.parse(fs.readFileSync(file, 'utf8'))
      assert.deepEqual(stored.bounds, expected)
      assert.equal(stored.maximized, settings.phase === 'maximized')
      assert.deepEqual(violations, [])
      fs.writeFileSync(settings.report, JSON.stringify({ passed: true, phase: settings.phase, initial, stored,
        hidden: true, presentationsIntercepted: presentations, maximizeCalls, violations,
        scope: 'Real hidden Electron bounds and production lifecycle; native maximize presentation is intercepted.' }, null, 2))
    } catch (error) { console.error(error); process.exitCode = 1 }
  })
  app.quit()
})().catch(error => { console.error(error); app.exit(1) })
