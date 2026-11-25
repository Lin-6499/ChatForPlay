const { fixture } = require('./entertainment-fixture.cjs')
const { chromium } = require('playwright-core')
const fs = require('node:fs')
const path = require('node:path')
const assert = require('node:assert/strict')
const { randomUUID } = require('node:crypto')
;(async () => {
  const f = await fixture(), out = path.resolve('dist/verification/memory')
  fs.mkdirSync(out, { recursive: true })
  const results = [], errors = []
  const browser = await chromium.launch({ executablePath: ['C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe', 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe'].find(p => fs.existsSync(p)), headless: true })
  const pass = s => { results.push(s); console.log('PASS', s) }
  try {
    async function save(entry) {
      return (await f.api(f.route('lore'), 'POST', { client_id: randomUUID(), version: (await f.state()).version,
        entry: { id: 0, version: 0, status: 'active', in_context: false, visibility: 'public', ...entry } }, f.owner.cookie)).data.entry
    }
    const chapter = await save({ kind: 'chapter', title: '第一幕：钟楼', content: '本幕调查钟楼与钥匙。' })
    await save({ kind: 'quest', title: '寻找钥匙', content: '调查守夜人手中的钥匙。', chapter_id: chapter.id, progress: 'in_progress' })
    await save({ kind: 'clue', title: 'SECRET-未揭露身份', content: '守夜人的幕后身份。', visibility: 'dm', chapter_id: chapter.id, progress: 'undiscovered' })
    await save({ kind: 'fact', title: '另一幕的资料', content: '远方山城的约定。', in_context: true })
    await f.api(f.route('state'), 'PATCH', { client_id: randomUUID(), version: (await f.state()).version, mode: 'external', phase: 'awaiting_action' }, f.owner.cookie)
    const agent = (await f.api(f.route('agent-tokens'), 'POST', { label: 'memory-check', secrets: false }, f.owner.cookie)).data
    async function host(route, body) {
      const response = await fetch(f.url + '/api' + f.route(route), { method: 'POST', headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${agent.token}`, 'X-LinChat': '1' }, body: JSON.stringify(body) })
      const result = await response.json(); assert.equal(response.status, 200, JSON.stringify(result)); return result
    }
    await f.api(f.route('actions'), 'POST', { client_id: randomUUID(), content: '我询问守夜人钥匙的下落。' }, f.player.cookie)
    const { turn } = await host('turns/claim', {})
    await host(`turns/${turn.id}/complete`, { client_id: randomUUID(), version: turn.base_version, lease: turn.lease,
      outcome: { narration: '守夜人交出钥匙，并提醒旅人在日落前归还。', phase: 'awaiting_action', changes: [],
        memory_suggestions: [{ kind: 'fact', title: '日落前归还钥匙', content: '旅人得到了钥匙，并被提醒在日落前归还。' }, { kind: 'npc', title: '待核对的人物变化', content: '请房主核对守夜人接下来的安排。' }] } })
    assert.equal((await f.api(f.route('lore'), 'GET', undefined, f.owner.cookie)).data.length, 4)
    const context = await browser.newContext({ viewport: { width: 1264, height: 900 } })
    await context.addCookies([{ name: 'linchat_session', value: f.owner.cookie.split('=')[1], url: f.url }])
    const page = await context.newPage(); page.on('pageerror', e => errors.push(e.message))
    await page.goto(f.url + `/#campaign=${f.cid}`)
    await page.getByRole('button', { name: '剧本与记忆', exact: true }).click()
    await page.getByLabel('本幕章节').selectOption(String(chapter.id))
    await page.getByRole('button', { name: '使用本幕资料', exact: true }).click()
    await page.getByText('本幕公开参考资料已切换', { exact: false }).waitFor()
    const model = (await f.api(f.route('context'), 'GET', undefined, f.owner.cookie)).data
    assert.equal(model.lore.entries.length, 2)
    assert.ok(model.lore.entries.every(e => e.id === chapter.id || e.chapter_id === chapter.id))
    assert.ok(!JSON.stringify(model).includes('SECRET-未揭露身份'))
    pass('Selecting a chapter atomically chooses its public active references and excludes secrets and unrelated lore')
    await page.getByRole('button', { name: /寻找钥匙/, exact: false }).click()
    assert.equal(await page.getByLabel('剧情进度').inputValue(), 'in_progress')
    await page.getByLabel('剧情进度').selectOption('completed')
    await page.getByRole('button', { name: '保存资料', exact: true }).click()
    await page.getByText('已保存到战役资料', { exact: true }).waitFor()
    pass('Quest progress saves independently from active/archive status')
    await page.locator('.lore-suggestions > summary').click()
    await page.getByText('待确认的记忆建议 · 2 条', { exact: true }).waitFor()
    const suggested = page.locator('.lore-suggestions article').filter({ has: page.getByText('日落前归还钥匙', { exact: true }) })
    await suggested.getByText('核对来源原文', { exact: true }).click()
    await suggested.getByText('守夜人交出钥匙，并提醒旅人在日落前归还。', { exact: true }).waitFor()
    for (const width of [1264, 390, 320]) {
      await page.setViewportSize({ width, height: 900 })
      await page.locator('.campaign-lore').evaluate(e => { e.scrollTop = 0 })
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
      await page.screenshot({ path: path.join(out, `suggestions-${width}.png`) })
    }
    await suggested.getByRole('button', { name: '核对并编辑', exact: true }).click()
    assert.equal(await page.getByLabel('资料标题').inputValue(), '日落前归还钥匙')
    assert.equal(await page.getByLabel('可见范围', { exact: true }).inputValue(), 'dm')
    await page.getByLabel('资料正文').fill('房主已核对：队伍持有钟楼钥匙，需要在日落前归还。')
    await page.getByLabel('可见范围', { exact: true }).selectOption('public')
    await page.getByLabel('所属章节').selectOption(String(chapter.id))
    await page.getByLabel('供主持参考（仅公开有效资料）').check()
    await page.getByRole('button', { name: '保存资料', exact: true }).click()
    await page.getByText('已保存到战役资料', { exact: true }).waitFor()
    await page.getByText('待确认的记忆建议 · 1 条', { exact: true }).waitFor()
    await page.getByRole('button', { name: '忽略此建议', exact: true }).click()
    await page.getByText('待确认的记忆建议 · 0 条', { exact: true }).waitFor()
    assert.equal((await f.api(f.route('lore'), 'GET', undefined, f.owner.cookie)).data.length, 5)
    pass('Suggestions expose their authoritative source, remain non-canonical until explicit edited approval, and can be rejected')
    await page.getByRole('button', { name: '冒险日志', exact: true }).click()
    await page.getByText('前情回顾与未解决事项', { exact: true }).click()
    await page.locator('.recap-content').getByText('房主已核对：队伍持有钟楼钥匙，需要在日落前归还。', { exact: true }).waitFor()
    assert.equal(await page.locator('.recap-content').getByText('寻找钥匙', { exact: true }).count(), 0)
    assert.equal(await page.locator('.recap-content').getByText('SECRET-未揭露身份', { exact: true }).count(), 0)
    await page.locator('.recap-content details summary').first().click()
    await page.screenshot({ path: path.join(out, 'recap-320.png') })
    pass('Recap uses saved facts and cited narration, excludes secrets and resolved objectives, and does not call a model')
    assert.deepEqual(errors, [])
    fs.writeFileSync(path.join(out, 'results.json'), JSON.stringify({ version: require('../package.json').version, results, errors }, null, 2))
  } finally { await browser.close(); await f.stop() }
})().catch(e => { console.error(e); process.exitCode = 1 })
