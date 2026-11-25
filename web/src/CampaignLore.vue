<script setup lang="ts">
import { computed, nextTick, onActivated, onBeforeUnmount, onDeactivated, ref, watch } from 'vue'
import type { Campaign, GameAPI } from './game-types'
type Entry = { id: number; kind: string; title: string; content: string; visibility: string; status: string; in_context: boolean; version: number; updated_at: number }
const props = defineProps<{ campaign: Campaign; api: GameAPI; owner: boolean; revision: number }>()
const emit = defineEmits<{ changed: [] }>()
const kinds: Record<string, string> = { chapter: '剧本章节', npc: '人物 / NPC', quest: '任务', clue: '线索', fact: '长期事实' }
const entries = ref<Entry[]>([]), history = ref<Entry[]>([])
const search = ref(''), kind = ref(''), archived = ref(false)
const loading = ref(true), busy = ref(false), error = ref(''), notice = ref(''), editing = ref(false)
const fresh = (): Entry => ({ id: 0, kind: 'chapter', title: '', content: '', visibility: 'dm', status: 'active', in_context: false, version: 0, updated_at: 0 })
const draft = ref<Entry>(fresh()), baseline = ref(''), titleInput = ref<HTMLInputElement>()
const dirty = computed(() => editing.value && JSON.stringify(draft.value) !== baseline.value)
const selected = ref<Entry | null>(null), discardTarget = ref<Entry | 'new' | null>(null)
const filtered = computed(() => entries.value.filter(e => (archived.value || e.status === 'active') && (!kind.value || e.kind === kind.value) &&
  `${e.title}\n${e.content}`.toLocaleLowerCase().includes(search.value.toLocaleLowerCase())))
