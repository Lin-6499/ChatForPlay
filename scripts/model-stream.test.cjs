const { test } = require('node:test')
const assert = require('node:assert/strict')
const { partialPublicText, modelStream } = require('./model-stream.cjs')
test('public JSON fields buffer escapes and ignore nested secrets', () => {
  assert.equal(partialPublicText('{"scene":{"narration":"secret"},"narration":"门后', 'narration'), '门后')
  assert.equal(partialPublicText('{"narration":"一\\uD83D', 'narration'), '一')
  assert.equal(partialPublicText('{"narration":"一\\uD83D\\uDE00', 'narration'), '一😀')
})
test('SSE parses split UTF-8 and delivers text before provider completion', async () => {
  let controller, resolve
  const seen = new Promise(r => { resolve = r })
  const body = new ReadableStream({ start(c) { controller = c } })
  const result = modelStream(new Response(body, { headers: { 'content-type': 'text/event-stream' } }), 'narration', resolve)
  const bytes = new TextEncoder().encode('data: ' + JSON.stringify({ choices: [{ index: 0, delta: { tool_calls: [{ index: 0, id: 'call', function: { name: 'finish_turn', arguments: '{"narration":"门后' } }] } }] }) + '\n\n')
  for (const byte of bytes) controller.enqueue(new Uint8Array([byte]))
  assert.equal(await seen, '门后')
  controller.enqueue(new TextEncoder().encode('data: [DONE]\n\n'))
  assert.equal((await result).tool_calls[0].function.name, 'finish_turn')
})
