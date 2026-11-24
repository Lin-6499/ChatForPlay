const { contextBridge, ipcRenderer } = require('electron')

contextBridge.exposeInMainWorld('linchatDesktop', Object.freeze({
  isDesktop: true,
  openConnections: () => ipcRenderer.invoke('connections:open'),
  notify: (title, body) => ipcRenderer.invoke('notification', { title, body }),
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
