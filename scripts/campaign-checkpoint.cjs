const fs = require('node:fs')
const path = require('node:path')
const os = require('node:os')
const crypto = require('node:crypto')
const hash = text => crypto.createHash('sha256').update(text).digest('hex')
function checkpoint(turn, messages, env = process.env) {
  if (![turn.id, turn.event_id, turn.base_version].every(n => Number.isSafeInteger(n) && n > 0)) throw Error('Invalid turn checkpoint identity')
  const scope = hash([env.LINCHAT_CAMPAIGN_URL, env.LINCHAT_AGENT_TOKEN, env.MODEL_BASE_URL || 'https://api.x.ai/v1', env.MODEL_ID].join('\n'))
  const root = env.LINCHAT_BOT_STATE_DIR || path.join(env.LOCALAPPDATA || path.join(os.homedir(), '.local', 'share'), 'LinChat', 'bot-state')
  const dir = path.join(root, scope), file = path.join(dir, `turn-${turn.id}.json`)
  let state
  if (fs.existsSync(file)) {
    if (fs.statSync(file).size > 4 * 1024 * 1024) throw Error('Bot checkpoint is too large; owner reconciliation required')
    try { state = JSON.parse(fs.readFileSync(file, 'utf8')) } catch { throw Error('Bot checkpoint is corrupt; do not reroll, reconcile with the owner') }
    if (state.version !== 1 || state.turn_id !== turn.id || state.event_id !== turn.event_id || state.base_version !== turn.base_version ||
      !Array.isArray(state.messages) || !Array.isArray(state.steps) || state.steps.length > 28) {
      throw Error('Bot checkpoint no longer matches the turn input; owner reconciliation required')
    }
    if (state.done) throw Error('A completed checkpoint was reclaimed by the server; owner reconciliation required')
  } else {
    if (turn.resuming) throw Error('Resuming turn has no local Bot checkpoint. Restore the original Bot state directory or ask the owner to take over; do not reroll.')
    state = { version: 1, turn_id: turn.id, event_id: turn.event_id, base_version: turn.base_version, messages, steps: [], done: false }
  }
  function save() {
    const text = JSON.stringify(state)
    if (Buffer.byteLength(text) > 4 * 1024 * 1024) throw Error('Bot checkpoint exceeded its size limit')
    fs.mkdirSync(dir, { recursive: true })
    const temp = file + '.tmp'
    fs.writeFileSync(temp, text, { mode: 0o600 })
    fs.renameSync(temp, file)
  }
  save()
  return { state, save }
}
module.exports = { checkpoint }
