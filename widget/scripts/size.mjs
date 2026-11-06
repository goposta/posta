// Fails the build when the widget grows past its budget. Every visitor of a
// customer's site downloads it, so it stays small on purpose.
import { readFileSync } from 'node:fs'
import { gzipSync } from 'node:zlib'

const BUDGET = 35 * 1024
const file = new URL('../../internal/web/widget/posta-chat.js', import.meta.url)
const size = gzipSync(readFileSync(file)).length
const kb = (n) => (n / 1024).toFixed(1) + ' KB'
if (size > BUDGET) {
  console.error(`posta-chat.js is ${kb(size)} gzipped, over the ${kb(BUDGET)} budget`)
  process.exit(1)
}
console.log(`posta-chat.js: ${kb(size)} gzipped (budget ${kb(BUDGET)})`)
