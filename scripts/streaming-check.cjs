const { fixture } = require('./entertainment-fixture.cjs')
const { chromium } = require('playwright-core')
const { spawn } = require('node:child_process')
const http = require('node:http')
const fs = require('node:fs')
const path = require('node:path')
const assert = require('node:assert/strict')
const { randomUUID } = require('node:crypto')

;(async () => {
  const f = await fixture(), results = [], errors = []
  const out = path.resolve('dist/verification/streaming'); fs.mkdirSync(out, { recursive: true })
  let release = () => {}, child, externalDone
  let label = '内置主持'
  const model = http.createServer(async (req, res) => {
    let raw = ''; for await (const b of req) raw += b
    const body = JSON.parse(raw)
    assert.equal(body.stream, true)
    res.writeHead(200, { 'Content-Type': 'text/event-stream' })
    const event = delta => res.write('data: ' + JSON.stringify({ choices: [{ index: 0, delta: { tool_calls: [delta] } }] }) + '\n\n')
    const gate = new Promise(resolve => { release = resolve })
    event({ index: 0, id: randomUUID(), function: { name: 'finish_turn', arguments: JSON.stringify({ narration: `${label}：门后传来轻轻的脚步声。\n\n守夜人提着灯，等待你们作出选择。` }).slice(0, -2) } })
    await gate
    event({ index: 0, function: { arguments: '","phase":"awaiting_action","changes":[]}' } })
    res.end('data: [DONE]\n\n')
  })
  await new Promise(resolve => model.listen(0, '127.0.0.1', resolve))
  const modelURL = `http://127.0.0.1:${model.address().port}/v1`
  const browser = await chromium.launch({ executablePath: ['C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe', 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe'].find(p => fs.existsSync(p)), headless: true })
  const pass = s => { results.push(s); console.log('PASS', s) }
  try {
    const context = await browser.newContext({ viewport: { width: 1264, height: 811 } })
    await context.addCookies([{ name: 'linchat_session', value: f.owner.cookie.split('=')[1], url: f.url }])
    const page = await context.newPage(); page.on('pageerror', e => errors.push(e.message))
    await page.goto(f.url + `/#campaign=${f.cid}`)
    await page.locator('.game-journal .game-dice-result').waitFor()
    await f.api('/v1/entertainment/ai', 'PUT', { base_url: modelURL, model: 'stream-fixture', api_key: 'fixture-key', enabled: true }, f.owner.cookie)
    for (const mode of ['builtin', 'external']) {
      label = mode === 'builtin' ? '内置主持' : '外部主持'
      await f.api(f.route('state'), 'PATCH', { client_id: randomUUID(), version: (await f.state()).version, mode, phase: 'awaiting_action' }, f.owner.cookie)
      await f.api(f.route('actions'), 'POST', { client_id: randomUUID(), content: `${label}流式验证` }, f.player.cookie)
      if (mode === 'external') {
        const agent = (await f.api(f.route('agent-tokens'), 'POST', { label: 'stream-check', secrets: false }, f.owner.cookie)).data
        child = spawn(process.execPath, [path.join(__dirname, 'campaign-agent.cjs'), '--once'], { windowsHide: true, stdio: ['ignore', 'pipe', 'pipe'],
          env: { ...process.env, LINCHAT_CAMPAIGN_URL: f.url + '/api' + f.route('').replace(/\/$/, ''), LINCHAT_AGENT_TOKEN: agent.token, MODEL_BASE_URL: modelURL, MODEL_ID: 'stream-fixture', MODEL_API_KEY: 'fixture-key' } })
        externalDone = new Promise(resolve => child.once('exit', resolve))
      }
      const live = page.getByLabel('主持人实时草稿')
      await live.getByText(`${label}：门后传来轻轻的脚步声。`, { exact: false }).waitFor()
      assert.equal((await f.state()).phase, 'narrating')
      assert.equal(await page.locator('.game-entry:not(.game-stream)').getByText(`${label}：门后传来`, { exact: false }).count(), 0)
      pass(`${mode}: live text appears while provider response is still open, before any canonical narration`)
      await page.reload()
      await page.getByLabel('主持人实时草稿').waitFor()
      pass(`${mode}: reload recovers the active draft`)
      for (const width of [1264, 390, 320]) {
        await page.setViewportSize({ width, height: 811 })
        await page.locator('.game-journal').evaluate(e => { e.scrollTop = e.scrollHeight })
        assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
        await page.screenshot({ path: path.join(out, `${mode}-${width}.png`) })
      }
      release()
      await page.locator('.game-entry:not(.game-stream)').getByText(`${label}：门后传来`, { exact: false }).waitFor()
      await live.waitFor({ state: 'hidden' })
      assert.equal(await page.locator('.game-entry:not(.game-stream)').getByText(`${label}：门后传来`, { exact: false }).count(), 1)
      if (externalDone) assert.equal(await externalDone, 0)
      pass(`${mode}: final completion replaces the draft with exactly one persisted narration`)
    }
    assert.deepEqual(errors, [])
    fs.writeFileSync(path.join(out, 'results.json'), JSON.stringify({ version: require('../package.json').version, results, errors }, null, 2))
  } finally {
    release(); if (child?.exitCode === null) child.kill()
    model.closeAllConnections(); model.close()
    await browser.close(); await f.stop()
  }
})().catch(e => { console.error(e); process.exitCode = 1 })
