import { readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'
import { gzipSync } from 'node:zlib'
import { fileURLToPath } from 'node:url'

const limitKB = 150
const outDir = fileURLToPath(new URL('../../staticFiles', import.meta.url))
const html = readFileSync(join(outDir, 'index.html'), 'utf8')
const initial = new Set()
for (const match of html.matchAll(/(?:src|href)="\/(assets\/[^"]+\.js)"/g)) initial.add(match[1])
let total = 0
for (const file of initial) total += gzipSync(readFileSync(join(outDir, file))).length
const kb = total / 1024
const all = readdirSync(join(outDir, 'assets')).filter((name) => name.endsWith('.js')).length
console.log(
  `initial JS: ${kb.toFixed(1)} KB gzip across ${initial.size} files (${all} chunks total, limit ${limitKB} KB)`,
)
if (kb > limitKB) {
  console.error(`initial JS budget exceeded: ${kb.toFixed(1)} KB > ${limitKB} KB`)
  process.exit(1)
}
