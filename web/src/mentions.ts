// Persisted offsets use Unicode code points; textarea selections use UTF-16.
export interface Mention { user_id: number; start: number; end: number }
export interface DraftMention extends Mention {}

export function editMentions(before: string, after: string, mentions: DraftMention[]): DraftMention[] {
  let start = 0, tail = 0
  while (start < before.length && start < after.length && before[start] === after[start]) start++
  while (tail < before.length - start && tail < after.length - start && before[before.length - 1 - tail] === after[after.length - 1 - tail]) tail++
  const oldEnd = before.length - tail, delta = after.length - before.length
  return mentions.flatMap(m => m.end <= start ? [{ ...m }] : m.start >= oldEnd
    ? [{ ...m, start: m.start + delta, end: m.end + delta }] : [])
}

export function mentionQuery(text: string, caret: number, mentions: DraftMention[]) {
  const start = text.lastIndexOf('@', caret - 1)
  if (start < 0 || caret <= start || (start > 0 && /[\p{L}\p{N}_@]/u.test(text[start - 1]))) return null
  const query = text.slice(start + 1, caret)
  if (/[\s@]/u.test(query) || Array.from(query).length > 30 || mentions.some(m => m.start === start)) return null
  return { start, end: caret, query }
}

export function prepareText(text: string, mentions: DraftMention[]) {
  const leading = text.length - text.trimStart().length, content = text.trim()
  const end = leading + content.length
  return { content, mentions: mentions.filter(m => m.start >= leading && m.end <= end).map(m => ({
    user_id: m.user_id,
    start: Array.from(text.slice(leading, m.start)).length,
    end: Array.from(text.slice(leading, m.end)).length,
  })) }
}

export function messageParts(content: string, mentions: Mention[] = []) {
  const chars = Array.from(content), parts: { text: string; userID?: number }[] = []
  let cursor = 0
  for (const m of mentions) {
    if (!Number.isInteger(m.start) || !Number.isInteger(m.end) || m.start < cursor || m.end <= m.start || m.end > chars.length) continue
    if (m.start > cursor) parts.push({ text: chars.slice(cursor, m.start).join('') })
    parts.push({ text: chars.slice(m.start, m.end).join(''), userID: m.user_id })
    cursor = m.end
  }
  if (cursor < chars.length) parts.push({ text: chars.slice(cursor).join('') })
  return parts
}
