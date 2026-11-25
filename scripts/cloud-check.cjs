const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const crypto = require('node:crypto')
const base = process.env.LINCHAT_CHECK_URL || 'https://chat.linblog.site'
const results = []
const assets = []
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
    if (process.env.LINCHAT_EXPECT_VERSION === require('../package.json').version) {
      for (const [, route] of html.matchAll(/(?:src|href)="(\/assets\/[^"]+\.(?:js|css))"/g)) {
        const downloaded = await fetch(base + route)
        assert.equal(downloaded.status, 200)
        const content = Buffer.from(await downloaded.arrayBuffer())
        const local = fs.readFileSync(path.resolve(__dirname, '../web/dist' + route))
        assert.ok(content.equals(local), `${route} matches the verified build byte-for-byte`)
        assets.push({ route, bytes: content.length, sha256: crypto.createHash('sha256').update(content).digest('hex') })
      }
    }
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
  if (process.env.LINCHAT_EXPECT_VERSION === '1.2.0') await check('Campaign, private notes and model settings require authentication', async () => {
    for (const route of ['/api/v1/campaigns', '/api/v1/campaigns/1/context', '/api/v1/campaigns/1/dm-notes', '/api/v1/entertainment/ai']) {
      const response = await fetch(base + route)
      assert.equal(response.status, 401)
    }
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
  fs.writeFileSync(path.join(out, 'cloud-check-results.json'), JSON.stringify({ url: base, expectedVersion: process.env.LINCHAT_EXPECT_VERSION, timestamp: new Date().toISOString(), results, assets }, null, 2))
})().catch(e => { console.error(e); process.exitCode = 1 })
