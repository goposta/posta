// Mirrors the identifier rules in internal/models/identifier.go, to show the identifier before the resource exists.

export const MAX_IDENTIFIER_LENGTH = 63

/** Derives an identifier from a label: "Contact form" becomes "contact-form". */
export function toIdentifier(label: string): string {
  let v = label.trim().toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '')
  if (v.length > MAX_IDENTIFIER_LENGTH) v = v.slice(0, MAX_IDENTIFIER_LENGTH).replace(/-+$/, '')
  return v
}
