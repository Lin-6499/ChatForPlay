const { fixture } = require('./entertainment-fixture.cjs')
const { chromium } = require('playwright-core')
const fs = require('node:fs')
const path = require('node:path')
const assert = require('node:assert/strict')
const { randomUUID } = require('node:crypto')
;(async () => {
  const f = await fixture(), out = path.resolve('dist/verification/reliability'), results = [], errors = []
  fs.mkdirSync(out, { recursive: true })
  const browser = await chromium.launch({ executablePath: ['C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe', 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe'].find(p => fs.existsSync(p)), headless: true })
  const pass = text => { results.push(text); console.log('PASS', text) }
  try {
    await f.api(f.route('state'), 'PATCH', { client_id: randomUUID(), version: (await f.state()).version, mode: 'external', phase: 'awaiting_action' }, f.owner.cookie)
    await f.api(f.route('flow'), 'PUT', { client_id: randomUUID(), version: (await f.state()).version, mode: 'collect', participants: [1, 2] }, f.owner.cookie)
    await f.api(f.route('actions'), 'POST', { client_id: randomUUID(), content: '我观察钟楼。' }, f.owner.cookie)
    const other = (await f.api('/v1/campaigns', 'POST', { title: '独立战役草稿隔离', ruleset: '5e-2024', members: [], scene: '', house_rules: '' }, f.owner.cookie)).data
    const context = await browser.newContext({ viewport: { width: 1264, height: 900 } })
    await context.addCookies([{ name: 'linchat_session', value: f.owner.cookie.split('=')[1], url: f.url }])
    const page = await context.newPage(); page.on('pageerror', e => errors.push(e.message))
    await page.goto(f.url + `/#campaign=${f.cid}`)
    await page.locator('.campaign-flow > summary').click()
    await page.getByRole('button', { name: '补充这条行动', exact: true }).click()
    await page.getByLabel('行动补充').fill('尚未发送的补充：我同时留意脚印。')
    await page.getByRole('button', { name: '角色卡', exact: true }).click()
    await page.getByRole('button', { name: '冒险日志', exact: true }).click()
    await page.locator('.campaign-flow > summary').click()
    assert.match(await page.getByLabel('行动补充').inputValue(), /尚未发送/)
    await page.locator('.game-campaign').filter({ hasText: other.title }).click()
    await page.locator('.campaign-flow > summary').click()
    assert.equal(await page.getByLabel('行动补充').count(), 0)
    await page.locator('.game-campaign').filter({ hasText: '雾港来信' }).click()
    await page.locator('.campaign-flow > summary').click()
    assert.match(await page.getByLabel('行动补充').inputValue(), /尚未发送/)
    pass('Unsent supplements survive tab and campaign round trips without leaking into another campaign')
    await f.api(f.route('actions'), 'POST', { client_id: randomUUID(), content: '我调查石门。' }, f.player.cookie)
    const agent = (await f.api(f.route('agent-tokens'), 'POST', { label: 'reliability-ui', secrets: false }, f.owner.cookie)).data
    const claimed = await fetch(f.url + '/api' + f.route('turns/claim'), { method: 'POST', headers: { Authorization: `Bearer ${agent.token}`, 'Content-Type': 'application/json', 'X-LinChat': '1' }, body: '{}' })
    assert.equal(claimed.status, 200)
    await page.getByText('原行动已领取、处理或冻结', { exact: false }).waitFor()
    assert.equal(await page.getByRole('button', { name: '发送行动补充', exact: true }).isDisabled(), true)
    await page.getByLabel('行动或叙述内容').fill('原有的新行动草稿')
    const before = (await f.api(f.route('events'), 'GET', undefined, f.owner.cookie)).data.length
    await page.getByRole('button', { name: '转为新行动草稿', exact: true }).click()
    const draft = await page.getByLabel('行动或叙述内容').inputValue()
    assert.match(draft, /原有的新行动草稿/); assert.match(draft, /尚未发送/)
    assert.equal((await f.api(f.route('events'), 'GET', undefined, f.owner.cookie)).data.length, before)
    pass('Claimed input locks further supplements, preserves unsent words and transfers them only to an unsent new-action draft')
    const all = Array.from({ length: 145 }, (_, i) => ({ id: i + 1, event_id: 1, turn_id: 1,
      status: i > 0 && i <= 120 ? 'accepted' : 'pending', source_content: '测试用已保存叙述。',
      suggestion: { kind: 'fact', title: i === 0 ? '最早未审核建议' : `建议${i + 1}`, content: '分页读取验证。' } }))
    await page.route('**/memory-suggestions?*', async route => {
      const url = new URL(route.request().url()), status = url.searchParams.get('status') || 'all', before = Number(url.searchParams.get('before')) || Infinity
      const filtered = all.filter(s => (status === 'all' || s.status === status) && s.id < before).sort((a, b) => b.id - a.id)
      const items = filtered.slice(0, 20)
      await route.fulfill({ json: { items, total_pending: all.filter(s => s.status === 'pending').length,
        has_more: filtered.length > 20, next_before: filtered.length > 20 ? items.at(-1).id : 0, status } })
    })
    await page.getByRole('button', { name: '剧本与记忆', exact: true }).click()
    await page.locator('.lore-suggestions > summary').click()
    await page.getByText('待确认的记忆建议 · 25 条', { exact: true }).waitFor()
    assert.equal(await page.locator('.lore-suggestions article').count(), 20)
    await page.getByRole('button', { name: '下一页建议', exact: true }).click()
    await page.getByText('最早未审核建议', { exact: true }).waitFor()
    assert.equal(await page.locator('.lore-suggestions article').count(), 5)
    for (const width of [1264, 390, 320]) {
      await page.setViewportSize({ width, height: 900 })
      await page.locator('.campaign-lore').evaluate(e => { e.scrollTop = 0 })
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
      await page.screenshot({ path: path.join(out, `pending-${width}.png`) })
    }
    await page.getByLabel('记忆建议状态').selectOption('accepted')
    await page.locator('.lore-suggestions article small').filter({ hasText: '已采用' }).first().waitFor()
    assert.equal(await page.getByRole('button', { name: '核对并编辑', exact: true }).count(), 0)
    pass('Pending-first pagination reaches old outstanding reviews; processed history has a separate filter and no approval controls')
    assert.deepEqual(errors, [])
    fs.writeFileSync(path.join(out, 'results.json'), JSON.stringify({ version: require('../package.json').version, results, errors,
      paginationUI: 'deterministic paged fixtures; actual API filtering/cursors tested in Go' }, null, 2))
  } finally { await browser.close(); await f.stop() }
})().catch(e => { console.error(e); process.exitCode = 1 })
