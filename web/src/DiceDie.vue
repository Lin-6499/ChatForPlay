<script setup lang="ts">
import { computed } from 'vue'
import { projectDie } from './dice-geometry'
const props = withDefaults(defineProps<{ sides: number; progress: number; index?: number; value?: number }>(), { index: 0 })
const faces = computed(() => projectDie(props.sides, props.progress, props.index))
const front = computed(() => faces.value.find(face => face.id === 0))
const label = computed(() => String(props.value ?? `d${props.sides}`))
</script>

<template>
  <div class="dice-object" :class="{ 'dice-small-face': ![4, 6, 8, 12].includes(sides) }" :data-sides="sides" aria-hidden="true">
    <svg class="dice-solid" viewBox="0 0 120 120">
      <polygon v-for="face in faces" :key="face.id" :points="face.points" :class="{ 'dice-front': face.id === 0 }" :style="{ '--face-light': `${face.light}%` }" />
      <text v-if="progress === 1" :x="front?.labelX ?? 60" :y="(front?.labelY ?? 60) + 2" text-anchor="middle" dominant-baseline="middle" :class="{ 'dice-value': value !== undefined, 'dice-wide-value': label.length > 2, 'dice-long-value': label.length > 3, 'dice-extra-long-value': label.length > 4 }">{{ label }}</text>
    </svg>
    <span class="dice-contact"></span>
  </div>
</template>
