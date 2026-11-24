interface Window {
  linchatDesktop?: {
    isDesktop: true
    openConnections(): Promise<void>
    notify(title: string, body: string): Promise<void>
  }
}
