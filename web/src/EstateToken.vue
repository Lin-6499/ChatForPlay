<script setup lang="ts">
defineProps<{ token: number }>()
</script>
<template>
  <svg class="estate-token" :class="`estate-token-${token}`" viewBox="0 0 24 24" aria-hidden="true">
    <circle cx="12" cy="12" r="10" />
    <text x="12" y="12" text-anchor="middle" dominant-baseline="central">{{ token + 1 }}</text>
  </svg>
</template>
