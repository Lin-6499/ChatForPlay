const { spawnSync } = require('node:child_process')
const fs = require('node:fs')
const path = require('node:path')
const root = path.resolve(__dirname, '..')
const out = path.join(root, 'dist', 'server')
fs.mkdirSync(out, { recursive: true })
for (const os of ['windows', 'linux']) {
  const file = path.join(out, `linchat-${os}-amd64${os === 'windows' ? '.exe' : ''}`)
  const result = spawnSync('go', ['build', '-trimpath', '-ldflags=-s -w', '-o', file, '.'], {
    cwd: root, env: { ...process.env, GOOS: os, GOARCH: 'amd64', CGO_ENABLED: '0' }, stdio: 'inherit',
  })
  if (result.status !== 0) process.exit(result.status || 1)
  console.log(`Built ${file}`)
}
