const { contextBridge, ipcRenderer } = require('electron')

// Keep this in preload so both the local launcher and deployed chat pages get
// the title bar, including servers that have not yet updated their web bundle.
if (process.platform === 'win32') {
  window.addEventListener('DOMContentLoaded', () => {
    // Constructed stylesheets work under the launcher's strict style-src CSP,
    // which rejects injected <style> elements on both local and remote pages.
    const style = new CSSStyleSheet()
    style.replaceSync(`
      html[data-linchat-titlebar] { --linchat-titlebar-height: env(titlebar-area-height, 32px); }
      html[data-linchat-titlebar] body { margin-top: var(--linchat-titlebar-height); min-height: calc(100vh - var(--linchat-titlebar-height)); }
      html[data-linchat-titlebar] .app-shell { min-height: calc(100dvh - var(--linchat-titlebar-height)); }
      html[data-linchat-titlebar] .app-shell.signed-in { height: calc(100dvh - var(--linchat-titlebar-height)); }
      html[data-linchat-titlebar] .loading-screen { height: calc(100dvh - var(--linchat-titlebar-height)); }
      html[data-linchat-titlebar] .modal-backdrop { top: var(--linchat-titlebar-height); }
    `)
    document.adoptedStyleSheets = [...document.adoptedStyleSheets, style]
    const bar = document.createElement('div')
    bar.id = 'linchat-window-titlebar'
    const shadow = bar.attachShadow({ mode: 'closed' })
    const rules = new CSSStyleSheet()
    rules.replaceSync(`
      :host { position: fixed; inset: 0 0 auto; height: env(titlebar-area-height, 32px); z-index: 2147483647;
        background: var(--chrome-background, #1b2220); color: var(--chrome-text, #edf2ef); }
      :host([hidden]) { display: none; }
      .title { height: 100%; width: env(titlebar-area-width, calc(100% - 144px)); box-sizing: border-box;
        display: flex; align-items: center; padding: 0 12px; -webkit-app-region: drag; user-select: none; }
      span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font: 12px "Segoe UI", "Microsoft YaHei", sans-serif; }
    `)
    shadow.adoptedStyleSheets = [rules]
    const drag = document.createElement('div'), title = document.createElement('span')
    drag.className = 'title'; drag.append(title); shadow.append(drag)
    document.documentElement.append(bar)
    function updateTitle() { title.textContent = document.title || 'LinChat · 让交流近一点' }
    updateTitle()
    const observer = new MutationObserver(updateTitle)
    observer.observe(document.head, { childList: true, subtree: true, characterData: true })
    const apply = state => {
      if (!state?.enabled) return
      bar.hidden = !!state.fullscreen
      document.documentElement.toggleAttribute('data-linchat-titlebar', !state.fullscreen)
      bar.style.setProperty('--chrome-background', state.color)
      bar.style.setProperty('--chrome-text', state.symbolColor)
    }
    ipcRenderer.on('window:chrome', (_event, state) => apply(state))
    ipcRenderer.invoke('window:chrome').then(apply).catch(() => {})
    // Reserve space immediately, before the first IPC response.
    document.documentElement.setAttribute('data-linchat-titlebar', '')
    window.addEventListener('pagehide', () => observer.disconnect(), { once: true })
  }, { once: true })
}

contextBridge.exposeInMainWorld('linchatDesktop', Object.freeze({
  isDesktop: true,
  openConnections: () => ipcRenderer.invoke('connections:open'),
  notify: (title, body) => ipcRenderer.invoke('notification', { title, body }),
  getUpdateState: () => ipcRenderer.invoke('updates:state'),
  checkUpdates: () => ipcRenderer.invoke('updates:check'),
  openUpdate: () => ipcRenderer.invoke('updates:open'),
  installUpdate: () => ipcRenderer.invoke('updates:install'),
  getAppearance: () => ipcRenderer.invoke('appearance:get'),
  setAppearance: value => ipcRenderer.invoke('appearance:set', value),
  onUpdateState: callback => {
    const listener = (_event, state) => callback(state)
    ipcRenderer.on('updates:state', listener)
    return () => ipcRenderer.removeListener('updates:state', listener)
  },
  getState: () => ipcRenderer.invoke('launcher:state'),
  connect: (url, name) => ipcRenderer.invoke('launcher:connect', { url, name }),
  removeProfile: (url) => ipcRenderer.invoke('launcher:remove', url),
  startHost: (mode) => ipcRenderer.invoke('launcher:start-host', mode),
  stopHost: () => ipcRenderer.invoke('launcher:stop-host'),
  openData: () => ipcRenderer.invoke('launcher:open-data'),
  copy: (text) => ipcRenderer.invoke('launcher:copy', text),
  onHostState: (callback) => {
    const listener = (_event, state) => callback(state)
    ipcRenderer.on('host:state', listener)
    return () => ipcRenderer.removeListener('host:state', listener)
  },
}))
