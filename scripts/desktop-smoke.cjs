const { app, BrowserWindow, session, Notification } = require('electron')
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
const notifications = []
Notification.prototype.show = function () { notifications.push({ title: this.title, body: this.body }) }
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
async function capture(name) {
  await evaluate(`new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))`)
  fs.writeFileSync(path.join(out, name), (await win.capturePage()).toPNG())
}
async function pointerClick(selector) {
  const point = await evaluate(`(() => {
    const e=document.querySelector(${JSON.stringify(selector)}), r=e.getBoundingClientRect();
    const x=Math.round(r.left+r.width/2), y=Math.round(r.top+r.height/2);
    return {x,y,visible:r.width>0&&r.height>0&&x>=0&&y>=0&&x<innerWidth&&y<innerHeight,
      reachable:e.contains(document.elementFromPoint(x,y))};
  })()`)
  assert.ok(point.visible && point.reachable, `${selector} must remain visible and pointer-accessible`)
  win.focus()
  win.webContents.sendInputEvent({ type: 'mouseDown', x: point.x, y: point.y, button: 'left', clickCount: 1 })
  win.webContents.sendInputEvent({ type: 'mouseUp', x: point.x, y: point.y, button: 'left', clickCount: 1 })
}
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
  await capture('electron-connection-center.png')
  const updateNetwork = session.fromPartition('linchat-updates')
  await updateNetwork.protocol.handle('https', () => new Response(JSON.stringify({
    tag_name: 'v99.0.0', draft: false, prerelease: false, assets: [{
      name: 'LinChat-Setup-99.0.0.exe', state: 'uploaded', size: 100000,
      browser_download_url: 'https://github.com/Lin-6499/ChatForPlay/releases/download/v99.0.0/LinChat-Setup-99.0.0.exe',
    }],
  }), { headers: { 'Content-Type': 'application/json' } }))
  const update = await evaluate(`window.linchatDesktop.checkUpdates()`)
  assert.equal(update.available, true)
  assert.equal(update.latestVersion, '99.0.0')
  await waitFor(`!document.querySelector('#update-banner').hidden`, 'desktop update reminder')
  assert.ok(notifications.some(n => n.title.includes('99.0.0')))
  record('Desktop update IPC and reminder use a validated release; native notification is captured without showing test alerts')
  await capture('electron-update-reminder.png')

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
  await capture('electron-update-chat-desktop.png')
  await pointerClick('.send-button')
  await waitFor(`document.querySelector('.message-list')?.textContent.includes('已发送') && !document.querySelector('.message.pending')`, 'message acknowledgement')
  const form = new FormData()
  form.append('file', new Blob(['任务,状态\n连接服务端,完成\n多人消息同步,完成\n文件传输,完成\n'], { type: 'text/csv' }), '协作清单.csv')
  const fileResponse = await fetch(host.url + '/api/conversations/1/files?client_id=desktop-smoke-file-0001', {
    method: 'POST', headers: { 'X-LinChat': '1', Origin: host.url, Cookie: bob.cookie }, body: form,
  })
  assert.equal(fileResponse.status, 201)
  await waitFor(`document.querySelector('.message-list')?.textContent.includes('协作清单.csv')`, 'attachment rendering')
  record('Three accounts exchange text and files live in the desktop chat UI')
  await capture('electron-multiplayer-chat.png')
  win.setMinimumSize(320, 480)
  win.setContentSize(390, 780)
  await waitFor(`innerWidth===390`, 'narrow layout with available update')
  await pointerClick('.conversation-row.selected')
  await fill('.composer-box textarea', '更新提示打开时，窄屏也可以正常发送。')
  await capture('electron-update-chat-mobile.png')
  await pointerClick('.send-button')
  await waitFor(`document.querySelector('.message-list')?.textContent.includes('窄屏也可以正常发送') && !document.querySelector('.message.pending')`, 'pointer send with update visible')
  assert.ok(await evaluate(`!!document.querySelector('.desktop-update-notice') && document.documentElement.scrollWidth<=innerWidth && document.documentElement.scrollHeight<=innerHeight`))
  record('Desktop and narrow-screen Send buttons remain hit-testable with the update reminder open')
  win.setContentSize(1264, 811)
  await waitFor(`innerWidth===1264`, 'desktop layout restored')

  await evaluate(`document.querySelector('[aria-label="新建群聊"]').click()`)
  await waitFor(`document.querySelector('[role="dialog"]')?.getAttribute('aria-label')==='新建群聊'`, 'group modal')
  await fill('input[placeholder="给你们的讨论空间起个名字"]', '周末交流小组')
  await waitFor(`document.querySelectorAll('.member-picker input[type="checkbox"]').length===2`, 'available group members')
  await evaluate(`(async()=>{for(const e of document.querySelectorAll('.member-picker input[type="checkbox"]')){e.click();await new Promise(resolve=>requestAnimationFrame(resolve))}document.querySelector('.modal form').requestSubmit()})()`)
  await waitFor(`document.querySelector('.chat-title h2')?.textContent.includes('周末交流小组')`, 'new group')
  const groupSummary = await evaluate(`document.querySelector('.chat-title p').textContent`)
  assert.match(groupSummary, /3\s*位成员/)
  record('Group creation and member selection work in Electron')
  await waitFor(`document.querySelector('.desktop-update-notice')?.textContent.includes('99.0.0')`, 'update reminder survives navigation into chat')
  await evaluate(`document.querySelector('[aria-label="暂时关闭更新提醒"]').click()`)
  await fill('.composer-box textarea', '@辽')
  await waitFor(`document.querySelectorAll('.mention-option').length===1 && document.querySelector('.mention-option').textContent.includes('辽宁的朋友')`, 'nickname mention search')
  // IME confirmation must not send a message or select the highlighted member.
  await evaluate(`document.querySelector('.composer-box textarea').dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',isComposing:true,bubbles:true,cancelable:true}))`)
  assert.equal(await evaluate(`document.querySelector('.composer-box textarea').value`), '@辽')
  await evaluate(`document.querySelector('.composer-box textarea').dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',bubbles:true,cancelable:true}))`)
  await waitFor(`document.querySelector('.composer-box textarea').value==='@辽宁的朋友 '`, 'keyboard member selection')
  assert.equal(await evaluate(`document.querySelectorAll('.message').length`), 0)
  await fill('.composer-box textarea', '@辽宁的朋友 请看 👋 @xiaoman')
  await waitFor(`document.querySelectorAll('.mention-option').length===1 && document.querySelector('.mention-option').textContent.includes('小满')`, 'username mention search')
  await evaluate(`document.querySelector('.mention-option').click()`)
  await waitFor(`document.querySelector('.composer-box textarea').value.endsWith('@小满 ')`, 'second member selection')
  await fill('.composer-box textarea', '@辽宁的朋友 请看 👋 @小满 一起讨论周末安排。')
  await evaluate(`document.querySelector('.send-button').click()`)
  await waitFor(`document.querySelectorAll('.message:not(.pending) .mention-text').length===2`, 'durable multi-person mention rendering')
  const all = await request(host.url, '/conversations', null, cookie)
  const group = all.data.find(c => c.name === '周末交流小组')
  const groupHistory = await request(host.url, `/conversations/${group.id}/messages`, null, bob.cookie)
  assert.deepEqual(groupHistory.data.messages[0].mentions.map(m => m.user_id), [2, 3])
  record('Group member search, keyboard selection, IME handling and multi-person mentions work in Electron')
  await evaluate(`Array.from(document.querySelectorAll('.conversation-row')).find(e=>e.textContent.includes('公共大厅')).click()`)
  await waitFor(`document.querySelector('.chat-title h2')?.textContent.includes('公共大厅')`, 'lobby before incoming mention')
  await request(host.url, `/conversations/${group.id}/messages`, {
    client_id: 'desktop-mention-incoming-0001', content: '@林同学 收到了，我们在群里讨论。',
    mentions: [{ user_id: 1, start: 0, end: 4 }],
  }, bob.cookie)
  await waitFor(`!!document.querySelector('.mention-unread')`, 'unread @me badge in background conversation')
  assert.ok((await evaluate(`document.querySelector('.toast')?.textContent || ''`)).includes('@了你'))
  await evaluate(`Array.from(document.querySelectorAll('.conversation-row')).find(e=>e.textContent.includes('周末交流小组')).click()`)
  await waitFor(`!!document.querySelector('.message.mentioned .mention-self')`, 'received mention highlights')
  await waitFor(`!document.querySelector('.mention-unread')`, 'read acknowledgement clears mention badge')
  win.hide()
  await waitFor(`document.hidden`, 'desktop window hidden for mention notification')
  await request(host.url, `/conversations/${group.id}/messages`, {
    client_id: 'desktop-mention-notification-0001', content: '@林同学 明天的时间已经确认，方便时看一下。',
    mentions: [{ user_id: 1, start: 0, end: 4 }],
  }, nora.cookie)
  await waitFor(`document.querySelectorAll('.message.mentioned').length===2`, 'background incoming mention')
  assert.ok(notifications.some(n => n.title.includes('@了你') && n.body.includes('时间已经确认')))
  win.show()
  await waitFor(`!document.hidden`, 'desktop window visible again')
  await fill('.composer-box textarea', '@')
  await waitFor(`document.querySelectorAll('.mention-option').length===2`, 'group mention candidates')
  await capture('electron-mentions-desktop.png')
  win.setMinimumSize(320, 480)
  win.setContentSize(390, 780)
  await waitFor(`innerWidth===390`, 'mobile layout')
  await evaluate(`new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))`)
  const mobileLayout = await evaluate(`(() => {const r=document.querySelector('.mention-picker').getBoundingClientRect();return {left:r.left,right:r.right,top:r.top,scroll:document.documentElement.scrollWidth,width:innerWidth}})()`)
  assert.ok(mobileLayout.left >= 0 && mobileLayout.right <= mobileLayout.width && mobileLayout.top >= 0 && mobileLayout.scroll <= mobileLayout.width)
  await capture('electron-mentions-mobile.png')
  await evaluate(`document.querySelector('.composer-box textarea').dispatchEvent(new KeyboardEvent('keydown',{key:'Escape',bubbles:true,cancelable:true}))`)
  assert.equal(await evaluate(`!!document.querySelector('.mention-picker')`), false)
  record('Incoming @me badge, highlight, read clearing and narrow-screen mention popup render correctly')
  win.setContentSize(1264, 811)

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
  fs.writeFileSync(path.join(out, 'desktop-smoke-results.json'), JSON.stringify({ timestamp: new Date().toISOString(), results, browserErrors, notifications }, null, 2))
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
