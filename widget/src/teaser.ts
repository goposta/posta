// A pattern is an exact path, or a prefix when it ends with *; no patterns means every page.
export function pathMatches(patterns: string[] | undefined, path: string): boolean {
  if (!patterns?.length) return true
  const clean = (p: string) => (p.length > 1 ? p.replace(/\/+$/, '') : p)
  return patterns.some((p) => (p.endsWith('*') ? path.startsWith(p.slice(0, -1)) : clean(path) === clean(p)))
}

const keyFor = (widgetKey: string) => `posta_chat_${widgetKey}_teaser`

// The dismissal remembers the text, so a new teaser shows again.
export function dismissed(widgetKey: string, text: string): boolean {
  try {
    return window.localStorage.getItem(keyFor(widgetKey)) === text
  } catch {
    return false
  }
}

export function dismiss(widgetKey: string, text: string): void {
  try {
    window.localStorage.setItem(keyFor(widgetKey), text)
  } catch {
    // Storage unavailable: it shows again on the next page.
  }
}
