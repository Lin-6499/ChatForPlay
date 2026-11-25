const { app, BrowserWindow, Menu, Tray, ipcMain, shell, Notification, clipboard, nativeImage, session, dialog } = require('electron')
const { RELEASE_API, UPDATE_FEED, createUpdater } = require('./updates.cjs')
const { spawn } = require('node:child_process')
const fs = require('node:fs')
const path = require('node:path')
const os = require('node:os')
const net = require('node:net')
const { pathToFileURL } = require('node:url')

const root = path.resolve(__dirname, '..')
if (!app.isPackaged) app.setPath('userData', process.env.LINCHAT_USER_DATA || path.join(root, '.desktop-data'))
const launcherPath = path.join(__dirname, 'index.html')
const launcherURL = pathToFileURL(launcherPath).href
const iconPath = path.join(__dirname, 'assets', 'icon.png')
let win, tray, host, quitting = false, shutdownStarted = false, selectedOrigin = ''
let hostState = { running: false, starting: false, url: '', setupURL: '', mode: '', error: '' }
let profiles = []
let updater, updateTimer, updateInterval, stoppingHost
const gotLock = app.requestSingleInstanceLock()
if (!gotLock) app.quit()

function configPath() { return path.join(app.getPath('userData'), 'connections.json') }
function hostData() { return path.join(app.getPath('userData'), 'server-data') }
function updateResumePath() { return path.join(app.getPath('userData'), 'update-resume.json') }
async function resumeAfterUpdate() {
  let resume
  try {
    resume = JSON.parse(fs.readFileSync(updateResumePath(), 'utf8'))
    fs.unlinkSync(updateResumePath())
  } catch { return }
  if (resume.version !== app.getVersion() || Date.now() - resume.created > 86400000) return
  try {
    if (resume.hostMode) await startHost(resume.hostMode)
    if (resume.localChat && hostState.running) await connect(hostState.setupURL || hostState.url)
    else if (resume.origin) await connect(resume.origin)
  } catch (error) {
    hostState.error = `更新已完成，但恢复连接失败：${error.message}`
    await openConnections()
  }
}
function isLauncher(event) {
  return event.sender === win?.webContents && event.senderFrame?.url.split('#')[0] === launcherURL
}
function requireLauncher(event) {
  if (!isLauncher(event)) throw new Error('此操作仅能从桌面连接中心执行')
}
function trustedChat(event) {
  try { return event.sender === win?.webContents && new URL(event.senderFrame.url).origin === selectedOrigin }
  catch { return false }
}
function requireDesktopPage(event) {
  if (!isLauncher(event) && !trustedChat(event)) throw new Error('来源无效')
}
async function fetchRelease() {
  const network = session.fromPartition('linchat-updates')
  let failure
  for (const mode of ['system', 'direct']) {
    await network.setProxy({ mode })
    await network.closeAllConnections()
    try {
      const response = await network.fetch(RELEASE_API, {
        headers: { Accept: 'application/vnd.github+json', 'User-Agent': `LinChat/${app.getVersion()}` },
        signal: AbortSignal.timeout(12000),
      })
      if (!response.ok) {
        if (response.status === 403 || response.status === 429) throw new Error('更新服务暂时限流，请稍后重试')
        throw new Error(`更新服务暂不可用（HTTP ${response.status}），请稍后重试`)
      }
      return await response.json()
    } catch (error) { failure = error }
  }
  throw new Error(/更新服务/.test(failure?.message || '') ? failure.message : '无法连接更新服务，请检查网络后重试')
}
async function checkUpdatesFromMenu() {
  const state = await updater.check()
  showWindow()
  const ready = state.status === 'ready'
  const action = ready || (state.available && !['downloading', 'installing'].includes(state.status))
  const response = await dialog.showMessageBox(win, {
    type: state.error ? 'warning' : 'info', title: 'LinChat · 检查更新',
    message: state.error || (ready ? `v${state.latestVersion} 已下载，等待重启` :
      state.status === 'downloading' ? `正在后台下载 v${state.latestVersion}（${Math.floor(state.progress)}%）` :
        state.available ? `发现新版本 v${state.latestVersion}` : `当前已是最新版本 v${state.currentVersion}`),
    detail: state.autoUpdate ? '新版会自动下载并校验。点击“重启并更新”后静默安装，保留安装目录和本机数据；本机服务会暂时停止。' :
      '当前为便携版或开发环境，请下载安装版以启用应用内自动更新。',
    buttons: action ? [ready ? '重启并更新' : state.autoUpdate ? '重试下载' : '下载安装版', '稍后'] : ['知道了'],
    defaultId: 0, cancelId: action ? 1 : 0,
  })
  if (action && response.response === 0) await updater.open()
}
function readProfiles() {
  try { profiles = JSON.parse(fs.readFileSync(configPath(), 'utf8')).profiles.filter(p => typeof p.url === 'string' && typeof p.name === 'string') }
  catch { profiles = [{ name: 'LinChat 云端', url: 'https://chat.linblog.site' }] }
}
function saveProfiles() {
  fs.mkdirSync(app.getPath('userData'), { recursive: true })
  fs.writeFileSync(configPath(), JSON.stringify({ profiles }, null, 2), 'utf8')
}
function validURL(raw) {
  if (typeof raw !== 'string' || raw.length > 1000) throw new Error('连接地址无效')
  const u = new URL(raw)
  if (!['http:', 'https:'].includes(u.protocol) || u.username || u.password || !['', '/'].includes(u.pathname)) throw new Error('请输入 http:// 或 https:// 开头的服务端地址')
  return u
}
function localNetwork(hostname) {
  if (hostname === 'localhost' || hostname === '[::1]') return true
  if (net.isIP(hostname) !== 4) return false
  const [a, b] = hostname.split('.').map(Number)
  return a === 26 || a === 10 || a === 127 || (a === 192 && b === 168) || (a === 172 && b >= 16 && b <= 31)
}
function showWindow() { if (win) { if (win.isMinimized()) win.restore(); win.show(); win.focus() } }
async function openConnections() {
  selectedOrigin = ''
  await win.loadFile(launcherPath)
  showWindow()
}
async function connect(raw, name) {
  const u = validURL(raw)
  const hash = u.hash
  selectedOrigin = u.origin
  // Virtual/private addresses must use their actual network adapter instead of a
  // system HTTP proxy. This setting is scoped to LinChat's own browser session.
  await win.webContents.session.setProxy({ mode: localNetwork(u.hostname) ? 'direct' : 'system' })
  await win.webContents.session.closeAllConnections()
  await win.webContents.session.clearHostResolverCache()
  profiles = [{ name: String(name || u.hostname).slice(0, 60), url: u.origin }, ...profiles.filter(p => p.url !== u.origin)].slice(0, 12)
  saveProfiles()
  let failure
  try { await win.loadURL(u.origin + '/' + hash); return }
  catch (error) { failure = error }
  // A broken system proxy must not prevent access to an otherwise reachable
  // cloud server. Only retry transport failures; TLS validation stays enabled.
  if (!localNetwork(u.hostname) && /ERR_(PROXY|TUNNEL|NAME_NOT_RESOLVED|CONNECTION|TIMED_OUT|ADDRESS_UNREACHABLE|SOCKET)/.test(failure?.code || failure?.message || '')) {
    await win.webContents.session.setProxy({ mode: 'direct' })
    await win.webContents.session.closeAllConnections()
    await win.webContents.session.clearHostResolverCache()
    try { await win.loadURL(u.origin + '/' + hash); return }
    catch (error) { failure = error }
  }
  {
    const reason = String(failure?.code || '').slice(0, 80)
    hostState.error = `无法连接 ${u.origin}${reason ? `（${reason}）` : ''}。请检查服务端、VPN 网络和防火墙端口。`
    await openConnections()
    throw new Error(hostState.error)
  }
}
function publishHostState() {
  if (win && !win.isDestroyed() && win.webContents.getURL().split('#')[0] === launcherURL) {
    win.webContents.send('host:state', hostState)
  }
  if (tray) tray.setToolTip(hostState.running ? `LinChat · 本机服务端 ${hostState.url}` : 'LinChat · 让交流近一点')
}
function serverExecutable() {
  return app.isPackaged
    ? path.join(process.resourcesPath, 'server', 'linchat-server.exe')
    : path.join(root, 'dist', 'server', `linchat-${process.platform === 'win32' ? 'windows-amd64.exe' : 'linux-amd64'}`)
}
async function startHost(mode) {
  if (quitting || updater?.getState().status === 'installing') throw new Error('正在准备退出，请在更新完成后启动本机服务')
  if (hostState.running) return hostState
  if (hostState.starting) throw new Error('服务端正在启动')
  if (!['radmin', 'local'].includes(mode)) throw new Error('部署模式无效')
  const exe = serverExecutable()
  if (!fs.existsSync(exe)) throw new Error('未找到内置服务端，请先完成项目构建')
  const data = hostData()
  fs.mkdirSync(data, { recursive: true })
  const args = ['-data', data, '-name', 'LinChat · 我的空间', '-managed']
  if (mode === 'radmin') args.push('-mode', 'radmin')
  else args.push('-listen', '127.0.0.1:8787', '-url', 'http://127.0.0.1:8787')
  hostState = { running: false, starting: true, url: '', setupURL: '', mode, error: '' }
  publishHostState()
  return new Promise((resolve, reject) => {
    const child = spawn(exe, args, { windowsHide: true, stdio: ['pipe', 'pipe', 'pipe'] })
    host = child
    let finished = false, tail = '', buffer = ''
    const deadline = setTimeout(() => {
      if (!finished) { child.kill(); fail('启动服务端超时，请检查端口或查看服务端日志') }
    }, 20000)
    const fail = message => {
      if (finished) return
      finished = true; clearTimeout(deadline)
      hostState = { ...hostState, running: false, starting: false, error: message }
      publishHostState(); reject(new Error(message))
    }
    const receive = chunk => {
      buffer += chunk.toString('utf8')
      let at
      while ((at = buffer.indexOf('\n')) >= 0) {
        const line = buffer.slice(0, at); buffer = buffer.slice(at + 1)
        if (!line.includes('首次初始化地址')) tail = (tail + '\n' + line).slice(-2500)
        const found = line.match(/LinChat 已启动：(https?:\/\/\S+) \|/)
        if (found && !finished) {
          finished = true; clearTimeout(deadline)
          let setupURL = ''
          try { setupURL = fs.readFileSync(path.join(data, '首次初始化.txt'), 'utf8').trim() } catch {}
          hostState = { running: true, starting: false, url: found[1], setupURL, mode, error: '' }
          publishHostState(); resolve(hostState)
        }
      }
    }
    child.stdout.setEncoding('utf8'); child.stderr.setEncoding('utf8')
    child.stdout.on('data', receive); child.stderr.on('data', receive)
    child.on('error', err => fail(err.message))
    child.on('exit', code => {
      clearTimeout(deadline)
      if (host === child) host = null
      if (!finished) fail(tail.trim() || `服务端退出，代码 ${code}`)
      else {
        hostState = { ...hostState, running: false, starting: false, setupURL: '',
          error: code && !quitting ? tail.trim() || '服务端意外退出' : '' }
        publishHostState()
      }
    })
  })
}
async function stopHost({ forUpdate = false } = {}) {
  if (stoppingHost) return stoppingHost
  const child = host
  if (!child) return
  stoppingHost = new Promise((resolve, reject) => {
    const exited = () => { clearTimeout(timer); resolve() }
    const timer = setTimeout(() => {
      if (forUpdate) {
        child.removeListener('exit', exited)
        reject(new Error('本机服务尚未停止，请稍后重试更新。'))
      } else child.kill()
    }, 12000)
    child.once('exit', exited)
    if (child.stdin.writable) child.stdin.end('shutdown\n')
    else if (!forUpdate) child.kill()
  }).then(() => {
    hostState.running = false; hostState.starting = false; hostState.setupURL = ''
    publishHostState()
  }).finally(() => { stoppingHost = null })
  return stoppingHost
}

