<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import type { GameAPI } from './game-types'
const props = defineProps<{ campaignID: number; api: GameAPI; revision: number }>()
type Capabilities = { server_version: string; protocol: { min: number; max: number }; features: string[] }
type Diagnostics = { mode: string; builtin_ready: boolean; contact_at: number; claim_at: number; stream_at: number; complete_at: number; fail_at: number }
const capabilities = ref<Capabilities | null>(null), diagnostics = ref<Diagnostics | null>(null), error = ref(''), loading = ref(false)
let stopped = false, pending = false, timer: ReturnType<typeof setInterval>
const format = (time: number) => time ? new Date(time).toLocaleString('zh-CN') : '尚无记录'
async function load() {
  if (stopped || document.hidden) return
  if (loading.value) { pending = true; return }
  loading.value = true
  try {
    const [caps, state] = await Promise.all([
      props.api<Capabilities>(`/v1/campaigns/${props.campaignID}/capabilities`),
      props.api<Diagnostics>(`/v1/campaigns/${props.campaignID}/host-diagnostics`),
    ])
    if (!stopped) { capabilities.value = caps; diagnostics.value = state; error.value = '' }
  } catch (e) { error.value = e instanceof Error ? e.message : '接入检查暂时无法读取' }
  finally { loading.value = false; if (pending) { pending = false; void load() } }
}
watch(() => props.revision, () => { void load() })
onMounted(() => { void load(); timer = setInterval(() => { void load() }, 10000) })
onBeforeUnmount(() => { stopped = true; clearInterval(timer) })
</script>

<template>
  <section class="host-diagnostics" aria-label="主持接入自检">
    <header><h3>主持接入自检</h3><button class="text-button" :disabled="loading" @click="load">{{ loading ? '正在检查…' : '刷新接入检查' }}</button></header>
    <p v-if="error" class="game-error" role="alert">{{ error }}</p>
    <template v-if="capabilities && diagnostics">
      <p>服务器 {{ capabilities.server_version }} · 协议 {{ capabilities.protocol.min }}–{{ capabilities.protocol.max }} · {{ capabilities.features.length }} 项能力</p>
      <p class="game-helper">支持流式正文、快捷回复、分级检定、剧本记忆和联合行动。版本支持不代表外部会话已加载新版工具。</p>
      <p v-if="diagnostics.mode === 'builtin' && !diagnostics.builtin_ready" class="game-error">内置模型未启用或尚未保存密钥，请管理员检查“模型设置”。</p>
      <p v-if="diagnostics.mode === 'external' && !diagnostics.contact_at" class="game-helper">尚未检测到主持领取请求。请启动新版Bot，或通过MCP／HTTP主动领取回合；仅创建令牌不会自动运行主持。</p>
      <dl><div v-for="(label, key) in { contact_at: '最近联系', claim_at: '最近领取', stream_at: '最近流式更新', complete_at: '最近完成', fail_at: '最近失败' }" :key="key"><dt>{{ label }}</dt><dd>{{ format(diagnostics[key]) }}</dd></div></dl>
      <p class="game-helper">最近联系不是在线保证；机器人可能已退出。这里不展示令牌、租约或模型密钥。</p>
      <p class="game-helper">统一接入说明：<code>docs/HOST_INTEGRATION.md</code>。外部Bot可运行 <code>node scripts/campaign-agent.cjs --check</code> 检查权限与协议，无需发起主持。</p>
    </template>
  </section>
</template>

<style scoped>
.host-diagnostics { padding: 20px 0; border-top: 1px solid var(--line); border-bottom: 1px solid var(--line); margin: 20px 0; }
.host-diagnostics header { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 12px; }
.host-diagnostics h3 { margin: 0; font-size: 15px; }
.host-diagnostics p { line-height: 1.8; overflow-wrap: anywhere; font-size: 12px; max-width: 74ch; }
.host-diagnostics dl { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 16px; margin: 20px 0; }
.host-diagnostics dt { color: var(--muted); font-size: 11px; }
.host-diagnostics dd { margin: 6px 0 0; font-size: 12px; font-variant-numeric: tabular-nums; }
.host-diagnostics code { overflow-wrap: anywhere; }
</style>
