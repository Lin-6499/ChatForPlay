// Browser regression with isolated in-memory conversations and real built Vue UI.
const { chromium } = require('playwright-core')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const http = require('node:http')

const root = path.resolve(__dirname, '..')
const output = path.join(root, 'dist', 'verification', 'reading-position')
const users = [
  { id: 1, username: 'reader', nickname: '林同学', admin: true, online: true },
  { id: 2, username: 'friend', nickname: '小满', admin: false, online: true },
]
const conversations = [
  { id: 1, name: '公共大厅', kind: 'lobby' },
  { id: 2, name: '阅读位置验证群', kind: 'group' },
  { id: 3, name: '空白会话', kind: 'group' },
]
const data = new Map(conversations.map(c => [c.id, Array.from({ length: c.id === 1 ? 520 : c.id === 2 ? 160 : 0 }, (_, i) => ({
  id: c.id * 10000 + i + 1, conversation_id: c.id, sender_id: i % 2 + 1,
  sender_name: users[i % 2].nickname, client_id: `fixture-message-${c.id}-${i}`,
  kind: i === 511 ? 'image' : 'text',
  content: `第 ${i + 1} 条消息 · 我们下次从这里继续。${i % 4 === 0 ? '\n这是一条包含第二行的阅读位置验证消息。' : ''}`,
  file_id: 'reading-fixture', file_name: '验证图片.svg', created_at: 1795590000000 + i * 60000,
}))]))
let userID = 1, holdImage, imageRequested, delayMessages
const reads = [], requests = [], sockets = new Set(), errors = [], results = []
const json = (response, body) => { response.setHeader('Content-Type', 'application/json'); response.end(JSON.stringify(body)) }
const server = http.createServer(async (request, response) => {
  const url = new URL(request.url, 'http://localhost')
  if (url.pathname === '/api/status') return json(response, { name: '阅读位置验证', initialized: true, max_upload: 26214400 })
  if (url.pathname === '/api/me') return json(response, users[userID - 1])
  if (url.pathname === '/api/users') return json(response, users)
  if (url.pathname === '/api/conversations') return json(response, conversations.map(c => ({
    ...c, owner_id: 1, member_count: 2, last_id: data.get(c.id).at(-1)?.id || 0,
    last_at: 1795590000000, preview: '继续阅读上次的对话', unread: 0, last_read: 0,
  })))
  if (url.pathname === '/api/v1/campaigns') return json(response, [])
  if (url.pathname === '/api/files/reading-fixture') {
    imageRequested?.()
    if (holdImage) await holdImage
    response.setHeader('Content-Type', 'image/svg+xml')
    response.end('<svg xmlns="http://www.w3.org/2000/svg" width="300" height="240"><rect width="300" height="240" fill="#577565"/></svg>')
    return
  }
  const match = url.pathname.match(/^\/api\/conversations\/(\d+)(?:\/(messages|read))?$/)
  if (match) {
    const cid = Number(match[1])
    if (!match[2]) return json(response, { members: users })
    if (match[2] === 'read') {
      let body = ''
      for await (const chunk of request) body += chunk
      reads.push({ cid, ...JSON.parse(body) })
      return json(response, { ok: true })
    }
    requests.push(request.url)
    if (delayMessages?.cid === cid) { const delay = delayMessages; delayMessages = null; delay.started(); await delay.promise }
    let items = data.get(cid)
    if (url.searchParams.has('before')) items = items.filter(m => m.id < Number(url.searchParams.get('before')))
    if (url.searchParams.has('after')) items = items.filter(m => m.id > Number(url.searchParams.get('after')))
    return json(response, { messages: url.searchParams.has('after') ? items.slice(0, 100) : items.slice(-100), more: items.length > 100 })
  }
  if (url.pathname.startsWith('/api/')) return json(response, {})
  const file = path.join(root, 'web', 'dist', url.pathname === '/' ? 'index.html' : url.pathname)
  const type = { '.html': 'text/html', '.js': 'text/javascript', '.css': 'text/css', '.svg': 'image/svg+xml' }[path.extname(file)]
  if (!fs.existsSync(file)) { response.writeHead(404); response.end(); return }
  response.setHeader('Content-Type', type || 'application/octet-stream')
  fs.createReadStream(file).pipe(response)
})
const record = text => { results.push(text); console.log('PASS', text) }
const frames = page => page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))))
const position = page => page.locator('.message-list').evaluate(list => {
  const top = list.getBoundingClientRect().top
  const row = [...list.querySelectorAll('[data-message-id]')].find(row => row.getBoundingClientRect().bottom > top)
  return { id: Number(row?.dataset.messageId), offset: row?.getBoundingClientRect().top - top, gap: list.scrollHeight - list.scrollTop - list.clientHeight }
})
async function ready(page) {
  await page.locator('.message-list[aria-busy="false"]').waitFor({ state: 'attached' })
  await frames(page)
}
async function scrollTo(page, id, offset = -19) {
  await page.locator(`[data-message-id="${id}"]`).evaluate((row, offset) => {
    const list = row.closest('.message-list')
    list.scrollTop += row.getBoundingClientRect().top - list.getBoundingClientRect().top - offset
  }, offset)
  await frames(page)
  return position(page)
}
async function samePosition(page, saved) {
  await ready(page)
  const actual = await position(page)
  assert.equal(actual.id, saved.id, JSON.stringify({ saved, actual }))
  assert.ok(Math.abs(actual.offset - saved.offset) <= 2, JSON.stringify({ saved, actual }))
}
async function select(page, cid) {
  await page.locator('.conversation-row').filter({ hasText: conversations[cid - 1].name }).click()
  await ready(page)
}
function push(cid) {
  const items = data.get(cid)
  const m = { ...items.at(-1), id: items.at(-1).id + 1, kind: 'text', content: '新到达的消息', client_id: `fixture-live-${items.length}` }
  items.push(m)
  for (const socket of sockets) socket.send(JSON.stringify({ type: 'message', message: m }))
  return m
}

