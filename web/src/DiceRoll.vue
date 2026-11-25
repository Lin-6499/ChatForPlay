<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref } from 'vue'
import DiceDie from './DiceDie.vue'
import Icon from './Icon.vue'
import { diceSpec } from './dice-geometry'
import type { DiceInput, DiceReceipt } from './game-types'
import './dice.css'

const props = defineProps<{ isOwner: boolean; submit: (input: DiceInput) => Promise<DiceReceipt> }>()
const form = defineModel<DiceInput>({ required: true })
const emit = defineEmits<{ close: [] }>()
const phase = ref<'ready' | 'rolling' | 'waiting' | 'settled' | 'error'>('ready')
const receipt = ref<DiceReceipt>(), failure = ref(''), progress = ref(1), submitButton = ref<HTMLButtonElement>()
const tray = ref<HTMLElement>()
const motion = matchMedia('(prefers-reduced-motion: reduce)')
let frame = 0, disposed = false, started = 0, toss: Animation | undefined
const locked = computed(() => phase.value === 'rolling' || phase.value === 'waiting')
const spec = computed(() => diceSpec(receipt.value?.roll.formula || form.value.formula))
const count = computed(() => spec.value?.count || 1)
const visibleCount = computed(() => Math.min(count.value, 6))
const revealed = computed(() => phase.value === 'settled' ? receipt.value : undefined)
const keptIndices = computed(() => {
  const remaining = [...(revealed.value?.roll.kept || [])]
  return (revealed.value?.roll.dice || []).map(value => {
    const at = remaining.indexOf(value)
    if (at < 0) return false
    remaining.splice(at, 1); return true
  })
})
const status = computed(() => ({
  ready: '准备掷骰', rolling: '骰子翻滚中…', waiting: '等待服务器结果…',
  settled: '已写入冒险日志', error: '未能确认结果',
})[phase.value])
const equation = computed(() => {
  const roll = revealed.value?.roll
  if (!roll) return ''
  const sum = roll.kept.reduce((a, b) => a + b, 0)
  return `${roll.kept.length > 6 ? `${roll.kept.length} 枚合计 ${sum}` : roll.kept.join(' + ')}${roll.modifier ? ` ${roll.modifier > 0 ? '+' : '−'} ${Math.abs(roll.modifier)}` : ''}`
})
const announcement = computed(() => revealed.value
  ? `掷骰已记录。${revealed.value.roll.formula}，合计 ${revealed.value.roll.total}。${spec.value?.keep ? `保留 ${revealed.value.roll.kept.join('、')}。` : ''}`
  : status.value)
const presets = [{ label: 'D20', value: '1d20' }, { label: '2D6', value: '2d6' }, { label: '优势', value: '2d20kh1' }, { label: '劣势', value: '2d20kl1' }]

function finishMotion() {
  cancelAnimationFrame(frame); progress.value = 1; toss = undefined
  if (locked.value) phase.value = receipt.value ? 'settled' : 'waiting'
}
function tick(now: number) {
  // Share the CSS flight's clock so the faceted solid and its landing stay in
  // sync even when a browser throttles or pauses the animation.
  toss ||= tray.value?.querySelector('.dice-solid')?.getAnimations()[0]
  const elapsed = typeof toss?.currentTime === 'number' ? toss.currentTime : now - started
  progress.value = Math.min(elapsed / 820, 1)
  if (progress.value < 1) frame = requestAnimationFrame(tick)
  else finishMotion()
}
function onVisibility() { if (document.hidden) finishMotion() }
function onMotionChange() { if (motion.matches) finishMotion() }
document.addEventListener('visibilitychange', onVisibility)
motion.addEventListener('change', onMotionChange)
onBeforeUnmount(() => {
  disposed = true; cancelAnimationFrame(frame)
  document.removeEventListener('visibilitychange', onVisibility); motion.removeEventListener('change', onMotionChange)
})
async function roll() {
  if (locked.value || revealed.value || !spec.value || !form.value.reason.trim()) return
  failure.value = ''; receipt.value = undefined
  phase.value = 'rolling'; progress.value = motion.matches || document.hidden ? 1 : 0
  started = performance.now(); toss = undefined
  if (progress.value < 1) frame = requestAnimationFrame(tick)
  else phase.value = 'waiting'
  try {
    const result = await props.submit({ ...form.value, visibility: props.isOwner ? form.value.visibility : 'public' })
    if (disposed) return
    receipt.value = result
    if (progress.value === 1) phase.value = 'settled'
  } catch (e) {
    if (disposed) return
    cancelAnimationFrame(frame); progress.value = 1; phase.value = 'error'
    failure.value = e instanceof TypeError ? '连接暂时中断，未能确认本次结果。'
      : e instanceof Error && e.name === 'AbortError' ? '服务器响应超时，请重试本次掷骰。'
      : e instanceof Error ? e.message : '连接暂时中断，请重试本次掷骰。'
  }
}
async function again() {
  receipt.value = undefined; failure.value = ''; phase.value = 'ready'
  await nextTick(); submitButton.value?.focus()
}
function preset(value: string) {
  const modifier = spec.value?.modifier || 0
  form.value.formula = value + (modifier ? `${modifier > 0 ? '+' : ''}${modifier}` : '')
}
</script>

