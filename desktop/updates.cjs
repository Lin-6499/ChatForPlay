const fs = require('node:fs')
const crypto = require('node:crypto')
const RELEASE_API = 'https://api.github.com/repos/Lin-6499/ChatForPlay/releases/latest'
const UPDATE_FEED = { provider: 'github', owner: 'Lin-6499', repo: 'ChatForPlay' }

function versionParts(value) {
  if (typeof value !== 'string' || !/^\d+\.\d+\.\d+$/.test(value)) return null
  const parts = value.split('.').map(Number)
  return parts.every(Number.isSafeInteger) ? parts : null
}

function newer(candidate, current) {
  const a = versionParts(candidate), b = versionParts(current)
  if (!a || !b) return false
  for (let i = 0; i < 3; i++) {
    if (a[i] !== b[i]) return a[i] > b[i]
  }
  return false
}

function releaseInfo(release, current) {
  if (!release || release.draft || release.prerelease) throw new Error('暂未找到可用的正式版本')
  const latestVersion = String(release.tag_name || '').replace(/^v/, '')
  if (!versionParts(latestVersion)) throw new Error('更新信息格式无效，请稍后重试')
  if (!newer(latestVersion, current)) return { latestVersion, available: false, downloadURL: '' }
  const filename = `LinChat-Setup-${latestVersion}.exe`
  const expected = `https://github.com/Lin-6499/ChatForPlay/releases/download/v${latestVersion}/${filename}`
  const asset = release.assets?.find(a => a.name === filename && a.state === 'uploaded')
  if (!asset || asset.browser_download_url !== expected || !Number.isSafeInteger(asset.size) || asset.size <= 0) {
    throw new Error('新版本安装包尚未就绪，请稍后重试')
  }
  return { latestVersion, available: true, downloadURL: expected }
}

function validateUpdate(info, current) {
  if (!info || !newer(info.version, current)) throw new Error('更新版本无效')
  const name = `LinChat-Setup-${info.version}.exe`
  const file = info.files?.find(file => file.url === name)
  if (info.packages || info.files?.length !== 1 || !file ||
      !/^[A-Za-z0-9+/]{86}==$/.test(file.sha512 || '') ||
      !Number.isSafeInteger(file.size) || file.size <= 0) {
    throw new Error('更新安装包信息无效，请稍后重试')
  }
  return file
}

async function verifyDownload(event, current) {
  const file = validateUpdate(event, current)
  const hash = crypto.createHash('sha512')
  let bytes = 0
  for await (const chunk of fs.createReadStream(event.downloadedFile)) { bytes += chunk.length; hash.update(chunk) }
  if (bytes !== file.size || hash.digest('base64') !== file.sha512) {
    // electron-updater trusts an existing file in its in-memory cache. Remove
    // only this rejected cache file so Retry performs a fresh, verified download.
    await fs.promises.unlink(event.downloadedFile).catch(() => {})
    throw Object.assign(new Error('更新安装包校验失败，请重新下载'), { code: 'ERR_UPDATER_CHECKSUM_MISMATCH' })
  }
}

function updateError(error, phase) {
  const text = String(error?.message || '')
  if (/checksum|sha512|校验失败/i.test(text)) return '更新包校验失败，请重试下载。当前版本仍可使用。'
  if (/ENOSPC/.test(text)) return '磁盘空间不足，请清理空间后重试更新。'
  if (/EACCES|EPERM/.test(text)) return '无法写入更新文件，请检查目录权限后重试。'
  if (/403|429/.test(text)) return '更新服务暂时限流，请稍后重试。'
  if (/^更新|^本机服务/.test(text)) return text
  return phase === 'installing' ? '无法安装更新，请重试。当前版本仍可使用。' :
    phase === 'downloading' ? '更新下载失败，请检查网络后重试。当前版本仍可使用。' :
      '无法连接更新服务，请检查网络后重试'
}

