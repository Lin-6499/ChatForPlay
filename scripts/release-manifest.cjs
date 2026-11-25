const fs = require('node:fs')
const path = require('node:path')
const crypto = require('node:crypto')
const { verifyUpdateManifest } = require('./verify-update-manifest.cjs')
const root = path.resolve(__dirname, '..')
const { version } = require(path.join(root, 'package.json'))
const desktopDirectory = `dist/desktop/${version}`
const files = [
  `${desktopDirectory}/LinChat-Setup-${version}.exe`,
  `${desktopDirectory}/LinChat-Portable-${version}.exe`,
  `${desktopDirectory}/latest.yml`,
  `${desktopDirectory}/LinChat-Setup-${version}.exe.blockmap`,
  `${desktopDirectory}/win-unpacked/LinChat.exe`,
  'dist/server/linchat-windows-amd64.exe',
  'dist/server/linchat-linux-amd64',
  'deploy/update-cloud.sh',
]
;(async () => {
  await verifyUpdateManifest(path.join(root, desktopDirectory), version)
  const artifacts = []
  for (const file of files) {
    const full = path.join(root, file), h = crypto.createHash('sha256')
    for await (const b of fs.createReadStream(full)) h.update(b)
    artifacts.push({ path: file, bytes: fs.statSync(full).size, sha256: h.digest('hex') })
  }
  const manifest = { version, created: new Date().toISOString(), cloud: 'https://chat.linblog.site', artifacts,
    verification: ['dist/verification/streaming/results.json', 'dist/verification/streaming/build-binding.json', 'dist/verification/quick-replies/results.json', 'dist/verification/entertainment-check-results.json'],
    verificationScope: 'Headless web/preload checks and hidden Electron geometry lifecycle; native maximize presentation intercepted. No visible client or installer launched.' }
  fs.writeFileSync(path.join(root, 'dist', 'release-manifest.json'), JSON.stringify(manifest, null, 2))
  const installer = artifacts[0]
  const filename = path.basename(installer.path)
  const desktop = { version, filename, bytes: installer.bytes, sha256: installer.sha256,
    url: `https://github.com/Lin-6499/ChatForPlay/releases/download/v${version}/${filename}` }
  fs.writeFileSync(path.join(root, 'desktop-release.json'), JSON.stringify(desktop, null, 2) + '\n')
  fs.writeFileSync(path.join(root, 'dist', 'SHA256SUMS.txt'), artifacts.filter(a => !a.path.includes('/win-unpacked/')).map(a => `${a.sha256}  ${path.basename(a.path)}`).join('\n') + '\n')
  console.log(JSON.stringify(manifest, null, 2))
})().catch(e => { console.error(e); process.exitCode = 1 })
