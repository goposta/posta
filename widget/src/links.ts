// Home links come from the workspace, so their scheme is checked again here before they reach the customer's page.

const SAFE = /^(https?:\/\/|mailto:|tel:)/i

export function safeHref(url?: string): string | null {
  const u = (url || '').trim()
  return SAFE.test(u) ? u : null
}

export function sameOrigin(url: string, origin = location.origin): boolean {
  try {
    return new URL(url).origin === origin
  } catch {
    return false
  }
}

// opensNewTab keeps same-origin pages in the tab, so the widget stays open across the navigation through its stored token.
export function opensNewTab(url: string, newTab: boolean, origin = location.origin): boolean {
  if (!/^https?:/i.test(url)) return false
  return newTab && !sameOrigin(url, origin)
}

function luminance(hex: string): number {
  const c = [1, 3, 5].map((i) => {
    const v = parseInt(hex.slice(i, i + 2), 16) / 255
    return v <= 0.03928 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4)
  })
  return 0.2126 * c[0] + 0.7152 * c[1] + 0.0722 * c[2]
}

// Chosen by WCAG contrast ratio, so the header stays readable whatever colour the workspace picks.
export function contrastText(hex: string): string {
  const l = luminance(hex)
  const onWhite = 1.05 / (l + 0.05)
  const onDark = (l + 0.05) / (luminance('#111827') + 0.05)
  return onWhite >= onDark ? '#ffffff' : '#111827'
}
