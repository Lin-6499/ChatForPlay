const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const base = process.env.LINCHAT_CHECK_URL || 'https://chat.linblog.site'
const results = []
async function check(name, work) {
  await work(); results.push({ name, passed: true }); console.log('PASS', name)
}
;(async () => {
  await check('HTTPS health endpoint and version', async () => {
    const r = await fetch(base + '/api/health', { signal: AbortSignal.timeout(15000) })
    assert.equal(r.status, 200)
    const health = await r.json()
    assert.equal(health.ok, true)
    assert.match(health.version, /^\d+\.\d+\.\d+$/)
    if (process.env.LINCHAT_EXPECT_VERSION) assert.equal(health.version, process.env.LINCHAT_EXPECT_VERSION)
  })
  await check('Production Vue assets served with security headers', async () => {
    const r = await fetch(base + '/', { signal: AbortSignal.timeout(15000) })
    assert.equal(r.status, 200)
    assert.equal(r.headers.get('x-content-type-options'), 'nosniff')
    assert.ok(r.headers.get('content-security-policy').includes("frame-ancestors 'none'"))
    const html = await r.text()
    const script = html.match(/src="(\/assets\/[^"]+\.js)"/)?.[1]
    assert.ok(script, 'compiled Vue entry point exists')
    const js = await fetch(base + script)
    assert.equal(js.status, 200)
    assert.ok((await js.text()).includes('LinChat'))
  })
  await check('Unauthenticated users cannot read conversations or files', async () => {
    for (const route of ['/api/conversations', '/api/users', '/api/files/abcdefghijklmnop']) {
      const r = await fetch(base + route)
      assert.equal(r.status, 401)
    }
  })
  await check('Cross-origin write requests are rejected', async () => {
    const r = await fetch(base + '/api/auth/login', { method: 'POST',
      headers: { 'Content-Type': 'application/json', Origin: 'https://foreign.example', 'X-LinChat': '1' },
      body: JSON.stringify({ username: 'invalid', password: 'invalid' }) })
    assert.equal(r.status, 403)
  })
  await check('Administrator bootstrap remains protected by a one-time token', async () => {
    const status = await (await fetch(base + '/api/status')).json()
    if (!status.initialized) {
      const r = await fetch(base + '/api/setup', { method: 'POST',
        headers: { 'Content-Type': 'application/json', Origin: base, 'X-LinChat': '1' },
        body: JSON.stringify({ username: 'invalid', nickname: 'invalid', password: 'invalid-password', token: 'not-the-bootstrap-token' }) })
      assert.equal(r.status, 403)
    }
  })
  const out = path.resolve(__dirname, '../dist/verification')
  fs.mkdirSync(out, { recursive: true })
  fs.writeFileSync(path.join(out, 'cloud-check-results.json'), JSON.stringify({ url: base, timestamp: new Date().toISOString(), results }, null, 2))
})().catch(e => { console.error(e); process.exitCode = 1 })
