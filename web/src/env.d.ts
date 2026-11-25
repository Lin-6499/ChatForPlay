interface DesktopUpdateState {
  currentVersion: string
  latestVersion: string
  available: boolean
  checking: boolean
  checkedAt: number
  error: string
}
interface Window {
  linchatDesktop?: {
    isDesktop: true
    openConnections(): Promise<void>
    notify(title: string, body: string): Promise<void>
    getUpdateState?(): Promise<DesktopUpdateState>
    checkUpdates?(): Promise<DesktopUpdateState>
    openUpdate?(): Promise<void>
    onUpdateState?(callback: (state: DesktopUpdateState) => void): () => void
  }
}
