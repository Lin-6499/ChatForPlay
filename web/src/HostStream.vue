<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, onActivated, onDeactivated } from 'vue'
import type { GameAPI } from './game-types'
const props = defineProps<{ campaignID: number; api: GameAPI; completed: number[] }>()
type Draft = { turn_id: number; seq: number; narration: string; expires_at: number }
const draft = ref<Draft | null>(null), disconnected = ref(false), now = ref(Date.now())
const visible = computed(() => draft.value && draft.value.expires_at > now.value && !props.completed.includes(draft.value.turn_id))
let stopped = false, active = true, loading = false, again = false, timer: ReturnType<typeof setInterval>
async function refresh() {
  if (stopped || !active || document.hidden) return
  if (loading) { again = true; return }
  loading = true
  try {
    const result = await props.api<{ draft: Draft | null }>(`/v1/campaigns/${props.campaignID}/stream`)
    if (!stopped && active) { draft.value = result.draft; disconnected.value = false }
  } catch { disconnected.value = true }
  finally {
    loading = false
    if (again) { again = false; void refresh() }
  }
}
function changed(e: Event) { if ((e as CustomEvent).detail === props.campaignID) void refresh() }
function reconnected() { void refresh() }
onMounted(() => {
  void refresh()
  window.addEventListener('linchat:campaign-stream', changed)
  window.addEventListener('linchat:reconnected', reconnected)
  document.addEventListener('visibilitychange', refresh)
  timer = setInterval(() => { now.value = Date.now(); void refresh() }, 3000)
})
onActivated(() => { active = true; void refresh() })
onDeactivated(() => { active = false; draft.value = null })
onBeforeUnmount(() => {
  stopped = true; clearInterval(timer)
  window.removeEventListener('linchat:campaign-stream', changed)
  window.removeEventListener('linchat:reconnected', reconnected)
  document.removeEventListener('visibilitychange', refresh)
})
</script>

<template>
  <article v-if="visible && draft" class="game-entry narration game-stream" aria-label="主持人实时草稿" aria-live="off">
    <div class="game-entry-meta"><b>主持人</b><span class="game-stream-status">{{ disconnected ? '连接中断，正在重连' : '正在生成 · 尚未入档' }}</span></div>
    <p class="game-entry-text">{{ draft.narration }}</p>
    <small>完成后会自动保存正式叙述；中断的草稿不会改变游戏状态。</small>
  </article>
</template>

<style scoped>
.game-stream { border: 1px solid var(--line); padding: 16px; border-radius: 9px; }
.game-stream-status { color: var(--warning); }
.game-stream small { display: block; margin-top: 12px; color: var(--muted); line-height: 1.6; }
</style>
