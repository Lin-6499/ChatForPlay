// Test-only entry point. Excluded from production build.files.
const { app, Notification } = require('electron')
const fs = require('node:fs')
const http = require('node:http')
const { UPDATE_FEED } = require('../desktop/updates.cjs')
const settings = JSON.parse(fs.readFileSync(require('../package.json').linchatUpdateTestConfig, 'utf8'))
Object.assign(UPDATE_FEED, { provider: 'generic', url: settings.feed })
app.setPath('userData', settings.userData)
app.commandLine.appendSwitch('remote-debugging-port', String(settings.debugPort))
Notification.prototype.show = function () {}
app.whenReady().then(() => {
  const server = http.createServer((req, res) => {
    if (req.url !== '/quit' || req.method !== 'POST') { res.writeHead(404); res.end(); return }
    res.end('quitting')
    setImmediate(() => app.quit())
  })
  server.listen(settings.controlPort, '127.0.0.1', () => {
    fs.writeFileSync(settings.report, JSON.stringify({
      version: app.getVersion(), pid: process.pid, exe: process.execPath, userData: app.getPath('userData'),
    }))
  })
  app.on('will-quit', () => server.close())
})
require('../desktop/main.cjs')
