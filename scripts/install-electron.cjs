// Recovery installer for environments where @electron/get's downloader stalls.
// Downloads from Electron's official release and verifies the package's pinned SHA-256.
const fs = require('node:fs')
const path = require('node:path')
const { Readable } = require('node:stream')
const { pipeline } = require('node:stream/promises')
const crypto = require('node:crypto')
const root = path.resolve(__dirname, '..')
const electronDir = path.join(root, 'node_modules', 'electron')
const { version } = require(path.join(electronDir, 'package.json'))
const filename = `electron-v${version}-win32-x64.zip`
const expected = require(path.join(electronDir, 'checksums.json'))[filename]
const cache = path.join(root, 'dist', 'downloads')
const archive = path.join(cache, filename)
async function hash(file) {
  const h = crypto.createHash('sha256')
  for await (const chunk of fs.createReadStream(file)) h.update(chunk)
  return h.digest('hex')
}
;(async () => {
  if (!expected) throw new Error('Electron archive checksum is unavailable')
  fs.mkdirSync(cache, { recursive: true })
  if (!fs.existsSync(archive) || await hash(archive) !== expected) {
    const base = process.env.LINCHAT_ELECTRON_MIRROR
    const url = base ? `${base.replace(/\/$/, '')}/${version}/${filename}` : `https://github.com/electron/electron/releases/download/v${version}/${filename}`
    console.log(`Downloading Electron ${version} from ${new URL(url).hostname}; official SHA-256 verification required`)
    const response = await fetch(url, { signal: AbortSignal.timeout(300000) })
    if (!response.ok) throw new Error(`Download returned ${response.status}`)
    await pipeline(Readable.fromWeb(response.body), fs.createWriteStream(archive))
    if (await hash(archive) !== expected) throw new Error('Electron archive checksum mismatch')
  }
  const { extract } = require(path.join(root, 'node_modules', '@electron-internal', 'extract-zip'))
  await extract(archive, { dir: path.join(electronDir, 'dist') })
  fs.writeFileSync(path.join(electronDir, 'path.txt'), 'electron.exe')
  console.log('Electron runtime installed; SHA-256 verified')
})().catch(e => { console.error(e); process.exitCode = 1 })