const latest = computed(() => entries.value.find(e => e.id === draft.value.id))
const stale = computed(() => editing.value && draft.value.id && latest.value && latest.value.version !== draft.value.version)
const included = computed(() => {
  let budget = 24000, count = 0, omitted = 0
  for (const e of entries.value) {
    if (e.status !== 'active' || e.visibility !== 'public' || !e.in_context) continue
    const size = Array.from(e.title + e.content).length
    if (size > budget) { omitted++; continue }
    budget -= size; count++
  }
  return { count, omitted }
})
let generation = 0, stopped = false, active = true
const operations = new Map<string, string>()
async function load() {
  const gen = ++generation
  try {
    const data = await props.api<Entry[]>(`/v1/campaigns/${props.campaign.id}/lore`)
    if (!stopped && gen === generation) {
      entries.value = data; error.value = ''
      if (!editing.value && selected.value) selected.value = data.find(e => e.id === selected.value!.id) || null
    }
  } catch (e) { if (gen === generation) error.value = e instanceof Error ? e.message : '资料暂时无法读取，请重试' }
  finally { if (gen === generation) loading.value = false }
}
async function open(entry: Entry | 'new', discard = false) {
  if (busy.value) return
  if (dirty.value && !discard) { discardTarget.value = entry; return }
  discardTarget.value = null; error.value = ''; notice.value = ''; history.value = []
  selected.value = entry === 'new' ? null : entry
  draft.value = entry === 'new' ? fresh() : { ...entry }
  baseline.value = JSON.stringify(draft.value); editing.value = props.owner
  if (editing.value) { await nextTick(); titleInput.value?.focus() }
}
async function save() {
  if (busy.value || !props.owner || props.campaign.active_turn || stale.value) return
  busy.value = true; error.value = ''; notice.value = ''
  const body = { version: props.campaign.version, entry: { ...draft.value, in_context: draft.value.visibility === 'public' && draft.value.in_context } }
  const fingerprint = JSON.stringify(body)
  if (!operations.has(fingerprint)) operations.set(fingerprint, crypto.randomUUID())
  try {
    const result = await props.api<{ entry: Entry }>(`/v1/campaigns/${props.campaign.id}/lore`, 'POST', { ...body, client_id: operations.get(fingerprint) })
    operations.delete(fingerprint)
    draft.value = result.entry; baseline.value = JSON.stringify(result.entry); selected.value = result.entry
    notice.value = '已保存到战役资料'; emit('changed'); await load()
  } catch (e) { error.value = e instanceof Error ? e.message : '保存失败，文字已保留，请重试' }
  finally { busy.value = false }
}
async function importText(event: Event) {
  const input = event.target as HTMLInputElement, file = input.files?.[0]
  if (!file) return
  error.value = ''; notice.value = ''
  try {
    if (!/\.(txt|md|markdown)$/i.test(file.name) || file.size > 65536) throw Error('请选择不超过 64 KB 的 UTF-8 TXT 或 Markdown 文件')
    const text = new TextDecoder('utf-8', { fatal: true }).decode(await file.arrayBuffer()).replace(/^\uFEFF/, '')
    if (!text.trim() || Array.from(text).length > 12000) throw Error('导入正文需为 1–12000 字，请按章节拆分后导入')
    if (draft.value.content.trim()) throw Error('当前正文已有内容。请先保存或新建资料，避免覆盖未保存的文字')
    draft.value.content = text
    if (!draft.value.title) draft.value.title = file.name.replace(/\.(txt|md|markdown)$/i, '').slice(0, 100)
    notice.value = '已载入预览，尚未保存。请核对分类、正文和可见范围后保存。'
  } catch (e) { error.value = e instanceof Error ? e.message : '文件读取失败' }
  finally { input.value = '' }
}
async function loadHistory() {
  if (!draft.value.id && !selected.value) return
  try { history.value = await props.api<Entry[]>(`/v1/campaigns/${props.campaign.id}/lore/${draft.value.id || selected.value!.id}/history`) }
  catch (e) { error.value = e instanceof Error ? e.message : '历史版本读取失败' }
}
function useHistory(e: Entry) {
  // Keep the current revision for optimistic concurrency; saving creates a new
  // audit revision rather than deleting or rewriting history.
  draft.value = { ...e, version: draft.value.version, updated_at: draft.value.updated_at }
  history.value = []; notice.value = '旧版本已载入编辑区，核对后保存会建立新版本。'
}
watch(() => props.revision, () => { if (active && !busy.value) void load() })
watch(() => props.campaign.version, () => { if (active && !busy.value) void load() })
onActivated(() => { active = true; void load() })
onDeactivated(() => { active = false; generation++ })
onBeforeUnmount(() => { stopped = true; generation++ })
</script>

