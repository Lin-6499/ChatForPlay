<script setup lang="ts">
import type { CheckEffect } from './check-types'
const props = defineProps<{ effect: CheckEffect; label: string }>()
const fields = { conditions: '状态', inventory: '物品', resources: '法术与资源' } as const
function toggle(field: keyof typeof fields, enabled: boolean) {
  if (enabled) props.effect[field] = ''
  else delete props.effect[field]
}
</script>
<template>
  <fieldset class="check-effect-fields">
    <legend>{{ label }}</legend>
    <label>后果说明<textarea v-model="effect.description" :aria-label="`${label}后果说明`" required rows="2" maxlength="1000"></textarea></label>
    <label>生命值变化<input v-model.number="effect.hp_delta" :aria-label="`${label}生命值变化`" type="number" min="-9999" max="9999" required /><small>0 不变，负数扣除，正数恢复。</small></label>
    <details><summary>修改状态、物品或资源</summary>
      <div v-for="(name, field) in fields" :key="field">
        <label class="game-check"><input type="checkbox" :checked="effect[field] !== undefined" @change="toggle(field, ($event.target as HTMLInputElement).checked)" />替换{{ name }}</label>
        <label v-if="effect[field] !== undefined">{{ name }}完整新值<textarea v-model="effect[field]" :aria-label="`${label}${name}完整新值`" rows="2" maxlength="6000"></textarea></label>
      </div>
      <p class="game-helper">勾选后保存完整新值，留空表示清空；未勾选的字段保持原值。</p>
    </details>
  </fieldset>
</template>
