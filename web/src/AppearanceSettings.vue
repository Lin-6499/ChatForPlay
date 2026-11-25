<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { mountAppearance } from '../../desktop/appearance.mjs'

const editor = ref<HTMLElement>()
let dispose: (() => void) | undefined
onMounted(() => { if (editor.value) dispose = mountAppearance(editor.value) })
onBeforeUnmount(() => dispose?.())
</script>

<template><div ref="editor"></div></template>