<template>
  <section class="campaign-lore">
    <div class="game-section-heading"><div><h3>剧本与记忆</h3><p>把章节、人物与重要事实留在战役里，由房主确认，不让推测变成既定事实。</p></div><button v-if="owner" class="game-button" :disabled="busy" @click="open('new')">新建资料</button></div>
    <p class="game-helper">公开且勾选“供主持参考”的有效资料会加入 AI 上下文，目前 {{ included.count }} 条。{{ included.omitted ? `另有 ${included.omitted} 条超出 24000 字预算；请取消无关条目后再主持。` : '秘密资料只供房主及获准的外部主持读取，不发送给内置公开模型。' }}</p>
    <p v-if="error" class="game-error" role="alert">{{ error }} <button class="text-button" @click="load">重新读取资料</button></p>
    <p v-if="notice" class="game-helper" role="status">{{ notice }}</p>
    <div v-if="discardTarget" class="lore-confirm" role="alert"><p>当前有未保存的修改。是否放弃这些修改并打开另一条资料？</p><div class="game-actions"><button class="game-button" @click="open(discardTarget!, true)">放弃修改并继续</button><button class="text-button" @click="discardTarget = null">保留当前编辑</button></div></div>
    <div class="lore-layout">
      <div class="lore-index">
        <label>查找资料<input v-model="search" type="search" placeholder="标题或正文关键词" /></label>
        <label>资料分类<select v-model="kind"><option value="">全部分类</option><option v-for="(label, value) in kinds" :key="value" :value="value">{{ label }}</option></select></label>
        <label class="game-check"><input v-model="archived" type="checkbox" />显示已归档资料</label>
        <p v-if="loading" class="game-helper" role="status">正在读取战役资料…</p>
        <p v-else-if="!filtered.length" class="game-helper">{{ entries.length ? '没有匹配的资料，请调整筛选。' : owner ? '还没有资料。新建一条，粘贴文字或导入 TXT / Markdown。' : '房主还没有共享公开资料。' }}</p>
        <ul v-else class="lore-list" aria-label="战役资料列表"><li v-for="entry in filtered" :key="entry.id"><button type="button" :aria-current="draft.id === entry.id || selected?.id === entry.id ? 'true' : undefined" @click="open(entry)"><b>{{ entry.title }}</b><small>{{ kinds[entry.kind] }} · {{ entry.visibility === 'dm' ? '主持秘密' : '公开' }} · {{ entry.status === 'archived' ? '已归档' : entry.in_context ? '供主持参考' : '仅保存' }}</small></button></li></ul>
      </div>
      <div class="lore-detail">
        <form v-if="editing && owner" class="game-form" @submit.prevent="save">
          <label>资料标题<input ref="titleInput" v-model="draft.title" maxlength="100" required /></label>
          <div class="game-form-row"><label>分类<select v-model="draft.kind" aria-label="分类"><option v-for="(label, value) in kinds" :key="value" :value="value">{{ label }}</option></select></label><label>可见范围<select v-model="draft.visibility" aria-label="可见范围"><option value="dm">主持秘密</option><option value="public">全体成员公开</option></select></label></div>
          <p v-if="draft.visibility === 'public'" class="game-helper lore-warning">保存后所有战役成员都能读取正文。已公开的内容无法从其他成员的记忆中撤回。</p>
          <label class="lore-import">导入文字文件<input type="file" accept=".txt,.md,.markdown,text/plain,text/markdown" @change="importText" /><small>UTF-8 · 最多 64 KB / 12000 字 · 先预览，再保存</small></label>
          <label>资料正文<textarea v-model="draft.content" rows="12" maxlength="12000" required placeholder="输入完整章节、NPC 信息、任务进度或经房主确认的长期事实。Markdown 以纯文本保存，不执行其中内容。"></textarea></label>
          <div class="game-form-row"><label>资料状态<select v-model="draft.status" aria-label="资料状态"><option value="active">有效</option><option value="archived">已归档</option></select></label><label class="game-check"><input v-model="draft.in_context" type="checkbox" :disabled="draft.visibility !== 'public' || draft.status === 'archived'" />供主持参考（仅公开有效资料）</label></div>
          <p v-if="campaign.active_turn" class="game-helper">主持正在处理回合，可继续编辑草稿；完成或暂停接管后才能保存。</p>
          <p v-if="stale" class="game-error" role="alert">资料已有新版本，当前文字仍保留。请复制需要保留的修改，再从左侧打开最新资料比较。</p>
          <div class="game-actions"><button class="game-button" :disabled="busy || !!campaign.active_turn || !!stale || !draft.title.trim() || !draft.content.trim()">{{ busy ? '正在保存…' : '保存资料' }}</button><button v-if="draft.id" type="button" class="text-button" :disabled="busy" @click="loadHistory">查看版本记录</button><span class="game-helper">{{ dirty ? '有未保存修改' : draft.id ? `已保存 · v${draft.version}` : '新资料尚未保存' }}</span></div>
        </form>
        <article v-else-if="selected" class="lore-reader"><h3>{{ selected.title }}</h3><p class="game-helper">{{ kinds[selected.kind] }} · {{ selected.status === 'archived' ? '已归档' : '有效' }} · v{{ selected.version }}</p><p class="lore-prose">{{ selected.content }}</p><button class="text-button" @click="loadHistory">查看版本记录</button></article>
        <div v-else class="lore-placeholder"><h3>{{ owner ? '为下一幕留下依据' : '阅读战役资料' }}</h3><p>从列表选择资料{{ owner ? '，或新建一条。秘密与公开资料分开保存，归档不会删除版本记录。' : '，查看已经共享的章节、人物和线索。' }}</p></div>
        <section v-if="history.length" class="lore-history" aria-label="资料版本记录"><h4>版本记录</h4><details v-for="entry in history" :key="entry.version"><summary>v{{ entry.version }} · {{ entry.title }} · {{ new Date(entry.updated_at).toLocaleString('zh-CN') }}</summary><p class="lore-prose">{{ entry.content }}</p><button v-if="owner && editing && entry.version !== draft.version" class="text-button" @click="useHistory(entry)">载入此版本到编辑区</button></details></section>
      </div>
    </div>
  </section>
