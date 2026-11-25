<script setup lang="ts">
import { computed, nextTick, onActivated, onDeactivated, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import Icon from './Icon.vue'
import AbilityChecks from './AbilityChecks.vue'
import { phases, rulesets, modes, abilityNames, type Campaign, type Character, type GameEvent, type Turn, type Proposal, type AgentToken, type AISettings, type GameAPI, type GameUser } from './game-types'
import './entertainment.css'
import { getScrollPosition } from '../../desktop/scroll-memory.mjs'

const props = defineProps<{ me: GameUser; people: GameUser[]; api: GameAPI; revision: number }>()
const emit = defineEmits<{ notify: [text: string]; chat: [conversationID: number] }>()
const campaigns = ref<Campaign[]>([]), current = ref<Campaign | null>(null), loading = ref(true), refreshing = ref(false)
const listError = ref('')
const characters = ref<Character[]>([]), events = ref<GameEvent[]>([]), turns = ref<Turn[]>([]), members = ref<GameUser[]>([])
const proposals = ref<Proposal[]>([]), tokens = ref<AgentToken[]>([]), ai = ref<AISettings>({ enabled: false, has_key: false })
const tab = ref('journal'), mobileOpen = ref(false), error = ref(''), busy = ref(false), draft = ref(''), composeKind = ref('action'), recipient = ref(0)
const journal = ref<HTMLElement>(), composing = ref(false), canOlder = ref(true)
const journalReady = ref(false)
const scrollKey = (part: string, cid = current.value?.id || 0) => `${props.me.id}/campaign/${cid}/${part}`
const savedView = (key: string) => { try { return localStorage.getItem(`linchat-game-view:${props.me.id}:${key}`) || '' } catch { return '' } }
const saveView = (key: string, value: string) => { try { localStorage.setItem(`linchat-game-view:${props.me.id}:${key}`, value) } catch {} }
const isOwner = computed(() => current.value?.owner_id === props.me.id)
const privateRecipients = computed(() => members.value.filter(p => p.id !== props.me.id && (isOwner.value || p.id === current.value?.owner_id)))
const validRecipient = computed(() => privateRecipients.value.some(p => p.id === recipient.value))
const myCharacter = computed(() => characters.value.find(c => c.user_id === props.me.id))
const pendingTurn = computed(() => [...turns.value].reverse().find(t => t.status !== 'done'))
const tabs = computed(() => [
  { id: 'journal', label: '冒险日志' }, { id: 'characters', label: '角色卡' }, { id: 'checks', label: '行动检定' }, { id: 'proposals', label: '设定提案' },
  ...(isOwner.value ? [{ id: 'host', label: '主持台' }, { id: 'agents', label: '智能体接入' }] : []),
])
const showCreate = ref(false), showAI = ref(false), showDice = ref(false), editCharacter = ref<Character | null>(null)
const createForm = ref({ title: '', ruleset: '5e-2024', members: [] as number[], scene: '', house_rules: '' })
const diceForm = ref({ formula: '1d20', reason: '', visibility: 'public' })
const hostForm = ref({ scene: '', mode: 'manual', auto_respond: true })
const hostVersion = ref(0), dialog = ref<HTMLElement>()
const note = ref({ content: '', version: 1 }), noteDraft = ref('')
const proposalForm = ref({ kind: 'scene', content: '', character_id: 0 })
const tokenForm = ref({ label: '', secrets: false }), issuedToken = ref<{ token: string; base_url: string; expires_at: number } | null>(null)
const aiForm = ref({ base_url: 'https://api.x.ai/v1', model: 'grok-4.7', api_key: '', enabled: true, remove_key: false })
let requestGeneration = 0, stopped = false, active = true, timer: ReturnType<typeof setInterval>, refreshTimer: ReturnType<typeof setTimeout> | undefined
const operationIDs = new Map<string, string>()
const drafts = new Map<number, { content: string; kind: string; recipient: number }>()
const uid = () => Array.from(crypto.getRandomValues(new Uint8Array(18)), n => n.toString(16).padStart(2, '0')).join('')
const endpoint = (suffix: string, id = current.value?.id) => `/v1/campaigns/${id}/${suffix}`
const message = (e: unknown) => e instanceof Error ? e.message : '暂时无法连接，请稍后重试'
const dateTime = (value: number) => new Date(value).toLocaleString('zh-CN', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
async function scrollBottom() { await nextTick(); if (journal.value) journal.value.scrollTop = journal.value.scrollHeight }
async function write<T = any>(suffix: string, body: Record<string, unknown>, method = 'POST'): Promise<T> {
  const path = endpoint(suffix), fingerprint = `${path}:${JSON.stringify(body)}`
  let id = operationIDs.get(fingerprint)
  if (!id) { id = uid(); operationIDs.set(fingerprint, id) }
  const result = await props.api<T>(path, method, { ...body, client_id: id })
  operationIDs.delete(fingerprint)
  return result
}
async function run(task: () => Promise<void>) {
  if (busy.value) return
  busy.value = true; error.value = ''
  try { await task() } catch (e) { error.value = message(e) }
  finally { busy.value = false }
}
async function refresh() {
  if (refreshing.value || stopped || !active || (current.value && !journalReady.value)) return
  refreshing.value = true
  const generation = requestGeneration, cid = current.value?.id
  let listed = false
  try {
    const [list, settings] = await Promise.all([props.api<Campaign[]>('/v1/campaigns'), props.api<AISettings>('/v1/entertainment/ai')])
    if (stopped || generation !== requestGeneration) return
    campaigns.value = list; ai.value = settings; listed = true
    if (error.value === listError.value) error.value = ''
    listError.value = ''
    if (cid) {
      const [state, chars, log, queue] = await Promise.all([
        props.api<Campaign>(endpoint('state', cid)), props.api<Character[]>(endpoint('characters', cid)),
        props.api<GameEvent[]>(endpoint('events', cid)), props.api<Turn[]>(endpoint('turns', cid)),
      ])
      if (stopped || generation !== requestGeneration || current.value?.id !== cid) return
      current.value = state; characters.value = chars; turns.value = queue
      const map = new Map(events.value.map(e => [e.id, e])); for (const e of log) map.set(e.id, e)
      events.value = [...map.values()].sort((a, b) => a.id - b.id)
    }
  } catch (e) {
    if (generation === requestGeneration) {
      error.value = message(e)
      if (!listed) listError.value = message(e)
    }
  }
  finally { refreshing.value = false; loading.value = false }
}
async function select(c: Campaign, afterCreate = false) {
  if (busy.value && !afterCreate) return
  if (current.value?.id === c.id && journalReady.value) { mobileOpen.value = true; return }
  if (current.value) drafts.set(current.value.id, { content: draft.value, kind: composeKind.value, recipient: recipient.value })
  const generation = ++requestGeneration
  const previousTab = savedView(`tab:${c.id}`)
  const position = getScrollPosition(scrollKey('journal', c.id))
  journalReady.value = false
  saveView('campaign', String(c.id))
  history.replaceState(null, '', `#campaign=${c.id}`)
  current.value = c; events.value = []; characters.value = []; turns.value = []; proposals.value = []; tokens.value = []; issuedToken.value = null
  const saved = drafts.get(c.id)
  members.value = []; mobileOpen.value = true; tab.value = 'journal'
  composeKind.value = saved?.kind || 'action'; recipient.value = saved?.recipient ?? (c.owner_id === props.me.id ? 0 : c.owner_id)
  draft.value = saved?.content || ''; error.value = ''; canOlder.value = true
  try {
    const [state, chars, log, queue, users] = await Promise.all([
      props.api<Campaign>(endpoint('state', c.id)), props.api<Character[]>(endpoint('characters', c.id)),
      props.api<GameEvent[]>(endpoint('events', c.id)), props.api<Turn[]>(endpoint('turns', c.id)),
      props.api<{ members: GameUser[] }>(`/conversations/${c.conversation_id}`),
    ])
    if (stopped || generation !== requestGeneration) return
    // Fetch older pages until the saved item is present. Each response is bounded
    // to 80 items; switching campaigns invalidates the entire restoration.
    let history = log, hasOlder = log.length === 80
    const anchor = Number(position?.anchor || 0)
    while (anchor > 0 && !position?.bottom && hasOlder && history.length && history[0]!.id > anchor) {
      const before = history[0]!.id
      const page = await props.api<GameEvent[]>(endpoint(`events?before=${before}`, c.id))
      if (stopped || generation !== requestGeneration) return
      if (!page.length || page[0]!.id >= before) { hasOlder = false; break }
      history = [...page, ...history]; hasOlder = page.length === 80
    }
    current.value = state; characters.value = chars; events.value = history; turns.value = queue
    members.value = users.members.filter(m => m.id !== c.bot_id); canOlder.value = hasOlder
    if (!validRecipient.value) recipient.value = privateRecipients.value[0]?.id || 0
    journalReady.value = true
    if (previousTab && tabs.value.some(t => t.id === previousTab)) await chooseTab(previousTab)
  } catch (e) { if (generation === requestGeneration) { error.value = message(e); journalReady.value = true } }
}
async function chooseTab(value: string) {
  if (busy.value) return
  tab.value = value; error.value = ''
  if (!current.value) return
  saveView(`tab:${current.value.id}`, value)
  const cid = current.value.id, generation = requestGeneration
  await run(async () => {
    if (value === 'host') {
      hostForm.value = { scene: current.value!.scene, mode: current.value!.mode, auto_respond: current.value!.auto_respond }
      hostVersion.value = current.value!.version
      const result = await props.api<typeof note.value>(endpoint('dm-notes', cid))
      if (generation !== requestGeneration) return
      note.value = result; noteDraft.value = note.value.content
    }
    if (value === 'proposals') {
      const result = await props.api<Proposal[]>(endpoint('story-proposals', cid))
      if (generation === requestGeneration) proposals.value = result
    }
    if (value === 'agents') {
      const result = await props.api<AgentToken[]>(endpoint('agent-tokens', cid))
      if (generation === requestGeneration) tokens.value = result
    }
  })
}
async function create() {
  await run(async () => {
    const c = await props.api<Campaign>('/v1/campaigns', 'POST', createForm.value)
    showCreate.value = false; createForm.value = { title: '', ruleset: '5e-2024', members: [], scene: '', house_rules: '' }
    campaigns.value.unshift(c); await select(c, true); emit('notify', '战役已创建。先准备角色，再由房主继续战役。')
  })
}
async function submit() {
  if (!current.value || !draft.value.trim() || composing.value || (composeKind.value === 'private' && !validRecipient.value)) return
  await run(async () => {
    if (composeKind.value === 'action') await write('actions', { content: draft.value.trim() })
    else await write('messages', { version: current.value!.version, content: draft.value.trim(), visibility: composeKind.value === 'private' ? 'private' : 'public', recipient_id: composeKind.value === 'private' ? recipient.value : 0, kind: composeKind.value === 'summary' ? 'summary' : 'narration' })
    draft.value = ''; drafts.delete(current.value!.id); await refresh(); await scrollBottom()
  })
}
async function togglePause() {
  if (!current.value) return
  await run(async () => {
    await write('state', { version: current.value!.version, phase: current.value!.phase === 'paused' ? 'awaiting_action' : 'paused' }, 'PATCH')
    await refresh()
  })
}
async function host(kind: string) {
  await run(async () => { await write('host', { kind }); await refresh() })
}
async function retry(turn: Turn, skip = false) {
  await run(async () => { await write(`turns/${turn.id}/retry`, { version: current.value!.version, skip }); await refresh() })
}
async function roll() {
  await run(async () => { await write('rolls', diceForm.value); showDice.value = false; diceForm.value.reason = ''; await refresh(); await scrollBottom() })
}
function newCharacter() {
  editCharacter.value = { id: 0, campaign_id: current.value!.id, user_id: props.me.id, name: '', role: '', level: 1, hp: 10, max_hp: 10, ac: 10, abilities: { str: 10, dex: 10, con: 10, int: 10, wis: 10, cha: 10 }, conditions: '', inventory: '', resources: '', background: '', private_notes: '', version: 0 }
}
async function saveCharacter() {
  await run(async () => { await write('characters', { version: current.value!.version, character: editCharacter.value }); editCharacter.value = null; await refresh() })
}
async function saveHost() {
  await run(async () => { await write('state', { version: hostVersion.value, ...hostForm.value }, 'PATCH'); await refresh(); hostVersion.value = current.value!.version; emit('notify', '主持设置已保存') })
}
async function saveNotes() {
  await run(async () => { note.value = await write('dm-notes', { content: noteDraft.value, note_version: note.value.version }, 'PUT'); emit('notify', '主持秘密已保存') })
}
async function propose() {
  await run(async () => { await write('story-proposals', proposalForm.value); proposalForm.value.content = ''; proposals.value = await props.api(endpoint('story-proposals')); emit('notify', '已提交提案，等待房主确认') })
}
async function decide(p: Proposal, accept: boolean) {
  await run(async () => { await write(`story-proposals/${p.id}/decide`, { version: current.value!.version, accept }); await refresh(); proposals.value = await props.api(endpoint('story-proposals')) })
}
async function issueToken() {
  await run(async () => { issuedToken.value = await props.api(endpoint('agent-tokens'), 'POST', tokenForm.value); tokens.value = await props.api(endpoint('agent-tokens')) })
}
async function revoke(t: AgentToken) {
  await run(async () => { await props.api(endpoint(`agent-tokens/${t.id}`), 'DELETE'); tokens.value = await props.api(endpoint('agent-tokens')); issuedToken.value = null })
}
function openAI() {
  aiForm.value = { base_url: ai.value.base_url || 'https://api.x.ai/v1', model: ai.value.model || 'grok-4.7', api_key: '', enabled: ai.value.enabled, remove_key: false }
  showAI.value = true; error.value = ''
}
async function saveAI() {
  await run(async () => { ai.value = await props.api('/v1/entertainment/ai', 'PUT', aiForm.value); aiForm.value.api_key = ''; emit('notify', '模型配置已保存，密钥只保存在服务端') })
}
async function testAI() {
  await run(async () => { await props.api('/v1/entertainment/ai/test', 'POST', {}, 65000); emit('notify', '模型连接及工具调用验证通过') })
}
async function older() {
  if (!events.value.length) return
  await run(async () => {
    const log = await props.api<GameEvent[]>(endpoint(`events?before=${events.value[0].id}`))
    events.value = [...log, ...events.value]; canOlder.value = log.length === 80
  })
}
function scheduleRefresh() { if (!active) return; clearTimeout(refreshTimer); refreshTimer = setTimeout(() => { void refresh() }, 180) }
const modalOpen = computed(() => showCreate.value || showAI.value || showDice.value || !!editCharacter.value)
let returnFocus: HTMLElement | null = null
watch(modalOpen, async open => {
  if (open) {
    returnFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
    await nextTick()
    dialog.value?.querySelector<HTMLElement>('input:not([type=checkbox]), textarea, select, button')?.focus()
  } else { aiForm.value.api_key = ''; returnFocus?.focus() }
})
function dialogKey(event: KeyboardEvent) {
  if (event.key === 'Escape' && !busy.value) {
    showCreate.value = false; showAI.value = false; showDice.value = false; editCharacter.value = null
    event.preventDefault(); return
  }
  if (event.key !== 'Tab') return
  const focusable = [...(dialog.value?.querySelectorAll<HTMLElement>('button:not(:disabled),input:not(:disabled),textarea:not(:disabled),select:not(:disabled),summary') || [])].filter(e => e.getClientRects().length)
  const first = focusable[0], last = focusable[focusable.length - 1]
  if (event.shiftKey && document.activeElement === first) { last?.focus(); event.preventDefault() }
  if (!event.shiftKey && document.activeElement === last) { first?.focus(); event.preventDefault() }
}
watch(() => props.revision, scheduleRefresh)
onActivated(() => { active = true; scheduleRefresh() })
onDeactivated(() => { active = false; clearTimeout(refreshTimer) })
function onVisible() { if (!document.hidden) scheduleRefresh() }
onMounted(async () => {
  await refresh()
  const linked = campaigns.value.find(c => location.hash === `#campaign=${c.id}`) ||
    campaigns.value.find(c => String(c.id) === savedView('campaign'))
  if (linked) await select(linked)
  else if (campaigns.value.length && !matchMedia('(max-width:760px)').matches) await select(campaigns.value[0])
  timer = setInterval(() => { if (!document.hidden) void refresh() }, 15000)
  document.addEventListener('visibilitychange', onVisible)
})
onBeforeUnmount(() => { stopped = true; requestGeneration++; clearInterval(timer); clearTimeout(refreshTimer); document.removeEventListener('visibilitychange', onVisible); issuedToken.value = null; aiForm.value.api_key = '' })
</script>

<template>
  <div class="game-layout" :class="{ 'game-open': mobileOpen && current }" :inert="modalOpen">
    <aside class="game-index">
      <header><div><h1>娱乐模式</h1><p>一起写下你们的冒险。</p></div><button class="icon-button outlined" aria-label="新建战役" @click="showCreate = true; error = ''"><Icon name="plus" /></button></header>
      <div class="game-campaigns" :data-scroll-key="`${me.id}/campaigns`" aria-label="我的战役">
        <p v-if="loading" class="game-empty-small">正在读取战役存档…</p>
        <div v-if="listError" class="game-index-error" role="alert"><b>战役暂时未能加载</b><p>{{ listError }}</p><button class="game-button" :disabled="refreshing" @click="refresh">{{ refreshing ? '正在重试…' : '重试加载战役' }}</button></div>
        <button v-for="c in campaigns" :key="c.id" class="game-campaign" :class="{ selected: current?.id === c.id }" :disabled="busy" @click="select(c)"><Icon name="dice" :size="24" /><span><b>{{ c.title }}</b><small>{{ phases[c.phase] }} · {{ rulesets[c.ruleset] }}</small></span><span v-if="c.owner_id === me.id" class="game-owner-label">房主</span></button>
        <p v-if="!loading && !listError && !campaigns.length" class="game-empty-small">还没有战役。创建一个空间，邀请伙伴入席。</p>
      </div>
      <footer><span class="game-model-state"><i class="status-dot" :class="{ offline: !ai.enabled || !ai.has_key }"></i>{{ ai.enabled && ai.has_key ? '内置 AI 已配置' : '可先使用手动主持' }}</span><button v-if="me.admin" class="text-button" @click="openAI"><Icon name="settings" :size="15" />模型设置</button></footer>
    </aside>
    <section v-if="current" class="game-room" :data-scroll-key="scrollKey('room')" :aria-label="current.title">
      <header class="game-room-header">
        <button class="icon-button game-back" aria-label="返回战役列表" @click="mobileOpen = false"><Icon name="arrow" /></button>
        <div class="game-room-title"><h2>{{ current.title }}</h2><p>{{ rulesets[current.ruleset] }}<span>·</span>{{ modes[current.mode] }}<span>·</span>存档 v{{ current.version }}</p></div>
        <button class="text-button game-chat-link" @click="emit('chat', current.conversation_id)"><Icon name="chat" :size="17" />群聊</button>
        <button v-if="isOwner" class="game-button" :disabled="busy" @click="togglePause">{{ current.phase === 'paused' ? '继续战役' : '暂停' }}</button>
      </header>
      <div class="game-state-strip" :class="{ paused: current.phase === 'paused' }"><span class="status-dot" :class="{ offline: current.phase === 'paused' }"></span><b>{{ phases[current.phase] }}</b><span>{{ pendingTurn ? `行动 #${pendingTurn.id} · ${pendingTurn.status === 'failed' ? '需要处理' : pendingTurn.status === 'running' ? '处理中' : '排队中'}` : '每一次行动，都留下记录' }}</span></div>
      <nav class="game-tabs" aria-label="战役内容"><button v-for="t in tabs" :key="t.id" :class="{ active: tab === t.id }" :disabled="busy" :aria-current="tab === t.id ? 'page' : undefined" @click="chooseTab(t.id)">{{ t.label }}</button></nav>
      <p v-if="error && !showCreate && !showAI && !showDice && !editCharacter" class="game-error" role="alert">{{ error }}<button class="text-button" @click="error = ''; refresh()">刷新</button></p>
      <template v-if="tab === 'journal'">
        <details class="game-scene" :open="!events.length"><summary><Icon name="book" :size="17" /><b>当前场景</b><span>{{ current.scene || '等待房主铺开第一幕' }}</span></summary><p>{{ current.scene || '房主可在主持台填写起始场景，再继续战役。' }}</p><div v-if="current.summary" class="game-summary"><h3>上次的故事</h3><p>{{ current.summary }}</p></div></details>
        <div v-if="pendingTurn?.status === 'failed' || (pendingTurn?.status === 'running' && Date.now() - pendingTurn.started_at > 180000)" class="game-turn-error" role="status"><div><b>主持暂未完成</b><p>{{ pendingTurn.error || '回合已超时，已掷骰结果会保留。' }}</p></div><template v-if="isOwner"><button class="game-button" :disabled="busy" @click="retry(pendingTurn)">重试</button><button class="text-button" :disabled="busy" @click="retry(pendingTurn, true)">跳过</button></template></div>
        <div ref="journal" class="game-journal" :data-scroll-key="scrollKey('journal')" :data-scroll-ready="journalReady" data-scroll-follow role="log" aria-label="战役日志">
          <button v-if="canOlder && events.length" class="history-button" :disabled="busy" @click="older">读取更早的记录</button>
          <div v-if="!events.length" class="game-journal-empty"><Icon name="book" :size="36" /><h3>第一幕，等你落笔</h3><p>准备角色卡，确认场景，然后由房主继续战役。<br />提交行动会写入存档；闲聊请使用右上角的群聊。</p><button v-if="!myCharacter" class="text-button" @click="newCharacter">创建我的角色<Icon name="arrow-right" :size="16" /></button></div>
          <article v-for="e in events" :key="e.id" class="game-entry" :class="[e.kind, { private: e.visibility !== 'public' }]" :data-event-id="e.id" :data-scroll-anchor="e.id">
            <div class="game-entry-meta"><b>{{ e.kind === 'narration' || e.kind === 'summary' ? '主持人' : e.actor }}</b><span>{{ ({ action: '行动', roll: '服务器掷骰', summary: '本次摘要', narration: '叙述', setting: '设定更新', request: '主持请求', whisper: '私密消息' } as Record<string,string>)[e.kind] || e.kind }}</span><span v-if="e.visibility !== 'public'" class="game-secret-label"><Icon name="lock" :size="12" />{{ e.visibility === 'dm' ? '主持秘密' : '仅双方与房主' }}</span><time>{{ dateTime(e.created_at) }}</time></div>
            <div v-if="e.payload.roll" class="game-dice-result"><Icon name="dice" :size="27" /><strong>{{ e.payload.roll.total }}</strong><div><b>{{ e.payload.roll.formula }}</b><p>{{ e.content.split('\n')[0].split(' · ')[0] }}</p><small>实际骰子 {{ e.payload.roll.dice.join('、') }} · 保留 {{ e.payload.roll.kept.join('、') }} · 修正 {{ e.payload.roll.modifier >= 0 ? '+' : '' }}{{ e.payload.roll.modifier }}</small></div></div>
            <p v-else class="game-entry-text">{{ e.content }}</p>
            <button v-if="e.kind === 'check'" class="text-button" @click="chooseTab('checks')">查看检定与代价选择</button>
          </article>
        </div>
        <footer class="game-composer">
          <div class="game-compose-controls"><label class="game-select-label">发送为<select v-model="composeKind" aria-label="战役消息类型"><option value="action">玩家行动</option><option value="private">私密消息</option><template v-if="isOwner"><option value="narration">主持叙述</option><option value="summary">结束并保存摘要</option></template></select></label><label v-if="composeKind === 'private'" class="game-select-label">给<select v-model.number="recipient" aria-label="私密消息接收者"><option :value="0" disabled>{{ privateRecipients.length ? '选择接收者' : '暂无其他成员' }}</option><option v-for="p in privateRecipients" :key="p.id" :value="p.id">{{ p.nickname }}</option></select></label><button class="text-button" :disabled="current.phase === 'paused' || busy" @click="showDice = true; error = ''"><Icon name="dice" :size="17" />掷骰</button></div>
          <textarea v-model="draft" class="game-action-input" rows="2" maxlength="8000" aria-label="行动或叙述内容" :placeholder="composeKind === 'action' ? '描述你的行动，例如：我举起火把，查看石门上的刻痕…' : composeKind === 'private' ? '只有接收者、发送者和房主能看到这条消息…' : '写下公开叙述或本次已经发生的事实…'" @compositionstart="composing = true" @compositionend="composing = false" @keydown.enter.ctrl.prevent="!composing && submit()"></textarea>
          <div class="game-compose-bottom"><span>{{ composeKind === 'action' ? current.auto_respond && current.mode !== 'manual' ? '提交后主持人将依次处理 · Ctrl + Enter 提交' : '行动写入日志，由房主继续主持' : composeKind === 'private' ? '私密消息不进入公开 AI 上下文' : '公开内容会同步到战役群聊' }}</span><button class="send-button" :disabled="busy || composing || !draft.trim() || (composeKind === 'action' && current.phase === 'paused') || (composeKind === 'private' && !validRecipient)" @click="submit">{{ composeKind === 'action' ? '提交行动' : composeKind === 'summary' ? '保存摘要' : '发送' }}<Icon name="send" :size="16" /></button></div>
        </footer>
      </template>
      <div v-else class="game-tab-content" :data-scroll-key="scrollKey(tab)" :data-scroll-ready="journalReady && !busy">
        <template v-if="tab === 'characters'">
          <div class="game-section-heading"><div><h3>角色与资源</h3><p>生命值、状态、物品和法术资源随战役保存。</p></div><button v-if="!myCharacter" class="game-button" @click="newCharacter">创建我的角色</button></div>
          <p v-if="!characters.length" class="game-empty-small">还没有角色卡。你和伙伴都可以为自己创建一个角色。</p>
          <div class="game-character-grid"><article v-for="ch in characters" :key="ch.id" class="game-character"><header><div><h3>{{ ch.name }}</h3><p>{{ ch.role }} · {{ ch.level }} 级<span v-if="ch.user_id === me.id"> · 我的角色</span></p></div><button v-if="isOwner || ch.user_id === me.id" class="text-button" @click="editCharacter = JSON.parse(JSON.stringify(ch))">编辑</button></header><div class="game-stats"><div><small>生命值</small><b>{{ ch.hp }}<span> / {{ ch.max_hp }}</span></b><meter :value="ch.hp" min="0" :max="ch.max_hp" :aria-label="`${ch.name} 生命值`"></meter></div><div><small>护甲等级</small><b>{{ ch.ac }}</b></div></div><dl class="game-abilities"><div v-for="(label, key) in abilityNames" :key="key"><dt>{{ label }}</dt><dd>{{ ch.abilities[key] }}</dd></div></dl><dl class="game-character-details"><div v-for="(label, key) in { conditions: '状态', inventory: '物品', resources: '法术与资源', background: '公开背景' }" :key="key"><dt>{{ label }}</dt><dd>{{ ch[key] || '尚未记录' }}</dd></div></dl></article></div>
        </template>
        <AbilityChecks v-else-if="tab === 'checks'" :key="current.id" :campaign="current" :characters="characters" :events="events" :members="members" :me="me" :api="api" :revision="revision" @changed="refresh" />
        <template v-else-if="tab === 'proposals'">
          <div class="game-section-heading"><div><h3>先提案，再成为设定</h3><p>修改场景、房规和角色背景，由房主采纳后写入正式存档。</p></div></div>
          <form class="game-form" @submit.prevent="propose"><div class="game-form-row"><label>提案类型<select v-model="proposalForm.kind"><option value="scene">下一幕 / 公开线索</option><option value="house_rules">房规与规则资料</option><option value="background">角色背景</option></select></label><label v-if="proposalForm.kind === 'background'">角色<select v-model.number="proposalForm.character_id" required><option :value="0" disabled>选择角色</option><option v-for="ch in characters" :key="ch.id" :value="ch.id">{{ ch.name }}</option></select></label></div><label>建议内容<textarea v-model="proposalForm.content" rows="4" required maxlength="12000" placeholder="写下完整的新设定，以及变更理由。草稿只对提交者和房主可见。"></textarea></label><button class="game-button" :disabled="busy">提交提案</button></form>
          <article v-for="p in proposals" :key="p.id" class="game-proposal"><header><b>{{ ({ scene: '场景提案', house_rules: '房规提案', background: '角色背景' } as Record<string,string>)[p.kind] }}</b><span>{{ ({ pending: '待房主确认', accepted: '已采纳', rejected: '未采纳' } as Record<string,string>)[p.status] }}</span></header><p>{{ p.content }}</p><footer v-if="isOwner && p.status === 'pending'"><button class="game-button" :disabled="busy" @click="decide(p, true)">采纳并公开</button><button class="text-button" :disabled="busy" @click="decide(p, false)">不采纳</button></footer></article>
        </template>
        <template v-else-if="tab === 'host' && isOwner">
          <div class="game-section-heading"><div><h3>主持台</h3><p>控制节奏，让每个玩家都有选择的空间。</p></div><div class="game-actions"><button class="game-button" :disabled="busy || current.phase === 'paused' || current.mode === 'manual'" @click="host('continue')">请主持人继续</button><button class="text-button" :disabled="busy || current.phase === 'paused' || current.mode === 'manual'" @click="host('summary')">结束本次并生成摘要</button></div></div>
          <p v-if="current.mode === 'builtin' && (!ai.enabled || !ai.has_key)" class="game-callout">内置 AI 尚未配置。{{ me.admin ? '请先打开“模型设置”保存 API 地址、模型与密钥。' : '请联系服务器管理员配置模型。' }}<button v-if="me.admin" class="text-button" @click="openAI">模型设置</button></p>
          <form class="game-form" @submit.prevent="saveHost"><div class="game-form-row"><label>主持方式<select v-model="hostForm.mode" aria-label="主持方式"><option v-for="(label, value) in modes" :key="value" :value="value">{{ label }}</option></select></label><label class="game-check"><input v-model="hostForm.auto_respond" type="checkbox" />提交行动或 @主持人后自动排队</label></div><label>当前公开场景<textarea v-model="hostForm.scene" rows="4" maxlength="8000" placeholder="玩家能看到的环境、人物、目标和已经公开的线索。"></textarea></label><p class="game-helper">规则版本固定为 {{ rulesets[current.ruleset] }}。房规与背景修改请通过设定提案保存。</p><button class="game-button" :disabled="busy || !!current.active_turn">保存主持设置</button></form>
          <section class="game-rules"><h3>本战役规则资料</h3><p>{{ current.house_rules || '尚无额外资料。请通过设定提案添加房规或有权使用的规则摘录；遇到规则不确定时由房主裁定。' }}</p></section>
          <form class="game-form game-private-form" @submit.prevent="saveNotes"><h3><Icon name="lock" :size="18" />主持秘密</h3><p class="game-helper">仅房主和获准读取秘密的外部主持智能体可访问。内置公开主持不会读取这里；要揭露的线索请作为场景提案采纳。</p><label>幕后笔记<textarea v-model="noteDraft" rows="6" maxlength="16000" placeholder="NPC 动机、尚未揭露的真相、陷阱等。"></textarea></label><button class="game-button" :disabled="busy">保存秘密</button></form>
        </template>
        <template v-else-if="tab === 'agents' && isOwner">
          <div class="game-section-heading"><div><h3>连接你的主持智能体</h3><p>令牌只授权当前战役，有效期 30 天，可随时撤销。</p></div></div>
          <form class="game-form" @submit.prevent="issueToken"><label>智能体名称<input v-model="tokenForm.label" maxlength="40" required placeholder="例如：Grok 跑团主持" /></label><label class="game-check"><input v-model="tokenForm.secrets" type="checkbox" />允许读取主持秘密、私密背景与私信</label><p class="game-helper">默认只可读取公开上下文、领取主持回合、掷骰、推进场景和提交设定提案。令牌不包含服务器或模型管理权限。</p><button class="game-button" :disabled="busy">创建战役令牌</button></form>
          <div v-if="issuedToken" class="game-token-result"><h3>令牌已创建，只在这里显示一次</h3><label>Bearer 令牌<input :value="issuedToken.token" readonly aria-label="新建智能体令牌" @focus="($event.target as HTMLInputElement).select()" /></label><label>战役 API 地址<input :value="issuedToken.base_url" readonly aria-label="战役 API 地址" @focus="($event.target as HTMLInputElement).select()" /></label><p>在外部智能体中保存后关闭此提示。请求带 <code>Authorization: Bearer …</code>；写入同时带 <code>X-LinChat: 1</code>。</p><button class="text-button" @click="issuedToken = null">已保存，关闭显示</button></div>
          <ol class="game-agent-steps"><li>将主持方式设为“外部智能体”，继续战役。</li><li>调用 <code>POST /turns/claim</code>，取得当前行动、公开上下文和回合租约。</li><li>需要检定时调用 <code>POST /rolls</code>。每次请求保留唯一的 <code>client_id</code>，重试时复用。</li><li>调用 <code>POST /turns/{id}/complete</code>，携带租约、版本和叙述结果，原子保存状态。</li><li>想改背景或规则时使用 <code>POST /story-proposals</code>，由房主采纳。</li></ol>
          <p class="game-helper">完整字段与示例见项目的 <code>docs/ENTERTAINMENT.md</code>，也提供可运行的外部主持示例。这里的接口可由连接器包装为 MCP；聊天中的当前智能体不会自动订阅这些事件。</p>
          <div class="game-token-list"><div v-for="t in tokens" :key="t.id"><span><b>{{ t.label }}</b><small>{{ t.revoked ? '已撤销' : `到期 ${dateTime(t.expires_at)}` }} · {{ t.secrets ? '含秘密权限' : '公开主持' }}</small></span><button v-if="!t.revoked" class="text-button danger" :disabled="busy" @click="revoke(t)">撤销</button></div></div>
        </template>
      </div>
    </section>
    <section v-else class="game-welcome"><Icon name="dice" :size="48" /><h2>把聊天变成一场冒险</h2><p>创建一个战役，邀请伙伴带上角色入席。<br />由你、Grok 或其他智能体主持，故事与每次掷骰都留在这里。</p><button class="primary" @click="showCreate = true">创建第一场战役<Icon name="plus" :size="18" /></button><p v-if="error" class="game-error" role="alert">{{ error }}</p></section>
  </div>

  <div v-if="modalOpen" class="modal-backdrop game-modal-backdrop" @keydown="dialogKey">
    <section ref="dialog" class="modal game-modal" :data-scroll-key="scrollKey(`modal/${showCreate ? 'create' : showAI ? 'ai' : showDice ? 'dice' : `character/${editCharacter?.id || 0}`}`)" role="dialog" aria-modal="true" :aria-label="showCreate ? '创建战役' : showAI ? '模型接入设置' : showDice ? '服务器掷骰' : '编辑角色卡'">
      <header class="modal-header"><h2>{{ showCreate ? '创建战役' : showAI ? '模型接入设置' : showDice ? '服务器掷骰' : '编辑角色卡' }}</h2><button class="icon-button" aria-label="关闭娱乐模式弹窗" :disabled="busy" @click="showCreate = false; showAI = false; showDice = false; editCharacter = null; aiForm.api_key = ''"><Icon name="close" /></button></header>
      <p v-if="error" class="game-error" role="alert">{{ error }}</p>
      <form v-if="showCreate" class="game-form" @submit.prevent="create">
        <label>战役名称<input v-model="createForm.title" required maxlength="40" placeholder="例如：雾港来信" autofocus /></label>
        <label>规则版本<select v-model="createForm.ruleset"><option v-for="(label, value) in rulesets" :key="value" :value="value">{{ label }}</option></select></label><p class="game-helper">创建后固定规则版本。准备完成前战役保持暂停。</p>
        <fieldset class="game-member-picker"><legend>邀请伙伴 · 已选 {{ createForm.members.length }} 人</legend><label v-for="p in people.filter(p => p.id !== me.id)" :key="p.id" class="game-check"><input v-model="createForm.members" type="checkbox" :value="p.id" />{{ p.nickname }}<small>@{{ p.username }}</small></label><p v-if="people.length < 2">可以先独自准备，之后在战役群聊里添加成员。</p></fieldset>
        <label>第一幕<textarea v-model="createForm.scene" rows="3" maxlength="8000" placeholder="夜色中的雾港，你们收到了一封没有署名的信…"></textarea></label>
        <details><summary>房规与规则资料（可选）</summary><label>授权资料<textarea v-model="createForm.house_rules" rows="3" maxlength="12000" placeholder="填写本次使用的房规和有权使用的规则摘录。"></textarea></label></details>
        <button class="primary wide" :disabled="busy">{{ busy ? '正在创建…' : '创建并入席' }}</button>
      </form>
      <form v-else-if="showAI" class="game-form" @submit.prevent="saveAI">
        <p class="game-helper">支持 Grok 及 OpenAI 兼容的 Chat Completions / function calling 模型。调用使用你填写的服务商账号额度。</p>
        <label>API 基础地址<input v-model="aiForm.base_url" type="url" required placeholder="https://api.x.ai/v1" /></label>
        <label>模型 ID<input v-model="aiForm.model" required maxlength="120" placeholder="服务商支持工具调用的模型 ID" /></label>
        <label>API 密钥<input v-model="aiForm.api_key" type="password" autocomplete="new-password" maxlength="4096" :placeholder="ai.has_key ? '已保存；留空保留当前密钥' : '粘贴服务商 API Key'" /></label>
        <label class="game-check"><input v-model="aiForm.enabled" type="checkbox" />启用内置 AI 主持</label><label v-if="ai.has_key" class="game-check"><input v-model="aiForm.remove_key" type="checkbox" />移除已保存的密钥</label>
        <p class="game-helper">密钥加密保存在当前服务端，不会下发给玩家。云端与本机服务分别配置。</p>
        <div class="game-actions"><button class="primary" :disabled="busy">保存配置</button><button type="button" class="game-button" :disabled="busy || !ai.has_key" @click="testAI">{{ busy ? '处理中…' : '测试已保存的连接' }}</button></div>
      </form>
      <form v-else-if="showDice" class="game-form" @submit.prevent="roll">
        <p class="game-helper">这里是通用骰子，不计算分级房规。属性／技能检定请使用“行动检定”，先锁定 DC 与后果。</p>
        <label>检定目的<input v-model="diceForm.reason" required maxlength="200" placeholder="例如：攻击或伤害掷骰" /></label><label>骰式<input v-model="diceForm.formula" required maxlength="30" spellcheck="false" placeholder="1d20+5" /></label><p class="game-helper">普通：1d20+5 · 伤害：2d6+3<br />优势：2d20kh1+4 · 劣势：2d20kl1+4</p>
        <label v-if="isOwner">可见范围<select v-model="diceForm.visibility"><option value="public">所有战役成员</option><option value="dm">仅主持秘密</option></select></label>
        <p class="game-helper">服务器产生随机点数并保存原始骰子与结果。重复提交同一请求不会重掷。</p><button class="primary wide" :disabled="busy">掷骰并记录<Icon name="dice" :size="18" /></button>
      </form>
      <form v-else-if="editCharacter" class="game-form" @submit.prevent="saveCharacter">
        <div class="game-form-row"><label>角色名称<input v-model="editCharacter.name" required maxlength="40" /></label><label>职业 / 身份<input v-model="editCharacter.role" required maxlength="60" placeholder="例如：游侠" /></label></div>
        <div class="game-form-row game-numbers"><label>等级<input v-model.number="editCharacter.level" type="number" min="1" max="20" required /></label><label>当前生命<input v-model.number="editCharacter.hp" type="number" min="0" :max="editCharacter.max_hp" required /></label><label>生命上限<input v-model.number="editCharacter.max_hp" type="number" min="1" max="9999" required /></label><label>护甲等级<input v-model.number="editCharacter.ac" type="number" min="0" max="99" required /></label></div>
        <div class="game-ability-inputs"><label v-for="(label, key) in abilityNames" :key="key">{{ label }}<input v-model.number="editCharacter.abilities[key]" type="number" min="1" max="30" required /></label></div>
        <label v-for="(label, key) in { conditions: '当前状态', inventory: '装备与物品', resources: '法术位与资源', background: '公开背景' }" :key="key">{{ label }}<textarea v-model="editCharacter[key]" rows="2" maxlength="6000" :readonly="key === 'background' && !!editCharacter.id && !isOwner"></textarea></label>
        <p v-if="editCharacter.id && !isOwner" class="game-helper">已有角色背景的变更，请在“设定提案”中提交，由房主确认。</p>
        <label>私密背景<textarea v-model="editCharacter.private_notes" rows="3" maxlength="6000" placeholder="只有你、房主及明确获准的主持智能体能读取。"></textarea></label>
        <button class="primary wide" :disabled="busy || !!current?.active_turn">保存角色卡</button>
      </form>
    </section>
  </div>
</template>
