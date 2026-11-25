const fs = require('node:fs')
const path = require('node:path')
const crypto = require('node:crypto')
const assert = require('node:assert/strict')
const asar = require('@electron/asar')
const pkg = require('../package.json')
const base = `dist/desktop/${pkg.version}/win-unpacked`
const archive = base + '/resources/app.asar'
const hash = b => crypto.createHash('sha256').update(b).digest('hex')
const files = []
function walk(dir) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const file = path.join(dir, entry.name)
    if (entry.isDirectory()) walk(file)
    else files.push(file)
  }
}
walk('desktop')
for (const file of files) assert.ok(asar.extractFile(archive, file).equals(fs.readFileSync(file)), file)
const packed = JSON.parse(asar.extractFile(archive, 'package.json'))
assert.equal(packed.version, pkg.version)
assert.equal(packed.main, pkg.main)
assert.deepEqual(packed.dependencies, pkg.dependencies)
assert.ok(fs.readFileSync(base + '/resources/server/linchat-server.exe').equals(fs.readFileSync('dist/server/linchat-windows-amd64.exe')))
assert.ok(!asar.listPackage(archive).some(p => /[\\/]scripts[\\/]/.test(p)))
const report = { passed: true, version: pkg.version, desktop: files.map(file => ({ file, sha256: hash(fs.readFileSync(file)) })),
  asarSha256: hash(fs.readFileSync(archive)), serverSha256: hash(fs.readFileSync('dist/server/linchat-windows-amd64.exe')),
  productionMetadataVerified: true, testEntrypointsExcluded: true }
fs.writeFileSync('dist/verification/streaming/build-binding.json', JSON.stringify(report, null, 2))
console.log('PASS packaged desktop, metadata and server match the verified inputs')
