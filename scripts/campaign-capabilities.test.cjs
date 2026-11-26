const { test } = require('node:test')
const assert = require('node:assert/strict')
const { checkCampaignCapabilities } = require('./campaign-capabilities.cjs')
const valid = { server_version: '1.2.13', protocol: { min: 1, max: 1 }, features: ['atomic_complete', 'stream_snapshots', 'd20_graded_v1', 'group_actions', 'lease_heartbeat', 'turn_budget'] }
test('hosting startup checks protocol and required capabilities', async () => {
  assert.deepEqual(await checkCampaignCapabilities(async () => valid), valid)
  await assert.rejects(checkCampaignCapabilities(async () => ({ ...valid, protocol: { min: 2, max: 3 } })), /incompatible/)
  await assert.rejects(checkCampaignCapabilities(async () => ({ ...valid, features: [] })), /missing required/)
})
test('self-check explains old endpoint, revoked permissions and network without credentials', async () => {
  for (const [status, expected] of [[404, /Upgrade/], [401, /refused/], [403, /refused/], [502, /network/]]) {
    await assert.rejects(checkCampaignCapabilities(async () => { const e = Error('secret must not be echoed'); e.status = status; throw e }), expected)
  }
})
