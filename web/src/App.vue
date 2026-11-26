<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import Icon from './Icon.vue'
import Entertainment from './Entertainment.vue'
import AppearanceSettings from './AppearanceSettings.vue'
import { editMentions, mentionQuery, prepareText, messageParts, type Mention, type DraftMention } from './mentions'
import { captureReadingPosition, readingPositionStore, restoreReadingPosition, type ReadingPosition } from './reading-position'

interface User { id: number; username: string; nickname: string; admin: boolean; online: boolean; last_read?: number }
interface Conversation { id: number; kind: string; name: string; owner_id: number; member_count: number; last_id: number; last_at: number; preview: string; unread: number; last_read: number; unread_mentions?: number }
interface Message { id: number; conversation_id: number; sender_id: number; sender_name: string; client_id: string; kind: string; content: string; file_id?: string; file_name?: string; file_size?: number; file_mime?: string; created_at: number; mentions?: Mention[] }
interface Pending { client_id: string; conversation_id: number; content: string; created_at: number; mentions?: Mention[]; error?: string; retryable?: boolean }
interface Page { messages: Message[]; more: boolean }
type Modal = '' | 'directory' | 'group' | 'invite' | 'settings' | 'appearance' | 'connection' | 'members' | 'addMembers'
class ApiError extends Error { constructor(message: string, public status: number) { super(message) } }

