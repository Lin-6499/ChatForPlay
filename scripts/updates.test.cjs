const { test } = require('node:test')
const assert = require('node:assert/strict')
const { newer, releaseInfo, createUpdater, validateUpdate, verifyDownload } = require('../desktop/updates.cjs')
const { EventEmitter } = require('node:events')
const fs = require('node:fs')
const path = require('node:path')
const os = require('node:os')
const crypto = require('node:crypto')

function release(version = '1.1.0') {
  const name = `LinChat-Setup-${version}.exe`
  return { tag_name: 'v' + version, draft: false, prerelease: false, assets: [{
    name, state: 'uploaded', size: 100000,
    browser_download_url: `https://github.com/Lin-6499/ChatForPlay/releases/download/v${version}/${name}`,
  }] }
}

test('compares stable versions numerically and rejects downgrade/prerelease', () => {
  assert.equal(newer('1.10.0', '1.9.9'), true)
  assert.equal(newer('2.0.0', '1.99.99'), true)
  assert.equal(newer('1.1.0', '1.1.0'), false)
  assert.equal(newer('1.0.1', '1.1.0'), false)
  assert.equal(newer('2.0.0-beta', '1.1.0'), false)
  assert.equal(newer('invalid', '1.1.0'), false)
  assert.equal(releaseInfo(release('1.0.1'), '1.1.0').available, false)
})

test('offers only the published installer on this repository', () => {
  assert.equal(releaseInfo(release(), '1.0.1').available, true)
  for (const modify of [
    r => { r.draft = true }, r => { r.prerelease = true }, r => { r.assets = [] },
    r => { r.assets[0].browser_download_url = 'https://evil.example/LinChat.exe' },
    r => { r.assets[0].browser_download_url = 'file:///C:/Windows/notepad.exe' },
    r => { r.assets[0].state = 'new' }, r => { r.assets[0].size = 0 },
    r => { r.assets[0].name = 'LinChat-Setup-9.0.0.exe' },
  ]) {
    const data = release(); modify(data)
    assert.throws(() => releaseInfo(data, '1.0.1'))
  }
})

test('coalesces checks, throttles requests, announces once and retains verified updates offline', async () => {
  let resolve, calls = 0, clock = 100, fail = false
  const announcements = [], opened = [], states = []
  const updater = createUpdater({
    currentVersion: '1.0.1', now: () => clock,
    fetchRelease: () => { calls++; if (fail) throw Error('offline'); return new Promise(r => { resolve = r }) },
    changed: state => states.push(state),
    announce: version => announcements.push(version), openExternal: url => opened.push(url),
  })
  await assert.rejects(() => updater.open())
  const first = updater.check(), concurrent = updater.check()
  await Promise.resolve()
  assert.equal(first, concurrent); assert.equal(calls, 1); assert.equal(updater.getState().checking, true)
  resolve(release())
  assert.equal((await first).available, true)
  await updater.check()
  assert.equal(calls, 1)
  clock += 61000
  const next = updater.check(); await Promise.resolve(); resolve(release()); await next
  assert.deepEqual(announcements, ['1.1.0'])
  clock += 61000; fail = true
  const offline = await updater.check()
  assert.match(offline.error, /无法连接/); assert.equal(offline.available, true); assert.equal(offline.checking, false)
  await updater.open()
  assert.deepEqual(opened, [release().assets[0].browser_download_url])
  assert.ok(states.some(s => s.checking))
  const copy = updater.getState(); copy.available = false
  assert.equal(updater.getState().available, true)
})

const payload = Buffer.from('verified installer fixture')
function info(version = '1.2.2') {
  return { version, files: [{ url: `LinChat-Setup-${version}.exe`, size: payload.length,
    sha512: crypto.createHash('sha512').update(payload).digest('base64') }] }
}
function fixture(options = {}) {
  const engine = new EventEmitter(), order = [], states = []
  engine.checkForUpdates = async () => { order.push('check'); return { updateInfo: info() } }
  engine.downloadUpdate = async () => {
    order.push('download')
    engine.emit('download-progress', { percent: 54, transferred: 10, total: payload.length })
    engine.emit('update-downloaded', { ...info(), downloadedFile: 'fixture.exe' })
  }
  engine.quitAndInstall = (silent, restart) => { order.push(['install', silent, restart]) }
  const updater = createUpdater({ currentVersion: '1.2.1', engine,
    changed: state => states.push(state), verify: async () => { order.push('verify') },
    prepareInstall: async () => { order.push('host-stopped') }, ...options })
  return { updater, engine, order, states }
}

