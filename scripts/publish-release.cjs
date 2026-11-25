// Publish desktop installers and server artifacts using the existing GitHub Git credential.
// Credentials stay in memory and are never written to files or printed.
const fs = require('node:fs')
const path = require('node:path')
const crypto = require('node:crypto')
const { spawnSync } = require('node:child_process')
const { fetch, ProxyAgent } = require('undici')
const { verifyUpdateManifest } = require('./verify-update-manifest.cjs')
const root = path.resolve(__dirname, '..')
const { version } = require(path.join(root, 'package.json'))
const owner = 'Lin-6499', repository = 'ChatForPlay'
const apiBase = `https://api.github.com/repos/${owner}/${repository}`
const proxy = process.env.HTTPS_PROXY || process.env.https_proxy
const dispatcher = proxy ? new ProxyAgent(proxy) : undefined
let token

function git(args, input) {
  const result = spawnSync('git', args, { cwd: root, input, encoding: 'utf8',
    env: { ...process.env, GIT_TERMINAL_PROMPT: '0', GCM_INTERACTIVE: 'never' } })
  if (result.status !== 0) throw new Error(`Git ${args[0]} failed. Check GitHub sign-in and repository permissions.`)
  return result.stdout.trim()
}
function credential() {
  if (process.env.GH_TOKEN || process.env.GITHUB_TOKEN) return process.env.GH_TOKEN || process.env.GITHUB_TOKEN
  const result = git(['-c', 'credential.interactive=never', 'credential', 'fill'],
    `protocol=https\nhost=github.com\npath=${owner}/${repository}.git\n\n`)
  const password = result.split(/\r?\n/).find(line => line.startsWith('password='))
  if (!password) throw new Error('No GitHub credential is available in Git Credential Manager.')
  return password.slice('password='.length)
}
async function request(url, options = {}) {
  return fetch(url, { ...options, dispatcher, signal: options.signal || AbortSignal.timeout(60000),
    headers: { Accept: 'application/vnd.github+json', 'User-Agent': 'LinChat-release-publisher',
      'X-GitHub-Api-Version': '2022-11-28', Authorization: `Bearer ${token}`, ...options.headers } })
}
async function api(route, method = 'GET', body) {
  const response = await request(apiBase + route, { method,
    ...(body ? { headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) } : {}) })
  const result = await response.json()
  if (!response.ok) throw new Error(`GitHub ${method} ${route}: ${response.status} ${result.message || ''}`)
  return result
}
async function digest(file) {
  const h = crypto.createHash('sha256')
  for await (const data of fs.createReadStream(file)) h.update(data)
  return h.digest('hex')
}

