<script setup lang="ts">
import { computed, onActivated, onBeforeUnmount, onDeactivated, onMounted, ref, watch } from 'vue'
import type { Campaign, GameAPI, GameUser } from './game-types'
type Action = { event_id: number; user_id: number; actor: string; content: string; supplements: { event_id: number; content: string }[] }
type Turn = { turn_id: number; kind: string; status: string; error: string; started_at: number; actions: Action[] }
type State = { flow: { mode: string; participants: number[]; round: number }; queue: Turn[]; collected: Action[]; waiting: { user_id: number; actor: string; left: boolean }[]; awaiting_choices: number }
const props = defineProps<{ campaign: Campaign; api: GameAPI; me: GameUser; members: GameUser[]; revision: number }>()
const emit = defineEmits<{ changed: [] }>()
const state = ref<State | null>(null), error = ref(''), readError = ref(''), busy = ref(false), now = ref(Date.now())
const mode = ref('individual'), participants = ref<number[]>([]), configDirty = ref(false), confirmRelease = ref(false)
const supplement = ref(0), text = ref('')
const owner = computed(() => props.me.id === props.campaign.owner_id)
const blocked = computed(() => !!state.value?.queue.length || !!state.value?.collected.length || !!props.campaign.active_turn)
const expired = (t: Turn) => t.status === 'running' && now.value - t.started_at > 180000
const status = (t: Turn) => expired(t) ? '已超时，待房主处理' : t.status === 'running' ? '主持处理中' : t.status === 'failed' ? '失败待处理' : '排队中'
const label = computed(() => {
  if (readError.value && !state.value) return '状态暂时无法读取'
  if (!state.value) return '正在读取'
  if (props.campaign.phase === 'paused') return '战役已暂停'
  const first = state.value.queue[0]
  if (first?.status === 'failed' || first && expired(first)) return status(first)
  if (state.value.awaiting_choices) return '等待角色本人选择代价'
  if (first) return `${status(first)} · ${first.actions.map(a => a.actor).join('、') || `请求 #${first.turn_id}`}`
  if (state.value.flow.mode === 'collect') return `第 ${state.value.flow.round} 轮收集中 · 等待 ${state.value.waiting.length} 人`
  return props.campaign.mode === 'manual' ? '手动主持' : '等待新行动'
})
let active = true, stopped = false, loading = false, pending = false, generation = 0, timer: ReturnType<typeof setInterval>
const ids = new Map<string, string>()
async function load() {
  if (!active || stopped || document.hidden) return
  if (loading) { pending = true; return }
  loading = true; const gen = generation
  try {
    const result = await props.api<State>(`/v1/campaigns/${props.campaign.id}/flow`)
    if (gen === generation && !stopped) {
      state.value = result; readError.value = ''
      if (!configDirty.value) { mode.value = result.flow.mode; participants.value = [...result.flow.participants] }
    }
  } catch (e) { if (gen === generation) readError.value = e instanceof Error ? e.message : '队列暂时无法读取' }
  finally { loading = false; if (pending) { pending = false; void load() } }
}
async function write(path: string, body: Record<string, unknown>, method = 'POST') {
  if (busy.value) return
  busy.value = true; error.value = ''
  const key = path + JSON.stringify(body)
  if (!ids.has(key)) ids.set(key, crypto.randomUUID())
  try {
    await props.api(`/v1/campaigns/${props.campaign.id}/${path}`, method, { ...body, client_id: ids.get(key) })
    ids.delete(key); if (path === 'flow') configDirty.value = false
    confirmRelease.value = false; supplement.value = 0; text.value = ''
    emit('changed'); await load()
  } catch (e) { error.value = e instanceof Error ? e.message : '操作失败，请重试；未发送内容仍保留' }
  finally { busy.value = false }
}
function openSupplement(id: number) { if (supplement.value && text.value.trim()) { error.value = '请先发送或取消当前补充'; return }; supplement.value = id; text.value = '' }
watch(() => props.revision, () => { void load() })
watch(() => props.campaign.version, () => { void load() })
onMounted(() => { void load(); timer = setInterval(() => { now.value = Date.now(); void load() }, 3000) })
onActivated(() => { active = true; void load() })
onDeactivated(() => { active = false; generation++ })
onBeforeUnmount(() => { stopped = true; generation++; clearInterval(timer) })
</script>