<template>
  <form class="dice-roll" @submit.prevent="roll">
    <div class="dice-scroll">
      <div ref="tray" class="dice-tray" :class="{ 'dice-revealed': revealed, 'dice-private': form.visibility === 'dm' && isOwner }" :data-phase="phase" :aria-busy="locked">
        <div class="dice-tray-heading">
          <span>{{ spec ? `${count} 枚 D${spec.sides}` : '自定义骰式' }}<span v-if="spec?.keep"> · {{ spec.keep === 'kh1' ? '保留最高' : '保留最低' }}</span></span>
          <span><Icon :name="form.visibility === 'dm' && isOwner ? 'lock' : 'users'" :size="13" />{{ form.visibility === 'dm' && isOwner ? '主持秘密' : '公开掷骰' }}</span>
        </div>
        <div class="dice-table" :class="{ 'dice-table-many': visibleCount > 3 }">
          <div v-for="(_, i) in visibleCount" :key="i" class="dice-slot" :class="{ 'dice-discarded': revealed && !keptIndices[i] }">
            <DiceDie :sides="spec?.sides || 20" :progress="progress" :index="i" :value="revealed?.roll.dice[i]" />
            <span class="dice-kept">{{ revealed && spec?.keep ? keptIndices[i] ? '保留' : '舍弃' : `D${spec?.sides || 20}` }}</span>
          </div>
          <span v-if="count > visibleCount" class="dice-extra">+{{ count - visibleCount }} 枚</span>
        </div>
        <div class="dice-outcome">
          <div><b>{{ status }}</b><p>{{ revealed ? equation : phase === 'waiting' ? '结果确认后自动揭晓，可先关闭弹窗。' : '点数由服务器生成并保存' }}</p></div>
          <div class="dice-total" aria-hidden="true"><small>合计</small><strong>{{ revealed?.roll.total ?? '—' }}</strong></div>
        </div>
      </div>
      <p class="dice-announcement" role="status" aria-live="polite" aria-atomic="true">{{ announcement }}</p>
      <p v-if="failure" class="game-error dice-error" role="alert">{{ failure }}<small>重试会复用本次请求，不会重复掷骰。</small></p>
      <details v-if="revealed && count > 6" class="dice-all-results"><summary>查看全部 {{ count }} 枚点数</summary><p>{{ revealed.roll.dice.join('、') }}</p><p>保留 {{ revealed.roll.kept.join('、') }} · 修正 {{ revealed.roll.modifier >= 0 ? '+' : '' }}{{ revealed.roll.modifier }}</p></details>
      <fieldset class="game-form dice-fields" :disabled="locked || !!revealed">
        <label>检定目的<input v-model="form.reason" required maxlength="200" placeholder="例如：调查石门刻痕" /></label>
        <div class="game-form-row">
          <label>骰式<input v-model="form.formula" required maxlength="30" spellcheck="false" placeholder="1d20+5" :aria-invalid="!spec" aria-describedby="dice-formula-help" /></label>
          <label v-if="isOwner">可见范围<select v-model="form.visibility" aria-label="可见范围"><option value="public">所有战役成员</option><option value="dm">仅主持秘密</option></select></label>
        </div>
        <div class="dice-presets" aria-label="常用骰式"><button v-for="item in presets" :key="item.value" class="game-button" type="button" :aria-pressed="form.formula.replace(/[+-]\d+$/, '') === item.value" @click="preset(item.value)">{{ item.label }}</button></div>
        <p id="dice-formula-help" class="game-helper" :class="{ 'dice-invalid': !spec }">{{ !spec ? '骰式示例：1d20+5、2d6+3、2d20kh1+4。最多 50 枚，面数 2–1000，修正不超过 ±10000。' : '可直接输入骰式，如 1d20+5；优势取高，劣势取低。' }}</p>
      </fieldset>
    </div>
    <footer class="dice-actions">
      <template v-if="revealed"><button class="game-button" type="button" @click="again">再掷一次</button><button class="primary" type="button" @click="emit('close')">完成<Icon name="check" :size="18" /></button></template>
      <button v-else ref="submitButton" class="primary wide" type="submit" :disabled="!spec" :aria-disabled="locked">{{ locked ? phase === 'waiting' ? '等待服务器…' : '正在掷骰…' : phase === 'error' ? '重试本次掷骰' : '掷骰并记录' }}<Icon name="dice" :size="18" /></button>
    </footer>
  </form>
</template>
