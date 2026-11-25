import { createApp } from 'vue'
import App from './App.vue'
import { initializeAppearance } from '../../desktop/theme.mjs'
import '../../desktop/theme.css'
import './style.css'
import { startScrollMemory } from '../../desktop/scroll-memory.mjs'

initializeAppearance(window.linchatDesktop).finally(() => {
  createApp(App).mount('#app')
  startScrollMemory()
})