</template>

<style scoped>
.lore-layout { display: grid; grid-template-columns: minmax(180px, 240px) minmax(0, 1fr); gap: 28px; margin-top: 24px; }
.lore-index { min-width: 0; }
.lore-index > label { display: grid; gap: 8px; margin-bottom: 16px; color: var(--muted); font-size: 12px; }
.lore-index > .game-check { display: flex; align-items: center; gap: 8px; }
.lore-index input:not([type=checkbox]), .lore-index select { width: 100%; border: 1px solid var(--line); border-radius: 7px; padding: 10px 12px; color: var(--text); background: var(--field); font: inherit; min-height: 42px; }
.lore-index input:focus-visible, .lore-index select:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
.lore-index input::placeholder { color: var(--muted); opacity: 1; }
.lore-list { list-style: none; padding: 0; margin: 20px 0 0; max-height: 480px; overflow-y: auto; }
.lore-list button { width: 100%; display: grid; gap: 6px; padding: 12px; text-align: left; border: 1px solid transparent; border-radius: 8px; color: var(--text); background: transparent; overflow-wrap: anywhere; }
.lore-list button:hover { background: var(--hover); }
.lore-list button[aria-current=true] { background: var(--selected); border-color: var(--selected-line); color: var(--selected-text); }
.lore-list button[aria-current=true] small { color: var(--selected-text); }
.lore-list small { color: var(--muted); font-size: 11px; line-height: 1.6; }
.lore-detail { min-width: 0; }
.lore-detail .game-form { max-width: none; margin: 0; }
.lore-import small { color: var(--muted); font-size: 11px; line-height: 1.6; }
.lore-import input { max-width: 100%; }
.lore-import input::file-selector-button { border: 1px solid var(--line); border-radius: 7px; padding: 6px 10px; margin-right: 10px; color: var(--text); background: var(--raised); font: inherit; cursor: pointer; }
.lore-warning { color: var(--warning); }
.lore-prose { white-space: pre-wrap; overflow-wrap: anywhere; font-size: 14px; line-height: 1.95; max-width: 74ch; }
.lore-reader h3, .lore-placeholder h3 { margin: 0 0 12px; }
.lore-placeholder { padding: 24px 0; }
.lore-placeholder p { color: var(--muted); line-height: 1.8; }
.lore-confirm { padding: 16px; border: 1px solid var(--warning-line); background: var(--warning-bg); border-radius: 9px; margin-top: 16px; }
.lore-confirm p { margin: 0 0 12px; }
.lore-history { margin-top: 28px; border-top: 1px solid var(--line); padding-top: 16px; }
.lore-history details { padding: 12px 0; border-bottom: 1px solid var(--line); }
.lore-history summary { cursor: pointer; line-height: 1.7; overflow-wrap: anywhere; }
@media (max-width: 900px) {
  .lore-layout { grid-template-columns: minmax(0, 1fr); gap: 24px; }
  .lore-list { max-height: 220px; }
  .lore-placeholder { padding: 0; }
}
</style>
