<script setup lang="ts">
import type { EstateView } from './estate-types'
import { nextTick, onBeforeUnmount, onDeactivated, ref, watch } from 'vue'
import EstateToken from './EstateToken.vue'
const props = defineProps<{ view: EstateView; selected: number }>()
const emit = defineEmits<{ select: [id: number] }>()
function occupants(id: number) {
  const names = props.view.game.players.filter(p => !p.bankrupt && p.position === id).map(p => p.name)
  return names.length ? `，当前停留：${names.join('、')}` : '，当前无人停留'
}
function landlord(id: number) { return props.view.game.players.find(p => p.id === props.view.game.properties[id].owner) }
const positions = ref<Record<number, number>>({})
const board = ref<HTMLElement>()
let timers: ReturnType<typeof setTimeout>[] = []
const animations = new Set<Animation>()
let previousID = 0, previousRoll = 0, generation = 0
const hopDuration = 300, diceDuration = 800
function stop() {
  generation++
  timers.forEach(clearTimeout); timers = []
  animations.forEach(a => a.cancel()); animations.clear()
}
async function hop(player: number, tile: number, gen: number) {
  const selector = `[data-pawn="${player}"]`
  const before = board.value?.querySelector(selector)?.getBoundingClientRect()
  positions.value[player] = tile
  await nextTick()
  if (gen !== generation || !before || matchMedia('(prefers-reduced-motion: reduce)').matches) return
  const pawn = board.value?.querySelector<HTMLElement>(selector)
  if (!pawn) return
  const after = pawn.getBoundingClientRect(), x = before.x - after.x, y = before.y - after.y
  const animation = pawn.animate([
    { transform: `translate(${x}px, ${y}px)`, offset: 0 },
    { transform: `translate(${x * .5}px, ${y * .5 - 14}px) scale(1.08)`, offset: .45 },
    { transform: 'translate(0, 0) scale(1.06, .94)', offset: .86 },
    { transform: 'translate(0, 0) scale(1)', offset: 1 },
  ], { duration: hopDuration, easing: 'cubic-bezier(.22,.61,.36,1)' })
  animations.add(animation)
  animation.onfinish = () => animations.delete(animation)
}
// Polls, receipts and chat updates often replace the same game object. Only a
// new roll or an actual position change may interrupt an in-flight journey.
watch(() => JSON.stringify([
  props.view.game.id,
  props.view.game.events.filter(e => e.text.includes('掷出')).at(-1)?.seq || 0,
  props.view.game.players.map(p => [p.id, p.position, p.bankrupt]),
]), () => {
  const g = props.view.game
  stop()
  const roll = g.events.filter(e => e.text.includes('掷出')).at(-1)?.seq || 0
  const animate = g.id === previousID && roll > previousRoll && !matchMedia('(prefers-reduced-motion: reduce)').matches
  const gen = generation
  for (const p of g.players) {
    const old = positions.value[p.id], steps = g.dice[0] + g.dice[1]
    if (animate && old !== undefined && old !== p.position && (old + steps) % 24 === p.position) {
      for (let step = 1; step <= steps; step++) timers.push(setTimeout(() => {
        void hop(p.id, (old + step) % 24, gen)
      }, diceDuration + (step - 1) * hopDuration))
    } else positions.value[p.id] = p.position
  }
  previousID = g.id; previousRoll = roll
}, { immediate: true })
onBeforeUnmount(stop)
onDeactivated(() => { stop(); for (const p of props.view.game.players) positions.value[p.id] = p.position })
function place(id: number) {
  if (id < 7) return { gridColumn: id + 1, gridRow: 1 }
  if (id < 13) return { gridColumn: 7, gridRow: id - 5 }
  if (id < 19) return { gridColumn: 19 - id, gridRow: 7 }
  return { gridColumn: 1, gridRow: 25 - id }
}
function arrow(event: KeyboardEvent, id: number) {
  if (!['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', 'Home', 'End'].includes(event.key)) return
  event.preventDefault()
  const next = event.key === 'Home' ? 0 : event.key === 'End' ? 23 : (id + (['ArrowLeft', 'ArrowUp'].includes(event.key) ? 23 : 1)) % 24
  const button = (event.currentTarget as HTMLElement).parentElement?.querySelector<HTMLButtonElement>(`[data-tile="${next}"]`)
  button?.focus(); emit('select', next)
}
</script>
<template>
  <div ref="board" class="estate-board" aria-label="环城商业街棋盘">
    <button v-for="t in view.tiles" :key="t.id" class="estate-cell" :class="[{ selected: selected === t.id, mortgaged: view.game.properties[t.id].mortgaged, 'estate-landing': view.game.phase !== 'lobby' && view.game.phase !== 'finished' && view.game.players[view.game.current]?.position === t.id }, `estate-group-${t.group}`]"
      :style="place(t.id)" :data-tile="t.id" :aria-pressed="selected === t.id"
      :aria-label="`${t.id}号 ${t.name}${t.price ? `，价格${t.price}，租金${view.rents[t.id]}` : ''}，${view.game.properties[t.id].owner ? view.game.players.find(p => p.id === view.game.properties[t.id].owner)?.name + '拥有' : '银行或公共区域'}${view.game.properties[t.id].mortgaged ? '，已抵押' : ''}${occupants(t.id)}`"
      @click="emit('select', t.id)" @keydown="arrow($event, t.id)">
      <span class="estate-cell-info">
      <span class="estate-cell-meta">{{ t.group }}{{ String(t.id).padStart(2, '0') }}<span v-if="view.game.properties[t.id].owner">{{ view.game.properties[t.id].mortgaged ? '抵押' : `L${view.game.properties[t.id].level}` }}</span></span>
      <strong>{{ t.name }}</strong>
      <small v-if="t.price">{{ view.game.properties[t.id].owner ? `租 ${view.rents[t.id]}` : t.price }}</small>
      <small v-else>{{ t.kind === 'start' ? '+300' : t.kind === 'chance' ? '抽取事件' : t.kind === 'tax' ? '缴纳费用' : t.kind === 'market' ? '免租券' : '公共区域' }}</small>
      <span v-if="landlord(t.id)" class="estate-cell-owner" :class="`estate-owner-${landlord(t.id)?.token}`" :title="`地主：${landlord(t.id)?.name}`">
        地主：{{ (landlord(t.id)?.token || 0) + 1 }}号
      </span>
      </span>
      <span class="estate-cell-pawns" aria-label="棋子停靠区">
        <span v-for="p in view.game.players.filter(p => !p.bankrupt && positions[p.id] === t.id)" :key="p.id" class="estate-pawn" :data-pawn="p.id">
          <EstateToken :token="p.token" :title="`${p.token + 1}号 ${p.name}当前停留`" />
        </span>
      </span>
    </button>
    <div class="estate-board-center"><slot /></div>
  </div>
</template>
