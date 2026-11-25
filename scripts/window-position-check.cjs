const { chromium } = require('playwright-core')
const { fixture } = require('./entertainment-fixture.cjs')
const fs = require('node:fs')
const path = require('node:path')
const assert = require('node:assert/strict')
const { randomUUID } = require('node:crypto')

async function main() {
  const f = await fixture()
  const executablePath = ['C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe',
    'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe'].find(p => fs.existsSync(p))
  const browser = await chromium.launch({ executablePath, headless: true })
  const output = path.resolve(__dirname, '../dist/verification/window-position')
  fs.mkdirSync(output, { recursive: true })
  const results = [], errors = []
  const pass = text => { results.push(text); console.log('PASS', text) }
  try {
    // Deterministic paginated history avoids rate-limiting fixture setup. The
    // real server still supplies authentication, campaigns, tabs and dialogs.
    const log = Array.from({ length: 125 }, (_, i) => ({
      id: 1000 + i, actor_id: 2, actor: '小满', kind: 'action', visibility: 'public',
      content: `阅读位置验证 ${i + 1}\n我停下来观察钟楼，保留这条记录作为下次继续阅读的位置。`,
      payload: {}, created_at: 1795580000000 + i * 60000,
    }))
    const second = (await f.api('/v1/campaigns', 'POST', {
      title: '另一场冒险', ruleset: '5e-2024', members: [2], scene: '第二处场景',
      house_rules: '另一组规则。\n'.repeat(100),
    }, f.owner.cookie)).data
    const context = await browser.newContext({ viewport: { width: 1264, height: 811 } })
    await context.route(`**/api/v1/campaigns/${f.cid}/events*`, route => {
      const before = Number(new URL(route.request().url()).searchParams.get('before') || Infinity)
      return route.fulfill({ json: log.filter(e => e.id < before).slice(-80) })
    })
    await context.addCookies([{ name: 'linchat_session', value: f.owner.cookie.split('=')[1], url: f.url }])
    let page = await context.newPage()
    const observe = p => p.on('pageerror', e => errors.push(e.message))
    observe(page)
    const journal = '.game-journal', panel = '.game-tab-content'
    async function scroll(selector, y) {
      await page.locator(selector).evaluate((el, y) => { el.scrollTop = y }, y)
      await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))))
      await page.waitForFunction(selector => {
        const el = document.querySelector(selector)
        if (!el.dataset.scrollKey) return true
        const value = JSON.parse(localStorage.getItem('linchat-scroll-v1:' + el.dataset.scrollKey) || 'null')
        return value && Math.abs(value.y - el.scrollTop) < 2
      }, selector)
      return page.locator(selector).evaluate(el => el.scrollTop)
    }
    async function anchor() {
      return page.locator(journal).evaluate(el => {
        const top = el.getBoundingClientRect().top
        const row = [...el.querySelectorAll('[data-event-id]')].find(row => row.getBoundingClientRect().bottom > top)
        return { id: row.dataset.eventId, offset: row.getBoundingClientRect().top - top }
      })
    }
    async function sameAnchor(saved) {
      await page.waitForFunction(({ saved }) => {
        const el = document.querySelector('.game-journal')
        const row = el?.querySelector(`[data-event-id="${saved.id}"]`)
        return row && Math.abs(row.getBoundingClientRect().top - el.getBoundingClientRect().top - saved.offset) < 2
      }, { saved })
    }
    await page.goto(f.url + `/#campaign=${f.cid}`)
    await page.locator(`${journal}[data-scroll-ready="true"]`).waitFor()
    await scroll(journal, 850)
    const saved = await anchor()
    await page.getByRole('button', { name: '角色卡', exact: true }).click()
    await page.getByRole('button', { name: '冒险日志', exact: true }).click()
    await sameAnchor(saved)
    await page.getByRole('button', { name: '群聊', exact: true }).click()
    await page.getByRole('button', { name: '娱乐模式', exact: true }).click()
    await sameAnchor(saved)
    await page.locator('.game-campaign').filter({ hasText: '另一场冒险' }).click()
    await page.locator('.game-campaign').filter({ hasText: '雾港来信' }).click()
    await sameAnchor(saved)
    pass('Journal anchor survives tab, chat and campaign round trips')

    // Readiness may become true before a browser scroll-reset event is
    // delivered. Preserve the saved anchor until the next restoration frame.
    await page.locator(journal).evaluate(async el => {
      el.dataset.scrollReady = 'false'
      await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)))
      el.scrollTop = 0
      el.dataset.scrollReady = 'true'
      el.dispatchEvent(new Event('scroll'))
    })
    await sameAnchor(saved)
    pass('A scroll-reset event between asynchronous readiness and restoration cannot overwrite the saved position')

    await scroll(journal, 0)
    await page.getByRole('button', { name: '读取更早的记录', exact: true }).click()
    await page.waitForFunction(() => document.querySelectorAll('.game-entry').length > 80)
    await scroll(journal, 280)
    const historical = await anchor()
    await page.reload()
    await sameAnchor(historical)
    await page.close()
    page = await context.newPage(); observe(page)
    await page.goto(f.url + '/#entertainment')
    await sameAnchor(historical)
    pass('Reload and page reopen recover the last campaign and an anchor older than the latest 80 records')

    await page.getByRole('button', { name: '主持台', exact: true }).click()
    const hostY = await scroll(panel, 240)
    assert.ok(hostY > 100)
    await page.getByRole('button', { name: '智能体接入', exact: true }).click()
    await scroll(panel, 120)
    await page.getByRole('button', { name: '主持台', exact: true }).click()
    await page.waitForFunction(y => Math.abs(document.querySelector('.game-tab-content').scrollTop - y) < 2, hostY)
    await page.reload()
    await page.waitForFunction(y => Math.abs(document.querySelector('.game-tab-content')?.scrollTop - y) < 2, hostY)
    pass('Each campaign tab retains independent coordinates and selected tab across reload')

    await page.setViewportSize({ width: 390, height: 780 })
    await page.getByRole('button', { name: '冒险日志', exact: true }).click()
    await sameAnchor(historical)
    await page.getByRole('button', { name: '返回战役列表', exact: true }).click()
    await page.locator('.game-campaign').filter({ hasText: '雾港来信' }).click()
    await sameAnchor(historical)
    await page.screenshot({ path: path.join(output, 'mobile-restored.png') })
    pass('Mobile hidden panels and viewport resizing preserve the same journal item and offset')

    await page.setViewportSize({ width: 820, height: 600 })
    await page.getByRole('button', { name: '模型设置', exact: true }).click()
    const dialogY = await scroll('.game-modal', 200)
    assert.ok(dialogY > 0)
    await page.getByRole('button', { name: '关闭娱乐模式弹窗' }).click()
    await page.getByRole('button', { name: '模型设置', exact: true }).click()
    await page.waitForFunction(y => Math.abs(document.querySelector('.game-modal').scrollTop - y) < 2, dialogY)
    await page.getByRole('button', { name: '关闭娱乐模式弹窗' }).click()
    await page.screenshot({ path: path.join(output, 'desktop-restored.png') })
    pass('Scrollable settings dialogs reopen at their saved position')

    await context.addCookies([{ name: 'linchat_session', value: f.player.cookie.split('=')[1], url: f.url }])
    await page.reload()
    await page.locator(`${journal}[data-scroll-ready="true"]`).waitFor()
    await page.waitForFunction(() => {
      const el = document.querySelector('.game-journal')
      return el.scrollHeight - el.scrollTop - el.clientHeight < 2
    })
    await scroll(journal, 650)
    await context.addCookies([{ name: 'linchat_session', value: f.owner.cookie.split('=')[1], url: f.url }])
    await page.reload()
    await sameAnchor(historical)
    pass('Accounts on the same server never share journal positions')

    log.push({ ...log.at(-1), id: 1125, content: '后来抵达的新记录' })
    await f.api(f.route('actions'), 'POST', { client_id: randomUUID(), content: '触发后台刷新' }, f.player.cookie)
    await page.getByText('后来抵达的新记录', { exact: true }).waitFor({ state: 'attached' })
    await sameAnchor(historical)
    pass('Background event refresh does not move a history reader')

    for (const resource of ['users', 'conversations']) {
      await context.route(`**/api/${resource}`, async route => {
        const response = await route.fetch(), data = await response.json()
        const extra = Array.from({ length: 60 }, (_, i) => resource === 'users'
          ? { id: 2000 + i, username: `fixture_${i}`, nickname: `验证成员 ${i}`, online: false }
          : { ...data[0], id: 2000 + i, kind: 'group', name: `验证会话 ${i}`, unread: 0, preview: '可滚动的会话列表' })
        await route.fulfill({ json: [...data, ...extra] })
      })
    }
    await page.reload()
    await page.getByRole('button', { name: '消息', exact: true }).click()
    const listY = await scroll('.conversation-list', 1050)
    await page.getByRole('button', { name: '娱乐模式', exact: true }).click()
    await page.getByRole('button', { name: '消息', exact: true }).click()
    await page.waitForFunction(y => Math.abs(document.querySelector('.conversation-list').scrollTop - y) < 2, listY)
    await page.getByRole('button', { name: '通讯录', exact: true }).click()
    const directoryY = await scroll('.member-picker', 550)
    await page.getByRole('button', { name: '关闭弹窗', exact: true }).click()
    await page.getByRole('button', { name: '通讯录', exact: true }).click()
    await page.waitForFunction(y => Math.abs(document.querySelector('.member-picker').scrollTop - y) < 2, directoryY)
    await page.getByRole('button', { name: '关闭弹窗', exact: true }).click()
    await page.getByRole('button', { name: '外观与配色', exact: true }).click()
    const appearanceY = await scroll('.appearance-scroll', 150)
    assert.ok(appearanceY > 0)
    await page.getByRole('button', { name: '关闭弹窗', exact: true }).click()
    await page.getByRole('button', { name: '外观与配色', exact: true }).click()
    await page.waitForFunction(y => Math.abs(document.querySelector('.appearance-scroll').scrollTop - y) < 2, appearanceY)
    pass('Conversation list, address-book picker and appearance editor each restore their own position')
    await context.addInitScript(() => {
      if (!location.pathname.startsWith('/desktop-test/')) return
      window.linchatDesktop = {
        getAppearance: async () => null,
        getState: async () => ({ version: 'fixture', adapters: [], host: { running: false },
          profiles: Array.from({ length: 30 }, (_, i) => ({ name: `保存的连接 ${i}`, url: `https://server${i}.example` })) }),
        getUpdateState: async () => ({ available: false }), onUpdateState: () => {}, onHostState: () => {},
      }
    })
    await context.route('**/desktop-test/**', route => {
      const relative = new URL(route.request().url()).pathname.split('/desktop-test/')[1]
      const base = path.resolve(__dirname, '../desktop'), file = path.resolve(base, relative)
      if (!file.startsWith(base + path.sep) || !fs.existsSync(file)) return route.fulfill({ status: 404 })
      const type = { '.html': 'text/html', '.js': 'text/javascript', '.mjs': 'text/javascript',
        '.css': 'text/css', '.png': 'image/png', '.svg': 'image/svg+xml' }[path.extname(file)]
      return route.fulfill({ body: fs.readFileSync(file), contentType: (type || 'application/octet-stream') + '; charset=utf-8' })
    })
    await page.goto(f.url + '/desktop-test/index.html')
    await page.locator('.profile').first().waitFor()
    const profilesY = await scroll('.profiles', 450)
    await page.reload()
    await page.waitForFunction(y => Math.abs(document.querySelector('.profiles')?.scrollTop - y) < 2, profilesY)
    await page.locator('#open-appearance').click()
    const launcherAppearance = await scroll('.appearance-scroll', 140)
    await page.locator('#close-appearance').click()
    await page.locator('#open-appearance').click()
    await page.waitForFunction(y => Math.abs(document.querySelector('.appearance-scroll')?.scrollTop - y) < 2, launcherAppearance)
    pass('Connection center saved-server list and appearance dialog restore independently')
    assert.deepEqual(errors, [])
    fs.writeFileSync(path.join(output, 'results.json'), JSON.stringify({ results, errors }, null, 2))
  } finally { await browser.close(); await f.stop() }
}
main().catch(error => { console.error(error); process.exitCode = 1 })
