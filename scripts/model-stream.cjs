// Chat Completions SSE: only public finish_turn fields are previewed.
function partialPublicText(source, field) {
  let i = 0
  const ws = () => { while (/\s/.test(source[i] || '') && i < source.length) i++ }
  const stringEnd = start => {
    for (let p = start + 1; p < source.length; p++) {
      if (source[p] === '\\') p++
      else if (source[p] === '"') return p + 1
    }
    return -1
  }
  ws(); if (source[i++] !== '{') return ''
  while (i < source.length) {
    ws(); if (source[i] !== '"') return ''
    const end = stringEnd(i); if (end < 0) return ''
    const key = JSON.parse(source.slice(i, end)); i = end; ws()
    if (source[i++] !== ':') return ''
    ws()
    if (key === field) {
      if (source[i] !== '"') return ''
      const end = stringEnd(i)
      if (end >= 0) return JSON.parse(source.slice(i, end))
      for (let n = source.length; n > i; n--) {
        try {
          const text = JSON.parse(source.slice(i, n) + '"')
          if (!/[\uD800-\uDBFF](?![\uDC00-\uDFFF])|(?<![\uD800-\uDBFF])[\uDC00-\uDFFF]/u.test(text)) return text
        } catch {}
      }
      return ''
    }
    let depth = 0
    for (; i < source.length; i++) {
      const ch = source[i]
      if (ch === '"') { const end = stringEnd(i); if (end < 0) return ''; i = end - 1 }
      else if (ch === '{' || ch === '[') depth++
      else if (ch === '}' || ch === ']') { if (!depth) return ''; depth-- }
      else if (ch === ',' && !depth) { i++; break }
    }
  }
  return ''
}
async function modelStream(response, field, publish) {
  if (!response.headers.get('content-type')?.startsWith('text/event-stream')) {
    return (await response.json()).choices?.[0]?.message
  }
  const answer = { role: 'assistant', content: null, tool_calls: [] }
  const decoder = new TextDecoder('utf-8', { fatal: true })
  let buffer = '', data = [], total = 0, done = false
  async function event() {
    const raw = data.join('\n'); data = []
    if (!raw) return
    if (raw === '[DONE]') { done = true; return }
    const parsed = JSON.parse(raw)
    if (parsed.error) throw Error('Model stream error')
    for (const choice of parsed.choices || []) {
      if (choice.index !== 0) throw Error('Multiple model choices')
      for (const delta of choice.delta?.tool_calls || []) {
        if (delta.index !== 0) throw Error('Parallel tool calls unsupported')
        const call = answer.tool_calls[0] ||= { id: '', type: 'function', function: { name: '', arguments: '' } }
        call.id += delta.id || ''; call.function.name += delta.function?.name || ''
        call.function.arguments += delta.function?.arguments || ''
        if (call.function.name === 'finish_turn') {
          const text = partialPublicText(call.function.arguments, field)
          if (text) await publish(text)
        }
      }
    }
  }
  for await (const chunk of response.body) {
    total += chunk.length
    if (total > 1048576) throw Error('Model response too large')
    buffer += decoder.decode(chunk, { stream: true })
    let pos
    while ((pos = buffer.indexOf('\n')) >= 0) {
      const line = buffer.slice(0, pos).replace(/\r$/, ''); buffer = buffer.slice(pos + 1)
      if (!line) await event()
      else if (line.startsWith('data:')) data.push(line.slice(5).replace(/^ /, ''))
      if (done) return answer
    }
  }
  throw Error('Model stream interrupted')
}
module.exports = { partialPublicText, modelStream }
