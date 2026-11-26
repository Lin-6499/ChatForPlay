<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { phases, type GameAPI } from './game-types'
const props = defineProps<{ campaignID: number; api: GameAPI; revision: number }>()
type Entry = { id: number; title: string; content: string; progress: string }
type Recap = { version: number; summary: string; scene: string; phase: string; objectives: Entry[]; objectives_omitted: number; facts: Entry[]; recent: { event_id: number; content: string; excerpt: boolean }[] }
const data = ref<Recap | null>(null), open = ref(false), error = ref(''), loading = ref(false)
const progress: Record<string, string> = { not_started: '未开始', in_progress: '进行中', undiscovered: '未发现', discovered: '已发现' }
let generation = 0, stopped = false, pending = false
async function load() {
  if (!open.value || stopped) return
  if (loading.value) { pending = true; return }
  const gen = ++generation; loading.value = true
  try {
    const result = await props.api<Recap>(`/v1/campaigns/${props.campaignID}/recap`)
    if (!stopped && gen === generation) { data.value = result; error.value = '' }
  } catch (e) { if (gen === generation) error.value = e instanceof Error ? e.message : '回顾暂时无法读取' }
  finally { loading.value = false; if (pending) { pending = false; void load() } }
}
function toggle(event: Event) { open.value = (event.target as HTMLDetailsElement).open; if (open.value) void load() }
watch(() => props.revision, () => { void load() })
onBeforeUnmount(() => { stopped = true; generation++ })
</script>

<template>
  <details class="campaign-recap" @toggle="toggle">
    <summary>前情回顾与未解决事项</summary>
    <div class="recap-content">
      <p class="game-helper">按已保存的公开摘要、正式叙述和房主资料整理，不调用模型、不新增事实。</p>
      <p v-if="error" class="game-error" role="alert">{{ error }} <button class="text-button" @click="load">重试读取回顾</button></p>
      <p v-if="loading && !data" class="game-helper" role="status">正在整理已保存的记录…</p>
      <template v-if="data">
        <p class="game-helper">当前：{{ phases[data.phase] || data.phase }} · 存档 v{{ data.version }}</p>
        <section v-if="data.summary"><h3>上次的故事</h3><p>{{ data.summary }}</p></section>
        <section><h3>还未解决</h3><p v-if="!data.objectives.length" class="game-helper">尚未记录公开的未完成任务或线索。</p><ul v-else><li v-for="e in data.objectives" :key="e.id"><b>{{ e.title }}</b><small>{{ progress[e.progress] || '待跟进' }} · 资料 #{{ e.id }}</small><p>{{ e.content }}</p></li></ul><p v-if="data.objectives_omitted" class="game-helper">另有 {{ data.objectives_omitted }} 项，请在“剧本与记忆”中查看。</p></section>
        <section v-if="data.facts.length"><h3>已确认的参考事实</h3><ul><li v-for="e in data.facts" :key="e.id"><b>{{ e.title }}</b><small>资料 #{{ e.id }}</small><p>{{ e.content }}</p></li></ul></section>
        <section v-if="data.recent.length"><h3>最近正式叙述</h3><details v-for="e in data.recent" :key="e.event_id"><summary>叙述 #{{ e.event_id }}{{ e.excerpt ? ' · 节选' : '' }}</summary><p>{{ e.content }}</p></details></section>
        <p v-if="!data.summary && !data.recent.length" class="game-helper">尚无前情记录。完成第一幕后，正式叙述会出现在这里。</p>
      </template>
    </div>
  </details>
</template>

<style scoped>
.campaign-recap { flex-shrink: 0; border-bottom: 1px solid var(--line); font-size: 12px; }
.campaign-recap > summary { cursor: pointer; padding: 12px 28px; color: var(--muted); }
.recap-content { padding: 0 28px 20px; max-height: 42vh; overflow-y: auto; }
.recap-content section { margin-top: 20px; }
.recap-content h3 { font-size: 13px; margin: 0 0 12px; }
.recap-content p { white-space: pre-wrap; overflow-wrap: anywhere; line-height: 1.85; max-width: 74ch; }
.recap-content ul { padding-left: 20px; margin: 0; }
.recap-content li { margin-bottom: 16px; }
.recap-content small { display: block; margin-top: 4px; color: var(--muted); }
.recap-content details { margin-top: 10px; }
.recap-content summary { cursor: pointer; }
@media (max-width: 760px) { .campaign-recap > summary { padding: 10px 16px; } .recap-content { padding: 0 16px 16px; } }
</style>
