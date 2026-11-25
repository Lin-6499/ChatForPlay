// Shared by the stdio MCP connector and the external host example. Credentials stay in process memory.
function campaignClient(env = process.env) {
  const base = String(env.LINCHAT_CAMPAIGN_URL || '').replace(/\/$/, '')
  const token = env.LINCHAT_AGENT_TOKEN
  let url
  try { url = new URL(base) } catch { throw Error('Set LINCHAT_CAMPAIGN_URL to the campaign API URL shown in LinChat.') }
  if (!['https:', 'http:'].includes(url.protocol) || url.username || url.password || url.search || url.hash || !/^\/api\/v1\/campaigns\/[1-9]\d*$/.test(url.pathname)) throw Error('Invalid campaign API URL.')
  if (!token?.startsWith('lcgame_') || /[\r\n]/.test(token)) throw Error('Set LINCHAT_AGENT_TOKEN to a campaign-scoped token.')
  return async (route, method = 'GET', body) => {
    const response = await fetch(base + route, {
      method, redirect: 'error', signal: AbortSignal.timeout(20000),
      headers: { Authorization: `Bearer ${token}`, 'X-LinChat': '1', ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}) },
      body: body === undefined ? undefined : JSON.stringify(body),
    })
    const result = await response.json()
    if (!response.ok) { const e = Error(result.error || `Campaign HTTP ${response.status}`); e.status = response.status; throw e }
    return result
  }
}
module.exports = { campaignClient }
