const fs = require('node:fs')
const path = require('node:path')
const crypto = require('node:crypto')
const { load } = require('js-yaml')
const { validateUpdate } = require('../desktop/updates.cjs')

async function verifyUpdateManifest(directory, version) {
  const info = load(fs.readFileSync(path.join(directory, 'latest.yml'), 'utf8'))
  if (info.version !== version) throw Error('Automatic update feed version does not match this release.')
  const file = validateUpdate(info, '0.0.0')
  const digest = crypto.createHash('sha512')
  const installer = path.join(directory, file.url)
  for await (const chunk of fs.createReadStream(installer)) digest.update(chunk)
  if (fs.statSync(installer).size !== file.size || digest.digest('base64') !== file.sha512 ||
      info.path !== file.url || info.sha512 !== file.sha512) {
    throw Error('Automatic update feed does not match the final installer bytes.')
  }
  if (!fs.statSync(installer + '.blockmap').size) throw Error('Installer blockmap is missing.')
  return info
}
module.exports = { verifyUpdateManifest }
