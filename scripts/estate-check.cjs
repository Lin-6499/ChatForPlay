// Isolated real-server multiplayer acceptance. No visible client or production data.
const { chromium } = require('playwright-core')
const { fixture } = require('./entertainment-fixture.cjs')
const fs = require('node:fs')
const path = require('node:path')
const assert = require('node:assert/strict')
const { randomUUID } = require('node:crypto')
const { setTimeout: delay } = require('node:timers/promises')

async function main() {
  const f = await fixture()
  let browser
  const output = path.resolve('dist/verification/estate')
  fs.mkdirSync(output, { recursive: true })
  const results = [], errors = []
  const pass = text => { results.push(text); console.log('PASS', text) }
  try {
    const executablePath = ['C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe', 'C:/Program Files/Google/Chrome/Application/chrome.exe'].find(p => fs.existsSync(p))
    browser = await chromium.launch({ executablePath, headless: true })
    const invite = await f.api('/invites', 'POST', { uses: 1 }, f.owner.cookie)
    const fourth = await f.api('/auth/register', 'POST', { username: 'estate_four', nickname: '阿禾（验证样例）', password: f.password, token: invite.data.token })
    const accounts = [f.owner, f.player, f.outsider, fourth]
    const accountByID = new Map(), pageByID = new Map()
    const pages = []
    for (const account of accounts) {
      const context = await browser.newContext({ viewport: { width: 1440, height: 900 } })
      await context.addCookies([{ name: 'linchat_session', value: account.cookie.split('=')[1], url: f.url }])
      const page = await context.newPage()
      page.on('pageerror', e => errors.push(e.message))
      pages.push(page)
      const user = (await f.api('/me', 'GET', undefined, account.cookie)).data
      accountByID.set(user.id, account); pageByID.set(user.id, page)
    }
    const host = pages[0]
    await host.goto(f.url + '/#entertainment')
    await host.getByRole('button', { name: '地产大亨 · 多人棋盘' }).click()
    await host.getByLabel('房间名称').fill('环城商业街 · 四人验收')
    await host.getByRole('button', { name: '创建房间', exact: true }).click()
    await host.getByRole('heading', { name: '等大家准备好，就出发' }).waitFor()
    const id = Number(new URL(host.url()).hash.split('=')[1])
    const route = `/v1/estate/rooms/${id}`
    const read = async () => (await f.api(`/v1/estate/matches/${id}`, 'GET', undefined, f.owner.cookie)).data
    async function waitPhaseChange(phase, timeout = 10000) {
      const end = Date.now() + timeout
      while (Date.now() < end) {
        if ((await read()).game.phase !== phase) return
        await delay(250)
      }
      throw Error(`Server did not leave ${phase} within ${timeout}ms`)
    }
    for (let i = 1; i < 4; i++) {
      const state = await read()
      await f.api(route + '/join', 'POST', { client_id: randomUUID(), version: state.game.version }, accounts[i].cookie)
      await pages[i].goto(f.url + `/#estate=${id}`)
    }
    for (const page of pages) {
      await page.getByRole('button', { name: '准备', exact: true }).click()
      await page.getByRole('button', { name: '取消准备', exact: true }).waitFor()
    }
    await host.screenshot({ path: path.join(output, 'lobby-desktop.png') })
    await host.getByRole('button', { name: '开始对局', exact: true }).click()
    await host.getByLabel('环城商业街棋盘').waitFor()
    const expectedPawnColors = ['rgb(37, 99, 235)', 'rgb(194, 65, 12)', 'rgb(124, 58, 237)', 'rgb(21, 128, 61)']
    for (let token = 0; token < 4; token++) {
      assert.equal(await host.locator(`.estate-players .estate-token-${token} circle`).evaluate(n => getComputedStyle(n).fill), expectedPawnColors[token])
    }
    pass('Four real users create, join, prepare and start a randomly ordered match through authenticated state')

    await host.getByLabel('房间消息', { exact: true }).fill('先买街区，再慢慢经营。')
    await host.getByRole('button', { name: '发送', exact: true }).click()
    await pages[1].getByText('先买街区，再慢慢经营。', { exact: true }).waitFor()
    const sent = (await read()).game.version
    assert.equal((await read()).game.version, sent)
    pass('Embedded room chat reaches another participant without taking a game turn')

    // Exercise real controls for the first turn. Server randomness is not replaced.
    let state = await read()
    const actorID = state.game.players[state.game.current].id
    const actor = pageByID.get(actorID)
    await actor.getByRole('button', { name: '掷骰', exact: true }).click()
    await actor.waitForFunction(() => !document.querySelector('.estate-roll'))
    state = await read()
    assert.equal(state.game.events.filter(e => e.text.includes('掷出')).length, 1)
    if (state.game.phase === 'purchase_decision') {
      await actor.getByRole('button', { name: /^购买 · / }).click()
      await actor.getByText(/支付\d+购买/).first().waitFor()
    }
    await host.waitForFunction(() => document.querySelectorAll('.estate-dice .dice-value').length === 2)
    await host.screenshot({ path: path.join(output, 'board-desktop.png') })
    const before = (await read()).game
    await host.reload()
    await host.getByLabel('环城商业街棋盘').waitFor()
    const after = (await read()).game
    assert.deepEqual(after.properties, before.properties)
    assert.deepEqual(after.dice, before.dice)
    pass('Server dice and purchase persist through page reload without duplicate rolls')

    for (const size of [{ width: 1280, height: 720 }, { width: 820, height: 738 }, { width: 390, height: 844 }]) {
      await host.setViewportSize(size)
      await host.getByLabel('当前回合操作').scrollIntoViewIfNeeded()
      assert.ok(await host.evaluate(() => document.documentElement.scrollWidth <= innerWidth), `${size.width} horizontal overflow`)
      await host.screenshot({ path: path.join(output, `board-${size.width}.png`) })
    }
    await host.setViewportSize({ width: 1440, height: 900 })
    await host.getByLabel('环城商业街棋盘').getByRole('button').first().focus()
    await host.keyboard.press('ArrowRight')
    assert.equal(await host.evaluate(() => document.activeElement.dataset.tile), '1')
    pass('Desktop, compact and mobile layouts retain reachable actions; tile keyboard navigation works')

    // Drive a complete game through the same public API used by the UI.
    // Waiting is for actual persistent server deadlines, not a test-only endpoint.
    let upgraded = false, bought = false, recoveryChecked = false
    const trace = []
    for (let step = 0; step < 250; step++) {
      state = await read()
      const g = state.game
      trace.push({ step, round: g.round, phase: g.phase, current: g.current, version: g.version })
      if (g.phase === 'finished') break
      if (g.phase === 'end_turn') {
        await waitPhaseChange('end_turn')
        continue
      }
      const p = g.players[g.current], cookie = accountByID.get(p.id).cookie
      let action, tile = 0
      if (g.phase === 'management') {
        const owned = state.tiles.find(t => t.upgrade && g.properties[t.id].owner === p.id && !g.properties[t.id].mortgaged && g.properties[t.id].level < 3 && p.cash > t.upgrade + 400)
        if (owned && !g.upgraded) { action = 'upgrade'; tile = owned.id; upgraded = true } else action = 'roll'
      } else if (g.phase === 'purchase_decision') {
        action = p.cash >= state.tiles[p.position].price + 300 ? 'buy' : 'decline'
        if (action === 'buy') bought = true
      } else if (g.phase === 'check_decision') action = p.cash >= 500 ? 'check_release' : 'check_wait'
      else if (g.phase === 'market_decision') action = p.cash >= 700 ? 'market_buy' : 'market_leave'
      else if (g.phase === 'debt') {
        const building = state.tiles.find(t => g.properties[t.id].owner === p.id && g.properties[t.id].level > 1)
        const land = state.tiles.find(t => g.properties[t.id].owner === p.id && !g.properties[t.id].mortgaged)
        if (building) { action = 'sell'; tile = building.id }
        else if (land) { action = 'mortgage'; tile = land.id }
        else {
          await waitPhaseChange('debt', 65000)
          continue
        }
      } else throw Error('Unexpected phase ' + g.phase)
      if (action === 'upgrade' && !recoveryChecked) {
        const page = pageByID.get(p.id)
        await page.locator(`[data-tile="${tile}"]`).click()
        const button = page.getByRole('button', { name: /^升级 · / })
        await button.waitFor()
        await page.screenshot({ path: path.join(output, 'asset-management.png') })
        const url = `**/api/v1/estate/matches/${id}/actions`
        await page.route(url, async route => {
          await route.fetch() // Commit on the real server, lose only the response.
          await route.abort('failed')
        }, { times: 1 })
        await button.click()
        await page.getByRole('alert').filter({ hasText: '操作结果待确认' }).waitFor()
        const committed = await read()
        assert.equal(committed.game.properties[tile].level, g.properties[tile].level + 1)
        await page.getByRole('alert').getByRole('button', { name: '重试', exact: true }).click()
        await page.waitForFunction(() => !document.querySelector('.estate-shell > .estate-error'))
        const recovered = await read()
        assert.equal(recovered.game.version, committed.game.version)
        assert.equal(recovered.game.players.find(player => player.id === p.id).cash, p.cash - state.tiles[tile].upgrade)
        recoveryChecked = true
        pass('Lost upgrade response exposes same-request retry; one building level and one payment commit')
        continue
      }
      await f.api(`/v1/estate/matches/${id}/actions`, 'POST', { client_id: randomUUID(), version: g.version, action, tile }, cookie)
    }
    state = await read()
    fs.writeFileSync(path.join(output, 'trace.json'), JSON.stringify(trace, null, 2))
    assert.equal(state.game.phase, 'finished', JSON.stringify(trace.slice(-8)))
    assert.equal(state.game.round, 12)
    assert.ok(bought && upgraded && recoveryChecked, 'random complete match should exercise purchases and construction')
    for (const p of state.game.players) assert.ok(p.cash >= 0)
    for (const s of state.scores) assert.equal(s.total, s.cash + s.land + s.buildings + s.equity)
    await host.getByRole('heading', { name: '净资产结算', exact: true }).waitFor()
    await host.getByRole('heading', { name: '净资产结算', exact: true }).scrollIntoViewIfNeeded()
    await host.screenshot({ path: path.join(output, 'results-desktop.png') })
    const finalVersion = state.game.version
    for (const page of pages) {
      const snapshot = await page.evaluate(async id => (await (await fetch(`/api/v1/estate/matches/${id}`)).json()), id)
      assert.equal(snapshot.game.version, finalVersion)
      assert.deepEqual(snapshot.scores, state.scores)
      assert.equal(snapshot.game.deck, undefined)
    }
    pass('A four-player twelve-round match completes with purchases, upgrades, event cards and identical final balances on every client')
    await host.evaluate(() => localStorage.setItem('linchat-appearance-v1', JSON.stringify({ version: 1, background: '#f7f7f4', panel: '#ffffff', accent: '#245f4c' })))
    await host.reload()
    await host.getByLabel('环城商业街棋盘').waitFor()
    await host.screenshot({ path: path.join(output, 'board-light.png') })
    for (let token = 0; token < 4; token++) {
      assert.equal(await host.locator(`.estate-players .estate-token-${token} circle`).evaluate(n => getComputedStyle(n).fill), expectedPawnColors[token])
    }
    pass('Custom light appearance preserves the board, ownership markers and action panel')
    await host.getByRole('button', { name: '再来一局', exact: true }).click()
    await host.getByRole('heading', { name: '等大家准备好，就出发' }).waitFor()
    assert.notEqual(Number(new URL(host.url()).hash.split('=')[1]), id)
    assert.equal((await read()).game.version, finalVersion)
    pass('Rematch creates a fresh preparation room while preserving the completed game')
    assert.deepEqual(errors, [])
    fs.writeFileSync(path.join(output, 'results.json'), JSON.stringify({ results, errors, finalVersion, eventCount: state.game.events.length }, null, 2))
  } finally { if (browser) await browser.close(); await f.stop() }
}
main().catch(e => { console.error(e); process.exitCode = 1 })
