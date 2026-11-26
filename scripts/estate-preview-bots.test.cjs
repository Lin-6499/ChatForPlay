const { test } = require('node:test')
const assert = require('node:assert/strict')
const { choosePreviewAction, startPreviewBots } = require('./estate-preview-bots.cjs')

const view = () => ({ game: { id: 1, version: 1, current: 0, phase: 'management', upgraded: false,
  players: [{ id: 2, cash: 2000, position: 1 }], properties: [{ owner: 0 }, { owner: 0, level: 0 }] },
  tiles: [{ id: 0, price: 0 }, { id: 1, price: 200, upgrade: 100 }] })

test('preview companions choose normal actions without altering dice or balances', () => {
  const v = view(), before = JSON.stringify(v)
  assert.deepEqual(choosePreviewAction(v), { action: 'roll' })
  assert.equal(JSON.stringify(v), before)
  v.game.phase = 'purchase_decision'
  assert.deepEqual(choosePreviewAction(v), { action: 'buy' })
  v.game.players[0].cash = 100
  assert.deepEqual(choosePreviewAction(v), { action: 'decline' })
  v.game.phase = 'debt'; v.game.properties[1] = { owner: 2, level: 2 }
  assert.deepEqual(choosePreviewAction(v), { action: 'sell', tile: 1 })
  v.game.properties[1].level = 1
  assert.deepEqual(choosePreviewAction(v), { action: 'mortgage', tile: 1 })
})
test('automatic companions act promptly but never for the human account', { timeout: 5000 }, async () => {
  let v = view(), count = 0, resolve
  const acted = new Promise(r => { resolve = r })
  const stop = startPreviewBots({ accounts: new Map([[2, 'bot-cookie']]), delay: 5, interval: 5,
    read: async () => v,
    send: async (body, cookie) => { assert.equal(cookie, 'bot-cookie'); assert.equal(body.action, 'roll'); count++; v = { ...v, game: { ...v.game, current: 0, players: [{ id: 1 }], version: 2 } }; resolve() },
    onError: e => { throw e },
  })
  try { await acted; assert.equal(count, 1) } finally { stop() }
})
test('preview companion submits a real server roll without waiting 45 seconds', { timeout: 25000 }, async () => {
  const { fixture } = require('./entertainment-fixture.cjs')
  const { randomUUID } = require('node:crypto')
  const { setTimeout: wait } = require('node:timers/promises')
  const f = await fixture()
  let stop = () => {}
  try {
    let v = (await f.api('/v1/estate/rooms', 'POST', { client_id: randomUUID(), title: '自动陪玩验收', capacity: 2 }, f.owner.cookie)).data
    const id = v.game.id, room = `/v1/estate/rooms/${id}`
    v = (await f.api(room + '/join', 'POST', { client_id: randomUUID(), version: v.game.version }, f.player.cookie)).data
    for (const cookie of [f.owner.cookie, f.player.cookie]) v = (await f.api(room + '/ready', 'POST', { client_id: randomUUID(), version: v.game.version, ready: true }, cookie)).data
    v = (await f.api(room + '/start', 'POST', { client_id: randomUUID(), version: v.game.version }, f.owner.cookie)).data
    const bot = (await f.api('/me', 'GET', undefined, f.player.cookie)).data.id
    const route = `/v1/estate/matches/${id}`, read = async () => (await f.api(route, 'GET', undefined, f.owner.cookie)).data
    const started = Date.now()
    stop = startPreviewBots({ accounts: new Map([[bot, f.player.cookie]]), read,
      send: async (body, cookie) => f.api(route + '/actions', 'POST', body, cookie),
      delay: 150, interval: 50, onError: e => { console.error(e.message) } })
    while (Date.now() - started < 10000) {
      v = await read()
      if (v.game.events.some(e => e.text.includes('小满') && e.text.includes('掷出'))) return
      if (v.game.players[v.game.current].id !== bot) {
        const action = choosePreviewAction(v)
        if (action) await f.api(route + '/actions', 'POST', { ...action, client_id: randomUUID(), version: v.game.version }, f.owner.cookie)
      }
      await wait(100)
    }
    assert.fail('companion did not roll promptly')
  } finally { stop(); await f.stop() }
})
