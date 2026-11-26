const { test } = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const { checkpoint } = require('./campaign-checkpoint.cjs')
test('checkpoint preserves pending tool input and receipts without credential material', () => {
  const dir = fs.mkdtempSync(path.join(process.env.TEMP || os.tmpdir(), 'linchat-bot-checkpoint-'))
  const env = { LINCHAT_BOT_STATE_DIR: dir, LINCHAT_CAMPAIGN_URL: 'https://example.test/api/v1/campaigns/1',
    LINCHAT_AGENT_TOKEN: 'token-must-not-be-written', MODEL_API_KEY: 'key-must-not-be-written', MODEL_ID: 'fixture' }
  const turn = { id: 1, event_id: 2, base_version: 3 }
  const first = checkpoint(turn, [{ role: 'user', content: 'public context' }], env)
  first.state.steps.push({ answer: { role: 'assistant', tool_calls: [{ id: 'roll', function: { name: 'roll_dice', arguments: '{"formula":"1d20","reason":"调查"}' } }] } })
  first.save()
  const resumed = checkpoint({ ...turn, resuming: true }, [], env)
  assert.equal(resumed.state.steps[0].answer.tool_calls[0].function.name, 'roll_dice')
  resumed.state.steps[0].result = { roll: { total: 17 } }; resumed.save()
  assert.equal(checkpoint({ ...turn, resuming: true }, [], env).state.steps[0].result.roll.total, 17)
  const scoped = fs.readdirSync(dir)[0]
  const raw = fs.readFileSync(path.join(dir, scoped, 'turn-1.json'), 'utf8')
  assert.ok(!raw.includes(env.LINCHAT_AGENT_TOKEN) && !raw.includes(env.MODEL_API_KEY))
  assert.throws(() => checkpoint({ ...turn, base_version: 4, resuming: true }, [], env), /no longer matches/)
  assert.throws(() => checkpoint({ ...turn, id: 2, resuming: true }, [], env), /no local Bot checkpoint/)
})
