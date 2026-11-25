const RELEASE_API = 'https://api.github.com/repos/Lin-6499/ChatForPlay/releases/latest'

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

function createUpdater({ currentVersion, fetchRelease, changed, announce, openExternal, now = Date.now }) {
  let state = { currentVersion, latestVersion: '', available: false, checking: false, checkedAt: 0, error: '' }
  let downloadURL = '', pending = null, lastAttempt = null, announced = ''
  const publish = () => changed?.({ ...state })
  const getState = () => ({ ...state })
  function check() {
    if (pending) return pending
    if (lastAttempt !== null && now() - lastAttempt < 60_000) return Promise.resolve(getState())
    lastAttempt = now()
    state = { ...state, checking: true, error: '' }; publish()
    pending = (async () => {
      try {
        const info = releaseInfo(await fetchRelease(), currentVersion)
        downloadURL = info.downloadURL
        state = { ...state, latestVersion: info.latestVersion, available: info.available, checkedAt: now() }
        if (info.available && announced !== info.latestVersion) {
          announced = info.latestVersion
          announce?.(info.latestVersion)
        }
      } catch (error) {
        state = { ...state, error: error.message || '无法检查更新，请稍后重试' }
      } finally {
        state = { ...state, checking: false }; publish(); pending = null
      }
      return getState()
    })()
    return pending
  }
  async function open() {
    if (!state.available || !downloadURL) throw new Error('当前没有可下载的更新')
    await openExternal(downloadURL)
  }
  return { getState, check, open }
}

module.exports = { RELEASE_API, releaseInfo, newer, createUpdater }
