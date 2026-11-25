const test = require('node:test')
const assert = require('node:assert/strict')
const theme = import('../desktop/theme.mjs')

test('appearance inputs accept hex colors and discard malformed stored values', async () => {
  const { normalizeAppearance, normalizeColor, DEFAULT_APPEARANCE } = await theme
  assert.equal(normalizeColor(' #AbC '), '#aabbcc')
  for (const value of [null, {}, 123, 'red', '#12345g', 'url(https://example.com)', '#000000; color:red']) {
    assert.equal(normalizeColor(value), null)
  }
  assert.deepEqual(normalizeAppearance(null), DEFAULT_APPEARANCE)
  assert.deepEqual(normalizeAppearance({ accent: '#123', background: 'bad', panel: '#FEFEFE', injected: 'value' }), {
    version: 1, accent: '#112233', background: DEFAULT_APPEARANCE.background, panel: '#fefefe',
  })
})

test('custom palettes keep text legible on canvas, panels, state surfaces and navigation', async () => {
  const { appearanceTokens, contrast, APPEARANCE_PRESETS } = await theme
  const colors = ['#000000', '#ffffff', '#777777', '#666666', '#888888', '#ff0000', '#00ff00', '#0000ff', '#ff00ff', '#a8c7b5']
  const samples = [...APPEARANCE_PRESETS]
  for (const background of colors) for (const accent of colors) samples.push({ background, panel: accent, accent })
  for (const sample of samples) {
    const t = appearanceTokens(sample)
    for (const scope of ['', 'panel-', 'rail-']) {
      const surface = t[scope ? `--${scope.slice(0, -1)}` : '--bg']
      const pairs = [
        [t[`--${scope}text`], surface],
        [t[`--${scope}muted`], surface],
        [t[`--${scope}text`], t[`--${scope}field`]],
        [t[`--${scope}muted`], t[`--${scope}field`]],
        [t[`--${scope}raised-text`], t[`--${scope}raised`]],
        [t[`--${scope}muted`], t[`--${scope}raised`]],
        [t[`--${scope}selected-text`], t[`--${scope}selected`]],
        [t[`--${scope}selected-muted`], t[`--${scope}selected`]],
        [t[`--${scope}selected-accent`], t[`--${scope}selected`]],
        [t[`--${scope}danger`], t[`--${scope}danger-bg`]],
        [t[`--${scope}warning`], t[`--${scope}warning-bg`]],
      ]
      for (const [foreground, background] of pairs) assert.ok(contrast(foreground, background) >= 4.5,
        `${JSON.stringify(sample)} ${scope} ${foreground} on ${background}: ${contrast(foreground, background)}`)
    }
    assert.ok(contrast(t['--on-accent'], t['--accent']) >= 4.5)
  }
})
