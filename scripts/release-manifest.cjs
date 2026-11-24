const fs = require('node:fs')
const path = require('node:path')
const crypto = require('node:crypto')
const root = path.resolve(__dirname, '..')
const files = [
  'dist/desktop/LinChat-Setup-1.0.0.exe',
  'dist/desktop/LinChat-Portable-1.0.0.exe',
  'dist/desktop/win-unpacked/LinChat.exe',
  'dist/server/linchat-windows-amd64.exe',
  'dist/server/linchat-linux-amd64',
]
;(async () => {
  const artifacts = []
  for (const file of files) {
    const full = path.join(root, file), h = crypto.createHash('sha256')
    for await (const b of fs.createReadStream(full)) h.update(b)
    artifacts.push({ path: file, bytes: fs.statSync(full).size, sha256: h.digest('hex') })
  }
  const manifest = { version: '1.0.0', created: new Date().toISOString(), cloud: 'https://chat.linblog.site', artifacts,
    verification: ['dist/verification/desktop-smoke-results.json', 'dist/verification/cloud-check-results.json', 'dist/verification/packaged-check-results.json'] }
  fs.writeFileSync(path.join(root, 'dist', 'release-manifest.json'), JSON.stringify(manifest, null, 2))
  console.log(JSON.stringify(manifest, null, 2))
})().catch(e => { console.error(e); process.exitCode = 1 })
