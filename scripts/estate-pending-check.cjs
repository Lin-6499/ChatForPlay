// Verify in-flight receipts do not masquerade as failed/uncertain requests.
const { chromium } = require('playwright-core')
const { fixture } = require('./entertainment-fixture.cjs')
const { randomUUID } = require('node:crypto')
const fs = require('node:fs')
const path = require('node:path')
const assert = require('node:assert/strict')

async function main() {
  const f = await fixture()
  let browser
  const out = path.resolve('dist/verification/estate-pending')
  fs.mkdirSync(out, { recursive: true })
  const results = [], errors = []
  try {
    const room = (await f.api('/v1/estate/rooms', 'POST', {
      client_id: randomUUID(), title: '准备状态验收', capacity: 2,
    }, f.owner.cookie)).data.game
    const executablePath = ['C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe', 'C:/Program Files/Google/Chrome/Application/chrome.exe'].find(p => fs.existsSync(p))
    browser = await chromium.launch({ executablePath, headless: true })
    const context = await browser.newContext({ viewport: { width: 820, height: 738 } })
    await context.addCookies([{ name: 'linchat_session', value: f.owner.cookie.split('=')[1], url: f.url }])
    const page = await context.newPage()
    page.on('pageerror', e => errors.push(e.message))
    await page.goto(f.url + `/#estate=${room.id}`)
    const ready = page.getByRole('button', { name: '准备', exact: true })
    await ready.waitFor()
    const before = await page.getByRole('heading', { name: '等大家准备好，就出发' }).boundingBox()
    await page.evaluate(() => {
      window.estateAlerts = 0
      new MutationObserver(records => {
        for (const record of records) for (const node of record.addedNodes) {
          if (node.nodeType === 1 && (node.matches('.estate-shell > .estate-error') || node.querySelector('.estate-shell > .estate-error'))) window.estateAlerts++
        }
      }).observe(document.querySelector('.estate-shell'), { subtree: true, childList: true })
    })
    // Hold the response open, not a fixed timeout: test the exact pending state.
    let release, reached
    const held = new Promise(r => { release = r })
    const started = new Promise(r => { reached = r })
    const route = `**/api/v1/estate/rooms/${room.id}/ready`
    await page.route(route, async request => {
      const response = await request.fetch()
      reached()
      await held
      await request.fulfill({ response })
    }, { times: 1 })
    await ready.click()
    await started
    const sending = page.getByRole('button', { name: '正在提交…', exact: true })
    await sending.waitFor()
    assert.equal(await sending.isDisabled(), true)
    assert.equal(await page.locator('.estate-shell > .estate-error').count(), 0)
    const during = await page.getByRole('heading', { name: '等大家准备好，就出发' }).boundingBox()
    assert.equal(during.y, before.y, 'request inserted a layout-shifting banner')
    await page.screenshot({ path: path.join(out, 'sending.png') })
    release()
    await page.getByRole('button', { name: '取消准备', exact: true }).waitFor()
    assert.equal(await page.evaluate(() => window.estateAlerts), 0, 'transient error banner was inserted')
    results.push('Normal ready request stays inline while delayed and never inserts an error banner or shifts content')

    // Commit cancellation, drop the receipt, then retry it unchanged.
    let lostID, retriedID
    await page.route(route, async request => {
      lostID = request.request().postDataJSON().client_id
      await request.fetch()
      await request.abort('failed')
    }, { times: 1 })
    await page.getByRole('button', { name: '取消准备', exact: true }).click()
    const alert = page.locator('.estate-shell > .estate-error')
    await alert.filter({ hasText: '操作结果待确认' }).waitFor()
    await page.screenshot({ path: path.join(out, 'uncertain.png') })
    const current = (await f.api(`/v1/estate/matches/${room.id}`, 'GET', undefined, f.owner.cookie)).data.game
    assert.equal(current.players[0].ready, false)
    await page.reload()
    await alert.filter({ hasText: '有一笔操作结果尚未确认' }).waitFor()
    await page.route(route, async request => {
      retriedID = request.request().postDataJSON().client_id
      await request.continue()
    }, { times: 1 })
    await alert.getByRole('button', { name: '重试', exact: true }).click()
    await page.waitForFunction(() => !document.querySelector('.estate-shell > .estate-error') && !document.querySelector('[aria-busy="true"]'))
    assert.equal(lostID, retriedID)
    const after = (await f.api(`/v1/estate/matches/${room.id}`, 'GET', undefined, f.owner.cookie)).data.game
    assert.equal(after.version, current.version)
    assert.equal(after.players[0].ready, false)
    results.push('Lost receipt remains visibly recoverable after reload; retry uses original ID and never toggles readiness twice')
    assert.deepEqual(errors, [])
    fs.writeFileSync(path.join(out, 'results.json'), JSON.stringify({ results, errors }, null, 2))
    for (const result of results) console.log('PASS', result)
  } finally { if (browser) await browser.close(); await f.stop() }
}
main().catch(e => { console.error(e); process.exitCode = 1 })
