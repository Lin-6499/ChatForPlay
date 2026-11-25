// Read-only public deployment verification; never authenticates or writes game data.
const fs = require('node:fs')
const assert = require('node:assert/strict')
const crypto = require('node:crypto')
;(async () => {
  const base = 'https://chat.linblog.site'
  const health = await (await fetch(base + '/api/health')).json()
  assert.equal(health.version, require('../package.json').version)
  const html = await (await fetch(base + '/')).text()
  assert.equal(html, fs.readFileSync('web/dist/index.html', 'utf8'))
  const assets = [...html.matchAll(/(?:src|href)="(\/assets\/[^"]+)"/g)].map(m => m[1])
  assert.ok(assets.length >= 2)
  for (const file of assets) {
    const response = await fetch(base + file)
    assert.equal(response.status, 200)
    assert.ok(Buffer.from(await response.arrayBuffer()).equals(fs.readFileSync('web/dist' + file)), file)
  }
  const response = await fetch(base + '/api/v1/campaigns/1/stream')
  assert.equal(response.status, 401)
  const report = { health, assets, assetBytesMatch: true, streamUnauthorized: response.status,
    linuxSha256: crypto.createHash('sha256').update(fs.readFileSync('dist/server/linchat-linux-amd64')).digest('hex') }
  fs.writeFileSync('dist/verification/streaming/cloud.json', JSON.stringify(report, null, 2))
  console.log(report)
})().catch(e => { console.error(e); process.exitCode = 1 })