app.on('second-instance', showWindow)
app.on('before-quit', event => {
  clearTimeout(updateTimer); clearInterval(updateInterval)
  quitting = true
  if (host) {
    event.preventDefault()
    if (!shutdownStarted) {
      shutdownStarted = true
      stopHost().finally(() => { shutdownStarted = false; app.quit() })
    }
  }
})
app.on('window-all-closed', () => { if (quitting) app.quit() })
app.whenReady().then(async () => {
  if (!gotLock) return
  const { normalizeAppearance, normalizeColor } = await import('./theme.mjs')
  app.setAppUserModelId('site.linblog.linchat')
  readProfiles()
  const partition = session.fromPartition('persist:linchat')
  partition.setPermissionRequestHandler((contents, permission, callback) => {
    try { callback(contents === win?.webContents && new URL(contents.getURL()).origin === selectedOrigin && permission === 'notifications') }
    catch { callback(false) }
  })
  partition.setPermissionCheckHandler((contents, permission, requestingOrigin) =>
    contents === win?.webContents && permission === 'notifications' && requestingOrigin === selectedOrigin)
  win = new BrowserWindow({
    width: 1280, height: 850, minWidth: 820, minHeight: 600, show: false,
    title: 'LinChat · 让交流近一点', backgroundColor: '#111615', icon: iconPath,
    autoHideMenuBar: true,
    webPreferences: { preload: path.join(__dirname, 'preload.cjs'), contextIsolation: true, nodeIntegration: false,
      sandbox: true, webSecurity: true, partition: 'persist:linchat', spellcheck: false },
  })
  win.on('close', event => { if (!quitting) { event.preventDefault(); win.hide() } })
  win.once('ready-to-show', showWindow)
  const installed = app.isPackaged && process.platform === 'win32' && !process.env.PORTABLE_EXECUTABLE_FILE &&
    fs.existsSync(path.join(path.dirname(process.execPath), `Uninstall ${path.basename(process.execPath)}`))
  const engine = installed ? new (require('electron-updater').NsisUpdater)(UPDATE_FEED) : null
  if (engine) {
    engine.installDirectory = path.dirname(process.execPath)
    engine.disableWebInstaller = true
  }
  updater = createUpdater({
    currentVersion: app.getVersion(), fetchRelease, engine,
    network: async task => {
      let failure
      for (const mode of ['system', 'direct']) {
        await engine.netSession.setProxy({ mode })
        await engine.netSession.closeAllConnections()
        try { return await task() } catch (error) {
          failure = error
          if (!/net::|ECONN|ETIMEDOUT|ENOTFOUND|socket|timeout|network/i.test(String(error.message))) throw error
        }
      }
      throw failure
    },
    prepareInstall: async () => {
      const resume = { version: updater.getState().latestVersion, created: Date.now(),
        hostMode: hostState.running || hostState.starting ? hostState.mode : '',
        localChat: !!selectedOrigin && selectedOrigin === hostState.url, origin: selectedOrigin }
      await stopHost({ forUpdate: true })
      fs.mkdirSync(app.getPath('userData'), { recursive: true })
      fs.writeFileSync(updateResumePath(), JSON.stringify(resume), 'utf8')
      quitting = true
    },
    changed: state => {
      if (state.status === 'error') quitting = false
      if (win && !win.isDestroyed()) {
        const url = win.webContents.getURL()
        if (url.split('#')[0] === launcherURL || (selectedOrigin && url.startsWith(selectedOrigin + '/'))) {
          win.webContents.send('updates:state', state)
        }
      }
    },
    announce: (version, status) => {
      if (!Notification.isSupported()) return
      const notice = new Notification({ title: `LinChat v${version} ${status === 'ready' ? '已就绪' : '可更新'}`,
        body: status === 'ready' ? '新版已下载并校验。打开 LinChat，选择“重启并更新”。' : '下载安装版以启用应用内自动更新。', icon: iconPath })
      notice.on('click', () => { void checkUpdatesFromMenu() })
      notice.show()
    },
    openExternal: url => shell.openExternal(url),
  })
  if (app.isPackaged) {
    updateTimer = setTimeout(() => { void updater.check() }, 10000)
    updateInterval = setInterval(() => { void updater.check() }, 6 * 60 * 60 * 1000)
  }
  win.webContents.setWindowOpenHandler(({ url }) => {
    try { if (['https:', 'http:'].includes(new URL(url).protocol)) void shell.openExternal(url) } catch {}
    return { action: 'deny' }
  })
  win.webContents.on('will-navigate', (event, url) => {
    try {
      const u = new URL(url)
      if (u.origin !== selectedOrigin && url !== launcherURL) {
        event.preventDefault()
        if (['https:', 'http:'].includes(u.protocol)) void shell.openExternal(url)
      }
    } catch { event.preventDefault() }
  })
  win.webContents.on('will-attach-webview', event => event.preventDefault())
  win.webContents.on('render-process-gone', () => { selectedOrigin = ''; void openConnections() })
  const menu = Menu.buildFromTemplate([
    { label: 'LinChat', submenu: [
      { label: '连接中心', accelerator: 'CommandOrControl+Shift+H', click: () => openConnections() },
      { label: '显示窗口', click: showWindow },
      { label: '检查更新', click: () => { void checkUpdatesFromMenu() } },
      { type: 'separator' }, { label: '退出 LinChat', accelerator: 'CommandOrControl+Q', click: () => app.quit() },
    ] },
    { label: '编辑', submenu: [{ role: 'undo', label: '撤销' }, { role: 'redo', label: '重做' }, { type: 'separator' },
      { role: 'cut', label: '剪切' }, { role: 'copy', label: '复制' }, { role: 'paste', label: '粘贴' }, { role: 'selectAll', label: '全选' }] },
    { label: '查看', submenu: [{ role: 'reload', label: '刷新' }, { role: 'resetZoom', label: '实际大小' },
      { role: 'zoomIn', label: '放大' }, { role: 'zoomOut', label: '缩小' },
      ...(!app.isPackaged ? [{ role: 'toggleDevTools', label: '开发者工具' }] : [])] },
  ])
  Menu.setApplicationMenu(menu)
  tray = new Tray(nativeImage.createFromPath(path.join(__dirname, 'assets', 'tray.png')))
  tray.setToolTip('LinChat · 让交流近一点')
  tray.setContextMenu(Menu.buildFromTemplate([
    { label: '打开 LinChat', click: showWindow }, { label: '连接中心', click: () => openConnections() },
    { label: '检查更新', click: () => { void checkUpdatesFromMenu() } },
    { type: 'separator' }, { label: '退出（同时停止本机服务端）', click: () => app.quit() },
  ]))
  tray.on('double-click', showWindow)
  ipcMain.handle('connections:open', event => {
    if (!isLauncher(event) && !trustedChat(event)) throw new Error('来源无效')
    return openConnections()
  })
  ipcMain.handle('notification', (event, value) => {
    if (!trustedChat(event)) throw new Error('来源无效')
    if (Notification.isSupported() && typeof value?.title === 'string' && typeof value?.body === 'string') {
      const n = new Notification({ title: value.title.slice(0, 80), body: value.body.slice(0, 250), icon: iconPath, silent: false })
      n.on('click', showWindow); n.show()
    }
  })
  ipcMain.handle('updates:state', event => { requireDesktopPage(event); return updater.getState() })
  ipcMain.handle('updates:check', event => { requireDesktopPage(event); return updater.check() })
  ipcMain.handle('updates:open', event => { requireDesktopPage(event); return updater.open() })
  ipcMain.handle('updates:install', event => { requireDesktopPage(event); return updater.install() })
  ipcMain.handle('appearance:get', event => {
    requireDesktopPage(event)
    try { return normalizeAppearance(JSON.parse(fs.readFileSync(path.join(app.getPath('userData'), 'appearance.json'), 'utf8'))) }
    catch { return null }
  })
  ipcMain.handle('appearance:set', (event, value) => {
    requireDesktopPage(event)
    if (!value || !['accent', 'background', 'panel'].every(key => normalizeColor(value[key]))) throw new Error('配色无效，请输入 HEX 色值')
    const appearance = normalizeAppearance(value)
    fs.mkdirSync(app.getPath('userData'), { recursive: true })
    fs.writeFileSync(path.join(app.getPath('userData'), 'appearance.json'), JSON.stringify(appearance, null, 2), 'utf8')
    win.setBackgroundColor(appearance.background)
    return appearance
  })
  ipcMain.handle('launcher:state', event => {
    requireLauncher(event)
    if (hostState.setupURL && !fs.existsSync(path.join(hostData(), '首次初始化.txt'))) hostState.setupURL = ''
    return { profiles, host: hostState, dataDir: hostData(), version: app.getVersion(),
      adapters: Object.entries(os.networkInterfaces()).filter(([name]) => /radmin/i.test(name)).flatMap(([, addresses]) => (addresses || []).filter(a => a.family === 'IPv4').map(a => a.address)) }
  })
  ipcMain.handle('launcher:connect', (event, value) => { requireLauncher(event); return connect(value?.url, value?.name) })
  ipcMain.handle('launcher:remove', (event, url) => { requireLauncher(event); profiles = profiles.filter(p => p.url !== url); saveProfiles(); return profiles })
  ipcMain.handle('launcher:start-host', (event, mode) => { requireLauncher(event); return startHost(mode) })
  ipcMain.handle('launcher:stop-host', async event => { requireLauncher(event); await stopHost(); return hostState })
  ipcMain.handle('launcher:open-data', event => { requireLauncher(event); fs.mkdirSync(hostData(), { recursive: true }); return shell.openPath(hostData()) })
  ipcMain.handle('launcher:copy', (event, text) => { requireLauncher(event); if (typeof text === 'string' && text.length < 5000) clipboard.writeText(text) })
  void openConnections().then(resumeAfterUpdate)
})
