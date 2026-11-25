import {
  APPEARANCE_PRESETS, DEFAULT_APPEARANCE, appearanceTokens,
  getAppearance, normalizeColor, onAppearanceChange, saveAppearance,
} from './theme.mjs'

let editorID = 0
export function mountAppearance(container) {
  const id = `appearance-${++editorID}`
  let draft = getAppearance(), dirty = false, saving = false, disposed = false
  container.classList.add('appearance-editor')
  container.innerHTML = `
    <div class="appearance-scroll">
    <p class="appearance-description">让这个空间，更像你。选择一组配色，或调出自己的颜色。</p>
    <div class="appearance-preview" aria-label="配色效果预览">
      <div class="appearance-preview-header"><span class="appearance-preview-dot"></span><b>公共大厅</b><span>配色预览</span></div>
      <div class="appearance-preview-messages"><p>每一句话，都有自己的空间。</p><p class="appearance-preview-own">用喜欢的颜色，继续对话。</p></div>
      <div class="appearance-preview-composer"><span>发送消息…</span><span class="appearance-preview-send">发送</span></div>
    </div>
    <fieldset class="appearance-fieldset"><legend>主题预设</legend><div class="appearance-presets"></div></fieldset>
    <fieldset class="appearance-fieldset appearance-colors"><legend>自定义配色</legend></fieldset>
    <p class="appearance-help">文字与状态色会自动适配，保持清晰易读。配色保存在此设备。</p>
    </div><footer class="appearance-footer">
    <p class="appearance-status" role="status" aria-live="polite"></p>
    <div class="appearance-actions"><button type="button" class="appearance-reset">恢复默认</button><button type="button" class="appearance-apply">应用配色</button></div></footer>`
  const preview = container.querySelector('.appearance-preview')
  const status = container.querySelector('.appearance-status')
  const apply = container.querySelector('.appearance-apply')
  const fields = new Map()
  const presetButtons = new Map()
  for (const preset of APPEARANCE_PRESETS) {
    const button = document.createElement('button')
    button.type = 'button'; button.className = 'appearance-preset'
    const swatch = document.createElement('span')
    swatch.className = 'appearance-swatch'; swatch.setAttribute('aria-hidden', 'true')
    for (const key of ['background', 'panel', 'accent']) {
      const stripe = document.createElement('i')
      stripe.style.backgroundColor = preset[key]; swatch.append(stripe)
    }
    const label = document.createElement('span'); label.textContent = preset.name
    button.append(swatch, label)
    button.addEventListener('click', () => { draft = { ...preset }; dirty = true; render(true) })
    presetButtons.set(preset, button); container.querySelector('.appearance-presets').append(button)
  }
  for (const [key, label] of [['accent', '强调色'], ['background', '背景色'], ['panel', '面板色']]) {
    const row = document.createElement('div'); row.className = 'appearance-color-row'
    const name = document.createElement('label'); name.htmlFor = `${id}-${key}`; name.textContent = label
    const control = document.createElement('div'); control.className = 'appearance-color-control'
    const picker = document.createElement('input')
    picker.type = 'color'; picker.id = `${id}-${key}`; picker.setAttribute('aria-label', label)
    const hex = document.createElement('input')
    hex.type = 'text'; hex.maxLength = 7; hex.spellcheck = false; hex.autocomplete = 'off'
    hex.setAttribute('aria-label', `${label} HEX`); hex.setAttribute('aria-describedby', `${id}-error`)
    hex.addEventListener('input', () => {
      dirty = true
      const value = normalizeColor(hex.value)
      hex.setAttribute('aria-invalid', String(!value))
      if (value) { draft[key] = value; picker.value = value }
      render(false)
    })
    hex.addEventListener('blur', () => { const value = normalizeColor(hex.value); if (value) hex.value = value.toUpperCase() })
    picker.addEventListener('input', () => { draft[key] = picker.value; dirty = true; render(true) })
    control.append(picker, hex); row.append(name, control)
    fields.set(key, { picker, hex }); container.querySelector('.appearance-colors').append(row)
  }
  status.id = `${id}-error`
  const invalid = () => [...fields.values()].some(({ hex }) => !normalizeColor(hex.value))
  function render(syncFields) {
    if (disposed) return
    if (syncFields) for (const [key, { picker, hex }] of fields) {
      picker.value = draft[key]; hex.value = draft[key].toUpperCase(); hex.removeAttribute('aria-invalid')
    }
    for (const [key, value] of Object.entries(appearanceTokens(draft))) preview.style.setProperty(key, value)
    for (const [preset, button] of presetButtons) button.setAttribute('aria-pressed',
      String(!invalid() && ['accent', 'background', 'panel'].every(key => preset[key] === draft[key])))
    const bad = invalid()
    status.textContent = bad ? '请输入有效的 HEX 色值，例如 #A8C7B5。' : dirty ? '预览已更新，点击「应用配色」保存。' : '当前配色已应用。'
    status.classList.toggle('is-error', bad)
    apply.disabled = bad || saving || !dirty
    apply.textContent = saving ? '保存中…' : '应用配色'
  }
  container.querySelector('.appearance-reset').addEventListener('click', () => {
    draft = { ...DEFAULT_APPEARANCE }; dirty = true; render(true)
  })
  apply.addEventListener('click', async () => {
    if (invalid() || saving) return
    saving = true
    const submitted = { ...draft }
    render(false)
    try {
      await saveAppearance(submitted)
      if (disposed) return
      dirty = ['accent', 'background', 'panel'].some(key => draft[key] !== submitted[key]) || invalid()
      saving = false; render(false)
      if (!dirty) status.textContent = '配色已保存，下次打开依然生效。'
    } catch (error) {
      if (disposed) return
      saving = false; render(false)
      status.textContent = error.message || '配色保存失败，请重试。'; status.classList.add('is-error')
    }
  })
  const off = onAppearanceChange(value => {
    if (!dirty) { draft = value; render(true) }
  })
  render(true)
  return () => { disposed = true; off(); container.replaceChildren() }
}
