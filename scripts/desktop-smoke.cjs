const { app, BrowserWindow } = require('electron')
const fs = require('node:fs')
const path = require('node:path')
const assert = require('node:assert/strict')
const root = path.resolve(__dirname, '..')
const out = path.join(root, 'dist', 'verification')
fs.mkdirSync(out, { recursive: true })
const testData = fs.mkdtempSync(path.join(out, 'desktop-test-'))
process.env.LINCHAT_USER_DATA = testData
const results = []
const browserErrors = []
let win
const windowReady = new Promise(resolve => {
  app.once('browser-window-created', (_event, window) => {
    win = window
    window.webContents.on('console-message', (_event, level, message, line, source) => {
      if (level >= 2) browserErrors.push({ level, message, line, source })
    })
    window.webContents.once('did-finish-load', () => resolve(window))
  })
})
require('../desktop/main.cjs')

function record(name) { results.push({ test: name, passed: true }); console.log('PASS', name) }
async function evaluate(code) { return win.webContents.executeJavaScript(code, true) }
async function waitFor(code, description) {
  const end = Date.now() + 15000
  while (Date.now() < end) {
    if (await evaluate(code)) return
    await new Promise(resolve => setTimeout(resolve, 80))
  }
  throw new Error('Timed out: ' + description)
}
async function navigate(code) {
  const ready = new Promise((resolve, reject) => {
    const timer = setTimeout(() => reject(new Error('Navigation timeout')), 15000)
    win.webContents.once('did-finish-load', () => { clearTimeout(timer); resolve() })
  })
  await evaluate(`void (${code})`)
  await ready
}
async function request(base, url, body, cookie) {
  const res = await fetch(base + '/api' + url, {
    method: body ? 'POST' : 'GET',
    headers: { 'Content-Type': 'application/json', 'X-LinChat': '1', Origin: base, ...(cookie ? { Cookie: cookie } : {}) },
    body: body ? JSON.stringify(body) : undefined,
  })
  const data = await res.json()
  assert.ok(res.ok, `${url}: ${JSON.stringify(data)}`)
  return { data, cookie: res.headers.get('set-cookie')?.split(';')[0] || cookie }
}
async function fill(selector, value) {
  await evaluate(`(() => { const e=document.querySelector(${JSON.stringify(selector)}); if(!e)throw Error('Missing input'); const set=Object.getOwnPropertyDescriptor(e.tagName==='TEXTAREA'?HTMLTextAreaElement.prototype:HTMLInputElement.prototype,'value').set;set.call(e,${JSON.stringify(value)});e.dispatchEvent(new Event('input',{bubbles:true})); })()`)
}

