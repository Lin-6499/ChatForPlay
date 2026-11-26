<script setup lang="ts">
import { computed, nextTick, onActivated, onBeforeUnmount, onDeactivated, onMounted, ref, watch } from 'vue'
import type { GameAPI, GameUser } from './game-types'
import { phaseNames, tokenNames, type EstateRoom, type EstateView } from './estate-types'
import EstateBoard from './EstateBoard.vue'
import EstateToken from './EstateToken.vue'
import DiceDie from './DiceDie.vue'
import Icon from './Icon.vue'
import './dice.css'
import './estate.css'
import { getScrollPosition } from '../../desktop/scroll-memory.mjs'

const props = defineProps<{ me: GameUser; api: GameAPI; revision: number; connection: string }>()
const emit = defineEmits<{ campaign: []; notify: [text: string] }>()
const view = ref<EstateView | null>(null), rooms = ref<EstateRoom[]>([]), roomID = ref(0)
const loading = ref(true), busy = ref(false), error = ref(''), title = ref('环城好友局'), capacity = ref(4)
const selected = ref(0), showRules = ref(false), panel = ref('chat'), drawer = ref(false)
const inspectedPlayer = ref(props.me.id)
const inspectedName = computed(() => game.value?.players.find(p => p.id === inspectedPlayer.value)?.name || props.me.nickname)
const now = ref(Date.now()), clockOffset = ref(0), diceProgress = ref(1), unread = ref(0)
const messages = ref<{ id: number; sender_name: string; content: string; kind: string; created_at: number }[]>([])
const draft = ref(''), chatError = ref(''), chatBusy = ref(false), chatMore = ref(false), chatLoading = ref(false)
const chatLog = ref<HTMLElement>()
const chatReady = ref(false)
type Pending = { route: string; body: Record<string, unknown> }
const pending = ref<Pending | null>(null)
let active = true, generation = 0, polling = false, chatPolling = false
let timer: ReturnType<typeof setInterval> | undefined, animation = 0
const key = (part: string) => `linchat-estate:${props.me.id}:${part}`
function storageGet(part: string) { try { return localStorage.getItem(key(part)) || '' } catch { return '' } }
function storageSet(part: string, value: string) { try { if (value) localStorage.setItem(key(part), value); else localStorage.removeItem(key(part)) } catch {} }
function uid() { return Array.from(crypto.getRandomValues(new Uint8Array(18)), n => n.toString(16).padStart(2, '0')).join('') }
function explain(e: unknown) { return e instanceof Error ? e.message : '连接失败，请重试' }
function status(e: unknown) { return typeof e === 'object' && e !== null && 'status' in e ? Number(e.status) : 0 }
const game = computed(() => view.value?.game)
const mine = computed(() => game.value?.players.find(p => p.id === props.me.id))
const turn = computed(() => game.value?.players[game.value.current])
const myTurn = computed(() => turn.value?.id === props.me.id && !mine.value?.bankrupt)
const seconds = computed(() => Math.max(0, Math.ceil(((game.value?.deadline || 0) - now.value - clockOffset.value) / 1000)))
const playable = computed(() => !busy.value && !pending.value && props.connection === '实时在线' && (!game.value?.deadline || seconds.value > 0))
const chosen = computed(() => view.value?.tiles[selected.value])
const property = computed(() => game.value?.properties[selected.value])
const owner = computed(() => game.value?.players.find(p => p.id === property.value?.owner))
const quote = computed(() => view.value?.quotes[selected.value])
function rentImpact(tile: number, action: 'sell' | 'mortgage') {
  const changes = action === 'sell' ? view.value?.quotes[tile]?.sell_rents : view.value?.quotes[tile]?.mortgage_rents
  return changes?.length ? changes.map(c => `${view.value?.tiles[c.tile]?.name}租金 ${c.before} → ${c.after}`).join('；') : '租金无变化'
}
const landing = computed(() => view.value?.tiles[turn.value?.position || 0])
const owned = computed(() => view.value?.tiles.filter(t => game.value?.properties[t.id].owner === props.me.id) || [])
const inspectedProperties = computed(() => view.value?.tiles.filter(t => game.value?.properties[t.id].owner === inspectedPlayer.value) || [])
const canManage = computed(() => myTurn.value && ['management', 'debt'].includes(game.value?.phase || '') && property.value?.owner === props.me.id)
const canStart = computed(() => game.value && game.value.players.length >= 2 && game.value.players.every(p => p.ready))
const deadlineText = computed(() => ({
  management: '到期自动掷骰', purchase_decision: '到期自动放弃', market_decision: '到期自动离开',
  check_decision: '到期停业一回合', debt: '到期按规则自动清算', end_turn: '即将进入下一回合',
}[game.value?.phase || ''] || ''))
function keepPending(p: Pending | null) { pending.value = p; storageSet('operation', p ? JSON.stringify(p) : '') }
function apply(data: EstateView) {
  if (view.value?.game.id === data.game.id && view.value.game.version > data.game.version) return
  const previous = view.value?.game
  now.value = Date.now()
  clockOffset.value = data.server_time - now.value
  view.value = data; roomID.value = data.game.id
  storageSet('room', String(roomID.value))
  history.replaceState(null, '', `#estate=${roomID.value}`)
  if (previous?.id !== data.game.id) selected.value = data.game.players[data.game.current]?.position || 0
  const rolls = data.game.events.filter(e => e.text.includes('掷出'))
  if (previous && rolls.length > previous.events.filter(e => e.text.includes('掷出')).length) {
    selected.value = data.game.players[data.game.current]?.position || 0
    animateDice()
  }
}
function animateDice() {
  cancelAnimationFrame(animation)
  if (matchMedia('(prefers-reduced-motion: reduce)').matches) { diceProgress.value = 1; return }
  const start = performance.now()
  const step = (t: number) => { diceProgress.value = Math.min(1, (t - start) / 800); if (diceProgress.value < 1) animation = requestAnimationFrame(step) }
  animation = requestAnimationFrame(step)
}
async function load() {
  if (polling || !active) return
  polling = true
  const gen = generation, id = roomID.value
  try {
    if (id) {
      const data = await props.api<EstateView>(`/v1/estate/matches/${id}`)
      if (gen === generation && id === roomID.value) { apply(data); await loadChat() }
    } else {
      const data = await props.api<EstateRoom[]>('/v1/estate/rooms')
      if (gen === generation) rooms.value = data
    }
    if (gen === generation && !pending.value) error.value = ''
  } catch (e) { if (gen === generation) error.value = explain(e) }
  finally { polling = false; if (gen === generation) loading.value = false }
}
async function write(route: string, fields: Record<string, unknown>, version = game.value?.version || 0) {
  if (busy.value || pending.value) return
  keepPending({ route, body: { ...fields, version, client_id: uid() } })
  await retry()
}
async function retry() {
  const op = pending.value
  if (!op || busy.value) return
  busy.value = true; error.value = ''
  try {
    const data = await props.api<EstateView>(op.route, 'POST', op.body)
    keepPending(null)
    if (op.route.endsWith('/leave')) { back(); return }
    const changed = roomID.value !== data.game.id
    if (changed) { generation++; chatReady.value = false; messages.value = []; draft.value = storageGet(`draft:${data.game.id}`); unread.value = 0 }
    apply(data)
    // A repeated receipt may predate later turns. Read the latest snapshot.
    await load(); await loadChat()
  } catch (e) {
    const code = status(e)
    if (code >= 400 && code < 500) { keepPending(null); await load() }
    error.value = explain(e) + (pending.value ? '；操作结果待确认，请用原请求重试。' : '')
  } finally { busy.value = false }
}
function act(action: string, tile = selected.value) { return write(`/v1/estate/matches/${roomID.value}/actions`, { action, tile }) }
function roomAction(action: string, fields: Record<string, unknown> = {}) { return write(`/v1/estate/rooms/${roomID.value}/${action}`, fields) }
function create() { return write('/v1/estate/rooms', { title: title.value, capacity: capacity.value }, 0) }
async function enter(room: EstateRoom) {
  if (pending.value) return
  if (!room.joined) { await write(`/v1/estate/rooms/${room.id}/join`, {}, room.version); return }
  generation++; roomID.value = room.id; view.value = null; loading.value = true; chatReady.value = false; messages.value = []; unread.value = 0
  draft.value = storageGet(`draft:${room.id}`); await load(); await loadChat()
}
function back() {
  generation++; roomID.value = 0; view.value = null; chatReady.value = false; messages.value = []; unread.value = 0
  history.replaceState(null, '', '#estate'); storageSet('room', '')
  error.value = ''; loading.value = true; void load()
}
function campaign() { history.replaceState(null, '', '#entertainment'); emit('campaign') }
async function loadChat(older = false) {
  const g = game.value
  if (!g || !active || (older ? chatLoading.value : chatPolling)) return
  const id = g.id, gen = generation, initial = !messages.value.length
  if (older) chatLoading.value = true; else chatPolling = true
  const before = older ? messages.value[0]?.id : 0
  const after = !older && !initial ? messages.value.at(-1)?.id : 0
  const el = chatLog.value, pinned = !el || el.scrollHeight - el.scrollTop - el.clientHeight < 40
  const height = el?.scrollHeight || 0, top = el?.scrollTop || 0
  try {
    const data = await props.api<{ messages: typeof messages.value; more: boolean }>(`/conversations/${g.conversation_id}/messages${before ? `?before=${before}` : after ? `?after=${after}` : ''}`)
    if (gen !== generation || id !== roomID.value) return
    const existing = new Set(messages.value.map(m => m.id))
    const fresh = data.messages.filter(m => !existing.has(m.id))
    messages.value = [...messages.value, ...fresh].sort((a, b) => a.id - b.id)
    if (older || initial) chatMore.value = data.more
    await nextTick()
    if (chatLog.value) {
      if (older) chatLog.value.scrollTop = top + chatLog.value.scrollHeight - height
      else if ((initial && !getScrollPosition(`${props.me.id}/estate/${id}/chat`)) || (!initial && pinned && panel.value === 'chat')) chatLog.value.scrollTop = chatLog.value.scrollHeight
      else unread.value += fresh.length
    }
    chatReady.value = true
    chatError.value = ''
  } catch (e) { if (gen === generation) chatError.value = explain(e) }
  finally { if (older) chatLoading.value = false; else chatPolling = false }
}
async function sendChat() {
  const g = game.value
  if (!g || chatBusy.value || !draft.value.trim()) return
  chatBusy.value = true; chatError.value = ''
  const id = g.id, content = draft.value.trim()
  let receipt: { content: string; client_id: string } | null = null
  try { receipt = JSON.parse(storageGet(`chat-send:${id}`) || 'null') } catch {}
  if (!receipt || receipt.content !== content) receipt = { content, client_id: uid() }
  storageSet(`chat-send:${id}`, JSON.stringify(receipt))
  try {
    await props.api(`/conversations/${g.conversation_id}/messages`, 'POST', receipt)
    storageSet(`chat-send:${id}`, '')
    if (id === roomID.value && draft.value.trim() === content) draft.value = ''
    await loadChat()
  } catch (e) { if (id === roomID.value) chatError.value = explain(e) }
  finally { chatBusy.value = false }
}
function latest() { if (chatLog.value) chatLog.value.scrollTop = chatLog.value.scrollHeight; unread.value = 0 }
function changed() { if (active) { void load(); void loadChat() } }
watch(draft, v => { if (roomID.value) storageSet(`draft:${roomID.value}`, v) })
watch(() => props.revision, changed)
watch(panel, async v => { if (v === 'chat') { await nextTick(); if (!unread.value) latest() } })
onMounted(() => {
  const fromHash = Number(location.hash.match(/^#estate=(\d+)$/)?.[1])
  roomID.value = fromHash || Number(storageGet('room')) || 0
  draft.value = storageGet(`draft:${roomID.value}`)
  try { const saved = JSON.parse(storageGet('operation') || 'null'); if (saved?.route?.startsWith('/v1/estate/') && saved.body?.client_id) pending.value = saved } catch {}
  void load(); void loadChat()
  timer = setInterval(() => { now.value = Date.now(); if (active && !document.hidden && Math.floor(now.value / 1000) % 3 === 0) changed() }, 1000)
  window.addEventListener('linchat:estate-changed', changed)
  window.addEventListener('linchat:reconnected', changed)
})
onActivated(() => { active = true; changed() })
onDeactivated(() => { active = false; cancelAnimationFrame(animation); diceProgress.value = 1 })
onBeforeUnmount(() => { active = false; generation++; clearInterval(timer); cancelAnimationFrame(animation); window.removeEventListener('linchat:estate-changed', changed); window.removeEventListener('linchat:reconnected', changed) })
</script>

<template>
  <section class="estate-shell" aria-label="地产大亨">
    <header class="estate-header">
      <div><h1>地产大亨<span v-if="game"> / {{ game.title }}</span></h1><p>环城商业街 · {{ game && game.phase !== 'lobby' ? `第 ${game.round} / 12 轮` : '2–4人 · 12轮快速局' }}</p></div>
      <div class="estate-header-actions">
        <small :class="{ 'estate-warning': connection !== '实时在线' }">{{ connection }}</small>
        <button v-if="roomID" class="text-button" :disabled="busy" @click="back">房间列表</button>
        <button class="text-button" @click="showRules = !showRules">{{ showRules ? '收起规则' : '游戏规则' }}</button>
        <button class="text-button" @click="campaign">剧情战役</button>
      </div>
    </header>
    <div v-if="!busy && (error || pending)" class="estate-error" role="alert">
      {{ error || '有一笔操作结果尚未确认，请重试原请求。' }}
      <button class="text-button" :disabled="busy" @click="pending ? retry() : load()">重试</button>
    </div>
    <div v-if="connection !== '实时在线' && game" class="estate-offline">正在恢复实时连接，回合计时仍在继续。操作将在连接恢复后可用。</div>
    <section v-if="showRules" class="estate-rules" aria-label="游戏规则">
      <h2>买下街区，让每次经过都有回报。</h2>
      <p>初始现金2000，经过起点收入300。单块地就能升级，每回合掷骰前最多升级一次；完整且未抵押的同组租金增加25%。12轮结束比较净资产。</p>
      <p>管理/掷骰45秒，购买20秒，偿债60秒。超时分别自动掷骰、放弃购买、拆建及抵押。双骰相同不额外行动。检查可支付100继续，否则停业一回合；免租券遇到租金自动使用。</p>
      <p>拆建筑返还升级费50%；1级地产抵押获地价50%，赎回需本金加10%。无法偿债时剩余现金交给债权人，地产回银行。断线不暂停，重新进入可恢复。</p>
      <p>净资产＝现金＋未抵押土地原价＋建筑拆建价值＋抵押土地扣除赎回费的价值。金额为本局游戏货币。</p>
    </section>
    <p v-if="loading && !view" class="estate-loading" role="status">正在读取房间…</p>
    <div v-else-if="!roomID" class="estate-lobby">
      <section class="estate-room-list">
        <h2>一起开一局</h2><p class="muted">加入朋友的准备房，或继续你参与的对局。</p>
        <p v-if="!rooms.length && !error" class="estate-empty">还没有房间。创建一局，邀请朋友到这里加入。</p>
        <button v-for="room in rooms" :key="room.id" class="estate-room-row" :disabled="busy || !!pending || (!room.joined && room.count >= room.capacity)" @click="enter(room)">
          <span><strong>{{ room.title }}</strong><small>{{ room.count }}/{{ room.capacity }} 人 · {{ phaseNames[room.phase] }}</small></span>
          <span>{{ room.joined ? '进入' : room.count >= room.capacity ? '已满' : '加入' }} <Icon name="chevron-right" :size="16" /></span>
        </button>
        <button class="text-button" @click="load">刷新房间</button>
      </section>
      <form class="estate-create" @submit.prevent="create">
        <h2>创建房间</h2>
        <label>房间名称<input v-model="title" maxlength="40" required /></label>
        <label>人数上限<select v-model.number="capacity"><option :value="2">2人</option><option :value="3">3人</option><option :value="4">4人</option></select></label>
        <p class="muted">环城商业街 · 24格<br>固定12轮 · 所有人准备后开始</p>
        <button class="primary" :disabled="busy || !!pending || !title.trim()">{{ busy ? '正在创建…' : '创建房间' }}</button>
      </form>
    </div>
    <template v-else-if="game && view">
      <div v-if="game.phase === 'lobby'" class="estate-waiting">
        <section>
          <h2>等大家准备好，就出发</h2>
          <p class="muted">请朋友在“地产大亨”房间列表加入「{{ game.title }}」。</p>
          <ol class="estate-seats">
            <li v-for="p in game.players" :key="p.id"><EstateToken :token="p.token" /><strong>{{ p.name }}</strong><span>{{ p.id === game.owner_id ? '房主 · ' : '' }}{{ p.ready ? '已准备' : '未准备' }}</span></li>
            <li v-for="i in game.capacity - game.players.length" :key="`empty-${i}`" class="muted">等待玩家加入</li>
          </ol>
          <fieldset class="estate-token-picker"><legend>我的棋子</legend>
            <button v-for="(name, i) in tokenNames" :key="name" :disabled="!playable || game.players.some(p => p.id !== me.id && p.token === i)" :aria-pressed="mine?.token === i" @click="roomAction('token', { token: i })"><EstateToken :token="i" />{{ name }}</button>
          </fieldset>
          <div class="estate-row">
            <button class="primary" :disabled="!playable" :aria-busy="busy" @click="roomAction('ready', { ready: !mine?.ready })">{{ busy ? '正在提交…' : mine?.ready ? '取消准备' : '准备' }}</button>
            <button v-if="game.owner_id === me.id" class="secondary" :disabled="!playable || !canStart" @click="roomAction('start')">开始对局</button>
            <button class="text-button" :disabled="!playable" @click="roomAction('leave')">离开准备房</button>
          </div>
          <p class="muted">起始2000现金；顺序由服务器随机确定。退出已开始的对局后，将按超时规则继续。</p>
        </section>
      </div>
      <div v-else class="estate-game-layout">
        <aside class="estate-players" aria-label="玩家与资产">
          <h2>行动顺序</h2>
          <ol><li v-for="(p, i) in game.players" :key="p.id" :class="{ current: i === game.current && game.phase !== 'finished', bankrupt: p.bankrupt }">
            <div><EstateToken :token="p.token" /><button class="estate-player-inspect" :aria-pressed="inspectedPlayer === p.id" :aria-label="`查看${p.name}的地产`" @click="inspectedPlayer = p.id"><strong>{{ p.name }}</strong></button></div>
            <span v-if="i === game.current && game.phase !== 'finished'" class="estate-turn-label">行动中</span>
            <b>{{ p.cash.toLocaleString() }}<small>现金</small></b>
            <span>{{ p.bankrupt ? '已破产' : `${game.properties.filter(pr => pr.owner === p.id).length} 处地产` }}{{ p.coupon ? ' · 免租券' : '' }}{{ p.checked ? ' · 待检查' : '' }}</span>
            <span class="estate-player-position">{{ p.bankrupt ? '已退出棋盘' : `位于 ${p.position}号 ${view.tiles[p.position]?.name}` }}</span>
          </li></ol>
          <details open class="estate-owned"><summary>{{ inspectedPlayer === me.id ? '我的地产' : inspectedName + '的地产' }} · {{ inspectedProperties.length }}</summary>
            <p v-if="!inspectedProperties.length" class="muted">尚未持有地产。</p>
            <button v-for="t in inspectedProperties" :key="t.id" :aria-pressed="selected === t.id" @click="selected = t.id">{{ t.name }} <small>{{ game.properties[t.id].mortgaged ? '抵押' : `${game.properties[t.id].level}级` }}</small></button>
          </details>
        </aside>
        <main class="estate-map-area">
          <p class="estate-map-help">点选查看 · 虚线框为当前行动落点 · 小窗口可横向滚动棋盘</p>
          <div class="estate-board-scroll" tabindex="0" aria-label="可横向滚动的完整棋盘">
          <EstateBoard :view="view" :selected="selected" @select="selected = $event">
            <div class="estate-dice" :aria-label="`骰子结果：${game.dice.join('和')}`">
              <DiceDie v-for="(value, i) in game.dice" :key="i" :sides="6" :progress="diceProgress" :index="i" :value="value || undefined" />
            </div>
            <div v-if="chosen" class="estate-detail">
              <h2>{{ chosen.name }}</h2>
              <span class="estate-viewing-label">正在查看</span>
              <small>{{ chosen.group ? `${chosen.group}组地产` : '公共街区' }} · {{ selected }}号</small>
              <template v-if="chosen.price">
                <p>{{ owner ? `${owner.token + 1}号 ${owner.name}拥有 · ${property?.mortgaged ? '已抵押' : property?.level + '级'}` : '银行持有 · 可在落点购买' }}</p>
                <dl><div><dt>地价</dt><dd>{{ chosen.price }}</dd></div><div><dt>{{ owner ? '当前租金' : '初始租金' }}</dt><dd>{{ owner ? view.rents[selected] : chosen.rents[0] || 50 }}</dd></div></dl>
                <p v-if="chosen.upgrade" class="muted">升级费 {{ chosen.upgrade }} / 级<br>租金 {{ chosen.rents.join(' / ') }}，集齐组别另加25%</p>
                <div v-if="canManage" class="estate-manage">
                  <button v-if="game.phase === 'management' && chosen.upgrade && !property?.mortgaged && (property?.level || 0) < 3" class="secondary" :disabled="!playable || game.upgraded || (mine?.cash || 0) < chosen.upgrade" @click="act('upgrade')">升级 · {{ chosen.upgrade }}{{ game.upgraded ? '（本轮已用）' : '' }}</button>
                  <template v-if="(property?.level || 0) > 1"><button class="secondary" :disabled="!playable" @click="act('sell')">拆一级 · 回收{{ quote?.sell_income }}</button><p class="estate-rent-impact">{{ rentImpact(selected, 'sell') }}</p></template>
                  <template v-if="property?.level === 1 && !property.mortgaged"><button class="secondary" :disabled="!playable" @click="act('mortgage')">抵押 · 获得{{ quote?.mortgage_income }}</button><p class="estate-rent-impact">{{ rentImpact(selected, 'mortgage') }}</p></template>
                  <button v-if="property?.mortgaged && game.phase === 'management'" class="secondary" :disabled="!playable || (mine?.cash || 0) < (quote?.redeem_cost || 0)" @click="act('redeem')">赎回 · {{ quote?.redeem_cost }}</button>
                </div>
              </template>
              <p v-else class="muted">{{ chosen.kind === 'chance' ? '落点抽取一张机会牌，效果由服务器结算。' : chosen.kind === 'start' ? '正常经过或抵达获得300。' : chosen.kind === 'market' ? '花100购买一张自动使用的免租券。' : chosen.kind === 'tax' ? `落点支付${selected === 4 ? 100 : 150}。` : chosen.kind === 'go_check' ? '前往检查中心，不领取起点奖励。' : chosen.kind === 'check' ? '普通路过只参观；被送入时，下回合缴费或停业。' : '在这里休息片刻，不收取费用。' }}</p>
            </div>
          </EstateBoard>
          </div>
          <p class="estate-recent" aria-live="polite">{{ game.events.at(-1)?.text }}</p>
          <details class="estate-map-list"><summary>地图列表 · 24格</summary><button v-for="t in view.tiles" :key="t.id" @click="selected = t.id">{{ t.id }} · {{ t.name }}{{ t.price ? ` · ${t.price}` : '' }}</button></details>
        </main>
        <aside class="estate-right">
          <section class="estate-action" aria-label="当前回合操作">
            <template v-if="game.phase === 'finished'">
              <h2>本局结束</h2><p>查看下方净资产结算。</p>
              <button class="primary" :disabled="busy || !!pending" @click="title = game.title; capacity = game.capacity; create()">再来一局</button>
            </template>
            <template v-else>
              <div class="estate-action-heading"><h2>{{ myTurn ? '轮到你了' : `等待${turn?.name}` }}</h2><span v-if="game.deadline">{{ seconds }}s</span></div>
              <p>{{ phaseNames[game.phase] }}</p>
              <template v-if="myTurn">
                <button v-if="game.phase === 'management'" class="primary estate-roll" :disabled="!playable" @click="act('roll')"><Icon name="dice" :size="20" />掷骰</button>
                <template v-if="game.phase === 'purchase_decision'">
                  <h3>{{ landing?.name }}</h3><p>余额 {{ mine?.cash }} → {{ (mine?.cash || 0) - (landing?.price || 0) }}</p>
                  <button class="primary" :disabled="!playable || (mine?.cash || 0) < (landing?.price || 0)" @click="act('buy')">购买 · {{ landing?.price }}</button>
                  <p v-if="(mine?.cash || 0) < (landing?.price || 0)" class="muted">现金不足，不能购买。</p>
                  <button class="secondary" :disabled="!playable" @click="act('decline')">放弃</button>
                </template>
                <template v-if="game.phase === 'market_decision'"><p>免租券将在下一次支付租金时自动使用。</p><button class="primary" :disabled="!playable || (mine?.cash || 0) < 100" @click="act('market_buy')">购买免租券 · 100</button><button class="secondary" :disabled="!playable" @click="act('market_leave')">离开市集</button></template>
                <template v-if="game.phase === 'check_decision'"><button class="primary" :disabled="!playable || (mine?.cash || 0) < 100" @click="act('check_release')">支付100，继续行动</button><button class="secondary" :disabled="!playable" @click="act('check_wait')">停业一回合</button></template>
                <template v-if="game.phase === 'debt'">
                  <p class="estate-warning">待支付 {{ game.debt }} · 现金 {{ mine?.cash }} · 还差 {{ game.debt - (mine?.cash || 0) }}</p>
                  <p>选择自己的地产，拆建或抵押。凑够后自动付款。</p>
                  <div v-for="t in owned" :key="t.id" class="estate-debt-asset">
                    <button class="secondary" @click="selected = t.id">{{ t.name }} · {{ game.properties[t.id].level > 1 ? `拆建回收${view.quotes[t.id].sell_income}` : game.properties[t.id].mortgaged ? '已抵押' : `可抵押${view.quotes[t.id].mortgage_income}` }}</button>
                    <p v-if="!game.properties[t.id].mortgaged" class="estate-rent-impact">{{ rentImpact(t.id, game.properties[t.id].level > 1 ? 'sell' : 'mortgage') }}</p>
                  </div>
                </template>
              </template>
              <small>{{ deadlineText }}</small>
            </template>
          </section>
          <button class="secondary estate-drawer-toggle" :aria-expanded="drawer" @click="drawer = !drawer">{{ drawer ? '收起聊天与记录' : '聊天与对局记录' }}{{ unread ? ` · ${unread}条新消息` : '' }}</button>
          <section class="estate-social" :class="{ 'drawer-open': drawer }">
            <div class="estate-social-tabs"><button :aria-pressed="panel === 'chat'" @click="panel = 'chat'">聊天{{ unread ? ` (${unread})` : '' }}</button><button :aria-pressed="panel === 'events'" @click="panel = 'events'">对局记录</button></div>
            <div v-if="panel === 'events'" class="estate-events" :data-scroll-key="`${me.id}/estate/${game.id}/events`" data-scroll-follow>
              <p v-for="event in game.events" :key="event.seq" :data-scroll-anchor="String(event.seq)"><time>{{ new Date(event.at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) }}</time>{{ event.text }}</p>
            </div>
            <template v-else>
              <div ref="chatLog" class="estate-chat-log" :data-scroll-key="`${me.id}/estate/${game.id}/chat`" :data-scroll-ready="chatReady ? 'true' : 'false'" data-scroll-follow @scroll="chatLog && chatLog.scrollHeight - chatLog.scrollTop - chatLog.clientHeight < 40 && (unread = 0)">
                <button v-if="chatMore" class="text-button" :disabled="chatLoading" @click="loadChat(true)">加载更早消息</button>
                <p v-if="!messages.length" class="muted">和朋友聊聊你的下一步打算。</p>
                <article v-for="m in messages" :key="m.id" :data-scroll-anchor="String(m.id)"><strong>{{ m.sender_name }}</strong><p>{{ m.kind === 'text' ? m.content : '[附件，请在群聊查看]' }}</p></article>
              </div>
              <button v-if="unread" class="text-button" @click="latest">{{ unread }}条新消息 · 回到底部</button>
              <form class="estate-chat-compose" @submit.prevent="sendChat"><label class="sr-only" for="estate-chat-input">房间消息</label><textarea id="estate-chat-input" v-model="draft" rows="2" maxlength="4000" placeholder="房间消息…" /><button class="primary" :disabled="chatBusy || !draft.trim()">{{ chatBusy ? '发送中' : '发送' }}</button></form>
              <p v-if="chatError" class="estate-error" role="alert">{{ chatError }}<button class="text-button" @click="loadChat()">重读消息</button></p>
            </template>
          </section>
        </aside>
      </div>
      <section v-if="game.phase === 'finished'" class="estate-results" aria-label="结算明细">
        <h2>净资产结算</h2><p class="muted">同分依次比较现金、未抵押地产数；仍相同则并列。</p>
        <div class="estate-table-wrap"><table><thead><tr><th>名次 / 玩家</th><th>现金</th><th>土地</th><th>建筑清算</th><th>抵押剩余</th><th>净资产</th><th>收租 / 最大付款</th></tr></thead><tbody>
          <tr v-for="s in view.scores" :key="s.id"><th>{{ s.rank }} · {{ s.name }}{{ s.bankrupt ? '（破产）' : '' }}</th><td>{{ s.cash }}</td><td>{{ s.land }}</td><td>{{ s.buildings }}</td><td>{{ s.equity }}</td><td><strong>{{ s.total }}</strong></td><td>{{ game.players.find(p => p.id === s.id)?.rent_earned }} / {{ game.players.find(p => p.id === s.id)?.max_payment }}</td></tr>
        </tbody></table></div>
      </section>
    </template>
  </section>
</template>
