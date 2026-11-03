// Workspaces, forms, OAuth providers, templates and stylesheets share one
// naming scheme: name is the identifier the API and URLs use, like
// "contact-form"; display_name is the label people read. These mirror the
// server's rules in internal/models/identifier.go.

export const MAX_IDENTIFIER_LENGTH = 63

const IDENTIFIER = /^[a-z0-9]+(?:-[a-z0-9]+)*$/

/** Derives an identifier from a label: "Contact form" becomes "contact-form". */
export function toIdentifier(label: string): string {
  let v = label.trim().toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '')
  if (v.length > MAX_IDENTIFIER_LENGTH) v = v.slice(0, MAX_IDENTIFIER_LENGTH).replace(/-+$/, '')
  return v
}

/** Whether name can be an identifier. */
export function isIdentifier(name: string): boolean {
  return name.length <= MAX_IDENTIFIER_LENGTH && IDENTIFIER.test(name)
}

export const IDENTIFIER_HINT = 'Lowercase letters, digits and hyphens.'
