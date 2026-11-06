// Only http, https and mailto become links; nothing is ever parsed as HTML.

export type Part = { text: string; href?: string }

const LINK = /\b(https?:\/\/[^\s<>"']+|mailto:[^\s<>"']+)/gi
const TRAILING = /[.,;:!?)\]}]+$/

export function linkify(text: string): Part[] {
  const parts: Part[] = []
  let last = 0
  for (const m of text.matchAll(LINK)) {
    const start = m.index ?? 0
    let url = m[0]
    const trail = url.match(TRAILING)
    if (trail) url = url.slice(0, -trail[0].length)
    if (start > last) parts.push({ text: text.slice(last, start) })
    parts.push(safeHref(url) ? { text: url, href: url } : { text: url })
    last = start + url.length
  }
  if (last < text.length) parts.push({ text: text.slice(last) })
  return parts
}

function safeHref(url: string): boolean {
  try {
    const u = new URL(url)
    return u.protocol === 'http:' || u.protocol === 'https:' || u.protocol === 'mailto:'
  } catch {
    return false
  }
}
