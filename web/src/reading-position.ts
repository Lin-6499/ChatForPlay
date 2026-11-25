export interface ReadingPosition {
  messageID: number
  offset: number
  bottom: boolean
}

// localStorage is already isolated by server origin. Include both account and
// conversation so signing out never transfers a reading position to another user.
export function readingPositionStore() {
  const memory = new Map<string, ReadingPosition>()
  const key = (userID: number, conversationID: number) => `linchat-reading:${userID}:${conversationID}`
  return {
    get(userID: number, conversationID: number): ReadingPosition | undefined {
      const name = key(userID, conversationID)
      if (memory.has(name)) return memory.get(name)
      try {
        const value = JSON.parse(localStorage.getItem(name) || 'null')
        if (value && Number.isSafeInteger(value.messageID) && value.messageID >= (value.bottom ? 0 : 1) &&
          Number.isFinite(value.offset) && typeof value.bottom === 'boolean') {
          memory.set(name, value)
          return value
        }
      } catch {}
    },
    set(userID: number, conversationID: number, value: ReadingPosition) {
      const name = key(userID, conversationID)
      memory.set(name, value)
      try { localStorage.setItem(name, JSON.stringify(value)) } catch {}
    },
  }
}

export function captureReadingPosition(list: HTMLElement, latest = true): ReadingPosition {
  const top = list.getBoundingClientRect().top
  const rows = Array.from(list.querySelectorAll<HTMLElement>('[data-message-id]'))
  const anchor = rows.find(row => row.getBoundingClientRect().bottom > top) || rows.at(-1)
  return {
    messageID: Number(anchor?.dataset.messageId || 0),
    offset: anchor ? anchor.getBoundingClientRect().top - top : 0,
    bottom: latest && list.scrollHeight - list.scrollTop - list.clientHeight <= 24,
  }
}

export function restoreReadingPosition(list: HTMLElement, position: ReadingPosition) {
  if (position.bottom) { list.scrollTop = list.scrollHeight; return }
  const anchor = list.querySelector<HTMLElement>(`[data-message-id="${position.messageID}"]`)
  if (anchor) list.scrollTop += anchor.getBoundingClientRect().top - list.getBoundingClientRect().top - position.offset
}
