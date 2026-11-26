// Preview-only companions use normal authenticated game actions. They cannot
// choose dice, edit balances or act for the human preview account.
function choosePreviewAction(view) {
  const g = view.game, p = g.players[g.current]
  if (!p || p.bankrupt) return null
  if (g.phase === 'management') {
    const tile = view.tiles.find(t => t.upgrade && g.properties[t.id].owner === p.id &&
      !g.properties[t.id].mortgaged && g.properties[t.id].level < 3 && p.cash >= t.upgrade + 500)
    return tile && !g.upgraded ? { action: 'upgrade', tile: tile.id } : { action: 'roll' }
  }
  if (g.phase === 'purchase_decision') return { action: p.cash >= view.tiles[p.position].price + 250 ? 'buy' : 'decline' }
  if (g.phase === 'check_decision') return { action: p.cash >= 400 ? 'check_release' : 'check_wait' }
  if (g.phase === 'market_decision') return { action: p.cash >= 600 ? 'market_buy' : 'market_leave' }
  if (g.phase === 'debt') {
    const tiles = view.tiles.filter(t => g.properties[t.id].owner === p.id)
    const building = tiles.filter(t => g.properties[t.id].level > 1)
      .sort((a, b) => g.properties[b.id].level - g.properties[a.id].level || a.id - b.id)[0]
    if (building) return { action: 'sell', tile: building.id }
    const land = tiles.filter(t => !g.properties[t.id].mortgaged).sort((a, b) => a.price - b.price || a.id - b.id)[0]
    return land ? { action: 'mortgage', tile: land.id } : null
  }
  return null
}

function startPreviewBots({ read, send, accounts, delay = 1500, interval = 400, onError = console.error }) {
  let stopped = false, running = false, stateKey = '', since = 0, pending = null
  const { randomUUID } = require('node:crypto')
  async function tick() {
    if (stopped || running) return
    running = true
    try {
      const view = await read()
      if (stopped) return
      const g = view.game, player = g.players[g.current], cookie = accounts.get(player?.id)
      if (!cookie || ['lobby', 'finished', 'closed', 'end_turn'].includes(g.phase)) { stateKey = ''; pending = null; return }
      const key = `${g.id}:${g.version}:${player.id}:${g.phase}`
      if (stateKey !== key) { stateKey = key; since = Date.now(); pending = null; return }
      if (Date.now() - since < delay) return
      const action = choosePreviewAction(view)
      if (!action) return
      pending ||= { ...action, client_id: randomUUID(), version: g.version }
      await send(pending, cookie)
      pending = null; stateKey = ''
    } catch (e) { if (!stopped) onError(e) }
    finally { running = false }
  }
  const timer = setInterval(() => { void tick() }, interval)
  return () => { stopped = true; clearInterval(timer) }
}
module.exports = { choosePreviewAction, startPreviewBots }
