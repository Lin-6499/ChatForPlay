const { fixture } = require('./entertainment-fixture.cjs')
const { chromium } = require('playwright-core')
const { spawn } = require('node:child_process')
const fs = require('node:fs')
const path = require('node:path')
const assert = require('node:assert/strict')
const { randomUUID } = require('node:crypto')
;(async () => {
  const f = await fixture(), out = path.resolve('dist/verification/host-integration'), results = [], errors = []
  fs.mkdirSync(out, { recursive: true })
  const browser = await chromium.launch({ executablePath: ['C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe', 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe'].find(p => fs.existsSync(p)), headless: true })
  const pass = s => { results.push(s); console.log('PASS', s) }
  try {
    await f.api(f.route('state'), 'PATCH', { client_id: randomUUID(), version: (await f.state()).version, mode: 'external', phase: 'awaiting_action' }, f.owner.cookie)
    const token = (await f.api(f.route('agent-tokens'), 'POST', { label: 'integration-check', secrets: false }, f.owner.cookie)).data
    const child = spawn(process.execPath, [path.join(__dirname, 'campaign-agent.cjs'), '--check'], {
      windowsHide: true, stdio: ['ignore', 'pipe', 'pipe'],
      env: { ...process.env, LINCHAT_CAMPAIGN_URL: f.url + '/api' + f.route('').replace(/\/$/, ''), LINCHAT_AGENT_TOKEN: token.token, MODEL_ID: '', MODEL_API_KEY: '' },
    })
    let output = ''; child.stdout.on('data', b => { output += b }); child.stderr.on('data', b => { output += b })
    const exit = await new Promise((resolve, reject) => {
      const timer = setTimeout(() => { child.kill(); reject(Error('Self-check timed out')) }, 20000)
      child.once('exit', code => { clearTimeout(timer); resolve(code) })
    })
    assert.equal(exit, 0, output); assert.match(output, /capability check passed/); assert.ok(!output.includes(token.token))
    assert.equal((await f.api(f.route('turns'), 'GET', undefined, f.owner.cookie)).data.length, 0)
    pass('Bot --check validates capabilities without model credentials, claiming a turn or exposing its token')
    const context = await browser.newContext({ viewport: { width: 1264, height: 900 } })
    await context.addCookies([{ name: 'linchat_session', value: f.owner.cookie.split('=')[1], url: f.url }])
    const page = await context.newPage(); page.on('pageerror', e => errors.push(e.message))
    await page.goto(f.url + `/#campaign=${f.cid}`)
    await page.getByRole('button', { name: '智能体接入', exact: true }).click()
    await page.getByLabel('主持接入自检').getByText('服务器 ' + require('../package.json').version, { exact: false }).waitFor()
    await page.getByText('尚未检测到主持领取请求', { exact: false }).waitFor()
    pass('Owner sees version/protocol and actionable no-host guidance rather than a false online badge')
    async function host(route, body) {
      const response = await fetch(f.url + '/api' + f.route(route), { method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token.token}`, 'X-LinChat': '1' }, body: JSON.stringify(body) })
      const value = await response.json(); assert.equal(response.status, 200, JSON.stringify(value)); return value
    }
    await f.api(f.route('actions'), 'POST', { client_id: randomUUID(), content: '检查主持活动' }, f.player.cookie)
    const { turn } = await host('turns/claim', {})
    await host(`turns/${turn.id}/stream`, { lease: turn.lease, version: turn.base_version, seq: 1, narration: '主持正在检查门后的动静。' })
    await host(`turns/${turn.id}/fail`, { client_id: randomUUID(), lease: turn.lease })
    await page.getByRole('button', { name: '刷新接入检查', exact: true }).click()
    const diagnostics = (await f.api(f.route('host-diagnostics'), 'GET', undefined, f.owner.cookie)).data
    for (const key of ['contact_at', 'claim_at', 'stream_at', 'fail_at']) assert.ok(diagnostics[key] > 0)
    await page.getByText('尚未检测到主持领取请求', { exact: false }).waitFor({ state: 'hidden' })
    const text = await page.getByLabel('主持接入自检').innerText()
    assert.ok(!text.includes(token.token) && !text.includes(turn.lease))
    for (const width of [1264, 390, 320]) {
      await page.setViewportSize({ width, height: 900 })
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
      await page.screenshot({ path: path.join(out, `diagnostics-${width}.png`) })
    }
    pass('Claim/stream/failure timestamps update and remain readable at desktop and narrow widths without credentials')
    assert.deepEqual(errors, [])
    fs.writeFileSync(path.join(out, 'results.json'), JSON.stringify({ version: require('../package.json').version, results, errors }, null, 2))
  } finally { await browser.close(); await f.stop() }
})().catch(e => { console.error(e); process.exitCode = 1 })
