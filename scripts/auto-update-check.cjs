// Real, isolated NSIS update: custom Unicode directory, verified download, silent
// install/relaunch and persistent host/login/data. Never uses production appId.
const { build, Platform, Arch } = require('electron-builder')
const { _electron: electron } = require('playwright-core')
const { spawn } = require('node:child_process')
const fs = require('node:fs')
const path = require('node:path')
const http = require('node:http')
const crypto = require('node:crypto')
const assert = require('node:assert/strict')
const root = path.resolve(__dirname, '..')
const out = path.join(root, 'dist', 'verification')
const directory = path.join(out, 'auto-update')
const installDir = path.join(directory, '自选目录 with spaces', 'LinChat')
const results = []
let application, server, settings, badDownload = true
function record(test) { results.push({ test, passed: true }); console.log('PASS', test) }
async function waitFor(fn, label, timeout = 90000) {
  const deadline = Date.now() + timeout
  while (Date.now() < deadline) {
    try { const result = await fn(); if (result) return result } catch {}
    await new Promise(resolve => setTimeout(resolve, 200))
  }
  throw Error('Timed out: ' + label)
}
function run(file, args) {
  return new Promise((resolve, reject) => {
    const child = spawn(file, args, { stdio: 'inherit' })
    const timer = setTimeout(() => { child.kill(); reject(Error('Installer timeout')) }, 180000)
    child.once('error', reject)
    child.once('exit', code => { clearTimeout(timer); code === 0 ? resolve() : reject(Error(`Installer exited ${code}`)) })
  })
}
async function freePort() {
  const s = http.createServer()
  await new Promise(resolve => s.listen(0, '127.0.0.1', resolve))
  const port = s.address().port
  await new Promise(resolve => s.close(resolve))
  return port
}
function buildFingerprint() {
  const hash = crypto.createHash('sha256')
  const files = ['package.json', 'package-lock.json', 'scripts/auto-update-fixture.cjs', 'dist/server/linchat-windows-amd64.exe']
  function visit(directory) {
    for (const item of fs.readdirSync(path.join(root, directory), { withFileTypes: true })) {
      const file = directory + '/' + item.name
      if (item.isDirectory()) visit(file)
      else files.push(file)
    }
  }
  visit('desktop')
  for (const file of files.sort()) hash.update(file).update(fs.readFileSync(path.join(root, file)))
  return hash.digest('hex')
}
async function request(base, route, body, cookie) {
  const response = await fetch(base + '/api' + route, {
    method: body ? 'POST' : 'GET', headers: { 'Content-Type': 'application/json', Origin: base, 'X-LinChat': '1', ...(cookie ? { Cookie: cookie } : {}) },
    body: body ? JSON.stringify(body) : undefined,
  })
  const data = await response.json()
  assert.ok(response.ok, JSON.stringify(data))
  return { data, cookie: response.headers.get('set-cookie')?.split(';')[0] || cookie }
}
;(async () => {
  fs.mkdirSync(directory, { recursive: true })
  const pkg = require('../package.json')
  const fingerprint = buildFingerprint()
  for (const version of ['1.2.1', pkg.version]) {
    const marker = path.join(directory, version, 'build-input.sha256')
    if (fs.existsSync(marker) && fs.readFileSync(marker, 'utf8') === fingerprint) continue
    await build({ targets: Platform.WINDOWS.createTarget(['nsis'], Arch.x64), publish: 'never',
      config: { appId: 'site.linblog.linchat.update-test', productName: 'LinChatUpdateTest',
        executableName: 'LinChatUpdateTest', compression: 'store',
        extraMetadata: { name: 'linchat-update-test', version, main: 'scripts/auto-update-fixture.cjs',
          linchatUpdateTestConfig: path.join(directory, 'settings.json') },
        files: ['scripts/auto-update-fixture.cjs'],
        directories: { output: path.join(directory, version) },
        nsis: { createDesktopShortcut: false, createStartMenuShortcut: false, shortcutName: 'LinChatUpdateTest' },
      },
    })
    fs.writeFileSync(marker, fingerprint)
  }
  const target = path.join(directory, pkg.version)
  const installer = path.join(target, `LinChat-Setup-${pkg.version}.exe`)
  server = http.createServer((req, res) => {
    const name = decodeURIComponent(new URL(req.url, 'http://localhost').pathname).slice(1)
    if (!['latest.yml', path.basename(installer)].includes(name)) { res.writeHead(404); res.end(); return }
    if (name.endsWith('.exe') && badDownload) {
      const bytes = Buffer.from('corrupted download must never run')
      res.writeHead(200, { 'Content-Length': bytes.length }); res.end(bytes); return
    }
    const file = path.join(target, name)
    res.writeHead(200, { 'Content-Length': fs.statSync(file).size, 'Content-Type': name.endsWith('.yml') ? 'text/yaml' : 'application/octet-stream' })
    fs.createReadStream(file).pipe(res)
  })
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve))
  settings = { feed: `http://127.0.0.1:${server.address().port}/`, userData: fs.mkdtempSync(path.join(directory, 'profile-')),
    controlPort: await freePort(), report: path.join(directory, 'running.json') }
  const config = path.join(directory, 'settings.json')
  fs.writeFileSync(config, JSON.stringify(settings))
  fs.rmSync(path.join(process.env.LOCALAPPDATA, 'linchat-update-test-updater'), { recursive: true, force: true })
  await run(path.join(directory, '1.2.1', 'LinChat-Setup-1.2.1.exe'), ['/S', `/D=${installDir}`])
  const executablePath = path.join(installDir, 'LinChatUpdateTest.exe')
  assert.ok(fs.existsSync(path.join(installDir, 'Uninstall LinChatUpdateTest.exe')))
  application = await electron.launch({ executablePath, env: { ...process.env, LINCHAT_UPDATE_TEST_CONFIG: config }, timeout: 30000 })
  const page = await application.firstWindow()
  await page.locator('#adapter').filter({ hasNotText: '正在检查' }).waitFor({ state: 'attached' })
  assert.equal((await page.evaluate(() => window.linchatDesktop.getUpdateState())).autoUpdate, true)
  record('Installed fixture recognizes custom Unicode/space directory as auto-update capable')
  const host = await page.evaluate(() => window.linchatDesktop.startHost('local'))
  const token = new URL(host.setupURL).hash.replace('#setup=', '')
  const account = await request(host.url, '/setup', { token, username: 'updateowner', nickname: '升级验证', password: 'test-update-password-123' })
  await request(host.url, '/conversations/1/messages', { client_id: crypto.randomUUID(), content: '静默升级之前的聊天记录' }, account.cookie)
  await page.evaluate(url => { void window.linchatDesktop.connect(url, '升级保留连接') }, host.url)
  await page.locator('input[autocomplete="username"]').fill('updateowner')
  await page.locator('input[type="password"]').fill('test-update-password-123')
  await page.locator('.auth-form-wrap form').evaluate(form => form.requestSubmit())
  await page.locator('.workspace').waitFor()
  await page.evaluate(() => localStorage.setItem('update-preservation', 'retained'))
  const appearance = await page.evaluate(value => window.linchatDesktop.setAppearance(value),
    { accent: '#95afd0', background: '#10131a', panel: '#171c26' })
  const before = await request(host.url, '/conversations/1/messages', null, account.cookie)
  const campaign = await request(host.url, '/v1/campaigns', {
    title: '自动更新数据保留验证', ruleset: '5e-2024', members: [], scene: '更新前已保存的战役场景', house_rules: '升级验证',
  }, account.cookie)
  const campaignsBefore = await request(host.url, '/v1/campaigns', null, account.cookie)
  await page.evaluate(() => window.linchatDesktop.checkUpdates())
  await waitFor(async () => (await page.evaluate(() => window.linchatDesktop.getUpdateState())).status === 'error', 'checksum failure')
  assert.match((await page.evaluate(() => window.linchatDesktop.getUpdateState())).error, /校验失败/)
  assert.equal((await request(host.url, '/status')).data.initialized, true)
  record('Corrupt download is rejected while the existing application and host remain available')
  badDownload = false
  await page.evaluate(() => { void window.linchatDesktop.openUpdate() })
  await waitFor(async () => (await page.evaluate(() => window.linchatDesktop.getUpdateState())).status === 'ready', 'verified download', 180000)
  await page.screenshot({ path: path.join(out, 'auto-update-ready-desktop.png') })
  await application.evaluate(({ BrowserWindow }) => { const w = BrowserWindow.getAllWindows()[0]; w.setMinimumSize(320, 480); w.setContentSize(390, 780) })
  await page.screenshot({ path: path.join(out, 'auto-update-ready-mobile.png') })
  assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
  record('Real electron-updater downloads and SHA-512 validates installer; restart action renders at desktop and 390px')
  const closed = application.waitForEvent('close', { timeout: 30000 })
  await page.evaluate(() => { void window.linchatDesktop.installUpdate() })
  await closed
  application = null
  const running = await waitFor(() => {
    const value = JSON.parse(fs.readFileSync(settings.report, 'utf8'))
    return value.version === pkg.version ? value : null
  }, 'silent install and automatic relaunch', 180000)
  assert.equal(running.exe.toLowerCase(), executablePath.toLowerCase())
  assert.equal(running.userData, settings.userData)
  record('NSIS silently upgrades and automatically relaunches in the same custom directory')
  const control = `http://127.0.0.1:${settings.controlPort}`
  const restored = await waitFor(async () => {
    const state = await (await fetch(control + '/inspect')).json()
    return state.workspace ? state : null
  }, 'restored authenticated chat')
  assert.equal(restored.marker, 'retained')
  assert.deepEqual(restored.appearance, appearance)
  const after = await request(host.url, '/conversations/1/messages', null, account.cookie)
  assert.deepEqual(after.data.messages, before.data.messages)
  const campaignsAfter = await request(host.url, '/v1/campaigns', null, account.cookie)
  assert.deepEqual(campaignsAfter.data, campaignsBefore.data)
  assert.ok(campaign.data.id)
  assert.equal(restored.update.currentVersion, pkg.version)
  fs.writeFileSync(path.join(out, 'auto-update-restored-chat.png'), Buffer.from(await (await fetch(control + '/screenshot')).arrayBuffer()))
  record('Managed host restarts with identical saved messages, campaigns, login session and browser storage')
  await fetch(control + '/connections', { method: 'POST' })
  const state = await waitFor(async () => (await (await fetch(control + '/inspect')).json()).launcherState, 'restored launcher')
  assert.ok(state.profiles.some(p => p.url === host.url))
  assert.ok((await fetch(control + '/stop-host', { method: 'POST' })).ok)
  await fetch(`http://127.0.0.1:${settings.controlPort}/quit`, { method: 'POST' })
  await waitFor(() => { try { process.kill(running.pid, 0); return false } catch { return true } }, 'fixture exit')
  const hash = file => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex')
  assert.equal(hash(path.join(installDir, 'resources', 'app.asar')), hash(path.join(target, 'win-unpacked', 'resources', 'app.asar')))
  record('Installed app.asar exactly matches the verified target build')
  await run(path.join(installDir, 'Uninstall LinChatUpdateTest.exe'), ['/S', '/KEEP_APP_DATA'])
  fs.writeFileSync(path.join(out, 'auto-update-results.json'), JSON.stringify({ passed: true, version: pkg.version, results, installDir }, null, 2))
})().catch(async error => {
  console.error(error)
  if (application) {
    const page = await application.firstWindow().catch(() => null)
    if (page) {
      await page.screenshot({ path: path.join(out, 'auto-update-failure.png') }).catch(() => {})
      console.error(await page.locator('body').innerText().catch(() => 'Page closed'))
    }
  }
  if (settings) console.error('Restart diagnostics:', await fetch(`http://127.0.0.1:${settings.controlPort}/inspect`).then(r => r.text()).catch(() => 'Not running'))
  fs.writeFileSync(path.join(out, 'auto-update-results.json'), JSON.stringify({ passed: false, results, error: error.stack }, null, 2))
  process.exitCode = 1
}).finally(async () => {
  if (settings) await fetch(`http://127.0.0.1:${settings.controlPort}/quit`, { method: 'POST' }).catch(() => {})
  if (application) await application.close().catch(() => {})
  if (server) server.close()
})