<template>
  <details class="campaign-flow">
    <summary>行动队列 · {{ label }}</summary>
    <div class="flow-content">
      <p v-if="error || readError" class="game-error" role="alert">{{ error || readError }} <button class="text-button" @click="error = ''; load()">重新读取队列</button></p>
      <template v-if="state">
        <p v-if="state.awaiting_choices" class="game-helper">还有 {{ state.awaiting_choices }} 项代价选择，必须由角色本人在“行动检定”确认，主持不能代选。</p>
        <section v-if="state.flow.mode === 'collect'"><h3>第 {{ state.flow.round }} 轮收集</h3><p class="game-helper">已提交 {{ state.collected.length }} / {{ state.flow.participants.length }} 人。全部到齐后自动组成一个联合回合；不会代未提交者决定行动。</p><p v-if="state.waiting.length" class="game-helper">等待：{{ state.waiting.map(p => p.actor + (p.left ? '（已离队）' : '')).join('、') }}</p>
          <article v-for="a in state.collected" :key="a.event_id"><b>{{ a.actor }} · 已提交</b><p>{{ a.content }}</p><p v-for="s in a.supplements" :key="s.event_id">补充：{{ s.content }}</p><button v-if="a.user_id === me.id" class="text-button" :disabled="busy" @click="openSupplement(a.event_id)">补充这条行动</button></article>
          <template v-if="owner && state.collected.length"><button v-if="!confirmRelease" class="game-button" :disabled="busy" @click="confirmRelease = true">提前结束本轮收集</button><div v-else class="flow-confirm" role="alert"><p>只处理已提交的行动，未提交者跳过本轮，不会为他们虚构行动。确认继续？</p><button class="game-button" :disabled="busy" @click="write('flow/release', { version: campaign.version, allow_missing: true })">确认联合处理已提交行动</button><button class="text-button" @click="confirmRelease = false">继续等待</button></div></template>
        </section>
        <ol v-if="state.queue.length" class="flow-queue"><li v-for="t in state.queue" :key="t.turn_id"><header><b>{{ status(t) }}</b><span>回合 #{{ t.turn_id }}{{ t.kind === 'group' ? ' · 联合行动' : '' }}</span></header><p v-if="!t.actions.length" class="game-helper">主持请求或已记录的骰子事件。</p><article v-for="a in t.actions" :key="a.event_id"><b>{{ a.actor }} · 行动 #{{ a.event_id }}</b><p>{{ a.content }}</p><p v-for="s in a.supplements" :key="s.event_id">补充：{{ s.content }}</p><button v-if="a.user_id === me.id && t.status === 'queued'" class="text-button" :disabled="busy" @click="openSupplement(a.event_id)">补充这条行动</button></article><p v-if="t.error" class="game-error">{{ t.error }}</p><div v-if="owner && (t.status === 'failed' || expired(t))" class="game-actions"><button class="game-button" :disabled="busy" @click="write(`turns/${t.turn_id}/retry`, { version: campaign.version })">重试并保留已掷骰</button><button class="text-button" :disabled="busy" @click="write(`turns/${t.turn_id}/retry`, { version: campaign.version, skip: true })">跳过此回合</button></div></li></ol>
        <p v-else-if="state.flow.mode !== 'collect'" class="game-helper">没有待处理的行动。已完成的结果仍保留在日志。</p>
        <form v-if="supplement" class="game-form" @submit.prevent="write(`actions/${supplement}/supplement`, { content: text })"><label>行动补充<textarea v-model="text" rows="3" maxlength="1000" required placeholder="追加说明，不改写原行动；主持领取后将不能再补充。"></textarea></label><div class="game-actions"><button class="game-button" :disabled="busy || !text.trim()">发送行动补充</button><button type="button" class="text-button" :disabled="busy" @click="supplement = 0; text = ''">取消补充</button></div></form>
        <details v-if="owner" class="flow-settings"><summary>设置行动节奏</summary><form class="game-form" @submit.prevent="write('flow', { version: campaign.version, mode, participants }, 'PUT')"><label>处理方式<select v-model="mode" aria-label="行动处理方式" @change="configDirty = true"><option value="individual">逐人处理</option><option value="collect">全员收集后联合处理</option></select></label><fieldset v-if="mode === 'collect'" :disabled="busy || blocked"><legend>本轮收集名单（1–12人）</legend><label v-for="p in members" :key="p.id" class="game-check"><input v-model="participants" type="checkbox" :value="p.id" @change="configDirty = true" />{{ p.nickname }}</label></fieldset><p class="game-helper">有排队、运行中或已收集的行动时不可更换节奏。全员收集需要开启自动排队及AI／外部主持。离队或未提交者由房主决定是否提前结束本轮。</p><button class="game-button" :disabled="busy || blocked || mode === 'collect' && !participants.length">保存行动节奏</button></form></details>
      </template>
    </div>
  </details>
</template>

<style scoped>
.campaign-flow { flex-shrink: 0; border-bottom: 1px solid var(--line); font-size: 12px; }
.campaign-flow > summary { padding: 12px 28px; cursor: pointer; color: var(--muted); overflow-wrap: anywhere; }
.flow-content { padding: 0 28px 20px; max-height: 40vh; overflow-y: auto; }
.flow-content section { margin: 16px 0; }
.flow-content h3 { font-size: 13px; }
.flow-content p { white-space: pre-wrap; overflow-wrap: anywhere; max-width: 74ch; line-height: 1.8; }
.flow-content article { margin: 12px 0; }
.flow-queue { margin: 12px 0; padding-left: 20px; }
.flow-queue > li { border-bottom: 1px solid var(--line); padding: 16px 0; }
.flow-queue header { display: flex; flex-wrap: wrap; gap: 12px; }
.flow-queue header span { color: var(--muted); }
.flow-confirm { border: 1px solid var(--warning-line); border-radius: 9px; background: var(--warning-bg); padding: 12px; }
.flow-settings { margin-top: 20px; border-top: 1px solid var(--line); padding-top: 12px; }
.flow-settings > summary { cursor: pointer; }
.flow-settings fieldset { border: 1px solid var(--line); border-radius: 7px; padding: 12px; }
.flow-settings fieldset label { margin: 8px 0; }
@media (max-width: 760px) { .campaign-flow > summary { padding: 10px 16px; } .flow-content { padding: 0 16px 16px; } }
</style>