;(async () => {
  if (git(['status', '--porcelain'])) throw new Error('Commit the release source and manifest before publishing.')
  token = credential()
  const commit = git(['rev-parse', 'HEAD'])
  const remote = git(['-c', 'credential.interactive=never', 'ls-remote', 'origin', 'refs/heads/main']).split(/\s+/)[0]
  if (remote !== commit) throw new Error('Push this release commit to origin/main before publishing.')
  const manifest = JSON.parse(fs.readFileSync(path.join(root, 'desktop-release.json'), 'utf8'))
  if (manifest.version !== version) throw new Error('Rebuild and regenerate the release manifest first.')
  const installer = path.join(root, 'dist', 'desktop', version, manifest.filename)
  await verifyUpdateManifest(path.dirname(installer), version)
  if (await digest(installer) !== manifest.sha256) throw new Error('Installer SHA-256 does not match the pinned download manifest.')
  const files = [
    installer,
    path.join(root, 'dist', 'desktop', version, `LinChat-Portable-${version}.exe`),
    path.join(root, 'dist', 'desktop', version, 'latest.yml'),
    path.join(root, 'dist', 'desktop', version, `LinChat-Setup-${version}.exe.blockmap`),
    path.join(root, 'dist', 'server', 'linchat-windows-amd64.exe'),
    path.join(root, 'dist', 'server', 'linchat-linux-amd64'),
    path.join(root, 'deploy', 'update-cloud.sh'),
    path.join(root, 'desktop-release.json'),
    path.join(root, 'dist', 'SHA256SUMS.txt'),
  ]
  const lookup = await request(`${apiBase}/releases/tags/v${version}`)
  let release
  if (lookup.status === 404) {
    release = await api('/releases', 'POST', {
      tag_name: `v${version}`, target_commitish: commit, name: `LinChat ${version}`, draft: true, prerelease: false,
      body: fs.existsSync(path.join(root, 'releases', version + '.md')) ? fs.readFileSync(path.join(root, 'releases', version + '.md'), 'utf8') : [
        '## Windows 一键安装',
        '',
        `**直接使用：下载并双击 \`LinChat-Setup-${version}.exe\`，安装后自动打开 LinChat。**`,
        '',
        '- 自动安装到当前用户目录，并创建桌面和开始菜单快捷方式。',
        '- 从仓库下载源码的用户：完整解压后双击「一键安装使用.cmd」，自动下载、校验、安装并启动。',
        '- 已安装同版或更新版本时直接打开；首次使用无需安装 Node.js、Go 或执行 npm 命令。',
        `- 便携使用可以选择 \`LinChat-Portable-${version}.exe\`。`,
        '- 云端地址：https://chat.linblog.site',
        '- 新成员使用管理员提供的邀请链接注册，然后在客户端登录自己的账号。',
        '- 同时支持 Radmin 虚拟局域网及内置本机主机。',
        '',
        '下载校验值见 `SHA256SUMS.txt`，一键安装脚本使用仓库内固定 SHA-256 校验安装包。',
      ].join('\n'),
    })
    console.log(`Created draft release v${version}`)
  } else if (lookup.ok) {
    release = await lookup.json()
    if (!release.draft) throw new Error(`Release v${version} is already published; use a new version for changes.`)
    if (release.target_commitish !== commit && release.target_commitish !== 'main') {
      throw new Error('An existing draft targets another commit; inspect it before publishing.')
    }
  } else {
    throw new Error(`Release lookup failed: HTTP ${lookup.status}`)
  }
  const uploadBase = release.upload_url.split('{')[0]
  let assets = await api(`/releases/${release.id}/assets`)
  for (const file of files) {
    const name = path.basename(file)
    const sha256 = await digest(file)
    const size = fs.statSync(file).size
    const existing = assets.find(asset => asset.name === name)
    if (existing) {
      if (existing.state === 'uploaded' && existing.size === size && existing.digest === `sha256:${sha256}`) {
        console.log(`Verified existing asset ${name}`)
        continue
      }
      throw new Error(`Draft asset ${name} differs or is incomplete. Inspect this draft before replacing it.`)
    }
    console.log(`Uploading ${name} (${Math.ceil(size / 1048576)} MB)`)
    const response = await request(uploadBase + '?name=' + encodeURIComponent(name), {
      method: 'POST', body: fs.createReadStream(file), duplex: 'half',
      signal: AbortSignal.timeout(20 * 60 * 1000),
      headers: { 'Content-Type': name.endsWith('.exe') || name === 'linchat-linux-amd64' ? 'application/octet-stream' : 'text/plain', 'Content-Length': String(size) },
    })
    const asset = await response.json()
    if (!response.ok) throw new Error(`Uploading ${name}: HTTP ${response.status} ${asset.message || ''}`)
    if (asset.state !== 'uploaded' || asset.size !== size || asset.digest !== `sha256:${sha256}`) {
      throw new Error(`GitHub did not confirm the expected SHA-256 for ${name}. The release remains a draft.`)
    }
    assets.push(asset)
    console.log(`Uploaded and SHA-256 verified: ${name}`)
  }
  release = await api(`/releases/${release.id}`, 'PATCH', { draft: false, make_latest: 'true' })
  assets = await api(`/releases/${release.id}/assets`)
  const report = { version, commit, release: release.html_url, publishedAt: release.published_at,
    assets: assets.map(asset => ({ name: asset.name, size: asset.size, digest: asset.digest, url: asset.browser_download_url })) }
  fs.writeFileSync(path.join(root, 'dist', 'github-release.json'), JSON.stringify(report, null, 2) + '\n')
  console.log(`Published ${release.html_url}`)
})().catch(error => { console.error(error.message); process.exitCode = 1 }).finally(async () => {
  token = undefined
  if (dispatcher) await dispatcher.close()
})
