// Deterministic component-level motion regression; real Vue/CSS, no accounts.
// Vite runs in-process and Chromium is always headless.
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const { pathToFileURL } = require('node:url')
const { chromium } = require('playwright-core')
const { spawn } = require('node:child_process')

async function main() {
  const { createServer } = await import(pathToFileURL(path.resolve('web/node_modules/vite/dist/node/index.js')).href)
  const output = path.resolve('dist/verification/estate-motion')
  fs.mkdirSync(output, { recursive: true })
  const html = `<!doctype html><html><head><meta charset="UTF-8"></head><body>
  <div id="app"></div><script type="module">
  import { createApp, ref, h } from 'vue'
  import Board from '/src/EstateBoard.vue'
  import '/src/style.css'
  import '/src/estate.css'
  import { applyAppearance, DEFAULT_APPEARANCE } from '/@fs/${path.resolve('desktop/theme.mjs').replaceAll('\\', '/')}'
  applyAppearance(DEFAULT_APPEARANCE)
  const names = ['起点','晨光巷','机会','钟楼街','城市税','河畔市场','休息广场','星桥路','北站','银杏大道','机会','百货广场','检查中心','创客街','公共维护费','致远港','南站','未来园','临时检查','海风大道','机会','灯塔湾','市集','云顶酒店']
  const seed = () => ({ game: { id: 1, dice: [4,4], events: [], properties: names.map(() => ({ owner: 0, level: 1, mortgaged: false })), players: [{ id: 1, name: '移动棋子', token: 0, position: 22, bankrupt: false }, { id: 2, name: '等待棋子', token: 1, position: 0, bankrupt: false }] }, tiles: names.map((name,id) => ({ id,name,group:'',price:0,kind:'start' })), rents: names.map(() => 0) })
  createApp({ components: { Board }, setup() {
    const view = ref(seed()), visible = ref(true)
    window.snapshot = () => structuredClone(JSON.parse(JSON.stringify(view.value)))
    window.update = data => { view.value = data }
    window.move = () => { const next = window.snapshot(); next.game.players[0].position = 6; next.game.events.push({ seq: next.game.events.length + 1, text: '移动棋子掷出4＋4。' }); view.value = next }
    window.reset = () => { view.value = seed() }
    setInterval(() => { view.value = window.snapshot() }, 100)
    return () => h('section', { class: 'estate-shell', style: 'padding:24px' }, [
      h('button', { id: 'move', onClick: window.move }, '掷骰 · 八步'),
      visible.value ? h(Board, { view: view.value, selected: 6 }, () => [h('h2', '逐格跳跃验收'), h('p', '4＋4 · 从22号经过起点至6号')]) : null
    ])
  } }).mount('#app')
  </script></body></html>`
  const server = await createServer({
    root: path.resolve('web'), server: { host: '127.0.0.1', port: 0 },
    plugins: [{ name: 'estate-motion-fixture',
      resolveId(id) { if (id === '/__estate-motion.js') return id },
      load(id) { if (id === '/__estate-motion.js') return html.match(/<script type="module">([\s\S]*?)<\/script>/)[1] },
      configureServer(server) {
        server.middlewares.use((req, res, next) => {
          if (req.url !== '/__estate-motion') return next()
          res.setHeader('Content-Type', 'text/html; charset=utf-8')
          res.end(html.replace(/<script type="module">[\s\S]*?<\/script>/, '<script type="module" src="/__estate-motion.js"></script>'))
        })
      },
    }],
  })
  await server.listen()
  const url = `http://127.0.0.1:${server.httpServer.address().port}/__estate-motion`
  let browser
  try {
    const executablePath = ['C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe', 'C:/Program Files/Google/Chrome/Application/chrome.exe'].find(p => fs.existsSync(p))
    browser = await chromium.launch({ executablePath, headless: true })
    const page = await browser.newPage({ viewport: { width: 1000, height: 900 } })
    const errors = []
    page.on('pageerror', e => { errors.push(e.message); console.error(e.message) })
    page.on('console', message => { if (message.type() === 'error') console.error(message.text()) })
    await page.goto(url)
    await page.locator('[data-pawn="1"]').waitFor()
    const results = []
    const position = () => page.locator('[data-pawn="1"]').evaluate(el => Number(el.closest('[data-tile]').dataset.tile))
    await page.evaluate(() => {
      window.trace = []
      window.start = performance.now()
      const sample = () => {
        const el = document.querySelector('[data-pawn="1"]')
        const tile = Number(el.closest('[data-tile]').dataset.tile)
        if (window.trace.at(-1)?.tile !== tile) window.trace.push({ tile, time: performance.now() - window.start })
        if (performance.now() - window.start < 3800) requestAnimationFrame(sample)
      }
      sample(); window.move()
    })
    await page.waitForTimeout(450)
    assert.equal(await position(), 22, 'pawn waits for the dice before moving')
    await page.waitForTimeout(500)
    assert.equal(await position(), 23)
    assert.ok(await page.locator('[data-pawn="1"]').evaluate(el => el.getAnimations().length > 0), 'hop uses a rendered transform, not just tile replacement')
    await page.screenshot({ path: path.join(output, 'normal-mid-hop.png') })
    await page.waitForTimeout(2700)
    assert.equal(await position(), 6)
    const trace = await page.evaluate(() => window.trace)
    assert.deepEqual(trace.map(s => s.tile), [22,23,0,1,2,3,4,5,6], 'every tile, including the corner, is visible despite 100ms snapshot replacement')
    for (let i = 2; i < trace.length; i++) assert.ok(trace[i].time - trace[i-1].time > 240, 'readable per-tile dwell')
    results.push('800ms dice lead-in, 300ms visible arc per tile, eight hops through a corner survive 100ms polling')
    await page.setViewportSize({ width: 390, height: 844 })
    await page.evaluate(() => window.reset())
    await page.waitForTimeout(100)
    await page.evaluate(() => window.move())
    await page.waitForTimeout(970)
    await page.screenshot({ path: path.join(output, 'mobile-mid-hop.png') })
    assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
    // A different authoritative location interrupts safely; stale timers cannot win.
    await page.evaluate(() => { const next = window.snapshot(); next.game.players[0].position = 12; window.update(next) })
    await page.waitForTimeout(2800)
    assert.equal(await position(), 12)
    results.push('Mobile hop remains in the board; an authoritative relocation cancels stale hops')
    await page.emulateMedia({ reducedMotion: 'reduce' })
    await page.evaluate(() => window.reset())
    await page.waitForTimeout(100)
    await page.evaluate(() => window.move())
    await page.waitForTimeout(50)
    assert.equal(await position(), 6)
    assert.equal(await page.locator('[data-pawn="1"]').evaluate(el => el.getAnimations().length), 0)
    await page.screenshot({ path: path.join(output, 'reduced-settled.png') })
    results.push('Reduced motion presents the final position immediately, without spatial animation')
    assert.deepEqual(errors, [])
    // Supplemental real-time capture: the skill recorder replays individual
    // WAAPI effects, which cannot preserve the wall-clock timer hop sequence.
    for (const reduced of [false, true]) {
      await page.setViewportSize({ width: 1000, height: 900 })
      await page.emulateMedia({ reducedMotion: reduced ? 'reduce' : 'no-preference' })
      await page.evaluate(() => window.reset())
      await page.waitForTimeout(100)
      const dir = path.join(output, reduced ? 'realtime-reduced' : 'realtime-normal')
      fs.mkdirSync(dir, { recursive: true })
      const images = [], frames = [], start = Date.now()
      await page.evaluate(() => window.move())
      for (const time of [0, 750, 850, 950, 1050, 1150, 1250, 1350, 1450, 1750, 2050, 2350, 2650, 2950, 3250]) {
        await page.waitForTimeout(Math.max(0, start + time - Date.now()))
        const image = await page.locator('.estate-board').screenshot({ path: path.join(dir, `frame-${time}.png`) })
        images.push(image)
        frames.push({ at: Date.now() - start, tile: await position(), box: await page.locator('[data-pawn="1"]').boundingBox() })
      }
      fs.writeFileSync(path.join(dir, 'report.json'), JSON.stringify({ reducedMotion: reduced, frames, limitation: 'Isolated actual component; no full multiplayer server or low-end hardware performance measurement.' }, null, 2))
      const strip = await browser.newPage({ viewport: { width: 1908, height: 3180 } })
      await strip.setContent('<body style="margin:0;display:grid;grid-template-columns:repeat(3,636px)">' + images.map(image => `<img style="width:636px;display:block" src="data:image/png;base64,${image.toString('base64')}">`).join(''))
      await strip.locator('img').last().waitFor()
      await strip.screenshot({ path: path.join(dir, 'filmstrip.png'), fullPage: true })
      await strip.close()
    }
    fs.writeFileSync(path.join(output, 'results.json'), JSON.stringify({ results, trace, errors }, null, 2))
    for (const result of results) console.log('PASS', result)
    if (process.argv.includes('--capture')) {
      const launcher = path.join(process.env.USERPROFILE, '.config/opencode/skills/impeccable/scripts/impeccable.cmd')
      const steps = path.join(output, 'steps.json')
      fs.writeFileSync(steps, JSON.stringify([{ click: '#move' }, { wait: 1800 }, { wait: 1800 }]))
      for (const reduced of [false, true]) {
        const args = ['motion-capture', url, '--steps', steps, '--region', '.estate-board', '--frames', '9', '--out-dir', path.join(output, reduced ? 'reduced' : 'normal'), ...(reduced ? ['--reduced-motion'] : [])]
        await new Promise((resolve, reject) => {
          const quote = value => "'" + value.replaceAll("'", "''") + "'"
          const command = '& ' + quote(launcher) + ' ' + args.map(quote).join(' ')
          const child = spawn('powershell.exe', ['-NoProfile', '-NonInteractive', '-Command', command], { windowsHide: true, stdio: 'inherit' })
          child.on('error', reject)
          child.on('exit', code => code === 0 ? resolve() : reject(Error('Motion capture exited ' + code)))
        })
      }
    }
  } finally { if (browser) await browser.close(); await server.close() }
}
main().catch(e => { console.error(e); process.exitCode = 1 })
