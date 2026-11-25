const { test } = require('node:test')
const assert = require('node:assert/strict')
const http = require('node:http')
const { spawn } = require('node:child_process')
const path = require('node:path')
const { campaignClient } = require('./campaign-client.cjs')

test('campaign connector confines routes and sends credentials only to its configured service', async t => {
  const seen = []
  const server = http.createServer((req, res) => {
    seen.push({ path: req.url, auth: req.headers.authorization, csrf: req.headers['x-linchat'] })
    res.setHeader('Content-Type','application/json')
    res.end(JSON.stringify({ ok: true }))
  })
  await new Promise(resolve => server.listen(0,'127.0.0.1',resolve))
  t.after(() => server.close())
  const env = { LINCHAT_CAMPAIGN_URL: `http://127.0.0.1:${server.address().port}/api/v1/campaigns/7`, LINCHAT_AGENT_TOKEN: 'lcgame_test_only' }
  const request = campaignClient(env)
  assert.deepEqual(await request('/context'), { ok: true })
  assert.equal(seen[0].path,'/api/v1/campaigns/7/context')
  assert.equal(seen[0].auth,'Bearer lcgame_test_only')
  assert.equal(seen[0].csrf,'1')
  assert.throws(() => campaignClient({ ...env, LINCHAT_CAMPAIGN_URL: 'https://example.com/api/me' }))
  const child = spawn(process.execPath,[path.join(__dirname,'campaign-mcp.cjs')],{env:{...process.env,...env},stdio:['pipe','pipe','pipe']})
  t.after(() => child.kill())
  const responses = new Map()
  let buffer = ''
  child.stdout.on('data',chunk => {
    buffer+=chunk
    let end
    while((end=buffer.indexOf('\n'))>=0) {
      const rpc=JSON.parse(buffer.slice(0,end)); buffer=buffer.slice(end+1)
      responses.get(rpc.id)?.(rpc)
    }
  })
  const rpc = (id,method,params) => new Promise((resolve,reject)=>{
    const timeout=setTimeout(()=>reject(Error('MCP response timed out')),5000)
    responses.set(id,response=>{clearTimeout(timeout);resolve(response)})
    child.stdin.write(JSON.stringify({jsonrpc:'2.0',id,method,params})+'\n')
  })
  const init = await rpc(1,'initialize',{protocolVersion:'2024-11-05'})
  assert.equal(init.result.serverInfo.name,'linchat-campaign')
  const list = await rpc(2,'tools/list',{})
  assert.ok(list.result.tools.some(tool=>tool.name==='campaign_roll'))
  assert.ok(list.result.tools.some(tool=>tool.name==='campaign_propose'))
  const read=await rpc(3,'tools/call',{name:'campaign_context',arguments:{}})
  assert.equal(JSON.parse(read.result.content[0].text).ok,true)
  const rejected=await rpc(4,'tools/call',{name:'campaign_complete',arguments:{client_id:'unique-test-id-0001',turn_id:'../../other',version:1,lease:'x',outcome:{}}})
  assert.equal(rejected.error.code,-32602)
  const unknown=await rpc(5,'tools/call',{name:'shell',arguments:{}})
  assert.equal(unknown.error.code,-32602)
  assert.equal(seen.length,2)
})
