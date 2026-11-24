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
  if (!rounded(x, y, 1, 1, 63, 63, 17)) return [0, 0, 0, 0]
  let c = [24, 62, 54, 255]
  if (rounded(x, y, 16, 16, 48, 40, 6) || (x >= 20 && x <= 32 && y >= 35 && y <= 49 - (x - 20) * .75)) c = [197, 240, 177, 255]
  if (rounded(x, y, 24, 24, 40, 27, 1.5) || rounded(x, y, 24, 31, 35, 34, 1.5)) c = [24, 62, 54, 255]
  return c
}
function png(size) {
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
  return Buffer.concat([Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]), chunk('IHDR', ihdr), chunk('IDAT', zlib.deflateSync(raw)), chunk('IEND', Buffer.alloc(0))])
}
fs.writeFileSync(path.join(dir, 'icon.png'), png(256))
fs.writeFileSync(path.join(dir, 'tray.png'), png(32))
const sizes = [16, 32, 48, 64, 128, 256], images = sizes.map(png)
const head = Buffer.alloc(6 + 16 * sizes.length); head.writeUInt16LE(1, 2); head.writeUInt16LE(sizes.length, 4)
let offset = head.length
sizes.forEach((size, i) => {
  const at = 6 + i * 16; head[at] = size % 256; head[at + 1] = size % 256
  head.writeUInt16LE(1, at + 4); head.writeUInt16LE(32, at + 6); head.writeUInt32LE(images[i].length, at + 8); head.writeUInt32LE(offset, at + 12)
  offset += images[i].length
})
fs.writeFileSync(path.join(dir, 'icon.ico'), Buffer.concat([head, ...images]))
console.log('LinChat desktop icons generated')
