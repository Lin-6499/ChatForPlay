// Shared by the web workspace and connection center. Only coordinates and
// opaque item IDs are stored; no form values or message text leave the page.
const positions = new Map()
const prefix = 'linchat-scroll-v1:'
export function getScrollPosition(key) {
  if (positions.has(key)) return positions.get(key)
  try {
    const value = JSON.parse(localStorage.getItem(prefix + key) || 'null')
    if (value && ['x', 'y', 'offset'].every(k => Number.isFinite(value[k])) &&
        value.x >= 0 && value.y >= 0 && typeof value.bottom === 'boolean' && typeof value.anchor === 'string') {
      positions.set(key, value)
      return value
    }
  } catch {}
}

export function startScrollMemory(root = document) {
  const bindings = new Map()
  let frame = 0
  const visible = el => el.isConnected && el.clientHeight > 0 && el.getClientRects().length > 0
  function persist(state) {
    if (!state.position) return
    positions.set(state.key, state.position)
    try { localStorage.setItem(prefix + state.key, JSON.stringify(state.position)) } catch {}
  }
  function capture(el, state) {
    if (!visible(el) || state.owner.dataset.scrollReady === 'false') return
    const top = el.getBoundingClientRect().top
    const rows = [...el.querySelectorAll('[data-scroll-anchor]')].filter(row => row.closest('[data-scroll-key]') === el)
    const anchor = rows.find(row => row.getBoundingClientRect().bottom > top)
    state.position = {
      x: el.scrollLeft, y: el.scrollTop,
      anchor: anchor?.dataset.scrollAnchor || '', offset: anchor ? anchor.getBoundingClientRect().top - top : 0,
      bottom: state.owner.hasAttribute('data-scroll-follow') && el.scrollHeight - el.scrollTop - el.clientHeight <= 24,
    }
    persist(state)
  }
  function restore(el, state) {
    if (!visible(el) || state.owner.dataset.scrollReady === 'false') {
      state.restored = false
      return
    }
    const position = state.position
    if (position) {
      const anchor = position.anchor && [...el.querySelectorAll('[data-scroll-anchor]')].find(row => row.dataset.scrollAnchor === position.anchor)
      el.scrollLeft = position.x
      el.scrollTop = position.bottom ? el.scrollHeight : anchor
        ? el.scrollTop + anchor.getBoundingClientRect().top - el.getBoundingClientRect().top - position.offset
        : position.y
    } else if (state.owner.hasAttribute('data-scroll-follow')) el.scrollTop = el.scrollHeight
    state.x = el.scrollLeft; state.y = el.scrollTop
    state.restored = true
    // Don't replace a saved anchor while asynchronous content is still arriving.
    if (!position) capture(el, state)
  }
  const schedule = () => { if (!frame) frame = requestAnimationFrame(scan) }
  const resize = new ResizeObserver(schedule)
  function scan() {
    frame = 0
    const found = new Map()
    for (const owner of root.querySelectorAll('[data-scroll-key]')) {
      const key = owner.dataset.scrollKey
      if (!key) continue
      found.set(owner, { key, owner, ownerKey: key })
      // Dialogs have independently scrolling pickers and appearance editors.
      for (const selector of ['.member-picker', '.game-member-picker', '.appearance-scroll', '.details-panel', '.game-tabs', '.game-scene']) {
        for (const [index, child] of [...owner.querySelectorAll(selector)].entries()) {
          // The nearest annotated surface owns nested scroll areas.
          if (child.closest('[data-scroll-key]') !== owner) continue
          found.set(child, { key: `${key}/${selector}/${index}`, owner, ownerKey: key })
        }
      }
    }
    for (const [el, state] of bindings) {
      if (found.get(el)?.key === state.key) continue
      persist(state); el.removeEventListener('scroll', state.scroll)
      for (const child of state.observed) resize.unobserve(child)
      bindings.delete(el)
    }
    for (const [el, config] of found) {
      let state = bindings.get(el)
      if (!state) {
        state = { ...config, position: getScrollPosition(config.key), x: -1, y: -1, restored: false, observed: new Set() }
        state.scroll = () => {
          if (!visible(el) || state.owner.dataset.scrollReady === 'false' || state.owner.dataset.scrollKey !== state.ownerKey) return
          // A remounted/previously hidden panel can emit scroll(0) before its
          // first restoration frame. That is layout, not a new reading position.
          if (!state.restored) return
          if (Math.abs(el.scrollTop - state.y) < 1 && Math.abs(el.scrollLeft - state.x) < 1) return
          capture(el, state); state.x = el.scrollLeft; state.y = el.scrollTop
        }
        bindings.set(el, state)
        el.addEventListener('scroll', state.scroll, { passive: true })
      }
      restore(el, state)
      const children = new Set([el, ...el.children])
      for (const child of state.observed) if (!children.has(child)) resize.unobserve(child)
      for (const child of children) if (!state.observed.has(child)) resize.observe(child)
      state.observed = children
    }
  }
  const observer = new MutationObserver(schedule)
  observer.observe(root, { childList: true, subtree: true, attributes: true,
    attributeFilter: ['data-scroll-key', 'data-scroll-ready', 'class', 'style', 'hidden', 'open'] })
  const flush = () => { for (const state of bindings.values()) persist(state) }
  const visibility = () => { if (document.hidden) flush(); else schedule() }
  window.addEventListener('pagehide', flush)
  document.addEventListener('visibilitychange', visibility)
  schedule()
  return () => {
    flush(); observer.disconnect(); resize.disconnect(); cancelAnimationFrame(frame)
    for (const [el, state] of bindings) el.removeEventListener('scroll', state.scroll)
    window.removeEventListener('pagehide', flush)
    document.removeEventListener('visibilitychange', visibility)
  }
}
