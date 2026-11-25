import { initializeAppearance } from './theme.mjs'
import { mountAppearance } from './appearance.mjs'

const api = window.linchatDesktop
const $ = id => document.getElementById(id)
void initializeAppearance(api)
let disposeAppearance
$('open-appearance').addEventListener('click', () => {
  disposeAppearance?.()
  disposeAppearance = mountAppearance($('appearance-editor'))
  $('appearance-dialog').showModal()
})
$('close-appearance').addEventListener('click', () => $('appearance-dialog').close())
$('appearance-dialog').addEventListener('close', () => { disposeAppearance?.(); disposeAppearance = undefined })
$('appearance-dialog').addEventListener('click', e => {
  const bounds = e.currentTarget.getBoundingClientRect()
  if (e.target === e.currentTarget && (e.clientX < bounds.left || e.clientX > bounds.right || e.clientY < bounds.top || e.clientY > bounds.bottom)) e.currentTarget.close()
})
let state, busy = false, toastTimer
function toast(text) { $('toast').textContent = text; $('toast').hidden = false; clearTimeout(toastTimer); toastTimer = setTimeout(() => $('toast').hidden = true, 3000) }
function error(e) { $('error').textContent = e.message.replace(/^Error invoking remote method '[^']+': Error: /, ''); $('error').hidden = false }
function clearError() { $('error').hidden = true }
function renderHost(host) {
  state.host = host
  $('host-card').hidden = !host.running
  $('start-radmin').disabled = host.running || host.starting || busy
  $('start-local').disabled = host.running || host.starting || busy
  $('start-radmin').textContent = host.starting ? '正在启动…' : '启动 Radmin 聊天主机'
  $('host-url').textContent = host.url
  $('enter-host').textContent = host.setupURL ? '初始化聊天空间' : '打开聊天空间'
  if (host.error) error(new Error(host.error))
}
function renderProfiles() {
  $('profiles').replaceChildren()
  for (const p of state.profiles) {
    const row = document.createElement('div'); row.className = 'profile'
    const button = document.createElement('button'); button.className = 'profile-connect'
    const icon = document.createElementNS('http://www.w3.org/2000/svg', 'svg')
    icon.setAttribute('viewBox', '0 0 24 24'); icon.classList.add('profile-icon'); icon.setAttribute('aria-hidden', 'true')
    const shape = document.createElementNS('http://www.w3.org/2000/svg', 'path')
    shape.setAttribute('d', p.url.startsWith('https:') ? 'M7 18a5 5 0 1 1 1.1-9.9A6 6 0 0 1 19.5 9 4.5 4.5 0 0 1-.5 9H7Z' : 'M8 3h8v6H8zM2 15h8v6H2zM14 15h8v6h-8zM12 9v3M6 15v-3h12v3')
    icon.append(shape)
    const label = document.createElement('span'), title = document.createElement('b'), url = document.createElement('small')
    title.textContent = p.name; url.textContent = p.url; label.append(title, url); button.append(icon, label)
    button.addEventListener('click', () => connect(p.url, p.name))
    const remove = document.createElement('button'); remove.className = 'remove-profile'
    const removeIcon = document.createElementNS('http://www.w3.org/2000/svg', 'svg')
    removeIcon.setAttribute('viewBox', '0 0 24 24'); removeIcon.setAttribute('aria-hidden', 'true')
    const cross = document.createElementNS('http://www.w3.org/2000/svg', 'path'); cross.setAttribute('d', 'm6 6 12 12M6 18 18 6')
    removeIcon.append(cross); remove.append(removeIcon)
    remove.title = '移除保存的连接'; remove.setAttribute('aria-label', '移除 ' + p.name)
    remove.addEventListener('click', async () => { state.profiles = await api.removeProfile(p.url); renderProfiles() })
    row.append(button, remove); $('profiles').append(row)
  }
}
async function connect(url, name) {
  if (busy) return
  busy = true; clearError(); $('connect').disabled = true; $('connect').textContent = '正在连接…'
  try { await api.connect(url, name) } catch (e) { error(e) }
  finally { busy = false; $('connect').disabled = false; $('connect').textContent = '进入聊天空间' }
}
document.querySelectorAll('[data-tab]').forEach(button => {
  button.addEventListener('click', () => {
    clearError()
    document.querySelectorAll('[data-tab]').forEach(b => { b.classList.toggle('selected', b === button); b.setAttribute('aria-selected', String(b === button)) })
    $('join-panel').hidden = button.dataset.tab !== 'join'; $('host-panel').hidden = button.dataset.tab !== 'host'
  })
})
$('connect-form').addEventListener('submit', e => { e.preventDefault(); void connect($('url').value, $('name').value) })
async function start(mode) {
  if (busy) return
  busy = true; clearError(); $('start-radmin').disabled = true; $('start-local').disabled = true
  try { renderHost(await api.startHost(mode)) }
  catch (e) { error(e) }
  finally { busy = false; renderHost(state.host) }
}
$('start-radmin').addEventListener('click', () => start('radmin'))
$('start-local').addEventListener('click', () => start('local'))
$('stop-host').addEventListener('click', async () => {
  $('stop-host').disabled = true
  try { renderHost(await api.stopHost()); toast('本机服务端已停止，记录已保留') }
  catch (e) { error(e) }
  finally { $('stop-host').disabled = false }
})
$('enter-host').addEventListener('click', () => connect(state.host.setupURL || state.host.url, state.host.mode === 'radmin' ? '我的 Radmin 空间' : '本机体验'))
$('copy-host').addEventListener('click', async () => { await api.copy(state.host.url); toast('连接地址已复制') })
$('open-data').addEventListener('click', async () => { await api.openData() })
api.onHostState(host => { if (state) renderHost(host) })
function renderUpdate(update) {
  $('update-banner').hidden = !update.available
  const downloading = update.status === 'downloading', ready = update.status === 'ready', installing = update.status === 'installing'
  $('update-title').textContent = installing ? '正在准备重启…' : ready ? `v${update.latestVersion} 已就绪` :
    downloading ? `正在下载 v${update.latestVersion} · ${Math.floor(update.progress)}%` : `发现新版本 v${update.latestVersion}`
  $('update-detail').textContent = update.error || (update.autoUpdate ?
    installing ? '正在关闭本机服务，随后自动安装并重启。' : ready ? '重启后自动安装，保留安装目录和数据。本机服务会暂时停止。' :
      '后台下载并校验，你可以继续聊天。' : '下载安装版以启用自动更新；便携版需手动更换程序。')
  $('update-progress').hidden = !downloading
  $('update-progress').value = update.progress || 0
  $('download-update').textContent = installing ? '正在更新…' : ready ? '重启并更新' :
    downloading ? '下载中…' : update.autoUpdate ? '重试下载' : '下载安装版'
  $('download-update').disabled = downloading || installing
  $('check-update').disabled = update.checking || installing
  $('check-update').textContent = update.checking ? '检查中…' : '检查更新'
}
api.onUpdateState(renderUpdate)
api.getUpdateState().then(renderUpdate).catch(error)
$('check-update').addEventListener('click', async () => {
  try {
    const update = await api.checkUpdates()
    renderUpdate(update)
    if (update.error) toast(update.error)
    else if (!update.available) toast(`当前已是最新版本 v${update.currentVersion}`)
  } catch (e) { error(e) }
})
$('download-update').addEventListener('click', async () => {
  $('download-update').disabled = true
  try { await api.openUpdate() } catch (e) { error(e) }
  finally { renderUpdate(await api.getUpdateState()) }
})
api.getState().then(s => {
  state = s; $('version').textContent = 'v' + s.version
  $('adapter').textContent = s.adapters.length ? 'Radmin VPN 已就绪 · ' + s.adapters.join(' / ') : '未检测到 Radmin IPv4 网卡，请先安装并连接 VPN'
  $('adapter-dot').classList.toggle('off', !s.adapters.length)
  renderProfiles(); renderHost(s.host)
}).catch(error)
