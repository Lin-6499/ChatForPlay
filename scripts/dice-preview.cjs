// Loopback-only, disposable campaign preview. Used for headless motion review;
// authentication is injected only into this fixture's isolated backend.
const fs = require('node:fs')
const path = require('node:path')
const { fixture } = require('./entertainment-fixture.cjs')
const root = path.resolve(__dirname, '..')
async function preview(port = 5187) {
  const f = await fixture()
  // The harness closes stdin; keep Vite alive until explicit fixture cleanup.
  process.env.CI = 'true'
  const { createServer } = await import('../web/node_modules/vite/dist/node/index.js')
  let server
  try {
    server = await createServer({
      configFile: path.join(root, 'web/vite.config.ts'), root: path.join(root, 'web'),
      plugins: [{
        name: 'isolated-motion-capture-inputs',
        transformIndexHtml: html => ({ html, tags: [{ tag: 'script', injectTo: 'body', children: `
          if (location.search.includes('motion-demo') || location.search.includes('dice-shot')) {
            let opened = false;
            const observer = new MutationObserver(() => {
              if (location.search.includes('dice-shot') && !opened) {
                const trigger = document.querySelector('.game-compose-controls > .text-button');
                if (trigger) { opened = true; trigger.click(); }
              }
              const fields = document.querySelectorAll('.dice-fields input');
              if (fields.length !== 2) return;
              fields[0].value = '调查石门刻痕'; fields[1].value = '2d20kh1+4';
              for (const input of fields) input.dispatchEvent(new Event('input', { bubbles: true }));
              observer.disconnect();
            });
            observer.observe(document.body, { childList: true, subtree: true });
          }
        ` }] }),
      }],
      server: { host: '127.0.0.1', port, strictPort: true, proxy: {
        '/api': { target: f.url, ws: true, changeOrigin: true, headers: { Origin: f.url, Cookie: f.owner.cookie } },
      } },
    })
    await server.listen()
  } catch (error) { await server?.close(); await f.stop(); throw error }
  return { ...f, preview: `http://127.0.0.1:${server.httpServer.address().port}/#campaign=${f.cid}`, stop: async () => { try { await server.close() } finally { await f.stop() } } }
}
if (require.main === module) preview().then(f => {
  const out = path.join(root, 'dist/verification/dice')
  fs.mkdirSync(out, { recursive: true })
  fs.writeFileSync(path.join(out, 'preview.json'), JSON.stringify({ url: f.preview, fixture: f.url, pid: process.pid }))
  console.log(`Headless review fixture ready: ${f.preview}`)
  let closing = false
  const stop = async () => { if (closing) return; closing = true; await f.stop(); process.exit() }
  process.on('SIGINT', stop); process.on('SIGTERM', stop)
  process.stdin.on('data', text => { if (text.toString().trim() === 'shutdown') void stop() })
}).catch(error => { console.error(error); process.exitCode = 1 })
module.exports = { preview }
