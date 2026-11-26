// Headless layout acceptance for the actual preload, with isolated Electron IPC.
const { chromium } = require('playwright-core')
const fs = require('node:fs')
const path = require('node:path')
const assert = require('node:assert/strict')

async function main() {
  const { appearanceTokens, contrast } = await import('../desktop/theme.mjs')
  const executablePath = [
    process.env.CHROME_PATH,
    'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe',
    'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe',
  ].find(p => p && fs.existsSync(p))
  const browser = await chromium.launch({ executablePath, headless: true })
  const output = path.resolve(__dirname, '../dist/verification/titlebar')
  fs.mkdirSync(output, { recursive: true })
  try {
    const context = await browser.newContext()
    const preload = fs.readFileSync(path.resolve(__dirname, '../desktop/preload.cjs'), 'utf8')
    await context.addInitScript(({ preload }) => {
      const listeners = {}
      const initial = { enabled: true, color: '#1b2220', symbolColor: '#edf2ef', fullscreen: false }
      const electron = {
        contextBridge: { exposeInMainWorld: (name, value) => { window[name] = value } },
        ipcRenderer: {
          invoke: async () => initial,
          on: (name, callback) => { listeners[name] = callback },
          removeListener: () => {},
        },
      }
      window.applyChrome = state => listeners['window:chrome']?.({}, state)
      new Function('require', 'process', preload)(() => electron, { platform: 'win32' })
    }, { preload })
    const page = await context.newPage()
    const policyViolations = []
    await page.exposeFunction('recordPolicyViolation', directive => policyViolations.push(directive))
    await page.addInitScript(() => {
      document.addEventListener('securitypolicyviolation', event => window.recordPolicyViolation(event.violatedDirective))
    })
    const css = fs.readFileSync(path.resolve(__dirname, '../web/src/style.css'), 'utf8')
    const launcher = fs.readFileSync(path.resolve(__dirname, '../desktop/index.html'), 'utf8')
    const policy = launcher.match(/http-equiv="Content-Security-Policy" content="([^"]+)"/)[1]
    await page.route('http://titlebar.test/style.css', route => route.fulfill({ contentType: 'text/css', body: css }))
    await page.route('http://titlebar.test/', route => route.fulfill({
      contentType: 'text/html; charset=utf-8',
      body: `<html><head><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="${policy}"><title>LinChat · 让交流近一点</title><link rel="stylesheet" href="/style.css"></head>
        <body><div id="app"><main class="app-shell signed-in"><div class="workspace">
        <nav>LinChat</nav><aside>公共大厅</aside><section>继续对话</section>
        </div></main></div></body></html>`,
    }))
    for (const size of [{ width: 1280, height: 850 }, { width: 820, height: 600 }]) {
      await page.setViewportSize(size)
      await page.goto('http://titlebar.test/')
      await page.waitForSelector('html[data-linchat-titlebar]')
      for (const panel of ['#1b2220', '#fefefe', '#3355aa']) {
        const tokens = appearanceTokens({ panel })
        assert.ok(contrast(tokens['--panel'], tokens['--panel-text']) >= 4.5)
        await page.evaluate(tokens => {
          for (const [key, value] of Object.entries(tokens)) document.documentElement.style.setProperty(key, value)
          window.applyChrome({ enabled: true, color: tokens['--panel'], symbolColor: tokens['--panel-text'], fullscreen: false })
        }, tokens)
        const bounds = await page.locator('.app-shell').boundingBox()
        assert.equal(bounds.y, 32)
        assert.equal(bounds.y + bounds.height, size.height)
        assert.equal(await page.evaluate(() => document.documentElement.scrollHeight), size.height)
        const titlebar = await page.locator('#linchat-window-titlebar').boundingBox()
        assert.equal(titlebar.y, 0)
        assert.equal(titlebar.height, 32)
        assert.equal(titlebar.width, size.width)
        assert.equal(await page.locator('#linchat-window-titlebar').evaluate(el => el.style.getPropertyValue('--chrome-background')), panel)
        await page.screenshot({ path: path.join(output, `${size.width}-${panel.slice(1)}.png`) })
      }
      await page.evaluate(() => window.applyChrome({ enabled: true, fullscreen: true }))
      assert.equal(await page.locator('#linchat-window-titlebar').isVisible(), false)
      assert.equal((await page.locator('.app-shell').boundingBox()).height, size.height)
      await page.evaluate(() => window.applyChrome({ enabled: true, color: '#1b2220', symbolColor: '#edf2ef', fullscreen: false }))
      assert.equal((await page.locator('.app-shell').boundingBox()).y, 32)
    }
    assert.deepEqual(policyViolations, [], 'Titlebar must work without violating the launcher CSP')
    console.log('PASS: preload titlebar under launcher CSP, 2 window sizes × 3 palettes, contrast, viewport fit, fullscreen restoration')
  } finally { await browser.close() }
}
main().catch(error => { console.error(error); process.exitCode = 1 })
