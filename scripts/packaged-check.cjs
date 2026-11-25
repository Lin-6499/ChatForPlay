const { _electron: electron } = require('playwright-core')
const fs = require('node:fs')
const path = require('node:path')
const assert = require('node:assert/strict')
const root = path.resolve(__dirname, '..')
const { version } = require(path.join(root, 'package.json'))
const out = path.join(root, 'dist', 'verification')
fs.mkdirSync(out, { recursive: true })
const executablePath = process.env.LINCHAT_CHECK_EXE || path.join(root, 'dist', 'desktop', version, 'win-unpacked', 'LinChat.exe')
const label = process.env.LINCHAT_CHECK_EXE ? 'installed' : 'packaged'
let application
async function closeApplication() {
  if (!application) return
  const closed = application.waitForEvent('close', { timeout: 10000 })
  await application.evaluate(({ app }) => app.quit()).catch(() => {})
  await closed
}
;(async () => {
  application = await electron.launch({
    executablePath,
    args: ['--user-data-dir=' + path.join(out, label + '-profile-' + version)], timeout: 30000,
  })
  const page = await application.firstWindow()
  await page.locator('#adapter').filter({ hasNotText: '正在检查' }).waitFor({ state: 'attached' })
  const metadata = await application.evaluate(({ app }) => ({
    name: app.getName(), version: app.getVersion(), packaged: app.isPackaged, dataDir: app.getPath('userData'),
  }))
  assert.equal(metadata.packaged, true)
  assert.equal(metadata.version, version)
  const updates = await page.evaluate(() => window.linchatDesktop.getUpdateState())
  assert.equal(updates.currentVersion, version)
  assert.equal(typeof updates.checking, 'boolean')
  const checkedUpdate = await page.evaluate(() => window.linchatDesktop.checkUpdates())
  assert.equal(checkedUpdate.error, '')
  assert.match(checkedUpdate.latestVersion, /^\d+\.\d+\.\d+$/)
  const launcherState = await page.evaluate(() => window.linchatDesktop.getState())
  await page.screenshot({ path: path.join(out, label + '-connection-center.png') })
  await page.locator('#url').fill('https://chat.linblog.site')
  await page.locator('#connect').click()
  await page.locator('.auth-form-wrap, .workspace').waitFor({ timeout: 45000 })
  assert.ok(page.url().startsWith('https://chat.linblog.site'))
  assert.ok(await page.locator('input[autocomplete="username"], .workspace').first().isVisible())
  const layout = await page.evaluate(() => ({
    scrollWidth: document.documentElement.scrollWidth, viewportWidth: innerWidth,
  }))
  assert.ok(layout.scrollWidth <= layout.viewportWidth + 1)
  await page.screenshot({ path: path.join(out, label + '-cloud-onboarding.png') })
  fs.writeFileSync(path.join(out, label + '-check-results.json'), JSON.stringify({
    timestamp: new Date().toISOString(), passed: true, executablePath, metadata, updateCheck: checkedUpdate, serverDataDir: launcherState.dataDir,
    url: page.url(), checks: ['Packaged executable boots', 'Packaged version and application data path',
      'Desktop update checker reaches the official GitHub release API',
      'Desktop connects to deployed HTTPS service through configured system networking', 'Cloud onboarding form renders without horizontal overflow'],
  }, null, 2))
  console.log('PASS Packaged Windows executable and deployed HTTPS onboarding', JSON.stringify(metadata))
  await closeApplication()
})().catch(async e => {
  console.error(e)
  if (application) {
    try {
      const pages = application.windows()
      if (pages[0]) {
        console.error('Page diagnostics:', await pages[0].evaluate(() => ({ url: location.href, text: document.body.innerText })))
        await pages[0].screenshot({ path: path.join(out, label + '-failure.png') })
      }
      await closeApplication()
    } catch {}
  }
  process.exitCode = 1
})