function createUpdater({ currentVersion, fetchRelease, engine, changed, announce, openExternal,
  prepareInstall = async () => {}, verify = event => verifyDownload(event, currentVersion),
  network = task => task(), now = Date.now }) {
  let state = { currentVersion, latestVersion: '', available: false, checking: false, checkedAt: 0, error: '',
    autoUpdate: !!engine, status: 'idle', progress: 0, transferred: 0, total: 0 }
  let downloadURL = '', pending = null, downloading = null, installing = null, downloaded = null
  let lastAttempt = null, announced = ''
  const publish = () => changed?.({ ...state })
  const getState = () => ({ ...state })
  const set = value => { state = { ...state, ...value }; publish() }
  const fail = (error, phase) => set({ status: 'error', error: updateError(error, phase), checking: false })
  const notify = () => {
    if (announced === state.latestVersion) return
    announced = state.latestVersion
    announce?.(state.latestVersion, state.status)
  }
  if (engine) {
    engine.autoDownload = false
    // Installation is explicit, after the managed host has flushed its database.
    engine.autoInstallOnAppQuit = false
    engine.allowPrerelease = false
    engine.allowDowngrade = false
    engine.on('download-progress', progress => {
      if (state.status !== 'downloading') return
      set({ progress: Math.max(0, Math.min(100, Number(progress.percent) || 0)),
        transferred: progress.transferred, total: progress.total })
    })
    engine.on('update-downloaded', event => {
      try {
        validateUpdate(event, currentVersion)
        downloaded = event
        set({ status: 'ready', progress: 100, error: '' })
        notify()
      } catch (error) { downloaded = null; fail(error, 'downloading') }
    })
    // Check/download errors are handled by their promises (including proxy retry).
    engine.on('error', error => { if (state.status === 'installing') fail(error, 'installing') })
  }
  function download() {
    if (downloading) return downloading
    if (!engine || !state.available || ['ready', 'installing'].includes(state.status)) return Promise.resolve(getState())
    downloaded = null
    set({ status: 'downloading', error: '', progress: 0, transferred: 0, total: 0 })
    downloading = Promise.resolve().then(() => network(() => engine.downloadUpdate()))
      .catch(error => fail(error, 'downloading'))
      .finally(() => { downloading = null })
      .then(getState)
    return downloading
  }
  function check() {
    if (pending) return pending
    if (downloading || installing || state.status === 'ready') return Promise.resolve(getState())
    if (lastAttempt !== null && now() - lastAttempt < 60_000) return Promise.resolve(getState())
    lastAttempt = now()
    set({ checking: true, status: 'checking', error: '', ...(engine ? { available: false } : {}) })
    pending = Promise.resolve().then(async () => {
      try {
        if (engine) {
          const result = await network(() => engine.checkForUpdates())
          if (!result?.updateInfo) throw new Error('更新服务未返回版本信息')
          const info = result.updateInfo
          const available = newer(info.version, currentVersion)
          if (available) validateUpdate(info, currentVersion)
          set({ latestVersion: info.version, available, checkedAt: now(), status: available ? 'available' : 'idle' })
          if (available) void download()
        } else {
          const info = releaseInfo(await fetchRelease(), currentVersion)
          downloadURL = info.downloadURL
          set({ latestVersion: info.latestVersion, available: info.available, checkedAt: now(),
            status: info.available ? 'available' : 'idle' })
          if (info.available) notify()
        }
      } catch (error) {
        fail(error, 'checking')
      } finally {
        set({ checking: false }); pending = null
      }
      return getState()
    })
    return pending
  }
  function install() {
    if (installing) return installing
    if (!engine || state.status !== 'ready' || !downloaded) return Promise.reject(new Error('更新尚未下载并校验完成'))
    set({ status: 'installing', error: '' })
    installing = Promise.resolve().then(async () => {
      await verify(downloaded)
      await prepareInstall()
      engine.quitAndInstall(true, true)
    }).catch(error => {
      downloaded = null
      fail(error, 'installing')
    }).finally(() => { installing = null }).then(getState)
    return installing
  }
  async function open() {
    if (engine) {
      if (state.status === 'ready' || installing) return install()
      if (downloading) return getState()
      if (state.available) return download()
      return check()
    }
    if (!state.available || !downloadURL) throw new Error('当前没有可下载的更新')
    await openExternal(downloadURL)
  }
  return { getState, check, open, download, install }
}

module.exports = { RELEASE_API, UPDATE_FEED, releaseInfo, newer, validateUpdate, verifyDownload, createUpdater }
