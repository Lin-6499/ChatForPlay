async function checkCampaignCapabilities(request) {
  let data
  try { data = await request('/capabilities') }
  catch (e) {
    if (e.status === 404) throw Error('LinChat lacks the capabilities endpoint. Upgrade the campaign server before running this bot.')
    if (e.status === 401 || e.status === 403) throw Error('Campaign access was refused. Check the campaign URL and token permissions.')
    throw Error('Cannot check campaign capabilities. Check the server address and network.')
  }
  const version = 1
  if (!Number.isInteger(data.protocol?.min) || !Number.isInteger(data.protocol?.max) || data.protocol.min > version || data.protocol.max < version) {
    throw Error('Campaign protocol is incompatible with this bot. Update the bot and server together.')
  }
  const required = ['atomic_complete', 'stream_snapshots', 'd20_graded_v1', 'group_actions']
  if (!Array.isArray(data.features) || required.some(feature => !data.features.includes(feature))) {
    throw Error('Campaign server is missing required hosting capabilities. Update the server or use a matching bot version.')
  }
  return data
}
module.exports = { checkCampaignCapabilities }
