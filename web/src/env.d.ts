interface DesktopUpdateState {
  currentVersion: string
  latestVersion: string
  available: boolean
  checking: boolean
  checkedAt: number
  error: string
  autoUpdate?: boolean
  status?: 'idle' | 'checking' | 'available' | 'downloading' | 'ready' | 'installing' | 'error'
  progress?: number
  transferred?: number
  total?: number
}
interface Window {
  linchatDesktop?: {
    isDesktop: true
    openConnections(): Promise<void>
    notify(title: string, body: string): Promise<void>
    getUpdateState?(): Promise<DesktopUpdateState>
    checkUpdates?(): Promise<DesktopUpdateState>
    openUpdate?(): Promise<DesktopUpdateState | void>
    installUpdate?(): Promise<DesktopUpdateState>
    onUpdateState?(callback: (state: DesktopUpdateState) => void): () => void
  }
}
