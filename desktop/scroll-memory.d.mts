export interface ScrollPosition { x: number; y: number; anchor: string; offset: number; bottom: boolean }
export function getScrollPosition(key: string): ScrollPosition | undefined
export function startScrollMemory(root?: Document | HTMLElement): () => void
