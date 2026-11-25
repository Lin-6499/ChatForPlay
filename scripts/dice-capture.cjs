// Real-time supplement to motion-capture's scrubbed CSS timeline, plus a video.
const fs = require('node:fs')
const path = require('node:path')
const { chromium } = require('playwright-core')
const assert = require('node:assert/strict')
const root = path.resolve(__dirname, '..')
const review = path.join(root, '.impeccable/review/dice')
let browser
;(async () => {
  browser = await chromium.launch({ executablePath: 'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe', headless: true })
  for (const mode of ['normal', 'reduced']) {
    const out = path.join(review, mode + '-live')
    fs.mkdirSync(out, { recursive: true })
    const context = await browser.newContext({
      viewport: { width: 1000, height: 760 }, reducedMotion: mode === 'reduced' ? 'reduce' : 'no-preference',
      recordVideo: { dir: out, size: { width: 1000, height: 760 } },
    })
    const page = await context.newPage()
    await page.goto('http://127.0.0.1:5187/?motion-demo#campaign=1')
    await page.getByRole('button', { name: '掷骰', exact: true }).click()
    await page.getByLabel('检定目的', { exact: true }).waitFor()
    const tray = page.locator('.dice-tray')
    const box = await tray.boundingBox()
    const frames = []
    async function capture(label) {
      const metadata = await tray.evaluate(e => ({ phase: e.dataset.phase, elapsed: window.rollStart ? performance.now() - window.rollStart : 0, total: e.querySelector('.dice-total strong').textContent }))
      const file = `frame-${String(frames.length).padStart(2, '0')}.png`
      await page.screenshot({ path: path.join(out, file), clip: box })
      frames.push({ label, ...metadata, image: file })
    }
    await capture('before')
    await page.evaluate(() => { window.rollStart = performance.now() })
    await page.getByRole('button', { name: '掷骰并记录', exact: true }).click()
    for (const time of mode === 'normal' ? [0, 140, 280, 420, 560, 700, 840, 1120] : [0, 30, 70, 120, 240, 400]) {
      await page.waitForFunction(time => performance.now() - window.rollStart >= time, time, { polling: 'raf' })
      await capture(time === 0 ? 'first' : 'sample')
    }
    await page.locator('.dice-tray[data-phase="settled"]').waitFor()
    await page.waitForFunction(() => !document.querySelector('.dice-tray').getAnimations({ subtree: true }).some(a => a.playState === 'running'))
    await capture('settled')
    const video = page.video()
    await context.close()
    await video.saveAs(path.join(root, 'dist/verification/dice', `dice-${mode}.webm`))
    const canvasPage = await browser.newPage()
    const dataURLs = frames.map(frame => 'data:image/png;base64,' + fs.readFileSync(path.join(out, frame.image)).toString('base64'))
    const filmstrip = await canvasPage.evaluate(async sources => {
      const images = await Promise.all(sources.map(src => new Promise(resolve => { const image = new Image(); image.onload = () => resolve(image); image.src = src })))
      const canvas = document.createElement('canvas'), columns = 3
      canvas.width = images[0].width * columns; canvas.height = images[0].height * Math.ceil(images.length / columns)
      const ctx = canvas.getContext('2d')
      ctx.fillStyle = '#111615'; ctx.fillRect(0, 0, canvas.width, canvas.height)
      images.forEach((image, i) => ctx.drawImage(image, i % columns * image.width, Math.floor(i / columns) * image.height))
      return canvas.toDataURL('image/png').split(',')[1]
    }, dataURLs)
    await canvasPage.close()
    fs.writeFileSync(path.join(out, 'filmstrip.png'), Buffer.from(filmstrip, 'base64'))
    fs.writeFileSync(path.join(out, 'report.json'), JSON.stringify({
      mode, viewport: { width: 1000, height: 760 }, region: box, frames,
      capture: 'Headless, real elapsed time; CSS and JavaScript clocks run normally',
      limitations: ['Screenshots are sampled and take time to encode; recorded elapsed times are actual, not target timestamps.', 'This is visual evidence on one local device, not a cross-device performance guarantee.'],
    }, null, 2))
    console.log(`Captured ${mode}: ${frames.length} frames and real-time WebM`)
  }
  const pendingChecks = []
  for (const light of [false, true]) {
    const context = await browser.newContext({ viewport: { width: 1000, height: 760 } })
    if (light) await context.addInitScript(() => localStorage.setItem('linchat-appearance-v1', JSON.stringify({ version: 1, accent: '#24529b', background: '#111615', panel: '#f3f1eb' })))
    const page = await context.newPage()
    await page.goto('http://127.0.0.1:5187/?motion-demo#campaign=1')
    await page.getByRole('button', { name: '掷骰', exact: true }).click()
    let release
    const gate = new Promise(resolve => { release = resolve })
    await page.route('**/api/v1/campaigns/1/rolls', async route => { await gate; await route.continue() }, { times: 1 })
    await page.getByRole('button', { name: '掷骰并记录', exact: true }).click()
    await page.locator('.dice-tray[data-phase=waiting]').waitFor()
    const color = await page.locator('.dice-tray').evaluate(tray => {
      const label = tray.querySelector('.dice-outcome b')
      const probe = document.createElement('i'); probe.style.color = 'var(--warning)'; tray.append(probe)
      const expected = getComputedStyle(probe).color; probe.remove()
      const actual = getComputedStyle(label).color, bg = getComputedStyle(tray).backgroundColor
      const luminance = color => color.match(/\d+(?:\.\d+)?/g).slice(0, 3).map(n => Number(n) / 255).map(n => n <= .04045 ? n / 12.92 : ((n + .055) / 1.055) ** 2.4).reduce((sum, n, i) => sum + n * [.2126, .7152, .0722][i], 0)
      const a = luminance(actual), b = luminance(bg)
      return { actual, expected, bg, contrast: (Math.max(a, b) + .05) / (Math.min(a, b) + .05), total: tray.querySelector('.dice-total strong').textContent }
    })
    assert.equal(color.actual, color.expected)
    assert.equal(color.total, '—')
    assert.ok(color.contrast >= 4.5)
    const name = light ? 'waiting-light' : 'waiting-dark'
    await page.screenshot({ path: path.join(root, 'dist/verification/dice', name + '.png') })
    pendingChecks.push({ name, ...color, passed: true })
    release(); await page.locator('.dice-tray[data-phase=settled]').waitFor()
    await context.close()
  }
  fs.writeFileSync(path.join(root, 'dist/verification/dice/waiting-palette-results.json'), JSON.stringify({ passed: true, pendingChecks }, null, 2))
  console.log('PASS Waiting states use panel-scoped amber with readable contrast on dark and light palettes')
})().catch(error => { console.error(error); process.exitCode = 1 }).finally(async () => { await browser?.close() })
