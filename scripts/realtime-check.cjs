// Real isolated server + headless browser. Deliberately hold stale responses to
// reproduce notifications arriving while the entertainment view is refreshing.
const { fixture } = require('./entertainment-fixture.cjs')
const { chromium } = require('playwright-core')
const fs = require('node:fs')
const path = require('node:path')
const assert = require('node:assert/strict')
const { randomUUID } = require('node:crypto')
;(async () => {
  const f = await fixture(), out = path.resolve('dist/verification/realtime')
  fs.mkdirSync(out, { recursive: true })
  const browser = await chromium.launch({ executablePath: ['C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe', 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe'].find(p => fs.existsSync(p)), headless: true })
  const results = [], errors = []
  const pass = text => { results.push(text); console.log('PASS', text) }
  let release = () => {}
  try {
    const context = await browser.newContext({ viewport: { width: 1264, height: 811 } })
    await context.addCookies([{ name: 'linchat_session', value: f.owner.cookie.split('=')[1], url: f.url }])
    const page = await context.newPage()
    page.on('pageerror', e => errors.push(e.message))
    await page.goto(f.url + `/#campaign=${f.cid}`)
    await page.locator('.game-journal .game-dice-result').waitFor()
    const prompt = async text => f.api(f.route('messages'), 'POST', { client_id: randomUUID(), version: (await f.state()).version,
      visibility: 'public', content: '请选择：' + text, reply_prompt: { target_user_id: 0, options: [{ id: 'next', label: text, content: text }] } }, f.owner.cookie)
    await prompt('第一组选择')
    await page.getByRole('button', { name: '第一组选择', exact: true }).waitFor({ timeout: 2500 })
    pass('Quick replies arrive on a stationary page through the real WebSocket')
    let captured
    const hit = new Promise(resolve => { captured = resolve })
    const gate = new Promise(resolve => { release = resolve })
    let held = false
    await page.route('**/reply-prompt', async route => {
      const response = await route.fetch()
      if (!held) { held = true; captured(); await gate }
      await route.fulfill({ response })
    })
    await prompt('旧响应选择')
    await hit
    await prompt('刷新期间的新选择')
    // Let the WebSocket notification's 180ms refresh fire while the old request
    // is held. This is the dropped-refresh regression, not a fake provider delay.
    await page.waitForTimeout(350)
    release()
    await page.getByRole('button', { name: '刷新期间的新选择', exact: true }).waitFor({ timeout: 2500 })
    assert.equal(await page.getByRole('button', { name: '旧响应选择', exact: true }).count(), 0)
    await page.unroute('**/reply-prompt')
    pass('A notification during an in-flight refresh queues a trailing read, instead of retaining stale reply choices')

    await f.api(f.route('state'), 'PATCH', { client_id: randomUUID(), version: (await f.state()).version, mode: 'external', phase: 'awaiting_action' }, f.owner.cookie)
    const agent = (await f.api(f.route('agent-tokens'), 'POST', { label: 'realtime-fixture', secrets: false }, f.owner.cookie)).data
    async function host(route, body) {
      const response = await fetch(f.url + '/api' + f.route(route), { method: 'POST', headers: { Authorization: `Bearer ${agent.token}`, 'Content-Type': 'application/json', 'X-LinChat': '1' }, body: JSON.stringify(body) })
      const value = await response.json(); assert.equal(response.status, 200, JSON.stringify(value)); return value
    }
    await f.api(f.route('actions'), 'POST', { client_id: randomUUID(), content: '实时正文检查' }, f.player.cookie)
    const { turn } = await host('turns/claim', {})
    const stream = (seq, narration) => host(`turns/${turn.id}/stream`, { seq, narration, lease: turn.lease, version: turn.base_version })
    await stream(1, '第一段正在生成')
    await page.getByLabel('主持人实时草稿').getByText('第一段正在生成', { exact: true }).waitFor({ timeout: 2500 })
    await stream(2, '第一段正在生成。第二段立即更新。')
    await page.getByLabel('主持人实时草稿').getByText('第一段正在生成。第二段立即更新。', { exact: true }).waitFor({ timeout: 2500 })
    pass('Successive streaming snapshots update in place without leaving the page')
    await context.setOffline(true)
    await stream(3, '断线期间新增的正文')
    await context.setOffline(false)
    await page.getByLabel('主持人实时草稿').getByText('断线期间新增的正文', { exact: true }).waitFor({ timeout: 10000 })
    pass('Reconnect restores a snapshot that arrived while the browser was offline')
    await host(`turns/${turn.id}/complete`, { client_id: randomUUID(), version: turn.base_version, lease: turn.lease,
      outcome: { narration: '实时生成已完成', phase: 'awaiting_action', changes: [], reply_prompt: { target_user_id: 0, options: [{ id: 'answer', label: '完成后的新选择', content: '回答主持' }] } } })
    await page.getByRole('button', { name: '完成后的新选择', exact: true }).waitFor({ timeout: 4000 })
    await page.getByLabel('主持人实时草稿').waitFor({ state: 'hidden' })
    pass('Completion removes the live draft and immediately shows the new reply options')
    await page.getByLabel('行动或叙述内容').fill('未发送的行动草稿')
    await page.getByRole('button', { name: '群聊', exact: true }).click()
    await page.getByRole('button', { name: '娱乐模式', exact: true }).click()
    assert.equal(await page.getByLabel('行动或叙述内容').inputValue(), '未发送的行动草稿')
    await prompt('切回后仍实时更新')
    await page.getByRole('button', { name: '切回后仍实时更新', exact: true }).waitFor({ timeout: 2500 })
    pass('KeepAlive deactivation/reactivation preserves drafts and does not disable subsequent live updates')
    assert.deepEqual(errors, [])
    fs.writeFileSync(path.join(out, 'results.json'), JSON.stringify({ version: require('../package.json').version, results, errors }, null, 2))
  } finally {
    release(); await browser.close(); await f.stop()
  }
})().catch(e => { console.error(e); process.exitCode = 1 })