;(async () => {
  await windowReady
  await waitFor(`document.getElementById('adapter').textContent !== '正在检查 Radmin 网卡…'`, 'launcher readiness')
  win.show()
  await evaluate(`new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))`)
  assert.equal(await evaluate('typeof require'), 'undefined')
  assert.equal(win.webContents.getLastWebPreferences().contextIsolation, true)
  assert.equal(win.webContents.getLastWebPreferences().sandbox, true)
  record('Electron launcher starts with sandbox, context isolation, and no Node integration')
  fs.writeFileSync(path.join(out, 'electron-connection-center.png'), (await win.capturePage()).toPNG())

  const host = await evaluate(`window.linchatDesktop.startHost('local')`)
  assert.equal(host.running, true)
  assert.ok(host.setupURL.includes('#setup='))
  record('Bundled Go host launches and produces one-time setup URL')
  await navigate(`window.linchatDesktop.connect(${JSON.stringify(host.setupURL)},'本机测试空间')`)
  await waitFor(`document.querySelector('.auth-form-wrap h2')?.textContent.includes('创建')`, 'setup form')
  await fill('input[autocomplete="username"]', 'owner')
  await fill('input[autocomplete="nickname"]', '林同学')
  await fill('input[type="password"]', 'test-owner-password-123')
  await evaluate(`document.querySelector('.auth-form-wrap form').requestSubmit()`)
  await waitFor(`!!document.querySelector('.workspace')`, 'authenticated workspace')
  await waitFor(`document.querySelector('.server-status')?.textContent.includes('实时在线')`, 'websocket connection')
  record('First-run setup completes through Vue UI inside Electron')
  const denied = await evaluate(`window.linchatDesktop.startHost('local').then(()=>false,e=>e.message)`)
  assert.match(denied, /仅能从桌面连接中心/)
  record('Remote chat pages cannot invoke privileged host-management IPC')

  const cookieList = await win.webContents.session.cookies.get({ url: host.url, name: 'linchat_session' })
  const cookie = `linchat_session=${cookieList[0].value}`
  const invite = await request(host.url, '/invites', { uses: 2 }, cookie)
  const bob = await request(host.url, '/auth/register', { username: 'liaoning', nickname: '辽宁的朋友', password: 'test-friend-password-123', token: invite.data.token })
  const nora = await request(host.url, '/auth/register', { username: 'xiaoman', nickname: '小满', password: 'test-xiaoman-password-123', token: invite.data.token })
  await request(host.url, '/conversations/1/messages', { client_id: 'desktop-smoke-msg-00001', content: '大家好，我在辽宁，消息收到啦 👋' }, bob.cookie)
  await request(host.url, '/conversations/1/messages', { client_id: 'desktop-smoke-msg-00002', content: '云端和 Radmin 都可以连接，图片、文件也能直接发到这里。' }, nora.cookie)
  await waitFor(`document.querySelector('.message-list')?.textContent.includes('我在辽宁')`, 'real-time message push')
  await fill('.composer-box textarea', '已经连接成功，欢迎来到 LinChat！\n以后就在这里交流吧。')
  await evaluate(`document.querySelector('.send-button').click()`)
  await waitFor(`document.querySelector('.message-list')?.textContent.includes('已发送') && !document.querySelector('.message.pending')`, 'message acknowledgement')
  const form = new FormData()
  form.append('file', new Blob(['任务,状态\n连接服务端,完成\n多人消息同步,完成\n文件传输,完成\n'], { type: 'text/csv' }), '协作清单.csv')
  const fileResponse = await fetch(host.url + '/api/conversations/1/files?client_id=desktop-smoke-file-0001', {
    method: 'POST', headers: { 'X-LinChat': '1', Origin: host.url, Cookie: bob.cookie }, body: form,
  })
  assert.equal(fileResponse.status, 201)
  await waitFor(`document.querySelector('.message-list')?.textContent.includes('协作清单.csv')`, 'attachment rendering')
  record('Three accounts exchange text and files live in the desktop chat UI')
  fs.writeFileSync(path.join(out, 'electron-multiplayer-chat.png'), (await win.capturePage()).toPNG())

  await evaluate(`document.querySelector('[aria-label="新建群聊"]').click()`)
  await waitFor(`document.querySelector('[role="dialog"]')?.getAttribute('aria-label')==='新建群聊'`, 'group modal')
  await fill('input[placeholder="给你们的讨论空间起个名字"]', '周末交流小组')
  await waitFor(`document.querySelectorAll('.member-picker input[type="checkbox"]').length===2`, 'available group members')
  await evaluate(`(async()=>{for(const e of document.querySelectorAll('.member-picker input[type="checkbox"]')){e.click();await new Promise(resolve=>requestAnimationFrame(resolve))}document.querySelector('.modal form').requestSubmit()})()`)
  await waitFor(`document.querySelector('.chat-title h2')?.textContent.includes('周末交流小组')`, 'new group')
  const groupSummary = await evaluate(`document.querySelector('.chat-title p').textContent`)
  assert.match(groupSummary, /3\s*位成员/)
  record('Group creation and member selection work in Electron')

  await navigate(`window.linchatDesktop.openConnections()`)
  await evaluate(`window.linchatDesktop.stopHost()`)
  const state = await evaluate(`window.linchatDesktop.getState()`)
  if (state.adapters.length) {
    const vpn = await evaluate(`window.linchatDesktop.startHost('radmin')`)
    assert.ok(vpn.url.startsWith('http://26.'))
    const status = await request(vpn.url, '/status')
    assert.equal(status.data.initialized, true)
    await navigate(`window.linchatDesktop.connect(${JSON.stringify(vpn.url)},'Radmin 测试空间')`)
    await waitFor(`document.querySelector('input[autocomplete="username"]')`, 'VPN login')
    await fill('input[autocomplete="username"]', 'owner')
    await fill('input[type="password"]', 'test-owner-password-123')
    await evaluate(`document.querySelector('.auth-form-wrap form').requestSubmit()`)
    await waitFor(`!!document.querySelector('.workspace')`, 'VPN workspace')
    assert.ok(await evaluate(`document.querySelector('.conversation-list').textContent.includes('周末交流小组')`))
    record('Radmin interface binding works and the same local dataset survives host mode switching')
    await navigate(`window.linchatDesktop.openConnections()`)
    await evaluate(`window.linchatDesktop.stopHost()`)
  }
  fs.writeFileSync(path.join(out, 'desktop-smoke-results.json'), JSON.stringify({ timestamp: new Date().toISOString(), results, browserErrors }, null, 2))
  console.log(`Verified ${results.length} desktop scenarios. Screenshots: ${out}`)
  app.quit()
})().catch(async error => {
  console.error(error)
  fs.writeFileSync(path.join(out, 'desktop-smoke-results.json'), JSON.stringify({ results, error: error.stack, browserErrors }, null, 2))
  try {
    if (win && !win.isDestroyed()) {
      fs.writeFileSync(path.join(out, 'desktop-smoke-failure.png'), (await win.capturePage()).toPNG())
      await navigate(`window.linchatDesktop.openConnections()`)
      await evaluate(`window.linchatDesktop.stopHost()`)
    }
  } catch {}
  app.exit(1)
})
