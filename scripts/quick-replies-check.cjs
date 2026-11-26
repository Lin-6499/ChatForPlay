const { chromium } = require('playwright-core')
const { fixture } = require('./entertainment-fixture.cjs')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const path = require('node:path')
const { randomUUID } = require('node:crypto')

async function main() {
  const f = await fixture()
  const executablePath = ['C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe',
    'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe'].find(p => fs.existsSync(p))
  const browser = await chromium.launch({ executablePath, headless: true })
  const out = path.resolve(__dirname, '../dist/verification/quick-replies')
  fs.mkdirSync(out, { recursive: true })
  const results = [], errors = []
  const pass = text => { results.push(text); console.log('PASS', text) }
  try {
    const owner = await browser.newContext({ viewport: { width: 1264, height: 811 } })
    await owner.addCookies([{ name: 'linchat_session', value: f.owner.cookie.split('=')[1], url: f.url }])
    const host = await owner.newPage()
    host.on('pageerror', e => errors.push(e.message))
    await host.goto(f.url + `/#campaign=${f.cid}`)
    await host.getByLabel('战役消息类型').selectOption('narration')
    await host.getByText('添加快捷回复（可选）', { exact: true }).click()
    await host.getByLabel('快捷回复的回复者').selectOption('2')
    await host.getByLabel('快捷回复选项', { exact: true }).fill('接下委托\n先询问报酬\n暂时离开')
    await host.getByLabel('行动或叙述内容').fill('接待员递来一份委托，你准备怎么回复？')
    await host.getByRole('button', { name: '发送', exact: true }).click()
    await host.getByLabel('行动或叙述内容').waitFor()
    await host.waitForFunction(() => document.querySelector('.game-action-input').value === '')
    const prompt = (await f.api(f.route('reply-prompt'), 'GET', undefined, f.player.cookie)).data.prompt
    assert.equal(prompt.target_user_id, 2)
    assert.equal(prompt.options.length, 3)
    pass('Host composer publishes player-targeted choices alongside a public narration')

    const player = await browser.newContext({ viewport: { width: 390, height: 780 } })
    const preload = fs.readFileSync(path.resolve(__dirname, '../desktop/preload.cjs'), 'utf8')
    await player.addInitScript(preload => {
      // Exercise actual desktop titlebar space without launching Electron.
      const electron = {
        contextBridge: { exposeInMainWorld() {} },
        ipcRenderer: {
          on() {},
          invoke: async () => ({ enabled: true, color: '#1b2220', symbolColor: '#edf2ef', fullscreen: false }),
        },
      }
      new Function('require', 'process', preload)(() => electron, { platform: 'win32' })
    }, preload)
    await player.addCookies([{ name: 'linchat_session', value: f.player.cookie.split('=')[1], url: f.url }])
    const page = await player.newPage()
    page.on('pageerror', e => errors.push(e.message))
    await page.goto(f.url + `/#campaign=${f.cid}`)
    const replies = page.getByRole('region', { name: '主持人快捷回复' })
    await replies.getByRole('button', { name: '先询问报酬', exact: true }).click()
    assert.equal(await page.getByLabel('行动或叙述内容').inputValue(), '先询问报酬')
    assert.equal((await f.api(f.route('reply-prompt'), 'GET', undefined, f.player.cookie)).data.prompt.answered, false)
    await page.screenshot({ path: path.join(out, 'mobile-choice.png') })
    await page.getByRole('button', { name: '提交行动', exact: true }).click()
    await page.getByText('你已回复这组问题', { exact: true }).waitFor()
    let log = (await f.api(f.route('events'), 'GET', undefined, f.player.cookie)).data
    assert.equal(log.at(-1).payload.reply_to_event_id, prompt.event_id)
    assert.equal(log.at(-1).payload.option_id, 'choice_2')
    await page.reload()
    await page.getByText('你已回复这组问题', { exact: true }).waitFor()
    pass('Choosing fills the composer without submitting; explicit submission records the option once and survives reload')

    async function publish() {
      await f.api(f.route('messages'), 'POST', { client_id: randomUUID(), version: (await f.state()).version,
        content: '你还有其他打算吗？', reply_prompt: { target_user_id: 2, options: [
          { id: 'a', label: '调查线索', content: '我仔细调查桌面上的线索。' },
          { id: 'b', label: '寻找同伴', content: '我去寻找同伴。' },
        ] } }, f.owner.cookie)
    }
    await publish()
    await page.reload()
    await replies.getByRole('button', { name: '调查线索', exact: true }).click()
    await page.getByLabel('行动或叙述内容').fill('我先询问守夜人，再去调查钟楼。')
    await page.getByRole('button', { name: '提交行动', exact: true }).click()
    await page.getByText('你已回复这组问题', { exact: true }).waitFor()
    log = (await f.api(f.route('events'), 'GET', undefined, f.player.cookie)).data
    assert.equal(log.at(-1).content, '我先询问守夜人，再去调查钟楼。')
    assert.equal(log.at(-1).payload.option_id, '')
    pass('Editing a suggested response submits custom text linked to the same host question')

    await publish()
    await page.reload()
    await replies.getByRole('button', { name: '寻找同伴', exact: true }).click()
    await f.api(f.route('messages'), 'POST', { client_id: randomUUID(), version: (await f.state()).version, content: '场景已经更新，请重新考虑。' }, f.owner.cookie)
    await page.getByText('原问题已更新，请重新选择', { exact: true }).waitFor()
    await page.getByRole('button', { name: '提交行动', exact: true }).click()
    await page.getByRole('alert').filter({ hasText: '这组快捷回复已失效' }).waitFor()
    assert.equal(await page.getByLabel('行动或叙述内容').inputValue(), '我去寻找同伴。')
    pass('Superseded choices retain the draft and show an actionable error instead of answering a different question')

    await publish()
    await page.reload()
    await page.getByLabel('行动或叙述内容').fill('我选择自己的第三条道路。')
    await page.getByRole('button', { name: '提交行动', exact: true }).click()
    await page.getByText('你已回复这组问题', { exact: true }).waitFor()
    pass('Players can reply freely without selecting any suggested button')
    await publish(); await page.reload()
    await replies.getByRole('button', { name: '调查线索', exact: true }).waitFor()
    await page.getByLabel('行动或叙述内容').fill('还未提交的自由回复')
    await publish()
    await page.getByText('原问题已更新，请重新选择', { exact: true }).waitFor()
    await page.getByRole('button', { name: '提交行动', exact: true }).click()
    await page.getByRole('alert').filter({ hasText: '这组快捷回复已失效' }).waitFor()
    pass('A free-text draft is also bound to its original question when the host updates the prompt')
    for (const size of [{ width: 320, height: 640 }, { width: 820, height: 600 }, { width: 1280, height: 850 }]) {
      await publish(); await page.setViewportSize(size); await page.reload()
      await replies.getByRole('button', { name: '调查线索', exact: true }).waitFor()
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true)
      await page.getByLabel('行动或叙述内容').fill('布局验收')
      const box = await page.getByRole('button', { name: '提交行动', exact: true }).boundingBox()
      assert.ok(box.y + box.height <= size.height)
      const journal = await page.locator('.game-journal').boundingBox()
      assert.ok(journal.height >= 150, `Journal needs reading space: ${JSON.stringify({ size, journal })}`)
      const input = await page.getByLabel('行动或叙述内容').boundingBox()
      assert.ok(input.height >= 55 && input.y + input.height <= box.y, 'Whole input remains visible above submit')
      assert.equal(await page.evaluate(() => document.documentElement.scrollHeight <= innerHeight), true)
      await page.screenshot({ path: path.join(out, `layout-${size.width}.png`) })
      await page.getByLabel('行动或叙述内容').fill('这是一段较长的行动描述，用来确认输入增长时仍能阅读正文。\n'.repeat(25))
      const longBox = await page.getByRole('button', { name: '提交行动', exact: true }).boundingBox()
      assert.ok(longBox.y + longBox.height <= size.height)
      assert.ok((await page.locator('.game-journal').boundingBox()).height >= 150)
      await page.locator('.game-scene summary').click()
      const sceneBox = await page.getByRole('button', { name: '提交行动', exact: true }).boundingBox()
      assert.ok(sceneBox.y + sceneBox.height <= size.height, 'Expanded scene keeps submit visible')
      await page.locator('.game-scene summary').click()
      await page.getByLabel('行动或叙述内容').fill('布局验收')
      await page.getByRole('button', { name: '提交行动', exact: true }).click()
      await page.getByText('你已回复这组问题', { exact: true }).waitFor()
      await page.screenshot({ path: path.join(out, `answered-${size.width}.png`) })
    }
    await page.screenshot({ path: path.join(out, 'desktop-choice.png') })
    pass('Mobile, compact and large desktop with titlebar retain readable logs and visible submit through choices, long drafts, expanded scenes and answered state')
    assert.deepEqual(errors, [])
    fs.writeFileSync(path.join(out, 'results.json'), JSON.stringify({ results, errors }, null, 2))
  } finally { await browser.close(); await f.stop() }
}
main().catch(error => { console.error(error); process.exitCode = 1 })
