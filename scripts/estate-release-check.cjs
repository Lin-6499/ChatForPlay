// No Electron or installer process is launched. Bind tested web/server sources
// to the actual packaged runtime, then preserve checksums for release evidence.
const fs = require('node:fs')
const path = require('node:path')
const crypto = require('node:crypto')
const assert = require('node:assert/strict')
const asar = require('@electron/asar')
const { version } = require('../package.json')
const root = path.resolve(__dirname, '..')
process.chdir(root)
const directory = `dist/desktop/${version}/win-unpacked/resources`
const archive = `${directory}/app.asar`
const runtime = ['desktop/main.cjs', 'desktop/preload.cjs', 'desktop/updates.cjs', 'desktop/window-state.cjs',
  'desktop/scroll-memory.mjs', 'desktop/assets/icon.png', 'desktop/assets/tray.png']
for (const file of runtime) assert.ok(asar.extractFile(archive, path.normalize(file)).equals(fs.readFileSync(file)), file)
const packaged = JSON.parse(asar.extractFile(archive, 'package.json'))
assert.equal(packaged.version, version)
assert.deepEqual(packaged.dependencies, require('../package.json').dependencies)
assert.ok(fs.readFileSync(`${directory}/server/linchat-server.exe`).equals(fs.readFileSync('dist/server/linchat-windows-amd64.exe')))
assert.match(fs.readFileSync(`${directory}/app-update.yml`, 'utf8'), /ChatForPlay/)
const files = [...runtime, 'estate_rules.go', 'estate_api.go', 'estate_test.go', 'store.go', 'main.go',
  'web/src/Estate.vue', 'web/src/EstateBoard.vue', 'web/src/EstateToken.vue', 'web/src/estate.css',
  'web/src/estate-types.ts', 'web/dist/index.html', 'dist/server/linchat-windows-amd64.exe',
  'dist/server/linchat-linux-amd64', archive]
const build = { version, created: new Date().toISOString(), packagedRuntimeVerified: true,
  files: Object.fromEntries(files.map(p => [p, crypto.createHash('sha256').update(fs.readFileSync(p)).digest('hex')])) }
fs.mkdirSync('dist/verification/estate', { recursive: true })
fs.writeFileSync('dist/verification/estate/build-binding.json', JSON.stringify(build, null, 2))
for (const name of ['results.json', 'edge-results.json']) {
  const file = `dist/verification/estate/${name}`
  const result = JSON.parse(fs.readFileSync(file))
  assert.deepEqual(result.errors, [])
  result.build = build
  fs.writeFileSync(file, JSON.stringify(result, null, 2))
}
console.log(`PASS ${version}: packaged runtime, icons, bundled server and update source match; acceptance reports bound to build`)
