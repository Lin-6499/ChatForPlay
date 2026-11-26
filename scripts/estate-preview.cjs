// User-requested local interactive preview; isolated fixture accounts only.
const { fixture } = require('./entertainment-fixture.cjs')
const { randomUUID } = require('node:crypto')
const fs = require('node:fs')
const path = require('node:path')
const { startPreviewBots } = require('./estate-preview-bots.cjs')

async function main() {
  const f = await fixture()
  let state = (await f.api('/v1/estate/rooms', 'POST', {
    client_id: randomUUID(), title: '环城商业街 · 预览', capacity: 4,
  }, f.owner.cookie)).data
  const id = state.game.id
  const route = `/v1/estate/rooms/${id}`
  for (const cookie of [f.player.cookie, f.outsider.cookie]) {
    state = (await f.api(route + '/join', 'POST', {
      client_id: randomUUID(), version: state.game.version,
    }, cookie)).data
  }
  const accounts = new Map()
  for (const cookie of [f.player.cookie, f.outsider.cookie]) {
    const user = (await f.api('/me', 'GET', undefined, cookie)).data
    accounts.set(user.id, cookie)
  }
  const stopBots = startPreviewBots({
    accounts,
    read: async () => (await f.api(`/v1/estate/matches/${id}`, 'GET', undefined, f.owner.cookie)).data,
    send: async (body, cookie) => f.api(`/v1/estate/matches/${id}/actions`, 'POST', body, cookie),
    onError: e => console.error('Preview companion:', e.message),
  })
  // Keep the initial room ready for the user to start at their own pace.
  for (const cookie of [f.player.cookie, f.outsider.cookie]) {
    state = (await f.api(route + '/ready', 'POST', {
      client_id: randomUUID(), version: state.game.version, ready: true,
    }, cookie)).data
  }
  const url = f.preview.replace(/#.*$/, `#estate=${id}`)
  fs.writeFileSync(path.resolve('dist/verification/estate/preview.json'), JSON.stringify({
    url, room: id, pid: process.pid, isolated: true, autoCompanions: true,
  }, null, 2))
  console.log(`Estate preview ready: ${url}`)
  const stop = () => { stopBots(); void f.stop().then(() => process.exit()) }
  process.once('SIGINT', stop)
  process.once('SIGTERM', stop)
}
main().catch(e => { console.error(e); process.exitCode = 1 })