;(async () => {
  fs.mkdirSync(output, { recursive: true })
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve))
  const url = `http://127.0.0.1:${server.address().port}`
  const executablePath = ['C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe', 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe'].find(p => fs.existsSync(p))
  const browser = await chromium.launch({ executablePath, headless: true })
  let page
  try {
    const context = await browser.newContext({ viewport: { width: 1264, height: 811 } })
    await context.routeWebSocket('**/api/ws', socket => {
      sockets.add(socket)
      socket.onClose(() => sockets.delete(socket))
      socket.send(JSON.stringify({ type: 'hello' }))
    })
    const openPage = async (beforeLoad) => {
      const p = await context.newPage()
      p.on('pageerror', error => errors.push(error.message))
      if (beforeLoad) await p.addInitScript(beforeLoad)
      await p.goto(url)
      await ready(p)
      return p
    }
    page = await openPage()
    assert.ok((await position(page)).gap <= 1)
    const first = await scrollTo(page, 10465)
    await select(page, 2)
    const second = await scrollTo(page, 20110, -31)
    await select(page, 1)
    await samePosition(page, first)
    await select(page, 1)
    await samePosition(page, first)
    await select(page, 2)
    await samePosition(page, second)
    record('Each conversation restores its own anchor and offset, including re-selecting the current chat')

    await select(page, 1)
    await page.getByRole('button', { name: '娱乐模式', exact: true }).click()
    await page.locator('.message-list').waitFor({ state: 'detached' })
    await page.getByRole('button', { name: '消息', exact: true }).click()
    await samePosition(page, first)
    await page.reload()
    await samePosition(page, first)
    await page.close()
    page = await openPage()
    await samePosition(page, first)
    record('Entertainment round trip, reload and closing/reopening the page retain the reading position')

    for (let i = 0; i < 3; i++) {
      await page.locator('.message-list').evaluate(list => { list.scrollTop = 0 })
      await frames(page)
      await page.getByRole('button', { name: '查看更早的消息', exact: true }).click()
      await ready(page)
    }
    const old = await scrollTo(page, 10140)
    await select(page, 2)
    const beforeRequests = requests.length, beforeReads = reads.length
    await select(page, 1)
    await samePosition(page, old)
    assert.ok(await page.locator('[data-message-id]').count() <= 200)
    assert.ok(requests.slice(beforeRequests).filter(r => r.includes('/1/messages')).length <= 3)
    assert.equal(reads.slice(beforeReads).filter(r => r.cid === 1).length, 0)
    push(1)
    await frames(page)
    await samePosition(page, old)
    assert.equal(await page.locator('[data-message-id="10521"]').count(), 0)
    record('Old history restores with bounded pages, preserves unread state and does not splice live messages across a gap')

    await page.getByRole('button', { name: '查看更新的消息', exact: true }).click()
    await ready(page)
    assert.ok(await page.locator('[data-message-id="10339"]').count())
    await page.getByRole('button', { name: '回到最新消息', exact: true }).click()
    await ready(page)
    assert.ok((await position(page)).gap <= 1)
    assert.ok(await page.locator('[data-message-id="10521"]').count())
    record('Newer history pages and return-to-latest load contiguous messages')

    const latestAnchor = await scrollTo(page, 10470)
    const incoming = push(1)
    await page.locator(`[data-message-id="${incoming.id}"]`).waitFor()
    await samePosition(page, latestAnchor)
    await page.locator('.message-list').evaluate(list => { list.scrollTop = list.scrollHeight })
    await frames(page)
    const next = push(1)
    await page.locator(`[data-message-id="${next.id}"]`).waitFor()
    await frames(page)
    assert.ok((await position(page)).gap <= 1)
    await select(page, 2)
    push(1)
    await select(page, 1)
    assert.ok((await position(page)).gap <= 1)
    record('New messages leave a history reader in place and follow the end only for a bottom-pinned conversation')

    const imageAnchor = await scrollTo(page, 10513, -13)
    let releaseImage
    holdImage = new Promise(resolve => { releaseImage = resolve })
    const requested = new Promise(resolve => { imageRequested = resolve })
    await page.reload({ waitUntil: 'domcontentloaded' })
    await ready(page)
    await requested
    await samePosition(page, imageAnchor)
    holdImage = null; releaseImage()
    await page.locator('.image-bubble img').evaluate(img => img.decode())
    await frames(page)
    await samePosition(page, imageAnchor)
    record('Delayed image dimensions preserve the same message and pixel offset')

    const resizeAnchor = await scrollTo(page, 10475, -25)
    await select(page, 1)
    await page.setViewportSize({ width: 390, height: 780 })
    await frames(page)
    await samePosition(page, resizeAnchor)
    await page.getByRole('button', { name: '返回会话列表', exact: true }).click()
    await select(page, 2)
    await page.getByRole('button', { name: '返回会话列表', exact: true }).click()
    await select(page, 1)
    await samePosition(page, resizeAnchor)
    assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth))
    await page.screenshot({ path: path.join(output, 'mobile.png') })
    record('390px mobile return-to-list and viewport changes preserve position without horizontal overflow')

    await page.setViewportSize({ width: 1264, height: 811 })
    await frames(page)
    const beforeRace = await position(page)
    let releaseMessages, started
    const began = new Promise(resolve => { started = resolve })
    delayMessages = { cid: 2, started, promise: new Promise(resolve => { releaseMessages = resolve }) }
    await page.locator('.conversation-row').filter({ hasText: conversations[1].name }).click()
    await began
    await select(page, 1)
    releaseMessages()
    await samePosition(page, beforeRace)
    record('A delayed response from a previously selected conversation cannot overwrite the active history or position')

    userID = 2
    await page.reload()
    await ready(page)
    assert.ok((await position(page)).gap <= 1)
    userID = 1
    await page.reload()
    await samePosition(page, beforeRace)
    await page.screenshot({ path: path.join(output, 'desktop.png') })
    record('Accounts on the same server retain independent reading positions')
    await select(page, 3)
    assert.equal(await page.locator('[data-message-id]').count(), 0)
    await page.screenshot({ path: path.join(output, 'empty.png') })
    await select(page, 1)
    await samePosition(page, beforeRace)
    let releaseHistory, historyStarted
    const historyBegan = new Promise(resolve => { historyStarted = resolve })
    delayMessages = { cid: 1, started: historyStarted, promise: new Promise(resolve => { releaseHistory = resolve }) }
    await page.locator('.message-list').evaluate(list => { list.scrollTop = 0 })
    await frames(page)
    await page.getByRole('button', { name: '查看更早的消息', exact: true }).click()
    await historyBegan
    const duringPagination = push(1)
    releaseHistory()
    await ready(page)
    await page.locator(`[data-message-id="${duringPagination.id}"]`).waitFor()
    record('Messages arriving during a history request are recovered without a reconnect')
    // Close the old page before corrupting storage; its pending save otherwise
    // competes with this deliberate corruption test in the shared origin.
    await page.close()
    page = await openPage(() => localStorage.setItem('linchat-reading:1:1', '{"messageID":0,"offset":0,"bottom":false}'))
    const recovered = await position(page)
    assert.ok(recovered.gap <= 1, JSON.stringify({ recovered, stored: await page.evaluate(() => localStorage.getItem('linchat-reading:1:1')) }))
    assert.deepEqual(errors, [])
    record('Empty conversations and malformed saved positions remain usable; no uncaught browser errors')
    fs.writeFileSync(path.join(output, 'results.json'), JSON.stringify({ results, errors }, null, 2))
  } catch (error) {
    await page?.screenshot({ path: path.join(output, 'failure.png') }).catch(() => {})
    fs.writeFileSync(path.join(output, 'failure.json'), JSON.stringify({ error: error.stack, results, errors, requests: requests.slice(-20), position: await position(page).catch(() => null) }, null, 2))
    throw error
  } finally {
    await browser.close()
    server.closeAllConnections()
    await new Promise(resolve => server.close(resolve))
  }
})().catch(error => { console.error(error); process.exitCode = 1 })
