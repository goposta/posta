// The widget keeps one localStorage entry per widget key: the visitor token
// and the open conversation. It sets no cookies. Storage can be missing or
// throw (private mode, blocked site data); the widget then works for the
// page's lifetime only.

export interface Saved {
  token?: string
  conversation?: string
}

const keyFor = (widgetKey: string) => `posta_chat_${widgetKey}`

export function load(widgetKey: string): Saved {
  try {
    const raw = window.localStorage.getItem(keyFor(widgetKey))
    const parsed = raw ? JSON.parse(raw) : {}
    return typeof parsed === 'object' && parsed ? parsed : {}
  } catch {
    return {}
  }
}

export function save(widgetKey: string, value: Saved): void {
  try {
    window.localStorage.setItem(keyFor(widgetKey), JSON.stringify(value))
  } catch {
    // Storage unavailable: keep going in memory.
  }
}
