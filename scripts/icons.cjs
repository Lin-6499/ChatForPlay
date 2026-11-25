const fs = require('node:fs')
const path = require('node:path')
const zlib = require('node:zlib')
const dir = path.resolve(__dirname, '../desktop/assets')
fs.mkdirSync(dir, { recursive: true })
const crcTable = Array.from({ length: 256 }, (_, n) => {
  for (let k = 0; k < 8; k++) n = n & 1 ? 0xedb88320 ^ (n >>> 1) : n >>> 1
  return n >>> 0
})
function crc32(b) { let c = 0xffffffff; for (const x of b) c = crcTable[(c ^ x) & 255] ^ (c >>> 8); return (c ^ 0xffffffff) >>> 0 }
function chunk(type, data) {
  const name = Buffer.from(type), length = Buffer.alloc(4), crc = Buffer.alloc(4)
  length.writeUInt32BE(data.length); crc.writeUInt32BE(crc32(Buffer.concat([name, data])))
  return Buffer.concat([length, name, data, crc])
}
function rounded(x, y, l, t, r, b, radius) {
  const cx = Math.max(l + radius, Math.min(r - radius, x)), cy = Math.max(t + radius, Math.min(b - radius, y))
  return x >= l && x <= r && y >= t && y <= b && (x - cx) ** 2 + (y - cy) ** 2 <= radius ** 2
}
function pixel(x, y) {
  if (!rounded(x, y, 0, 0, 64, 64, 16)) return [0, 0, 0, 0]
  const l = x >= 13 && x <= 27 && y >= 18 && y <= 45 && (x <= 20 || y >= 38)
  const leftRounded = (left, top, right, bottom, radius) =>
    x >= left && x <= right && y >= top && y <= bottom &&
    (x >= left + radius || (y >= top + radius && y <= bottom - radius) ||
      (x - left - radius) ** 2 + (y - Math.max(top + radius, Math.min(bottom - radius, y))) ** 2 <= radius ** 2)
  const body = leftRounded(30, 18, 52, 45, 9)
  const counter = leftRounded(37, 25, 64, 38, 3)
  const tail = x >= 43 && x <= 52 && y >= 43 && y <= 95 - x
  const c = (body && !counter && !(x >= 43 && y > 95 - x)) || tail
  return l || c ? [255, 255, 255, 255] : [0, 0, 0, 255]
}
function png(size, origin = '') {
  const raw = Buffer.alloc(size * (size * 4 + 1))
  for (let y = 0; y < size; y++) for (let x = 0; x < size; x++) {
    const sums = [0, 0, 0, 0]
    for (let sy = 0; sy < 4; sy++) for (let sx = 0; sx < 4; sx++) {
      const p = pixel((x + (sx + .5) / 4) * 64 / size, (y + (sy + .5) / 4) * 64 / size)
      for (let k = 0; k < 4; k++) sums[k] += p[k]
    }
    for (let k = 0; k < 4; k++) raw[y * (size * 4 + 1) + 1 + x * 4 + k] = Math.round(sums[k] / 16)
  }
  const ihdr = Buffer.alloc(13); ihdr.writeUInt32BE(size, 0); ihdr.writeUInt32BE(size, 4); ihdr[8] = 8; ihdr[9] = 6
  return Buffer.concat([Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]), chunk('IHDR', ihdr),
    ...(origin ? [chunk('tEXt', Buffer.from('impeccable:prompt\0' + origin, 'utf8'))] : []),
    chunk('IDAT', zlib.deflateSync(raw)), chunk('IEND', Buffer.alloc(0))])
}
const origin = 'Original LinChat black-and-white LC conversation monogram. User selected LC letters fused with dialogue; authored vector geometry and deterministic rasterization in scripts/icons.cjs. White L and open C with a speech tail on a black rounded square. No image model or stock asset used.'
const glyph = '<path d="M13 18h7v20h7v7H13z"/><path d="M52 18H39a9 9 0 0 0-9 9v9a9 9 0 0 0 9 9h4v7l9-9v-5H40a3 3 0 0 1-3-3v-7a3 3 0 0 1 3-3h12z"/>'
const svg = (background, foreground) => `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><rect width="64" height="64" rx="16" fill="${background}"/><g fill="${foreground}">${glyph}</g></svg>\n`
fs.writeFileSync(path.join(dir, 'logo.svg'), svg('#000000', '#ffffff'))
fs.writeFileSync(path.join(dir, 'logo-inverse.svg'), svg('#ffffff', '#000000'))
fs.writeFileSync(path.resolve(__dirname, '../web/public/favicon.svg'), svg('#000000', '#ffffff'))
fs.writeFileSync(path.join(dir, 'icon.png'), png(512, origin))
fs.writeFileSync(path.join(dir, 'tray.png'), png(32, origin))
const sizes = [16, 24, 32, 48, 64, 128, 256], images = sizes.map(size => png(size, origin))
const head = Buffer.alloc(6 + 16 * sizes.length); head.writeUInt16LE(1, 2); head.writeUInt16LE(sizes.length, 4)
let offset = head.length
sizes.forEach((size, i) => {
  const at = 6 + i * 16; head[at] = size % 256; head[at + 1] = size % 256
  head.writeUInt16LE(1, at + 4); head.writeUInt16LE(32, at + 6); head.writeUInt32LE(images[i].length, at + 8); head.writeUInt32LE(offset, at + 12)
  offset += images[i].length
})
fs.writeFileSync(path.join(dir, 'icon.ico'), Buffer.concat([head, ...images]))
console.log('LinChat monochrome LC logo generated: SVG, inverse SVG, 512px PNG, tray PNG, seven-size ICO and web favicon')