test('automatically downloads, reports progress, waits for user, then verifies and stops host before silent restart', async () => {
  const announcements = []
  const { updater, engine, order, states } = fixture({ announce: (...args) => announcements.push(args) })
  await assert.rejects(updater.install, /尚未下载/)
  await updater.check()
  await updater.download()
  assert.equal(engine.autoInstallOnAppQuit, false)
  assert.equal(engine.allowDowngrade, false)
  assert.equal(engine.autoDownload, false)
  assert.ok(states.some(s => s.status === 'downloading' && s.progress === 54))
  assert.equal(updater.getState().status, 'ready')
  assert.deepEqual(order, ['check', 'download'])
  await updater.check()
  assert.deepEqual(announcements, [['1.2.2', 'ready']])
  await Promise.all([updater.open(), updater.install(), updater.install()])
  assert.deepEqual(order, ['check', 'download', 'verify', 'host-stopped', ['install', true, true]])
})

test('network failure can retry without duplicate downloads or dropping into an installer', async () => {
  const { updater, engine, order } = fixture()
  const success = engine.downloadUpdate
  engine.downloadUpdate = async () => { throw Error('net::ERR_CONNECTION_RESET') }
  await updater.check(); await updater.download()
  assert.equal(updater.getState().status, 'error')
  assert.match(updater.getState().error, /重试/)
  await assert.rejects(updater.install)
  engine.downloadUpdate = success
  await Promise.all([updater.open(), updater.open()])
  assert.equal(updater.getState().status, 'ready')
  assert.deepEqual(order, ['check', 'download'])
})

test('invalid feed, downgrade and prerelease never download an executable', async () => {
  for (const change of [
    data => { data.files[0].url = 'https://other.example/installer.exe' },
    data => { data.files[0].url = '../LinChat-Setup-1.2.2.exe' },
    data => { data.files[0].sha512 = '' },
    data => { data.files[0].size = 0 },
    data => { data.packages = { x64: {} } },
  ]) {
    const data = info(); change(data)
    assert.throws(() => validateUpdate(data, '1.2.1'))
    const { updater, engine, order } = fixture()
    engine.checkForUpdates = async () => ({ updateInfo: data })
    await updater.check()
    assert.equal(updater.getState().status, 'error')
    assert.deepEqual(order, [])
  }
  for (const version of ['1.2.0', '1.2.1', '2.0.0-beta.1']) {
    const { updater, engine, order } = fixture()
    engine.checkForUpdates = async () => ({ updateInfo: info(version) })
    assert.equal((await updater.check()).available, false)
    assert.deepEqual(order, [])
  }
})

test('install is blocked by modified cached bytes or an unflushed host; application can retry', async () => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'linchat-update-'))
  const file = path.join(directory, 'installer.exe')
  try {
    fs.writeFileSync(file, payload)
    await verifyDownload({ ...info(), downloadedFile: file }, '1.2.1')
    fs.writeFileSync(file, 'tampered')
    const { updater, engine, order } = fixture({ verify: () => verifyDownload({ ...info(), downloadedFile: file }, '1.2.1') })
    await updater.check(); await updater.download(); await updater.install()
    assert.equal(updater.getState().status, 'error')
    assert.match(updater.getState().error, /校验失败/)
    assert.deepEqual(order, ['check', 'download'])
    engine.emit('error', Error('late transport error')) // handled, never an unhandled EventEmitter error
    const stopped = fixture({ prepareInstall: async () => { throw Error('本机服务尚未停止，请稍后重试更新。') } })
    await stopped.updater.check(); await stopped.updater.download(); await stopped.updater.install()
    assert.equal(stopped.updater.getState().status, 'error')
    assert.match(stopped.updater.getState().error, /本机服务/)
    assert.deepEqual(stopped.order, ['check', 'download', 'verify'])
  } finally { fs.rmSync(directory, { recursive: true, force: true }) }
})
