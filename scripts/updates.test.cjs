const { test } = require('node:test')
const assert = require('node:assert/strict')
const { newer, releaseInfo, createUpdater } = require('../desktop/updates.cjs')

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
  assert.equal(first, concurrent); assert.equal(calls, 1); assert.equal(updater.getState().checking, true)
  resolve(release())
  assert.equal((await first).available, true)
  await updater.check()
  assert.equal(calls, 1)
  clock += 61000
  const next = updater.check(); resolve(release()); await next
  assert.deepEqual(announcements, ['1.1.0'])
  clock += 61000; fail = true
  const offline = await updater.check()
  assert.equal(offline.error, 'offline'); assert.equal(offline.available, true); assert.equal(offline.checking, false)
  await updater.open()
  assert.deepEqual(opened, [release().assets[0].browser_download_url])
  assert.ok(states.some(s => s.checking))
  const copy = updater.getState(); copy.available = false
  assert.equal(updater.getState().available, true)
})
