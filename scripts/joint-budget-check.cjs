const { fixture } = require('./entertainment-fixture.cjs')
const { spawn } = require('node:child_process')
const http = require('node:http')
const fs = require('node:fs')
const path = require('node:path')
const assert = require('node:assert/strict')
const { randomUUID } = require('node:crypto')
;(async () => {
  const f = await fixture(), out = path.resolve('dist/verification/reliability')
  fs.mkdirSync(out, { recursive: true })
  let provider, failed = false, calls = 0
  try {
    const invitation = (await f.api('/invites', 'POST', { uses: 1 }, f.owner.cookie)).data.token
    const fourth = await f.api('/auth/register', 'POST', { username: 'budget_four', nickname: '第四位玩家', password: f.password, token: invitation })
    const clients = [f.owner, f.player, f.outsider, fourth]
    const ids = []
    for (const client of clients) ids.push((await f.api('/me', 'GET', undefined, client.cookie)).data.id)
    const c = (await f.api('/v1/campaigns', 'POST', { title: '四人续跑验收', ruleset: '5e-2024', members: ids.slice(1), scene: '四人同时调查钟楼。', house_rules: '' }, f.owner.cookie)).data
    const route = s => `/v1/campaigns/${c.id}/${s}`
    const state = async () => (await f.api(route('state'), 'GET', undefined, f.owner.cookie)).data
    const baseCharacter = (await f.api(f.route('characters'), 'GET', undefined, f.owner.cookie)).data[0]
    for (let i = 0; i < clients.length; i++) await f.api(route('characters'), 'POST', { client_id: randomUUID(), version: (await state()).version,
      character: { ...baseCharacter, id: 0, campaign_id: c.id, user_id: ids[i], name: `调查者${i + 1}`, hp: 24, version: 0 } }, clients[i].cookie)
    await f.api(route('state'), 'PATCH', { client_id: randomUUID(), version: (await state()).version, mode: 'external', phase: 'awaiting_action' }, f.owner.cookie)
    await f.api(route('flow'), 'PUT', { client_id: randomUUID(), version: (await state()).version, mode: 'collect', participants: ids }, f.owner.cookie)
    for (const client of clients) await f.api(route('actions'), 'POST', { client_id: randomUUID(), content: '我调查自己的方向。' }, client.cookie)
    const agent = (await f.api(route('agent-tokens'), 'POST', { label: 'budget-fixture', secrets: false }, f.owner.cookie)).data
    provider = http.createServer(async (req, res) => {
      let raw = ''; for await (const b of req) raw += b
      const body = JSON.parse(raw), step = body.messages.filter(m => m.role === 'assistant').length
      calls++
      if (step === 4 && !failed) { failed = true; res.writeHead(503); res.end(); return }
      const source = JSON.parse(body.messages.find(m => m.role === 'user').content).context
      let name, args
      if (step < 8 && step % 2 === 0) {
        const action = source.turn_actions[step / 2], ch = source.characters.find(c => c.user_id === action.user_id)
        name = 'prepare_check'; args = { event_id: action.event_id, character_id: ch.id, kind: 'ability', ability: 'str', reason: '联合调查', dc: 10,
          cost: { description: '一次行动消耗', hp_delta: -1 },
          outcomes: Object.fromEntries(['critical_success', 'great_success', 'success', 'minor_failure', 'failure', 'critical_failure'].map(g => [g, { description: '已锁定后果', hp_delta: 0 }])) }
      } else if (step < 8) { name = 'roll_check'; args = { check_id: JSON.parse(body.messages.at(-1).content).id } }
      else { name = 'finish_turn'; args = { narration: '四人的调查均已完成，准备下一步行动。', phase: 'awaiting_action', changes: [] } }
      res.setHeader('Content-Type', 'application/json')
      res.end(JSON.stringify({ choices: [{ message: { role: 'assistant', tool_calls: [{ id: `step-${step}`, type: 'function', function: { name, arguments: JSON.stringify(args) } }] } }] }))
    })
    await new Promise(resolve => provider.listen(0, '127.0.0.1', resolve))
    async function run() {
      const child = spawn(process.execPath, [path.join(__dirname, 'campaign-agent.cjs'), '--once'], { windowsHide: true, stdio: ['ignore', 'pipe', 'pipe'],
        env: { ...process.env, LINCHAT_CAMPAIGN_URL: f.url + '/api' + route('').replace(/\/$/, ''), LINCHAT_AGENT_TOKEN: agent.token,
          MODEL_BASE_URL: `http://127.0.0.1:${provider.address().port}/v1`, MODEL_ID: 'joint-budget-fixture', MODEL_API_KEY: 'fixture-only' } })
      let output = ''; child.stdout.on('data', b => { output += b }); child.stderr.on('data', b => { output += b })
      return new Promise((resolve, reject) => {
        const timeout = setTimeout(() => { child.kill(); reject(Error('Bot fixture timed out')) }, 40000)
        child.once('exit', code => { clearTimeout(timeout); resolve({ code, output }) })
      })
    }
    const first = await run(); assert.equal(first.code, 1, first.output)
    const turn = (await f.api(route('turns'), 'GET', undefined, f.owner.cookie)).data[0]
    assert.equal(turn.status, 'failed')
    await f.api(route(`turns/${turn.id}/retry`), 'POST', { client_id: randomUUID(), version: (await state()).version }, f.owner.cookie)
    const resumed = await run(); assert.equal(resumed.code, 0, resumed.output)
    const chars = (await f.api(route('characters'), 'GET', undefined, f.owner.cookie)).data
    assert.ok(chars.every(c => c.hp === 23))
    const checks = (await f.api(route('checks'), 'GET', undefined, f.owner.cookie)).data
    assert.equal(checks.length, 4); assert.ok(checks.every(c => c.status === 'resolved'))
    assert.equal(calls, 10)
    assert.equal((await f.api(route('turns'), 'GET', undefined, f.owner.cookie)).data[0].status, 'done')
    const report = { version: require('../package.json').version, passed: true, players: 4, toolSteps: 9, modelRequestsIncludingFailure: calls, uniqueChecks: 4, costPerPlayer: 1, resumedFromLocalCheckpoint: true }
    fs.writeFileSync(path.join(out, 'joint-budget.json'), JSON.stringify(report, null, 2))
    console.log('PASS external four-player nine-step workflow resumes after model failure without repeated decisions, rolls or resource costs')
  } finally { if (provider) { provider.closeAllConnections(); provider.close() }; await f.stop() }
})().catch(e => { console.error(e); process.exitCode = 1 })
