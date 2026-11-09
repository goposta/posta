// Storage can be missing or throw (private mode, blocked site data); the widget then works for the page's lifetime only.

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

export type View = 'home' | 'messages' | 'help' | 'thread'

// The last view lives in sessionStorage only, so a new visit starts on Home.
export function loadView(widgetKey: string): View | null {
  try {
    const v = window.sessionStorage.getItem(`${keyFor(widgetKey)}_view`)
    return v === 'home' || v === 'messages' || v === 'help' || v === 'thread' ? v : null
  } catch {
    return null
  }
}

export function saveView(widgetKey: string, view: View): void {
  try {
    window.sessionStorage.setItem(`${keyFor(widgetKey)}_view`, view)
  } catch {
    // Storage unavailable: the view resets on the next page.
  }
}
