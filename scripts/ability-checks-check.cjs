// Headless UI acceptance. Real isolated server for prepare/roll; deterministic
// response fixtures for player-choice UI (choice transactions are Go-tested).
const { chromium } = require('playwright-core')
const { fixture } = require('./entertainment-fixture.cjs')
const fs = require('node:fs')
const path = require('node:path')
const assert = require('node:assert/strict')
const { randomUUID } = require('node:crypto')
const http = require('node:http')
const { spawn } = require('node:child_process')
const out = path.resolve(__dirname, '../dist/verification/ability-checks')
const results = [], errors = []
const pass = name => { results.push(name); console.log('PASS', name) }

;(async () => {
  fs.mkdirSync(out, { recursive: true })
  const f = await fixture()
  const executablePath = ['C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe', 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe'].find(p => fs.existsSync(p))
  const browser = await chromium.launch({ executablePath, headless: true })
  let page
  try {
    const action = (await f.api(f.route('actions'), 'POST', { client_id: randomUUID(), content: '检定验收：尝试打开木门' }, f.player.cookie)).data
    const chars = (await f.api(f.route('characters'), 'GET', undefined, f.owner.cookie)).data
    const character = chars.find(ch => ch.user_id === 2)
    const open = async (cookie, width = 1264) => {
      const context = await browser.newContext({ viewport: { width, height: 811 } })
      await context.addCookies([{ name: 'linchat_session', value: cookie.split('=')[1], url: f.url }])
      const p = await context.newPage()
      p.on('pageerror', error => errors.push(error.message))
      await p.goto(f.url + `/#campaign=${f.cid}`)
      await p.getByRole('button', { name: '行动检定', exact: true }).click()
      return p
    }
    page = await open(f.owner.cookie)
    await page.getByRole('button', { name: '创建检定', exact: true }).click()
    await page.getByLabel('检定对应的玩家行动').selectOption(String(action.event_id))
    await page.getByText(`角色：${character.name}`, { exact: false }).waitFor()
    await page.getByLabel('难度 DC', { exact: true }).fill('15')
    await page.getByLabel('额外修正', { exact: true }).fill('2')
    await page.getByLabel('额外修正依据', { exact: true }).fill('熟练 +2')
    await page.getByLabel('尝试消耗（投骰时必付）生命值变化', { exact: true }).fill('-1')
    for (const name of ['大成功', '出色成功', '普通成功', '轻度失败', '明显失败', '大失败']) {
      await page.getByLabel(`${name}后果说明`, { exact: true }).fill(`验收已声明后果：${name}`)
    }
    await page.screenshot({ path: path.join(out, 'prepare-desktop.png') })
    await page.getByRole('button', { name: '锁定条件', exact: true }).click()
    await page.getByText('条件已锁定，等待投骰', { exact: true }).waitFor()
    let checks = (await f.api(f.route('checks'), 'GET', undefined, f.owner.cookie)).data
    assert.equal(checks[0].user_id, 2)
    assert.equal(checks[0].modifier, 3)
    assert.equal(checks[0].natural, 0)
    assert.equal(checks[0].before.hp, character.hp)
    pass('Owner form binds the submitted action to its player character and locks all conditions before rolling')
    const playerPage = await open(f.player.cookie, 390)
    assert.equal(await playerPage.getByRole('button', { name: '创建检定', exact: true }).count(), 0)
    await playerPage.getByRole('button', { name: '投掷 d20 并结算', exact: true }).click()
    await playerPage.getByText('已结算', { exact: true }).waitFor()
    checks = (await f.api(f.route('checks'), 'GET', undefined, f.owner.cookie)).data
    const rolled = checks[0]
    assert.ok(rolled.natural >= 1 && rolled.natural <= 20)
    assert.equal(rolled.after.hp, character.hp - 1)
    await playerPage.reload()
    await playerPage.getByRole('button', { name: '行动检定', exact: true }).click()
    await playerPage.getByText('已结算', { exact: true }).waitFor()
    assert.equal(await playerPage.getByRole('button', { name: '投掷 d20 并结算', exact: true }).count(), 0)
    assert.equal((await f.api(f.route('characters'), 'GET', undefined, f.owner.cookie)).data[0].hp, character.hp - 1)
    await playerPage.screenshot({ path: path.join(out, 'rolled-mobile.png') })
    pass('Player rolls their own locked check on mobile; reload preserves the result and a single resource deduction')
    let view = { ...rolled, status: 'awaiting_choice', natural: 10, total: 13, margin: -2, grade: 'minor_failure',
      resolution: '等待角色所属玩家选择是否接受代价', offer: { description: '承担擦伤，成功开门', hp_delta: -2 } }
    let chosen
    const url = `**/api/v1/campaigns/${f.cid}/checks`
    await playerPage.route(url, route => route.fulfill({ json: [view] }))
    await playerPage.route(`**/api/v1/campaigns/${f.cid}/checks/${rolled.id}/choose`, async route => {
      chosen = route.request().postDataJSON()
      view = { ...view, status: 'resolved', accepted: chosen.accept, resolution: '承担擦伤，成功开门', after: { ...view.after, hp: view.after.hp - 2 } }
      await route.fulfill({ json: view })
    })
    await playerPage.reload()
    await playerPage.getByRole('button', { name: '行动检定', exact: true }).click()
    await playerPage.getByRole('button', { name: '接受代价，换取成功', exact: true }).waitFor()
    assert.ok(await playerPage.getByRole('button', { name: '拒绝，保留失败结果', exact: true }).isEnabled())
    await page.route(url, route => route.fulfill({ json: [view] }))
    await page.reload()
    await page.getByRole('button', { name: '行动检定', exact: true }).click()
    await page.getByText('等待玩家选择', { exact: true }).waitFor()
    assert.equal(await page.getByRole('button', { name: '接受代价，换取成功', exact: true }).count(), 0)
    await playerPage.screenshot({ path: path.join(out, 'choice-mobile.png') })
    await page.screenshot({ path: path.join(out, 'choice-desktop.png') })
    await playerPage.getByRole('button', { name: '接受代价，换取成功', exact: true }).click()
    await playerPage.getByText('已结算', { exact: true }).waitFor()
    assert.equal(chosen.accept, true)
    assert.match(chosen.client_id, /^[A-Za-z0-9_-]{16,80}$/)
    pass('Choice UI shows both consequences and only the character owner can submit an explicit choice')
    for (const p of [page, playerPage]) {
      assert.ok(await p.evaluate(() => document.documentElement.scrollWidth <= innerWidth), 'No horizontal overflow')
    }
    await page.route(url, route => route.fulfill({ status: 503, json: { error: '验收：检定连接暂时中断' } }))
    await page.reload()
    await page.getByRole('button', { name: '行动检定', exact: true }).click()
    await page.getByRole('button', { name: '重新读取检定', exact: true }).waitFor()
    await page.unroute(url)
    await page.getByRole('button', { name: '重新读取检定', exact: true }).click()
    await page.getByText('已结算', { exact: true }).waitFor()
    pass('Desktop and 390px layouts have no horizontal overflow; failed reads expose a working retry')
    const provider = http.createServer(async (request, response) => {
      let raw = ''; for await (const chunk of request) raw += chunk
      const input = JSON.parse(raw), last = input.messages.at(-1)
      let name, args
      if (last.role !== 'tool') {
        name = 'prepare_check'
        args = { character_id: character.id, kind: 'ability', ability: 'str', reason: '外部主持锁定检定验收', dc: 15,
          cost: { description: '尝试消耗', hp_delta: -1 },
          outcomes: Object.fromEntries(['critical_success', 'great_success', 'success', 'minor_failure', 'failure', 'critical_failure'].map(grade => [grade, { description: grade, hp_delta: 0 }])) }
      } else {
        const check = JSON.parse(last.content)
        if (check.status === 'prepared') { name = 'roll_check'; args = { check_id: check.id } }
        else { name = 'finish_turn'; args = { narration: '依照软件检定结果继续。', phase: 'awaiting_action', changes: [] } }
      }
      response.setHeader('Content-Type', 'application/json')
      response.end(JSON.stringify({ choices: [{ message: { role: 'assistant', content: null, tool_calls: [{ id: randomUUID(), type: 'function', function: { name, arguments: JSON.stringify(args) } }] } }] }))
    })
    await new Promise(resolve => provider.listen(0, '127.0.0.1', resolve))
    try {
      await f.api(f.route('state'), 'PATCH', { client_id: randomUUID(), version: (await f.state()).version, mode: 'external', phase: 'awaiting_action' }, f.owner.cookie)
      const token = (await f.api(f.route('agent-tokens'), 'POST', { label: '检定验收机器人', secrets: false }, f.owner.cookie)).data.token
      const nextAction = (await f.api(f.route('actions'), 'POST', { client_id: randomUUID(), content: '外部检定：我继续开门' }, f.player.cookie)).data
      const child = spawn(process.execPath, [path.join(__dirname, 'campaign-agent.cjs'), '--once'], {
        windowsHide: true, stdio: ['ignore', 'pipe', 'pipe'],
        env: { ...process.env, LINCHAT_CAMPAIGN_URL: f.url + '/api' + f.route('').replace(/\/$/, ''), LINCHAT_AGENT_TOKEN: token,
          MODEL_BASE_URL: `http://127.0.0.1:${provider.address().port}/v1`, MODEL_ID: 'fixture-checks', MODEL_API_KEY: 'fixture-only' },
      })
      let output = ''
      child.stdout.on('data', data => { output += data }); child.stderr.on('data', data => { output += data })
      const code = await new Promise((resolve, reject) => {
        const timeout = setTimeout(() => { child.kill(); reject(Error('External check timed out')) }, 20000)
        child.once('error', error => { clearTimeout(timeout); reject(error) })
        child.once('exit', code => { clearTimeout(timeout); resolve(code) })
      })
      assert.equal(code, 0, output)
      const actual = (await f.api(f.route('checks'), 'GET', undefined, f.owner.cookie)).data.find(c => c.event_id === nextAction.event_id)
      assert.equal(actual.status, 'resolved')
      assert.equal(actual.after.hp, actual.before.hp - 1)
      assert.match(output, /Completed campaign turn/)
      pass('External agent prepares, rolls and completes a real leased check through HTTP with one resource deduction')
    } finally { provider.closeAllConnections(); await new Promise(resolve => provider.close(resolve)) }
    assert.deepEqual(errors, [])
    fs.writeFileSync(path.join(out, 'results.json'), JSON.stringify({ results, errors, choiceUI: 'deterministic response fixtures; server choice permissions and transactions covered in Go tests' }, null, 2))
  } catch (error) {
    await page?.screenshot({ path: path.join(out, 'failure.png') }).catch(() => {})
    throw error
  } finally { await browser.close(); await f.stop() }
})().catch(error => { console.error(error); process.exitCode = 1 })
