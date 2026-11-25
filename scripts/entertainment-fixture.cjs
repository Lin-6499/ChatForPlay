// Isolated loopback-only demo data for UI verification. No production accounts or model calls.
const fs = require('node:fs')
const path = require('node:path')
const http = require('node:http')
const net = require('node:net')
const { spawn } = require('node:child_process')
const { randomUUID } = require('node:crypto')
const root = path.resolve(__dirname,'..')
const listen = server => new Promise(resolve => server.listen(0,'127.0.0.1',() => resolve(server.address().port)))
async function fixture() {
  const out=path.join(root,'dist','verification')
  fs.mkdirSync(out,{recursive:true})
  const data=fs.mkdtempSync(path.join(out,'campaign-demo-'))
  const reserve=net.createServer(), port=await listen(reserve)
  await new Promise(resolve=>reserve.close(resolve))
  const url=`http://127.0.0.1:${port}`
  const executable=path.join(data,'fixture-server.exe')
  fs.copyFileSync(path.join(root,'dist','server','linchat-windows-amd64.exe'),executable)
  const child=spawn(executable,['-listen',`127.0.0.1:${port}`,'-data',data,'-url',url,'-managed'],{stdio:['pipe','pipe','pipe']})
  await new Promise((resolve,reject)=>{
    const timeout=setTimeout(()=>reject(Error('Test server did not start')),15000)
    child.stderr.on('data',data=>{if(data.toString().includes('LinChat 已启动')){clearTimeout(timeout);resolve()}})
    child.once('exit',code=>{clearTimeout(timeout);reject(Error(`Test server exited ${code}`))})
  })
  async function api(route,method='GET',body,cookie) {
    const response=await fetch(url+'/api'+route,{method,headers:{'Content-Type':'application/json','X-LinChat':'1',...(cookie?{Cookie:cookie}:{})},body:body===undefined?undefined:JSON.stringify(body)})
    const result=await response.json()
    if(!response.ok)throw Error(`${route}: ${response.status} ${JSON.stringify(result)}`)
    return {data:result,cookie:response.headers.get('set-cookie')?.split(';')[0]||cookie}
  }
  const password='Fixture-only-password-2026'
  const owner=await api('/setup','POST',{username:'demo_owner',nickname:'林同学（验证样例）',password,token:fs.readFileSync(path.join(data,'setup-token'),'utf8').trim()})
  const invite=await api('/invites','POST',{uses:2},owner.cookie)
  const player=await api('/auth/register','POST',{username:'demo_player',nickname:'小满（验证样例）',password,token:invite.data.token})
  const outsider=await api('/auth/register','POST',{username:'demo_other',nickname:'另一位玩家',password,token:invite.data.token})
  const campaign=(await api('/v1/campaigns','POST',{title:'雾港来信 · 验证样例',ruleset:'5e-2024',members:[2],scene:'雨后的雾港，旧钟楼在暮色中敲了十三下。你们在门口找到一封没有署名的信，信封上沾着银色的细沙。远处，一位提灯的守夜人正在向码头走去。',house_rules:'这是用于验证界面的虚构战役。\n先描述行动，再由主持人决定是否需要检定；玩家自行决定角色的选择。'},owner.cookie)).data
  const cid=campaign.id, route=suffix=>`/v1/campaigns/${cid}/${suffix}`
  const character={id:0,campaign_id:cid,user_id:2,name:'伊岚',role:'游侠',level:3,hp:24,max_hp:28,ac:15,abilities:{str:12,dex:16,con:14,int:10,wis:14,cha:10},conditions:'无异常状态',inventory:'长弓、皮甲、火把 × 3、信件',resources:'箭 × 20；法术位：一环 2/3',background:'一位熟悉北地森林的旅行者。',private_notes:'验证秘密：失踪的导师曾在钟楼留下暗号。',version:0}
  await api(route('characters'),'POST',{client_id:randomUUID(),version:campaign.version,character},player.cookie)
  const state=async()=>(await api(route('state'),'GET',undefined,owner.cookie)).data
  await api(route('state'),'PATCH',{client_id:randomUUID(),version:(await state()).version,phase:'awaiting_action'},owner.cookie)
  await api(route('messages'),'POST',{client_id:randomUUID(),version:(await state()).version,content:'钟声落下，街道重新安静下来。潮湿的石门上有一道新鲜的划痕，守夜人的灯光正渐渐远去。\n\n你们准备先调查石门，还是追上守夜人？',visibility:'public',kind:'narration'},owner.cookie)
  await api(route('actions'),'POST',{client_id:randomUUID(),content:'我举起火把，查看石门上的刻痕，同时留意门后是否有声音。'},player.cookie)
  await api(route('rolls'),'POST',{client_id:randomUUID(),formula:'1d20+4',reason:'调查石门刻痕',visibility:'public'},player.cookie)
  await api(route('dm-notes'),'PUT',{client_id:randomUUID(),content:'验证主持秘密：守夜人知道银沙的来源。这条内容不应出现在玩家或公开模型上下文中。',note_version:1},owner.cookie)
  const model=http.createServer(async(req,res)=>{
    let text='';for await(const chunk of req)text+=chunk
    const body=JSON.parse(text), last=body.messages.at(-1)
    let name,args
    if(body.tools[0].function.name==='connection_test'){name='connection_test';args={ok:true}}
    else {
      const context=JSON.parse(body.messages.find(m=>m.role==='user').content)
      if(context.kind==='summary'||context.turn?.kind==='summary') {name='finish_turn';args={narration:'本次冒险暂告一段落。',phase:'awaiting_action',summary:'旅人们调查了旧钟楼，发现石门上的银沙线索。下次从石门入口继续。',changes:[]}}
      else if(last.role==='tool'){name='finish_turn';args={narration:'检定已经由服务器记录。你发现刻痕指向门轴边的一枚小小徽记，门后没有传来脚步声。\n\n接下来，你打算如何处理这枚徽记？',phase:'awaiting_action',changes:[]}}
      else {name='roll_dice';args={formula:'1d20+4',reason:'辨认石门徽记'}}
    }
    res.setHeader('Content-Type','application/json')
    res.end(JSON.stringify({choices:[{message:{role:'assistant',content:null,tool_calls:[{id:randomUUID(),type:'function',function:{name,arguments:JSON.stringify(args)}}]}}]}))
  })
  const modelPort=await listen(model)
  const proxy=http.createServer((req,res)=>{
    const headers={...req.headers,host:`127.0.0.1:${port}`,cookie:owner.cookie}
    if(headers.origin===`http://127.0.0.1:${proxy.address().port}`)headers.origin=url
    const forwarded=http.request(url+req.url,{method:req.method,headers},upstream=>{
      res.writeHead(upstream.statusCode,upstream.headers);upstream.pipe(res)
    })
    forwarded.on('error',()=>{res.writeHead(502);res.end()});req.pipe(forwarded)
  })
  proxy.on('upgrade',(req,socket,head)=>{
    const target=net.connect(port,'127.0.0.1',()=>{
      const headers={...req.headers,host:`127.0.0.1:${port}`,origin:url,cookie:owner.cookie}
      target.write(`${req.method} ${req.url} HTTP/1.1\r\n${Object.entries(headers).map(([k,v])=>`${k}: ${v}`).join('\r\n')}\r\n\r\n`)
      if(head.length)target.write(head);socket.pipe(target);target.pipe(socket)
    })
    target.on('error',()=>socket.destroy());socket.on('error',()=>target.destroy());socket.on('close',()=>target.destroy())
  })
  const proxyPort=await listen(proxy)
  const stop=async()=>{
    proxy.closeAllConnections();proxy.close();model.closeAllConnections();model.close()
    if(child.exitCode===null){child.stdin.write('shutdown\n');await new Promise(resolve=>child.once('exit',resolve))}
  }
  return {url,preview:`http://127.0.0.1:${proxyPort}/#campaign=${cid}`,modelURL:`http://127.0.0.1:${modelPort}/v1`,data,api,owner,player,outsider,route,state,cid,password,stop}
}
if(require.main===module)fixture().then(f=>{
  fs.writeFileSync(path.join(root,'dist','verification','campaign-preview.json'),JSON.stringify({pid:process.pid,url:f.url,preview:f.preview,modelURL:f.modelURL,data:f.data,campaign:f.cid},null,2))
  console.log(`Isolated campaign preview: ${f.preview}`)
  process.once('SIGINT',()=>f.stop().then(()=>process.exit()))
  process.once('SIGTERM',()=>f.stop().then(()=>process.exit()))
}).catch(e=>{console.error(e);process.exitCode=1})
module.exports={fixture}