const location = window.location
const desktop = window.linchatDesktop
const me = ref<User | null>(null), people = ref<User[]>([]), conversations = ref<Conversation[]>([])
const messages = ref<Message[]>([]), members = ref<User[]>([]), pending = ref<Pending[]>([])
const currentID = ref(0), ready = ref(false), initialized = ref(true), busy = ref(false), error = ref('')
const serverName = ref('LinChat'), maxUpload = ref(25 * 1024 * 1024), authMode = ref('login')
const form = ref({ username: '', nickname: '', password: '', token: '' })
const profile = ref({ nickname: '', old_password: '', new_password: '' })
const query = ref(''), directoryQuery = ref(''), groupName = ref(''), selectedUsers = ref<number[]>([])
const draft = ref(''), modal = ref<Modal>(''), showMobileChat = ref(false), showDetails = ref(false)
const moreHistory = ref(false), loadingHistory = ref(false), toast = ref(''), connection = ref('连接中')
const moreRecent = ref(false)
const toastKind = ref('')
const inviteUses = ref(10), inviteLink = ref(''), upload = ref<{ name: string; percent: number } | null>(null)
const uploadRetry = ref<{ file: File; cid: number; clientID: string } | null>(null)
const messageList = ref<HTMLElement>(), messageContent = ref<HTMLElement>(), composer = ref<HTMLTextAreaElement>(), fileInput = ref<HTMLInputElement>()
const connectionURL = ref(''), connectionLabel = ref(''), recentServers = ref<{ name: string; url: string }[]>([])
const dragging = ref(false)
const entertainment = ref(/^#(?:entertainment|campaign=\d+)$/.test(location.hash)), gameRevision = ref(0)
const draftMentions = ref<DraftMention[]>([]), mentionSearch = ref<ReturnType<typeof mentionQuery>>(null)
const mentionIndex = ref(0), composing = ref(false), membersReady = ref(false)
const canMention = computed(() => current.value?.kind === 'group' || current.value?.kind === 'lobby')
const mentionOptions = computed(() => {
  const search = mentionSearch.value?.query.toLocaleLowerCase() || ''
  return members.value.filter(p => p.id !== me.value?.id && `${p.nickname} ${p.username}`.toLocaleLowerCase().includes(search))
    .sort((a, b) => Number(b.online) - Number(a.online) || a.nickname.localeCompare(b.nickname, 'zh-CN')).slice(0, 30)
})
const mentionOpen = computed(() => canMention.value && mentionSearch.value !== null && !composing.value)
const selectedMention = computed(() => mentionOptions.value[Math.min(mentionIndex.value, mentionOptions.value.length - 1)])
const mentionedPeople = computed(() => new Set(draftMentions.value.map(m => m.user_id)).size)
const desktopUpdate = ref<DesktopUpdateState | null>(null), dismissedUpdate = ref('')
let offUpdate: (() => void) | undefined
let ws: WebSocket | null = null, reconnectTimer: ReturnType<typeof setTimeout> | undefined
let toastTimer: ReturnType<typeof setTimeout> | undefined, refreshTimer: ReturnType<typeof setTimeout> | undefined
let reconnectAttempt = 0, intentionalClose = false, flushing = false, selectionVersion = 0
const cursors = new Map<number, number>(), readSent = new Map<number, number>()
const current = computed(() => conversations.value.find(c => c.id === currentID.value))
const filteredConversations = computed(() => conversations.value.filter(c => c.name.toLowerCase().includes(query.value.toLowerCase())))
const otherUsers = computed(() => people.value.filter(p => p.id !== me.value?.id && `${p.nickname} ${p.username}`.toLowerCase().includes(directoryQuery.value.toLowerCase())))
const groupCandidates = computed(() => otherUsers.value.filter(p => modal.value !== 'addMembers' || !members.value.some(m => m.id === p.id)))
const onlineCount = computed(() => people.value.filter(p => p.online).length)
const pendingHere = computed(() => pending.value.filter(p => p.conversation_id === currentID.value))
const totalUnread = computed(() => conversations.value.reduce((n, c) => n + c.unread, 0))
const networkMode = /^26\./.test(location.hostname) ? 'Radmin 虚拟局域网' : location.hostname === '127.0.0.1' || location.hostname === 'localhost' ? '本地服务' : '云端连接'
const activeOnline = computed(() => members.value.filter(p => p.online).length)
const modalElement = ref<HTMLElement>()
let modalTrigger: HTMLElement | null = null
watch(modal, async (value, previous) => {
  if (value && !previous) modalTrigger = document.activeElement as HTMLElement
  await nextTick()
  if (value) modalElement.value?.focus()
  else if (modalTrigger?.isConnected) modalTrigger.focus()
})
function trapModalFocus(event: KeyboardEvent) {
  if (event.key !== 'Tab') return
  const items = Array.from(modalElement.value?.querySelectorAll<HTMLElement>('button:not(:disabled), input:not(:disabled), textarea:not(:disabled), select:not(:disabled), summary, a[href]') || [])
    .filter(el => el.getClientRects().length)
  const first = items[0], last = items.at(-1)
  if (!first || !last) { event.preventDefault(); return }
  if (!modalElement.value?.contains(document.activeElement)) { event.preventDefault(); (event.shiftKey ? last : first).focus() }
  else if (event.shiftKey && (document.activeElement === first || document.activeElement === modalElement.value)) { event.preventDefault(); last.focus() }
  else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
}
function onModalKey(event: KeyboardEvent) {
  if (!modal.value) return
  if (event.key === 'Escape') { event.preventDefault(); modal.value = '' }
  else trapModalFocus(event)
}

async function api<T = any>(path: string, method = 'GET', body?: unknown, timeout = 20000): Promise<T> {
  const controller = new AbortController(), timer = setTimeout(() => controller.abort(), timeout)
  try {
    const r = await fetch('/api' + path, { method, credentials: 'same-origin', signal: controller.signal,
      headers: { 'X-LinChat': '1', ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}) },
      body: body !== undefined ? JSON.stringify(body) : undefined })
    const data = await r.json()
    if (!r.ok) throw new ApiError(data.error || '请求失败', r.status)
    return data as T
  } finally { clearTimeout(timer) }
}
function explain(e: unknown) { return e instanceof ApiError ? e.message : '连接暂时不可用，请检查网络或服务端是否运行' }
function notify(text: string, kind = '') { toast.value = text; toastKind.value = kind; clearTimeout(toastTimer); toastTimer = setTimeout(() => toast.value = '', 4200) }
function avatar(name: string) { return Array.from(name || 'L').slice(0, 1).join('').toUpperCase() }
function hue(id: number) { return ['mint', 'blue', 'peach', 'lilac', 'sand'][id % 5] }
function time(ts: number) { return new Date(ts).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' }) }
function date(ts: number) { return new Date(ts).toLocaleDateString('zh-CN', { month: 'long', day: 'numeric' }) }
function bytes(n = 0) { return n >= 1048576 ? `${(n / 1048576).toFixed(1)} MB` : `${Math.max(1, Math.round(n / 1024))} KB` }
function uid() {
  const a = crypto.getRandomValues(new Uint8Array(18))
  return Array.from(a, n => n.toString(16).padStart(2, '0')).join('')
}
function localGet(key: string) { try { return localStorage.getItem(key) } catch { return null } }
function localSet(key: string, value: string) { try { localStorage.setItem(key, value) } catch { notify('浏览器本地存储不可用，请保持页面打开以保留待发消息') } }
function pendingKey() { return `linchat-pending:${me.value?.id}` }
function savePending() { if (me.value) localSet(pendingKey(), JSON.stringify(pending.value)) }
function mergeMessages(items: Message[]) {
  const map = new Map(messages.value.map(m => [m.id, m]))
  for (const m of items) if (m.conversation_id === currentID.value) map.set(m.id, m)
  messages.value = Array.from(map.values()).sort((a, b) => a.id - b.id)
  const acknowledgements = new Set(items.filter(m => m.sender_id === me.value?.id).map(m => m.client_id))
  pending.value = pending.value.filter(p => !acknowledgements.has(p.client_id))
  savePending()
}
const readingPositions = readingPositionStore()
let readingPosition: ReadingPosition | undefined, historyReady = false, appliedScrollTop = -1
let positionTimer: ReturnType<typeof setTimeout> | undefined, messageResize: ResizeObserver | undefined
function persistReadingPosition() {
  clearTimeout(positionTimer)
  if (me.value && currentID.value && readingPosition && historyReady) readingPositions.set(me.value.id, currentID.value, readingPosition)
}
function rememberReadingPosition() {
  const list = messageList.value
  if (!historyReady || loadingHistory.value || !list?.clientHeight) return
  readingPosition = captureReadingPosition(list, !moreRecent.value)
}
function saveReadingPosition() { rememberReadingPosition(); persistReadingPosition() }
function applyReadingPosition() {
  const list = messageList.value
  if (!historyReady || loadingHistory.value || !list?.clientHeight || !readingPosition) return
  restoreReadingPosition(list, readingPosition)
  appliedScrollTop = list.scrollTop
}
function onMessageScroll() {
  const list = messageList.value
  if (!list?.clientHeight || loadingHistory.value || !historyReady) return
  if (Math.abs(list.scrollTop - appliedScrollTop) > 1) {
    rememberReadingPosition()
    appliedScrollTop = list.scrollTop
    clearTimeout(positionTimer)
    positionTimer = setTimeout(persistReadingPosition, 200)
  }
  void markRead()
}
function atBottom() {
  const e = messageList.value
  return historyReady && !loadingHistory.value && !moreRecent.value && !!e?.clientHeight && e.scrollHeight - e.scrollTop - e.clientHeight <= 24
}
function followingLatest() { return historyReady && !loadingHistory.value && !moreRecent.value && readingPosition?.bottom === true }
async function bottom(version = selectionVersion) {
  await nextTick()
  if (version !== selectionVersion || moreRecent.value || !historyReady) return
  readingPosition = { messageID: messages.value.at(-1)?.id || 0, offset: 0, bottom: true }
  applyReadingPosition(); persistReadingPosition()
}
// Images, wrapping, window resizing and re-mounting after Entertainment all keep
// the same message anchor. Hidden mobile panels must not overwrite it with 0.
watch([messageList, messageContent], () => {
  messageResize?.disconnect()
  if (!messageList.value || !messageContent.value) return
  messageResize = new ResizeObserver(() => { applyReadingPosition(); void markRead() })
  messageResize.observe(messageList.value)
  messageResize.observe(messageContent.value)
  applyReadingPosition()
}, { flush: 'post' })
watch([entertainment, showMobileChat], async () => {
  saveReadingPosition()
  await nextTick()
  applyReadingPosition()
  void markRead()
})
async function refreshConversations() {
  conversations.value = await api<Conversation[]>('/conversations')
  document.title = `${totalUnread.value ? `(${totalUnread.value}) ` : ''}LinChat · 让交流近一点`
}
function scheduleRefresh() {
  clearTimeout(refreshTimer)
  refreshTimer = setTimeout(() => refreshConversations().catch(() => {}), 200)
}
let peopleVersion = 0
async function refreshPeople() {
  const version = ++peopleVersion
  const list = await api<User[]>('/users')
  if (version === peopleVersion) people.value = list
}
async function loadMembers(cid = currentID.value) {
  if (!cid) return
  const result = await api<{ members: User[] }>(`/conversations/${cid}`)
  if (currentID.value === cid) { members.value = result.members; membersReady.value = true }
}
async function selectConversation(c: Conversation, reveal = true) {
  if (toastKind.value === 'mention') { toast.value = ''; clearTimeout(toastTimer) }
  if (currentID.value === c.id && historyReady) { showMobileChat.value = true; await nextTick(); applyReadingPosition(); await markRead(); return }
  saveReadingPosition()
  const version = ++selectionVersion
  ++syncVersion
  historyReady = false; appliedScrollTop = -1
  readingPosition = me.value ? readingPositions.get(me.value.id, c.id) : undefined
  currentID.value = c.id; messages.value = []; members.value = []; draft.value = ''
  draftMentions.value = []; mentionSearch.value = null; membersReady.value = false
  moreHistory.value = false; moreRecent.value = false; loadingHistory.value = true; showMobileChat.value = reveal; showDetails.value = false
  try {
    const saved = readingPosition
    const [page] = await Promise.all([api<Page>(`/conversations/${c.id}/messages`), loadMembers(c.id)])
    if (version !== selectionVersion) return
    let items = page.messages, more = page.more
    if (saved && !saved.bottom && saved.messageID && !items.some(m => m.id === saved.messageID)) {
      // Jump directly to old history instead of downloading every intervening page.
      const [before, after] = await Promise.all([
        api<Page>(`/conversations/${c.id}/messages?before=${saved.messageID}`),
        api<Page>(`/conversations/${c.id}/messages?after=${saved.messageID - 1}`),
      ])
      if (version !== selectionVersion) return
      if (after.messages.some(m => m.id === saved.messageID)) {
        items = [...before.messages, ...after.messages]; more = before.more; moreRecent.value = after.more
      } else readingPosition = undefined
    }
    mergeMessages(items); moreHistory.value = more
    cursors.set(c.id, items.at(-1)?.id || 0)
    readingPosition ||= { messageID: items.at(-1)?.id || 0, offset: 0, bottom: true }
    historyReady = true; loadingHistory.value = false
    await nextTick()
    if (version !== selectionVersion) return
    applyReadingPosition(); persistReadingPosition(); await markRead()
    await syncActive()
  } catch (e) { if (version === selectionVersion) notify(explain(e)) }
  finally { if (version === selectionVersion) loadingHistory.value = false }
}
async function older() {
  const cid = currentID.value, version = selectionVersion, first = messages.value[0]?.id
  if (!first || loadingHistory.value) return
  rememberReadingPosition()
  loadingHistory.value = true
  try {
    const page = await api<Page>(`/conversations/${cid}/messages?before=${first}`)
    if (version !== selectionVersion) return
    mergeMessages(page.messages); moreHistory.value = page.more
  } catch (e) { if (version === selectionVersion) notify(explain(e)) }
  finally {
    if (version === selectionVersion) {
      loadingHistory.value = false
      await nextTick()
      if (version === selectionVersion) {
        applyReadingPosition()
        void syncActive().catch(() => {})
      }
    }
  }
}
async function newer() {
  const cid = currentID.value, version = selectionVersion, last = messages.value.at(-1)?.id
  if (!last || loadingHistory.value || !moreRecent.value) return
  rememberReadingPosition(); loadingHistory.value = true
  try {
    const page = await api<Page>(`/conversations/${cid}/messages?after=${last}`)
    if (version !== selectionVersion) return
    mergeMessages(page.messages); moreRecent.value = page.more
    cursors.set(cid, page.messages.at(-1)?.id || last)
  } catch (e) { if (version === selectionVersion) notify(explain(e)) }
  finally {
    if (version === selectionVersion) {
      loadingHistory.value = false
      await nextTick()
      if (version === selectionVersion) {
        applyReadingPosition(); await markRead()
        if (version === selectionVersion) void syncActive().catch(() => {})
      }
    }
  }
}
async function latest() {
  const cid = currentID.value, version = selectionVersion
  if (!cid || loadingHistory.value) return false
  rememberReadingPosition(); loadingHistory.value = true; ++syncVersion
  try {
    const page = await api<Page>(`/conversations/${cid}/messages`)
    if (version !== selectionVersion) return false
    messages.value = []; mergeMessages(page.messages)
    moreHistory.value = page.more; moreRecent.value = false; historyReady = true
    cursors.set(cid, page.messages.at(-1)?.id || 0)
    readingPosition = { messageID: page.messages.at(-1)?.id || 0, offset: 0, bottom: true }
    loadingHistory.value = false
    await nextTick()
    if (version !== selectionVersion) return false
    applyReadingPosition(); persistReadingPosition(); await markRead(); await syncActive()
    return version === selectionVersion
  } catch (e) { if (version === selectionVersion) notify(explain(e)); return false }
  finally { if (version === selectionVersion) loadingHistory.value = false }
}
let syncVersion = 0
async function syncActive() {
  const cid = currentID.value, version = ++syncVersion, selection = selectionVersion
  if (!cid || !historyReady || loadingHistory.value || moreRecent.value) return
  let cursor = cursors.get(cid) || 0, more = true
  while (more && selection === selectionVersion && version === syncVersion) {
    const page = await api<Page>(`/conversations/${cid}/messages?after=${cursor}`)
    if (selection !== selectionVersion || version !== syncVersion || loadingHistory.value || moreRecent.value) return
    const scroll = followingLatest()
    mergeMessages(page.messages)
    cursor = page.messages.at(-1)?.id || cursor
    cursors.set(cid, cursor)
    more = page.more && page.messages.length > 0
    if (scroll) await bottom(selection)
  }
  await markRead()
}
async function markRead() {
  if (entertainment.value || document.hidden || (!showMobileChat.value && matchMedia('(max-width:760px)').matches) || !atBottom() || !me.value) return
  const cid = currentID.value, last = messages.value.at(-1)?.id
  if (!last || (readSent.get(cid) || 0) >= last) return
  readSent.set(cid, last)
  try {
    await api(`/conversations/${cid}/read`, 'POST', { message_id: last })
    scheduleRefresh()
  } catch { readSent.delete(cid) }
}
async function enterApp() {
  me.value = await api<User>('/me')
  profile.value = { nickname: me.value.nickname, old_password: '', new_password: '' }
  try { pending.value = JSON.parse(localGet(pendingKey()) || '[]') } catch { pending.value = [] }
  await Promise.all([refreshPeople(), refreshConversations()])
  if (conversations.value.length) await selectConversation(conversations.value[0], false)
  showMobileChat.value = false
  intentionalClose = false; reconnectAttempt = 0
  connect()
}
function connect() {
  if (intentionalClose || !me.value) return
  clearTimeout(reconnectTimer)
  connection.value = reconnectAttempt ? '重新连接中' : '连接中'
  const socket = new WebSocket(`${location.protocol === 'https:' ? 'wss:' : 'ws:'}//${location.host}/api/ws`)
  ws = socket
  socket.onopen = () => { if (ws === socket) { reconnectAttempt = 0; connection.value = '实时在线' } }
  socket.onmessage = async e => {
    if (ws !== socket) return
    try {
      const event = JSON.parse(e.data)
      if (event.type === 'hello') {
        window.dispatchEvent(new Event('linchat:reconnected'))
        gameRevision.value++
        await Promise.all([refreshConversations(), syncActive(), loadMembers()])
        await refreshPeople()
        for (const p of pending.value) if (p.retryable) p.error = ''
        await flushPending()
      } else if (event.type === 'message') {
        const m = event.message as Message
        const alreadyReceived = messages.value.some(item => item.id === m.id)
        const mentioned = mentionedMe(m)
        const readingHere = !entertainment.value && m.conversation_id === currentID.value && atBottom() && !document.hidden &&
          (showMobileChat.value || !matchMedia('(max-width:760px)').matches)
        if (m.conversation_id === currentID.value && historyReady && !loadingHistory.value && !moreRecent.value) {
          const scroll = followingLatest(); mergeMessages([m])
          if (scroll) { await bottom(); await markRead() }
        } else if (m.sender_id === me.value?.id) {
          pending.value = pending.value.filter(p => p.client_id !== m.client_id); savePending()
        }
        scheduleRefresh()
        if (mentioned && !readingHere && !alreadyReceived && !document.hidden) {
          const name = conversations.value.find(c => c.id === m.conversation_id)?.name || '群聊'
          notify(`${m.sender_name} 在「${name}」中 @了你`, 'mention')
        }
        if (document.hidden && m.sender_id !== me.value?.id && !alreadyReceived) {
          const body = m.kind === 'text' ? m.content.slice(0, 100) : '[附件]'
          const title = mentioned ? `${m.sender_name} 在群聊中 @了你` : m.sender_name
          if (desktop) void desktop.notify(title, body)
          else if ('Notification' in window && Notification.permission === 'granted') {
            const n = new Notification(title, { body, tag: `conv-${m.conversation_id}` })
            n.onclick = () => { window.focus(); const c = conversations.value.find(c => c.id === m.conversation_id); if (c) void selectConversation(c); n.close() }
          }
        }
      } else if (event.type === 'presence') {
        for (const list of [people.value, members.value]) {
          const p = list.find(p => p.id === event.user_id); if (p) p.online = event.online
        }
      } else if (event.type === 'read') {
        if (event.conversation_id === currentID.value) {
          const p = members.value.find(p => p.id === event.user_id)
          if (p) p.last_read = Math.max(p.last_read || 0, event.last_read)
        }
        if (event.user_id === me.value?.id) scheduleRefresh()
      } else if (event.type === 'campaign.stream') {
        window.dispatchEvent(new CustomEvent('linchat:campaign-stream', { detail: event.campaign_id }))
      } else if (event.type === 'campaign.changed') {
        window.dispatchEvent(new CustomEvent('linchat:campaign-stream', { detail: event.campaign_id }))
        gameRevision.value++; scheduleRefresh()
        if (event.conversation_id === currentID.value) await syncActive()
      } else if (event.type === 'users.changed' || event.type === 'conversations.changed') {
        gameRevision.value++
        await Promise.all([refreshPeople(), refreshConversations(), loadMembers()])
      }
    } catch (e) { if (e instanceof ApiError && e.status === 401) sessionExpired() }
  }
  socket.onclose = () => {
    if (ws !== socket || intentionalClose) return
    connection.value = '连接已断开'
    const delay = Math.min(30000, 1000 * 2 ** Math.min(reconnectAttempt++, 5)) + Math.random() * 500
    reconnectTimer = setTimeout(async () => {
      try { await api('/me'); connect() }
      catch (e) {
        if (e instanceof ApiError && e.status === 401) sessionExpired()
        else connect()
      }
    }, delay)
  }
  socket.onerror = () => socket.close()
}
function sessionExpired() {
  saveReadingPosition(); ++selectionVersion; ++syncVersion; historyReady = false
  intentionalClose = true; clearTimeout(reconnectTimer); ws?.close(); ws = null
  me.value = null; currentID.value = 0; messages.value = []; error.value = '登录已过期，请重新登录'
  cursors.clear(); readSent.clear()
}
async function submitAuth() {
  if (busy.value) return
  busy.value = true; error.value = ''
  try {
    const path = !initialized.value ? '/setup' : authMode.value === 'register' ? '/auth/register' : '/auth/login'
    const body = path === '/auth/login' ? { username: form.value.username, password: form.value.password } : form.value
    await api(path, 'POST', body)
    initialized.value = true; form.value.password = ''; form.value.token = ''
    await enterApp()
  } catch (e) { error.value = explain(e) }
  finally { busy.value = false }
}
async function send() {
  if (composing.value) return
  const { content, mentions } = prepareText(draft.value, draftMentions.value)
  if (!content || !currentID.value || !me.value) return
  if (Array.from(content).length > 4000) { notify('每条消息最多 4000 个字符'); return }
  const version = selectionVersion
  if (moreRecent.value && !(await latest())) return
  if (version !== selectionVersion) return
  pending.value.push({ client_id: uid(), conversation_id: currentID.value, content, mentions, created_at: Date.now() })
  draft.value = ''; draftMentions.value = []; mentionSearch.value = null; savePending(); await bottom()
  await flushPending()
}
async function flushPending() {
  if (flushing || !me.value) return
  flushing = true
  try {
    for (const p of [...pending.value]) {
      if (p.error) continue
      try {
        const m = await api<Message>(`/conversations/${p.conversation_id}/messages`, 'POST', {
          client_id: p.client_id, content: p.content, ...(p.mentions?.length ? { mentions: p.mentions } : {}),
        })
        const scroll = m.conversation_id === currentID.value && followingLatest()
        if (m.conversation_id === currentID.value && !moreRecent.value && !loadingHistory.value) mergeMessages([m])
        else { pending.value = pending.value.filter(item => item.client_id !== m.client_id); savePending() }
        scheduleRefresh()
        if (scroll) { await bottom(); await markRead() }
      } catch (e) {
        const entry = pending.value.find(x => x.client_id === p.client_id)
        if (entry) { entry.error = explain(e); entry.retryable = !(e instanceof ApiError) || e.status >= 500 || e.status === 429 }
        if (e instanceof ApiError && e.status === 401) sessionExpired()
        savePending(); break
      }
    }
  } finally { flushing = false }
}
function retry(p: Pending) { p.error = ''; savePending(); void flushPending() }
function discard(p: Pending) { pending.value = pending.value.filter(x => x.client_id !== p.client_id); savePending() }
function mentionedMe(m: Message) { return m.sender_id !== me.value?.id && !!m.mentions?.some(item => item.user_id === me.value?.id) }
function updateMentionSearch() {
  const input = composer.value
  if (!input || !canMention.value || input.selectionStart !== input.selectionEnd) { mentionSearch.value = null; return }
  const next = mentionQuery(draft.value, input.selectionStart, draftMentions.value)
  if (next?.query !== mentionSearch.value?.query || next?.start !== mentionSearch.value?.start) mentionIndex.value = 0
  mentionSearch.value = next
}
function onDraftInput(e: Event) {
  const value = (e.target as HTMLTextAreaElement).value
  draftMentions.value = editMentions(draft.value, value, draftMentions.value)
  draft.value = value
  updateMentionSearch()
}
async function chooseMention(person: User) {
  const search = mentionSearch.value
  if (!search || !canMention.value) return
  if (draftMentions.value.length >= 100) { notify('每条消息最多包含 100 处 @成员'); return }
  const label = '@' + person.nickname
  const value = draft.value.slice(0, search.start) + label + ' ' + draft.value.slice(search.end)
  draftMentions.value = editMentions(draft.value, value, draftMentions.value)
  draftMentions.value.push({ user_id: person.id, start: search.start, end: search.start + label.length })
  draftMentions.value.sort((a, b) => a.start - b.start)
  draft.value = value; mentionSearch.value = null
  await nextTick()
  composer.value?.focus()
  composer.value?.setSelectionRange(search.start + label.length + 1, search.start + label.length + 1)
}
async function startMention() {
  const input = composer.value
  if (!input || !canMention.value) return
  const start = input.selectionStart, end = input.selectionEnd
  const prefix = start > 0 && !/\s/u.test(draft.value[start - 1]) ? ' @' : '@'
  const value = draft.value.slice(0, start) + prefix + draft.value.slice(end)
  draftMentions.value = editMentions(draft.value, value, draftMentions.value)
  draft.value = value
  await nextTick(); input.focus(); input.setSelectionRange(start + prefix.length, start + prefix.length)
  updateMentionSearch()
}
function onComposerKey(e: KeyboardEvent) {
  if (e.isComposing || composing.value || e.keyCode === 229) return
  if (mentionOpen.value) {
    if (e.key === 'Escape') { e.preventDefault(); mentionSearch.value = null; return }
    if ((e.key === 'ArrowDown' || e.key === 'ArrowUp') && mentionOptions.value.length) {
      e.preventDefault()
      mentionIndex.value = (mentionIndex.value + (e.key === 'ArrowDown' ? 1 : -1) + mentionOptions.value.length) % mentionOptions.value.length
      void nextTick(() => document.getElementById(`mention-option-${selectedMention.value?.id}`)?.scrollIntoView({ block: 'nearest' }))
      return
    }
    if ((e.key === 'Enter' || e.key === 'Tab') && !e.shiftKey && selectedMention.value) {
      e.preventDefault(); void chooseMention(selectedMention.value); return
    }
  }
  if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); void send() }
}
function onComposerCursor(e: KeyboardEvent) {
  if (['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(e.key)) updateMentionSearch()
}
async function checkDesktopUpdate() {
  if (!desktop?.checkUpdates) return
  try {
    desktopUpdate.value = await desktop.checkUpdates()
    const state = desktopUpdate.value
    if (state.error) notify(state.error)
    if (state.available) dismissedUpdate.value = ''
    else if (!state.error) notify(`当前已是最新版本 v${state.currentVersion}`)
  } catch { notify('无法检查更新，请稍后重试') }
}
const updateBusy = computed(() => ['downloading', 'installing'].includes(desktopUpdate.value?.status || ''))
const updateDetail = computed(() => {
  const state = desktopUpdate.value
  if (state?.error) return state.error
  if (!state?.autoUpdate) return '下载新版安装包，安装后即可使用新功能。'
  if (state.status === 'ready') return '重启后自动安装，保留安装目录和数据。本机服务会暂时停止。'
  if (state.status === 'installing') return '正在关闭本机服务，随后自动安装并重启。'
  return `正在后台下载并校验 · ${Math.floor(state.progress || 0)}%，你可以继续聊天。`
})
const updateAction = computed(() => {
  const state = desktopUpdate.value
  if (state?.status === 'ready') return '重启并更新'
  if (state?.status === 'installing') return '正在更新…'
  if (state?.status === 'downloading') return '下载中…'
  return state?.autoUpdate ? '重试下载' : '下载更新'
})
async function openDesktopUpdate() {
  try { await desktop?.openUpdate?.() } catch { notify('无法更新，请稍后重试') }
}
async function sendFile(file: File, cid = currentID.value, clientID = uid()) {
  if (upload.value || !cid) return
  if (file.size > maxUpload.value) { notify(`每个文件最多 ${bytes(maxUpload.value)}`); return }
  upload.value = { name: file.name, percent: 0 }; uploadRetry.value = null
  try {
    const m = await new Promise<Message>((resolve, reject) => {
      const xhr = new XMLHttpRequest()
      xhr.open('POST', `/api/conversations/${cid}/files?client_id=${encodeURIComponent(clientID)}`)
      xhr.setRequestHeader('X-LinChat', '1'); xhr.timeout = 150000
      xhr.upload.onprogress = e => { if (e.lengthComputable && upload.value) upload.value.percent = Math.round(e.loaded / e.total * 100) }
      xhr.onerror = () => reject(new Error('network'))
      xhr.ontimeout = () => reject(new Error('timeout'))
      xhr.onload = () => {
        try {
          const data = JSON.parse(xhr.responseText)
          if (xhr.status >= 200 && xhr.status < 300) resolve(data)
          else reject(new ApiError(data.error || '上传失败', xhr.status))
        } catch (e) { reject(e) }
      }
      const body = new FormData(); body.append('file', file); xhr.send(body)
    })
    scheduleRefresh()
    if (cid === currentID.value) {
      if (moreRecent.value && !(await latest())) return
      if (cid !== currentID.value) return
      mergeMessages([m]); await bottom(); await markRead()
    }
  } catch (e) { uploadRetry.value = { file, cid, clientID }; notify(explain(e)) }
  finally { upload.value = null; if (fileInput.value) fileInput.value.value = '' }
}
function pickFile(e: Event) { const f = (e.target as HTMLInputElement).files?.[0]; if (f) void sendFile(f) }
function dropped(e: DragEvent) { dragging.value = false; const f = e.dataTransfer?.files[0]; if (f) void sendFile(f) }
function paste(e: ClipboardEvent) { const f = e.clipboardData?.files[0]; if (f) { e.preventDefault(); void sendFile(f) } }
function readCount(m: Message) { return members.value.filter(p => p.id !== m.sender_id && (p.last_read || 0) >= m.id).length }
function openModal(m: Modal) {
  error.value = ''; directoryQuery.value = ''; selectedUsers.value = []; groupName.value = ''
  if (m === 'settings' && me.value) profile.value = { nickname: me.value.nickname, old_password: '', new_password: '' }
  if (m === 'connection') { connectionURL.value = location.origin; connectionLabel.value = serverName.value }
  modal.value = m
  if (m === 'group' || m === 'directory' || m === 'addMembers') void refreshPeople().catch(e => { error.value = explain(e) })
}
async function direct(p: User) {
  try {
    const c = await api<{ id: number }>('/conversations', 'POST', { kind: 'direct', name: '', members: [p.id] })
    await refreshConversations(); modal.value = ''
    const target = conversations.value.find(x => x.id === c.id); if (target) await selectConversation(target)
  } catch (e) { error.value = explain(e) }
}
async function saveGroup() {
  if (busy.value) return
  busy.value = true; error.value = ''
  try {
    if (modal.value === 'addMembers') {
      await api(`/conversations/${currentID.value}/members`, 'POST', { members: selectedUsers.value })
      await Promise.all([refreshConversations(), loadMembers()])
    } else {
      const result = await api<{ id: number }>('/conversations', 'POST', { kind: 'group', name: groupName.value, members: selectedUsers.value })
      await refreshConversations()
      const target = conversations.value.find(c => c.id === result.id); if (target) await selectConversation(target)
    }
    modal.value = ''
  } catch (e) { error.value = explain(e) }
  finally { busy.value = false }
}
async function createInvite() {
  busy.value = true; error.value = ''
  try {
    const result = await api<{ token: string }>('/invites', 'POST', { uses: inviteUses.value })
    inviteLink.value = `${location.origin}/#invite=${result.token}`
  } catch (e) { error.value = explain(e) }
  finally { busy.value = false }
}
async function copy(text: string) {
  try { await navigator.clipboard.writeText(text); notify('已复制') }
  catch {
    const el = document.createElement('textarea'); el.value = text; el.style.position = 'fixed'; el.style.opacity = '0'
    document.body.appendChild(el); el.select()
    const ok = document.execCommand('copy'); el.remove()
    notify(ok ? '已复制' : '请选中文字后手动复制')
  }
}
async function saveProfile() {
  busy.value = true; error.value = ''
  try {
    await api('/me', 'PATCH', profile.value)
    me.value = await api<User>('/me'); modal.value = ''; notify('个人设置已保存')
  } catch (e) { error.value = explain(e) }
  finally { busy.value = false }
}
async function logout() {
  try {
    await api('/auth/logout', 'POST', {})
    saveReadingPosition(); ++selectionVersion; ++syncVersion; historyReady = false
    intentionalClose = true; clearTimeout(reconnectTimer); ws?.close(); ws = null
    me.value = null; currentID.value = 0; messages.value = []; pending.value = []; modal.value = ''
    cursors.clear(); readSent.clear(); authMode.value = 'login'
  } catch (e) { notify(explain(e)) }
}
function switchServer(raw = connectionURL.value) {
  try {
    const u = new URL(raw)
    if (!['http:', 'https:'].includes(u.protocol) || u.username || u.password || (u.pathname !== '/' && u.pathname !== '')) throw new Error()
    const url = u.origin
    const entries = [{ name: connectionLabel.value.trim() || u.hostname, url }, ...recentServers.value.filter(s => s.url !== url)].slice(0, 8)
    localSet('linchat-servers', JSON.stringify(entries))
    location.assign(url)
  } catch { error.value = '请输入完整地址，例如 https://chat.linblog.site 或 http://26.1.2.3:8787' }
}
async function notifications() {
  if (desktop) { await desktop.notify('LinChat', '桌面通知已就绪，最小化后收到新消息会在这里提醒。'); return }
  if (!('Notification' in window) || !window.isSecureContext) { notify('系统通知需要 HTTPS 或本机 localhost；当前入口可使用页面未读提示'); return }
  const permission = await Notification.requestPermission()
  notify(permission === 'granted' ? '已开启系统消息通知' : '可在浏览器网站设置中管理通知权限')
}
function onVisible() {
  if (document.hidden) saveReadingPosition()
  if (!document.hidden && me.value) void Promise.all([syncActive(), refreshConversations()]).catch(() => {})
}
async function openCampaignChat(cid: number) {
  await refreshConversations()
  const target = conversations.value.find(c => c.id === cid)
  if (target) { entertainment.value = false; await nextTick(); await selectConversation(target) }
}
onMounted(async () => {
  if (desktop?.getUpdateState) {
    desktopUpdate.value = await desktop.getUpdateState().catch(() => null)
    offUpdate = desktop.onUpdateState?.(state => desktopUpdate.value = state)
  }
  try { recentServers.value = JSON.parse(localGet('linchat-servers') || '[]') } catch {}
  const hash = new URLSearchParams(location.hash.slice(1))
  if (hash.get('setup')) form.value.token = hash.get('setup')!
  if (hash.get('invite')) { form.value.token = hash.get('invite')!; authMode.value = 'register' }
  if (hash.has('setup') || hash.has('invite')) history.replaceState(null, '', location.pathname)
  try {
    const status = await api<{ name: string; initialized: boolean; max_upload: number }>('/status')
    serverName.value = status.name; initialized.value = status.initialized; maxUpload.value = status.max_upload
    if (status.initialized) {
      try { await enterApp() } catch (e) { if (!(e instanceof ApiError && e.status === 401)) error.value = explain(e) }
    }
  } catch (e) { error.value = explain(e) }
  ready.value = true
  document.addEventListener('visibilitychange', onVisible)
  document.addEventListener('keydown', onModalKey)
  window.addEventListener('pagehide', saveReadingPosition)
})
onBeforeUnmount(() => {
  saveReadingPosition(); ++selectionVersion; ++syncVersion
  messageResize?.disconnect(); clearTimeout(positionTimer)
  offUpdate?.()
  intentionalClose = true; ws?.close(); clearTimeout(reconnectTimer); clearTimeout(refreshTimer); clearTimeout(toastTimer)
  document.removeEventListener('visibilitychange', onVisible)
  document.removeEventListener('keydown', onModalKey)
  window.removeEventListener('pagehide', saveReadingPosition)
})
</script>

<template>
  <div class="app-shell" :class="{ 'signed-in': me }">
  <aside v-if="desktopUpdate?.available && dismissedUpdate !== desktopUpdate.latestVersion" class="desktop-update-notice" role="status">
    <Icon name="download" :size="20" /><div><b>LinChat v{{ desktopUpdate.latestVersion }} {{ desktopUpdate.status === 'ready' ? '已就绪' : '可更新' }}</b><p>{{ updateDetail }}</p></div><button class="text-button" :disabled="updateBusy" @click="openDesktopUpdate">{{ updateAction }}</button><button class="icon-button" aria-label="暂时关闭更新提醒" @click="dismissedUpdate = desktopUpdate.latestVersion"><Icon name="close" :size="16" /></button>
  </aside>
  <div v-if="!ready" class="loading-screen"><img src="/favicon.svg" alt="" width="56" /><p>正在连接 LinChat…</p></div>
  <main v-else-if="!me" class="auth-page" :inert="modal ? true : undefined">
    <section class="auth-story">
      <a class="brand" href="/"><img src="/favicon.svg" alt="" /><span>LinChat<span class="brand-dot">.</span></span></a>
      <div class="story-content">
        <h1>距离远一点，<br />交流<span>近一点。</span></h1>
        <p>一处属于你们的交流空间。<br />从云端到虚拟局域网，让每一句话及时抵达。</p>
        <div class="story-conversation">
          <p class="story-example-label">对话示例</p>
          <div class="story-message"><span class="avatar peach">林</span><div>无论在哪里，都可以一起聊。<small>刚刚 · 已送达</small></div></div>
          <div class="story-message reply"><div>我们在这里，连接成功！<small>刚刚 <Icon name="check" :size="12" /></small></div><span class="avatar mint">友</span></div>
          <div class="network-line"><span><Icon name="cloud" :size="16" /> 云端</span><i></i><Icon name="chat" /><i></i><span><Icon name="network" :size="16" /> Radmin VPN</span></div>
        </div>
      </div>
      <footer>为真实的交流而设计 <span>云端 / Radmin</span></footer>
    </section>
    <section class="auth-panel">
      <div class="auth-actions"><button class="text-button" @click="openModal('appearance')"><Icon name="palette" :size="17" />外观</button><button class="text-button auth-switch" @click="openModal('connection')"><Icon name="globe" :size="17" /> 切换服务端</button></div>
      <div class="auth-form-wrap">
        <span class="pill"><span class="live-dot"></span>{{ networkMode }}</span>
        <h2>{{ !initialized ? '创建你的聊天空间' : authMode === 'register' ? '加入你们的对话' : '欢迎回来' }}</h2>
        <p class="muted">{{ !initialized ? '首次使用，设置管理员账号。' : authMode === 'register' ? '使用管理员的邀请，创建自己的账号。' : '登录 ' + serverName + '，继续未完的对话。' }}</p>
        <form class="form" @submit.prevent="submitAuth">
          <label v-if="!initialized || authMode === 'register'">{{ !initialized ? '初始化令牌' : '邀请码' }}<input v-model="form.token" autocomplete="off" required :placeholder="!initialized ? '从服务端的首次初始化地址获取' : '粘贴管理员提供的邀请码'" /></label>
          <label>账号<input v-model="form.username" autocomplete="username" required pattern="[a-zA-Z0-9_]{3,32}" placeholder="字母、数字或下划线" /></label>
          <label v-if="!initialized || authMode === 'register'">昵称<input v-model="form.nickname" autocomplete="nickname" required maxlength="30" placeholder="大家怎么称呼你？" /></label>
          <label>密码<input v-model="form.password" type="password" :autocomplete="initialized && authMode === 'login' ? 'current-password' : 'new-password'" required :minlength="initialized && authMode === 'login' ? 1 : 10" maxlength="72" placeholder="请输入密码（新密码至少 10 位）" /></label>
          <p v-if="error" class="error" role="alert">{{ error }}</p>
          <button class="primary wide" :disabled="busy">{{ busy ? '正在连接…' : !initialized ? '创建空间' : authMode === 'register' ? '注册并加入' : '登录 LinChat' }}<Icon name="send" :size="17" /></button>
        </form>
        <p v-if="initialized" class="auth-alternative">{{ authMode === 'login' ? '收到了邀请？' : '已有账号？' }}<button class="text-button" @click="authMode = authMode === 'login' ? 'register' : 'login'; error = ''">{{ authMode === 'login' ? '创建账号' : '去登录' }}</button></p>
        <div class="auth-foot"><Icon name="lock" :size="15" /><span>邀请制加入 · 独立数据空间</span></div>
      </div>
      <div class="auth-server">{{ location.host }}<span>LinChat</span></div>
    </section>
  </main>

   <main v-else class="workspace" :data-scroll-key="`${me.id}/workspace/${currentID}`" :inert="modal ? true : undefined" :class="{ 'mobile-chat': showMobileChat && !entertainment, 'entertainment-workspace': entertainment }">
    <nav class="rail" aria-label="主导航">
      <a class="rail-logo" href="/" title="LinChat"><img src="/favicon.svg" alt="LinChat" /></a>
       <button class="rail-button" :class="{ active: !entertainment }" title="消息" aria-label="消息" @click="entertainment = false; showMobileChat = false"><Icon name="chat" /><span v-if="totalUnread" class="rail-unread"></span></button>
       <button class="rail-button" :class="{ active: entertainment }" title="娱乐模式" aria-label="娱乐模式" @click="entertainment = true"><Icon name="dice" /></button>
      <button class="rail-button" title="通讯录" aria-label="通讯录" @click="openModal('directory')"><Icon name="users" /></button>
      <button v-if="me.admin" class="rail-button" title="邀请成员" aria-label="邀请成员" @click="openModal('invite')"><Icon name="plus" /></button>
      <div class="rail-bottom">
        <button class="rail-button" title="外观与配色" aria-label="外观与配色" @click="openModal('appearance')"><Icon name="palette" /></button>
        <button class="rail-button" title="服务端连接" aria-label="服务端连接" @click="openModal('connection')"><Icon name="globe" /></button>
        <button class="rail-button" title="个人设置" aria-label="个人设置" @click="openModal('settings')"><Icon name="settings" /></button>
        <button class="avatar me-avatar" :class="hue(me.id)" :title="me.nickname" aria-label="我的账号" @click="openModal('settings')">{{ avatar(me.nickname) }}</button>
      </div>
    </nav>
     <KeepAlive>
       <Entertainment v-if="entertainment" :key="me.id" :me="me" :people="people" :api="api" :revision="gameRevision" @notify="notify" @chat="openCampaignChat" />
     </KeepAlive>
     <template v-if="!entertainment">
     <aside class="conversation-panel">
      <header class="list-heading"><h1>消息<span class="count-pill">{{ conversations.length }}</span></h1><button class="icon-button outlined" title="新建群聊" aria-label="新建群聊" @click="openModal('group')"><Icon name="plus" /></button></header>
      <div class="search-box"><Icon name="search" :size="17" /><input v-model="query" aria-label="搜索会话" placeholder="搜索会话…" /></div>
      <div class="list-subtitle"><span>全部会话</span><span>{{ onlineCount }} 人在线</span></div>
       <div class="conversation-list" :data-scroll-key="`${me.id}/conversations/${query ? 'filtered' : 'all'}`">
        <button v-for="c in filteredConversations" :key="c.id" class="conversation-row" :class="{ selected: currentID === c.id }" @click="selectConversation(c)">
          <span class="avatar" :class="c.kind === 'lobby' ? 'dark' : hue(c.id)"><Icon v-if="c.kind === 'lobby'" name="globe" :size="22" /><Icon v-else-if="c.kind === 'group'" name="users" :size="21" /><template v-else>{{ avatar(c.name) }}</template></span>
          <span class="conversation-text"><span class="conversation-title"><span :title="c.name">{{ c.name }}</span><small>{{ time(c.last_at) }}</small></span><span class="preview"><strong v-if="c.unread_mentions" class="mention-unread" :aria-label="`${c.unread_mentions} 条消息提到了你`">@我</strong><span class="preview-text">{{ c.preview }}</span><b v-if="c.unread" class="unread">{{ c.unread > 99 ? '99+' : c.unread }}</b></span></span>
        </button>
        <p v-if="!filteredConversations.length" class="small-empty">没有找到相关会话</p>
      </div>
      <button class="server-status" @click="openModal('connection')"><span class="status-dot" :class="{ offline: connection !== '实时在线' }"></span><span>{{ serverName }}<small>{{ connection }}</small></span><Icon name="globe" :size="17" /></button>
    </aside>
    <section class="chat-panel" @dragover.prevent="dragging = true" @dragleave.self="dragging = false" @drop.prevent="dropped">
      <template v-if="current">
        <header class="chat-header">
          <button class="icon-button mobile-back" aria-label="返回会话列表" @click="showMobileChat = false"><Icon name="arrow" /></button>
          <span class="avatar header-avatar" :class="hue(current.id)"><Icon v-if="current.kind !== 'direct'" name="users" /><template v-else>{{ avatar(current.name) }}</template></span>
          <div class="chat-title"><h2>{{ current.name }}<span v-if="current.kind === 'lobby'" class="tag">公共空间</span></h2><p><span class="live-dot"></span>{{ current.member_count }} 位成员 <span class="dot-divider">·</span> {{ activeOnline }} 人在线</p></div>
          <button class="icon-button" aria-label="会话成员" title="会话成员" @click="showDetails = !showDetails"><Icon name="users" /></button>
          <button class="icon-button" aria-label="会话信息" title="会话信息" @click="openModal('members')"><Icon name="info" /></button>
        </header>
        <div v-if="connection !== '实时在线'" class="connection-banner"><span class="spinner"></span>{{ connection }}，待发送消息会保留在此设备上。<button class="text-button" @click="flushPending">重试发送</button></div>
        <div class="chat-body">
          <div ref="messageList" class="message-list" role="log" aria-label="聊天记录" :aria-busy="loadingHistory" @scroll.passive="onMessageScroll">
            <div ref="messageContent" :class="{ 'empty-message-flow': !messages.length && !pendingHere.length }">
            <button v-if="moreHistory" class="history-button" :disabled="loadingHistory" @click="older">{{ loadingHistory ? '加载中…' : '查看更早的消息' }}</button>
            <div v-if="!messages.length && !pendingHere.length" class="chat-welcome"><span class="welcome-icon"><Icon name="chat" :size="30" /></span><h3>{{ loadingHistory ? '正在加载对话…' : '对话，从一句你好开始' }}</h3><p>{{ current.kind === 'lobby' ? '这里是大家的公共空间，打个招呼吧。' : '文字、图片、文件，让想法自由流动。' }}</p><span class="welcome-rule"></span></div>
            <template v-for="(m, i) in messages" :key="m.id">
              <div v-if="i === 0 || date(m.created_at) !== date(messages[i - 1].created_at)" class="date-separator"><span>{{ date(m.created_at) }}</span></div>
              <article class="message" :class="{ own: m.sender_id === me.id, mentioned: mentionedMe(m) }" :data-message-id="m.id">
                <span v-if="m.sender_id !== me.id" class="avatar small-avatar" :class="hue(m.sender_id)">{{ avatar(m.sender_name) }}</span>
                <div class="message-content"><div class="message-meta"><b>{{ m.sender_id === me.id ? '我' : m.sender_name }}</b><time>{{ time(m.created_at) }}</time><span v-if="mentionedMe(m)" class="mention-label">提到了你</span></div>
                  <div v-if="m.kind === 'text'" class="bubble"><span v-for="(part, partIndex) in messageParts(m.content, m.mentions)" :key="partIndex" :class="{ 'mention-text': part.userID, 'mention-self': part.userID === me.id }">{{ part.text }}</span></div>
                  <a v-else-if="m.kind === 'image'" class="image-bubble" :href="`/api/files/${m.file_id}?download=1`" :download="m.file_name" :title="`下载 ${m.file_name}`"><img :src="`/api/files/${m.file_id}`" :alt="m.file_name" loading="lazy" /><span><Icon name="download" :size="14" />{{ m.file_name }}</span></a>
                  <a v-else class="file-bubble" :href="`/api/files/${m.file_id}?download=1`" :download="m.file_name"><span class="file-icon"><Icon name="file" :size="25" /></span><span><b>{{ m.file_name }}</b><small>{{ bytes(m.file_size) }} · 点击下载</small></span><Icon name="download" :size="17" /></a>
                  <span v-if="m.sender_id === me.id" class="message-receipt"><Icon name="check" :size="12" />{{ readCount(m) ? `${readCount(m)} 人已读` : '已发送' }}</span>
                </div>
              </article>
            </template>
            <div v-if="moreRecent" class="history-actions">
              <button class="history-button" :disabled="loadingHistory" @click="newer">{{ loadingHistory ? '加载中…' : '查看更新的消息' }}</button>
              <button class="history-button" :disabled="loadingHistory" @click="latest">回到最新消息</button>
            </div>
            <article v-for="p in pendingHere" :key="p.client_id" class="message own pending"><div class="message-content"><div class="message-meta"><b>我</b><time>{{ time(p.created_at) }}</time></div><div class="bubble">{{ p.content }}</div><div class="message-receipt" :class="{ failed: p.error }"><template v-if="p.error"><span>{{ p.error }}</span><button @click="retry(p)">重试</button><button @click="discard(p)">取消</button></template><template v-else><span class="spinner"></span>发送中…</template></div></div></article>
            </div>
          </div>
          <aside v-if="showDetails" class="details-panel"><div class="details-heading"><h3>会话成员</h3><button class="icon-button" aria-label="关闭成员列表" @click="showDetails = false"><Icon name="close" :size="16" /></button></div><button v-for="p in members" :key="p.id" class="member-row" :disabled="p.id === me.id" @click="direct(p)"><span class="avatar small-avatar" :class="hue(p.id)">{{ avatar(p.nickname) }}</span><span>{{ p.nickname }}<small>{{ p.online ? '在线' : '离线' }}</small></span><i class="status-dot" :class="{ offline: !p.online }"></i></button><button v-if="current.kind === 'group' && (current.owner_id === me.id || me.admin)" class="text-button add-members" @click="openModal('addMembers')"><Icon name="plus" :size="16" />添加成员</button></aside>
        </div>
        <div v-if="upload" class="upload-progress"><Icon name="file" :size="16" /><span>{{ upload.name }}</span><progress :value="upload.percent" max="100"></progress><b>{{ upload.percent }}%</b></div>
        <div v-if="uploadRetry" class="upload-progress failed"><span>附件发送未确认：{{ uploadRetry.file.name }}</span><button class="text-button" @click="sendFile(uploadRetry.file, uploadRetry.cid, uploadRetry.clientID)">重试</button><button class="text-button" @click="uploadRetry = null">取消</button></div>
        <footer class="composer-area">
          <div class="composer-box">
            <div v-if="mentionOpen" class="mention-picker">
              <div class="mention-heading"><b>提及成员</b><span>{{ mentionSearch?.query ? '按昵称或账号搜索' : '↑↓ 选择 · Enter 确认' }}</span></div>
              <div id="mention-options" class="mention-options" role="listbox" aria-label="可提及的群成员">
                <button v-for="(person, index) in mentionOptions" :id="`mention-option-${person.id}`" :key="person.id" class="mention-option" :class="{ active: selectedMention?.id === person.id }" role="option" :aria-selected="selectedMention?.id === person.id" tabindex="-1" @mousedown.prevent @mouseenter="mentionIndex = index" @click="chooseMention(person)">
                  <span class="avatar small-avatar" :class="hue(person.id)">{{ avatar(person.nickname) }}</span><span class="mention-person"><b>{{ person.nickname }}</b><small>@{{ person.username }}</small></span><span class="mention-presence">{{ person.online ? '在线' : '离线' }}</span>
                </button>
                <p v-if="!mentionOptions.length" class="mention-empty" role="status">{{ !membersReady ? '正在加载群成员…' : '没有匹配的成员，试试昵称或账号' }}</p>
              </div>
            </div>
            <textarea ref="composer" :value="draft" :placeholder="`发送消息到 ${current.name}${canMention ? '，输入 @ 提及成员' : ''}…`" aria-label="消息内容" role="combobox" aria-autocomplete="list" aria-haspopup="listbox" :aria-expanded="mentionOpen" :aria-controls="mentionOpen ? 'mention-options' : undefined" :aria-activedescendant="mentionOpen && selectedMention ? `mention-option-${selectedMention.id}` : undefined" rows="2" maxlength="8000" @input="onDraftInput" @keydown="onComposerKey" @keyup="onComposerCursor" @click="updateMentionSearch" @blur="mentionSearch = null" @compositionstart="composing = true" @compositionend="composing = false; updateMentionSearch()" @paste="paste"></textarea>
            <div class="composer-tools"><div><button class="icon-button" :disabled="!!upload" aria-label="发送图片或文件" title="图片或文件（最大 25 MB）" @click="fileInput?.click()"><Icon name="attach" /></button><button v-if="canMention" class="icon-button" aria-label="提及群成员" title="提及群成员" @mousedown.prevent @click="startMention"><Icon name="at" /></button><span class="composer-hint">{{ mentionedPeople ? `将提醒 ${mentionedPeople} 位成员 · Enter 发送` : 'Enter 发送 · Shift + Enter 换行' }}</span></div><button class="send-button" :disabled="!draft.trim() || composing" @click="send">发送<Icon name="send" :size="16" /></button></div>
          </div>
          <p class="composer-note"><Icon name="lock" :size="11" />仅会话成员可访问消息和附件</p>
          <input ref="fileInput" type="file" class="visually-hidden" aria-label="选择文件" @change="pickFile" />
        </footer>
      </template>
      <div v-else class="empty-space"><Icon name="chat" :size="48" /><h2>你的下一段对话，从这里开始</h2><button class="primary" @click="openModal('directory')">开始聊天</button></div>
      <div v-if="dragging && current" class="drop-overlay"><Icon name="attach" :size="40" /><h3>松开以发送文件</h3><p>最大 {{ bytes(maxUpload) }}</p></div>
     </section>
     </template>
   </main>

  <div v-if="modal" class="modal-backdrop" @click.self="modal = ''" @keydown.esc="modal = ''">
    <section ref="modalElement" class="modal" :data-scroll-key="`${me?.id || 0}/modal/${modal}/${['members', 'addMembers'].includes(modal) ? currentID : ''}/${modal === 'directory' && directoryQuery ? 'filtered' : 'all'}`" :class="{ 'appearance-modal': modal === 'appearance' }" role="dialog" aria-modal="true" tabindex="-1" :aria-label="{directory:'通讯录',group:'新建群聊',invite:'邀请成员',settings:'个人设置',appearance:'外观与配色',connection:'服务端连接',members:'会话成员',addMembers:'添加群成员'}[modal]">
      <header class="modal-header"><h2>{{ {directory:'通讯录',group:'新建群聊',invite:'邀请成员',settings:'个人设置',appearance:'外观与配色',connection:'服务端连接',members:'会话成员',addMembers:'添加群成员'}[modal] }}</h2><button class="icon-button" aria-label="关闭弹窗" @click="modal = ''"><Icon name="close" /></button></header>
      <p v-if="error" class="error" role="alert">{{ error }}</p>
      <template v-if="modal === 'directory'">
        <div class="search-box"><Icon name="search" :size="17" /><input v-model="directoryQuery" placeholder="搜索昵称或账号" aria-label="搜索成员" /></div>
        <div class="member-picker"><button v-for="p in otherUsers" :key="p.id" class="member-row" @click="direct(p)"><span class="avatar" :class="hue(p.id)">{{ avatar(p.nickname) }}</span><span>{{ p.nickname }}<small>@{{ p.username }}</small></span><span class="member-online">{{ p.online ? '在线' : '离线' }}</span><Icon name="chat" :size="17" /></button><p v-if="!otherUsers.length" class="small-empty">暂时没有其他成员。管理员可以生成邀请链接。</p></div>
      </template>
      <form v-else-if="modal === 'group' || modal === 'addMembers'" class="form" @submit.prevent="saveGroup">
        <label v-if="modal === 'group'">群聊名称<input v-model="groupName" required maxlength="40" placeholder="给你们的讨论空间起个名字" /></label>
        <label>选择成员 <small class="muted">已选 {{ selectedUsers.length }} 人</small></label>
        <div class="member-picker"><label v-for="p in groupCandidates" :key="p.id" class="member-row"><span class="avatar small-avatar" :class="hue(p.id)">{{ avatar(p.nickname) }}</span><span>{{ p.nickname }}<small>@{{ p.username }}</small></span><input v-model="selectedUsers" type="checkbox" :value="p.id" :aria-label="`选择 ${p.nickname}`" /></label><p v-if="!groupCandidates.length" class="small-empty">暂无可添加的成员</p></div>
        <button class="primary wide" :disabled="busy || !selectedUsers.length">{{ busy ? '保存中…' : modal === 'group' ? '创建群聊' : '添加成员' }}</button>
      </form>
      <template v-else-if="modal === 'invite'">
        <p class="muted modal-description">把邀请链接发送给伙伴。加入后，每个人都有独立账号，并自动进入公共大厅。</p>
        <form class="form" @submit.prevent="createInvite"><label>可使用人数<input v-model.number="inviteUses" type="number" min="1" max="100" required /></label><p class="helper">邀请有效期 7 天。Radmin 模式下，伙伴需要先加入同一个 VPN 网络。</p><button class="primary wide" :disabled="busy">{{ busy ? '生成中…' : '生成邀请链接' }}<Icon name="plus" :size="17" /></button></form>
        <div v-if="inviteLink" class="invite-result"><label>邀请链接<textarea :value="inviteLink" readonly rows="3" aria-label="邀请链接"></textarea></label><button class="text-button" @click="copy(inviteLink)"><Icon name="copy" :size="16" />复制邀请链接</button></div>
      </template>
      <form v-else-if="modal === 'settings'" class="form" @submit.prevent="saveProfile">
        <div class="profile-heading"><span class="avatar large-avatar" :class="hue(me?.id || 0)">{{ avatar(me?.nickname || '') }}</span><div><b>{{ me?.nickname }}</b><p>@{{ me?.username }}<span v-if="me?.admin" class="tag">管理员</span></p></div></div>
        <button type="button" class="appearance-link" @click="openModal('appearance')"><Icon name="palette" /><span><b>外观与配色</b><small>主题预设与自定义颜色</small></span><Icon name="arrow-right" :size="17" /></button>
        <label>昵称<input v-model="profile.nickname" required maxlength="30" /></label>
        <details><summary>修改密码</summary><label>当前密码<input v-model="profile.old_password" type="password" autocomplete="current-password" /></label><label>新密码<input v-model="profile.new_password" type="password" autocomplete="new-password" minlength="10" maxlength="72" placeholder="至少 10 位" /></label></details>
        <button class="primary wide" :disabled="busy">保存设置</button>
        <div class="settings-actions"><button type="button" class="text-button" @click="notifications"><Icon name="bell" :size="16" />开启通知</button><button type="button" class="text-button danger" @click="logout"><Icon name="logout" :size="16" />退出登录</button></div>
        <div v-if="desktop?.checkUpdates" class="desktop-version"><span>桌面版 v{{ desktopUpdate?.currentVersion || '…' }}</span><button type="button" class="text-button" :disabled="desktopUpdate?.checking" @click="checkDesktopUpdate">{{ desktopUpdate?.checking ? '检查中…' : '检查更新' }}</button></div>
      </form>
      <AppearanceSettings v-else-if="modal === 'appearance'" />
      <template v-else-if="modal === 'connection'">
        <div class="connection-card"><span class="avatar mint"><Icon :name="networkMode.includes('Radmin') ? 'network' : 'cloud'" /></span><div><b>{{ networkMode }}</b><span>{{ location.origin }}</span></div><span class="status-dot"></span></div>
        <p class="muted modal-description">云端和 Radmin 使用同一套聊天功能。独立部署的服务端各自保存账号与聊天记录；切换地址后需登录对应服务端。</p>
        <button v-if="desktop" class="primary wide" style="margin-bottom:20px" @click="desktop.openConnections()"><Icon name="network" :size="17" />打开桌面连接中心</button>
        <form class="form" @submit.prevent="switchServer()"><label>连接名称<input v-model="connectionLabel" placeholder="例如：云端空间 / 家里的 Radmin" maxlength="40" /></label><label>服务端地址<input v-model="connectionURL" type="url" required placeholder="https://chat.linblog.site" /></label><p class="helper">Radmin 地址示例：http://26.1.2.3:8787。地址由运行服务端的电脑提供。</p><button class="primary wide">连接此服务端<Icon name="globe" :size="17" /></button></form>
        <div v-if="recentServers.length" class="recent-servers"><h3>此浏览器保存的地址</h3><button v-for="s in recentServers" :key="s.url" @click="switchServer(s.url)"><Icon name="globe" :size="16" /><span>{{ s.name }}<small>{{ s.url }}</small></span><Icon name="send" :size="15" /></button></div>
      </template>
      <template v-else-if="modal === 'members'">
        <p class="muted modal-description">{{ current?.name }} · {{ members.length }} 位成员</p>
        <div class="member-picker"><div v-for="p in members" :key="p.id" class="member-row"><span class="avatar small-avatar" :class="hue(p.id)">{{ avatar(p.nickname) }}</span><span>{{ p.nickname }}<small>{{ p.id === current?.owner_id ? '创建者' : '成员' }}</small></span><span class="member-online">{{ p.online ? '在线' : '离线' }}</span></div></div>
        <button v-if="current?.kind === 'group' && (current.owner_id === me?.id || me?.admin)" class="primary wide" @click="openModal('addMembers')">添加成员<Icon name="plus" :size="17" /></button>
      </template>
    </section>
  </div>
  <div v-if="toast" class="toast" :class="{ 'mention-toast': toastKind === 'mention', 'game-toast': entertainment }" role="status"><Icon name="info" :size="17" />{{ toast }}</div>
  </div>
</template>
