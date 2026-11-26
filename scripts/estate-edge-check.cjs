const { chromium } = require('playwright-core')
const { fixture } = require('./entertainment-fixture.cjs')
const { spawnSync } = require('node:child_process')
const fs = require('node:fs')
const path = require('node:path')
const assert = require('node:assert/strict')

async function main() {
  const f = await fixture()
  let browser
  const out = path.resolve('dist/verification/estate')
  fs.mkdirSync(out, { recursive: true })
  const results = [], errors = []
  const pass = text => { results.push(text); console.log('PASS', text) }
  try {
    const seed = spawnSync('go', ['test', '-run', '^TestEstateUIFixtures$', '-count=1', '.'], {
      env: { ...process.env, LINCHAT_ESTATE_FIXTURE_DATA: f.data }, encoding: 'utf8', windowsHide: true, timeout: 120000,
    })
    assert.equal(seed.status, 0, seed.stderr + seed.stdout)
    const ids = JSON.parse(fs.readFileSync(path.join(f.data, 'estate-ui-fixtures.json'), 'utf8'))
    const executablePath = ['C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe', 'C:/Program Files/Google/Chrome/Application/chrome.exe'].find(p => fs.existsSync(p))
    browser = await chromium.launch({ executablePath, headless: true })
    const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, reducedMotion: 'reduce' })
    await context.addCookies([{ name: 'linchat_session', value: f.owner.cookie.split('=')[1], url: f.url }])
    const page = await context.newPage()
    page.on('pageerror', e => errors.push(e.message))
    const read = async id => (await f.api(`/v1/estate/matches/${id}`, 'GET', undefined, f.owner.cookie)).data
    for (const [scenario, tile, cost] of [['redeem110', 1, 110], ['redeem220', 13, 220]]) {
      const id = ids[scenario]
      await page.goto(f.url + `/#estate=${id}`)
      await page.reload()
      await page.locator(`[data-tile="${tile}"]`).click()
      const redeem = page.getByRole('button', { name: `赎回 · ${cost}`, exact: true })
      await redeem.waitFor()
      assert.equal(await redeem.isEnabled(), true)
      await page.screenshot({ path: path.join(out, `${scenario}.png`) })
      await redeem.click()
      await page.getByRole('button', { name: `抵押 · 获得${cost === 110 ? 100 : 200}`, exact: true }).waitFor()
      const state = await read(id)
      assert.equal(state.game.players[0].cash, 0)
      assert.equal(state.game.properties[tile].mortgaged, false)
    }
    pass('Cash exactly 110 or 220 enables redemption and deducts exactly the server-displayed quote')
    const id = ids.debt
    await page.goto(f.url + `/#estate=${id}`)
    await page.reload()
    await page.getByText('待支付 270 · 现金 10 · 还差 260', { exact: true }).waitFor()
    await page.locator('[data-tile="3"]').click()
    const detail = page.locator('.estate-detail')
    await detail.getByText('晨光巷租金 112 → 90；钟楼街租金 37 → 0；河畔市场租金 37 → 30', { exact: true }).waitFor()
    await page.screenshot({ path: path.join(out, 'debt-group-desktop.png') })
    const startLabel = await page.locator('[data-tile="0"]').getAttribute('aria-label')
    assert.ok(startLabel.includes('当前停留：林同学、小满'))
    assert.ok(!startLabel.includes('已破产玩家'))
    const pawns = page.locator('[data-tile="0"] .estate-cell-pawns .estate-token')
    assert.equal(await pawns.count(), 2)
    assert.deepEqual(await pawns.locator('text').allTextContents(), ['1', '2'])
    assert.equal(await pawns.locator('circle').count(), 2)
    assert.equal(await pawns.locator('path, rect').count(), 0)
    assert.deepEqual(await pawns.locator('circle').evaluateAll(nodes => nodes.map(n => getComputedStyle(n).fill)), ['rgb(37, 99, 235)', 'rgb(194, 65, 12)'])
    assert.deepEqual(await pawns.locator('text').evaluateAll(nodes => nodes.map(n => getComputedStyle(n).fill)), ['rgb(255, 255, 255)', 'rgb(255, 255, 255)'])
    assert.equal((await page.locator('[data-tile="3"] .estate-cell-owner').textContent()).trim(), '地主：1号')
    await page.locator('[data-tile="8"]').click()
    await detail.getByText('北站租金 100 → 0；南站租金 100 → 50', { exact: true }).waitFor()
    await page.screenshot({ path: path.join(out, 'debt-stations-desktop.png') })
    await page.setViewportSize({ width: 390, height: 844 })
    await page.getByLabel('当前回合操作').scrollIntoViewIfNeeded()
    assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
    await page.screenshot({ path: path.join(out, 'debt-mobile.png') })
    // Sell then mortgage the group tile. At 160 cash the debt is still pending.
    await page.setViewportSize({ width: 1440, height: 900 })
    await page.locator('[data-tile="1"]').click()
    await detail.getByText('晨光巷租金 112 → 37', { exact: true }).waitFor()
    await page.getByRole('button', { name: '拆一级 · 回收50', exact: true }).click()
    await page.getByRole('button', { name: '抵押 · 获得100', exact: true }).click()
    await page.locator('[data-tile="8"]').click()
    await page.getByRole('button', { name: '抵押 · 获得150', exact: true }).click()
    await page.getByText(/林同学支付270/).first().waitFor()
    const settled = await read(id)
    assert.equal(settled.game.debt, 0)
    assert.equal(settled.game.players[0].cash, 40)
    assert.equal(settled.game.players[1].cash, 2270)
    assert.equal(settled.game.properties[1].mortgaged, true)
    pass('Real debt actions disclose building, complete-group and paired-station rent changes, then settle once')
    pass('Accessible tile names include multiple authoritative occupants and exclude bankrupt players')
    pass('Uniform circular pawns contain distinct numbers; landowners use a separate explicit ownership label')
    // On the redeemed fixture, rolling must update the accessible position
    // immediately, independent of any pawn animation.
    await page.goto(f.url + `/#estate=${ids.redeem110}`)
    await page.reload()
    await page.getByRole('button', { name: '掷骰', exact: true }).click()
    await page.waitForFunction(() => !document.querySelector('.estate-roll'))
    const moved = await read(ids.redeem110)
    const position = moved.game.players[0].position
    assert.notEqual(position, 0)
    assert.ok((await page.locator(`[data-tile="${position}"]`).getAttribute('aria-label')).includes('林同学'))
    assert.ok(!(await page.locator('[data-tile="0"]').getAttribute('aria-label')).includes('当前停留：林同学'))
    pass('A moved player is exposed at the server position rather than an animation frame')
    await page.goto(f.url + `/#estate=${ids.crowded}`)
    await page.reload()
    await page.getByLabel('环城商业街棋盘').waitFor()
    await page.getByRole('button', { name: '查看小满的地产', exact: true }).click()
    await page.getByText('小满的地产 · 2', { exact: true }).waitFor()
    for (const size of [{ width: 1440, height: 900 }, { width: 1280, height: 720 }, { width: 820, height: 738 }, { width: 390, height: 844 }]) {
      await page.setViewportSize(size)
      await page.locator('[data-tile="1"]').scrollIntoViewIfNeeded()
      const tile = page.locator('[data-tile="1"]')
      assert.equal(await tile.locator('.estate-pawn').count(), 4)
      const info = await tile.locator('.estate-cell-info').boundingBox()
      const dock = await tile.locator('.estate-cell-pawns').boundingBox()
      assert.ok(info.y + info.height <= dock.y + .5, `text overlaps pawn dock at ${size.width}`)
      assert.ok(await tile.locator('strong').isVisible())
      assert.equal((await tile.locator('.estate-cell-owner').innerText()).trim(), '地主：2号')
      assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
      if (size.width < 960) {
        const boardBefore = await page.locator('.estate-map-area').evaluate(n => n.offsetTop)
        const toggle = page.getByRole('button', { name: /聊天与对局记录|收起聊天与记录/ })
        if ((await toggle.getAttribute('aria-expanded')) === 'false') await toggle.click()
        const boardAfter = await page.locator('.estate-map-area').evaluate(n => n.offsetTop)
        assert.equal(boardBefore, boardAfter, 'chat opening moved board')
      }
      await page.evaluate(() => {
        const shell = document.querySelector('.estate-shell')
        const board = document.querySelector('.estate-board')
        const action = document.querySelector('.estate-action')
        const inset = innerWidth < 960 ? action.getBoundingClientRect().height + 14 : 120
        shell.scrollTop += board.getBoundingClientRect().top - inset
        document.querySelector('.estate-board-scroll').scrollLeft = 0
      })
      await page.screenshot({ path: path.join(out, `ownership-${size.width}.png`) })
    }
    pass('Four numbered pawns have reserved space below readable property details; player selection lists holdings and compact chat does not move the board')
    assert.deepEqual(errors, [])
    fs.writeFileSync(path.join(out, 'edge-results.json'), JSON.stringify({ results, errors }, null, 2))
  } finally { if (browser) await browser.close(); await f.stop() }
}
main().catch(e => { console.error(e); process.exitCode = 1 })
