const fs = require('node:fs')
const path = require('node:path')
const assert = require('node:assert/strict')
const { spawn } = require('node:child_process')
const root = path.resolve(__dirname, '..')
const out = path.join(root, 'dist/verification/window-state')
const main = process.env.LINCHAT_WINDOW_STATE_MAIN || path.join(root, 'desktop/main.cjs')
async function run(settings) {
  const config = path.join(out, 'settings.json')
  fs.writeFileSync(config, JSON.stringify(settings))
  const env = { ...process.env, LINCHAT_WINDOW_CHECK: config }
  delete env.ELECTRON_RUN_AS_NODE
  await new Promise((resolve, reject) => {
    const child = spawn(require('electron'), [path.join(__dirname, 'window-state-fixture.cjs')], {
      env, windowsHide: true, stdio: ['ignore', 'pipe', 'pipe'],
    })
    let output = ''
    child.stdout.on('data', data => { output += data })
    child.stderr.on('data', data => { output += data })
    const timer = setTimeout(() => { child.kill(); reject(new Error('Hidden window-state fixture timed out')) }, 30000)
    child.once('error', error => { clearTimeout(timer); reject(error) })
    child.once('exit', code => { clearTimeout(timer); code === 0 ? resolve() : reject(new Error(`Hidden fixture failed (${code}): ${output}`)) })
  })
}
;(async () => {
  fs.mkdirSync(out, { recursive: true })
  const profile = fs.mkdtempSync(path.join(out, 'profile-'))
  const profiles = JSON.stringify({ profiles: [{ name: '记忆窗口验证', url: 'https://chat.linblog.site' }] })
  const appearance = JSON.stringify({ version: 1, accent: '#95afd0', background: '#10131a', panel: '#171c26' })
  fs.writeFileSync(path.join(profile, 'connections.json'), profiles)
  fs.writeFileSync(path.join(profile, 'appearance.json'), appearance)
  const results = []
  for (const phase of ['save', 'restore', 'maximized']) {
    if (phase === 'maximized') {
      const file = path.join(profile, 'window-state.json'), value = JSON.parse(fs.readFileSync(file, 'utf8'))
      value.maximized = true
      fs.writeFileSync(file, JSON.stringify(value))
    }
    const report = path.join(out, phase + '.json')
    await run({ main, profile, phase, report })
    const result = JSON.parse(fs.readFileSync(report, 'utf8'))
    assert.equal(result.passed, true)
    results.push(result)
    console.log('PASS hidden Electron window state:', phase)
  }
  assert.equal(fs.readFileSync(path.join(profile, 'connections.json'), 'utf8'), profiles)
  assert.equal(fs.readFileSync(path.join(profile, 'appearance.json'), 'utf8'), appearance)
  fs.writeFileSync(path.join(out, 'results.json'), JSON.stringify({ passed: true, main, profile, results,
    existingPreferencesPreserved: true, visibleWindows: 0, focusedWindows: 0 }, null, 2))
})().catch(error => { console.error(error); process.exitCode = 1 })
