// Lightweight helpers to read/write operator tokens inside a single search
// string (e.g. `from:alice subject:"weekly report" has:attachment after:2026-01-01`).
// The string stays the single source of truth — the backend does the real
// parsing — so UI controls only nudge tokens in and out of it.

function tokenize(q: string): string[] {
  const tokens: string[] = []
  let cur = ''
  let inQuote = false
  for (const ch of q) {
    if (ch === '"') {
      inQuote = !inQuote
      continue
    }
    if (/\s/.test(ch) && !inQuote) {
      if (cur) {
        tokens.push(cur)
        cur = ''
      }
      continue
    }
    cur += ch
  }
  if (cur) tokens.push(cur)
  return tokens
}

function serialize(tokens: string[]): string {
  return tokens
    .map((t) => {
      const i = t.indexOf(':')
      if (i > 0) {
        const key = t.slice(0, i)
        const val = t.slice(i + 1)
        return /\s/.test(val) ? `${key}:"${val}"` : t
      }
      return /\s/.test(t) ? `"${t}"` : t
    })
    .join(' ')
}

/** Returns the value of the first `key:value` token, or '' when absent. */
export function getToken(q: string, key: string): string {
  const k = key.toLowerCase()
  for (const t of tokenize(q)) {
    const i = t.indexOf(':')
    if (i > 0 && t.slice(0, i).toLowerCase() === k) return t.slice(i + 1)
  }
  return ''
}

/** Sets/replaces a `key:value` token; an empty value removes it. */
export function setToken(q: string, key: string, value: string): string {
  const k = key.toLowerCase()
  const out: string[] = []
  let done = false
  for (const t of tokenize(q)) {
    const i = t.indexOf(':')
    if (i > 0 && t.slice(0, i).toLowerCase() === k) {
      if (!done && value) {
        out.push(`${key}:${value}`)
        done = true
      }
      continue
    }
    out.push(t)
  }
  if (!done && value) out.push(`${key}:${value}`)
  return serialize(out)
}

/** True when the exact flag token (e.g. `has:attachment`) is present. */
export function hasFlag(q: string, flag: string): boolean {
  const f = flag.toLowerCase()
  return tokenize(q).some((t) => t.toLowerCase() === f)
}

/** Adds or removes a standalone flag token. */
export function toggleFlag(q: string, flag: string, on: boolean): string {
  const f = flag.toLowerCase()
  const tokens = tokenize(q).filter((t) => t.toLowerCase() !== f)
  if (on) tokens.push(flag)
  return serialize(tokens)
}
