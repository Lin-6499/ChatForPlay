const { test } = require('node:test')
const assert = require('node:assert/strict')
const { editMentions, mentionQuery, prepareText, messageParts } = require('../web/src/mentions.ts')

test('caret detection ignores email addresses, whitespace and already selected mentions', () => {
  assert.deepEqual(mentionQuery('你好，@小', 5, []), { start: 3, end: 5, query: '小' })
  assert.equal(mentionQuery('foo@bar', 7, []), null)
  assert.equal(mentionQuery('@小 林', 4, []), null)
  assert.equal(mentionQuery('@小林', 3, [{ user_id: 2, start: 0, end: 3 }]), null)
  assert.deepEqual(mentionQuery('@', 1, []), { start: 0, end: 1, query: '' })
})

test('edits shift retained identities and remove an edited mention instead of silently pinging someone', () => {
  const text = '👋 @小林 请查看', mentions = [{ user_id: 2, start: 3, end: 6 }]
  assert.deepEqual(editMentions(text, '你好 ' + text, mentions), [{ user_id: 2, start: 6, end: 9 }])
  assert.deepEqual(editMentions(text, text + '！', mentions), mentions)
  assert.deepEqual(editMentions(text, '👋 @小李 请查看', mentions), [])
  assert.deepEqual(editMentions(text, '👋  请查看', mentions), [])
  assert.deepEqual(editMentions(text, text, mentions), mentions)
})

test('Unicode trimming and round-trip rendering preserve exact content and member IDs', () => {
  const source = '  👋 @小林 @朋友😀  '
  const result = prepareText(source, [{ user_id: 2, start: 5, end: 8 }, { user_id: 3, start: 9, end: 14 }])
  assert.deepEqual(result, { content: '👋 @小林 @朋友😀', mentions: [{ user_id: 2, start: 2, end: 5 }, { user_id: 3, start: 6, end: 10 }] })
  const parts = messageParts(result.content, result.mentions)
  assert.equal(parts.map(p => p.text).join(''), result.content)
  assert.deepEqual(parts.filter(p => p.userID).map(p => p.userID), [2, 3])
  assert.deepEqual(messageParts('<img onerror=alert(1)>'), [{ text: '<img onerror=alert(1)>' }])
})
