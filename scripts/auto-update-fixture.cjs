// Test-only entry point. Excluded from production build.files.
const { app, BrowserWindow, Notification } = require('electron')
const fs = require('node:fs')
const http = require('node:http')
const { UPDATE_FEED } = require('../desktop/updates.cjs')
const settings = JSON.parse(fs.readFileSync(require('../package.json').linchatUpdateTestConfig, 'utf8'))
Object.assign(UPDATE_FEED, { provider: 'generic', url: settings.feed })
app.setPath('userData', settings.userData)
Notification.prototype.show = function () {}
app.whenReady().then(() => {
  const server = http.createServer(async (req, res) => {
    try {
      if (req.url === '/quit' && req.method === 'POST') {
        res.end('quitting'); setImmediate(() => app.quit()); return
      }
      const window = BrowserWindow.getAllWindows()[0]
      if (!window) { res.writeHead(503); res.end(); return }
      if (req.url === '/inspect') {
        res.setHeader('Content-Type', 'application/json')
        res.end(JSON.stringify(await window.webContents.executeJavaScript(`(async () => ({
          workspace: !!document.querySelector('.workspace'), launcher: !!document.querySelector('#adapter'),
          text: document.body.innerText, update: await window.linchatDesktop.getUpdateState(),
          appearance: await window.linchatDesktop.getAppearance(), marker: localStorage.getItem('update-preservation'),
          launcherState: document.querySelector('#adapter') ? await window.linchatDesktop.getState() : null,
        }))()`)))
        return
      }
      if (req.url === '/connections' && req.method === 'POST') {
        await window.webContents.executeJavaScript('void window.linchatDesktop.openConnections()')
        res.end('opening'); return
      }
      if (req.url === '/stop-host' && req.method === 'POST') {
        await window.webContents.executeJavaScript('window.linchatDesktop.stopHost()')
        res.end('stopped'); return
      }
      if (req.url === '/screenshot') {
        res.setHeader('Content-Type', 'image/png')
        res.end((await window.capturePage()).toPNG()); return
      }
      res.writeHead(404); res.end()
    } catch (error) { res.writeHead(500); res.end(error.message) }
  })
  server.listen(settings.controlPort, '127.0.0.1', () => {
    fs.writeFileSync(settings.report, JSON.stringify({
      version: app.getVersion(), pid: process.pid, exe: process.execPath, userData: app.getPath('userData'),
    }))
  })
  app.on('will-quit', () => server.close())
})
require('../desktop/main.cjs')
