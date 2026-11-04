// Workspaces, forms, OAuth providers, templates and stylesheets share one
// naming scheme: name is the identifier the API and URLs use, like
// "contact-form"; display_name is the label people read. The server derives
// the identifier from the name people type; toIdentifier mirrors its rules
// in internal/models/identifier.go, to show it before the resource exists.

export const MAX_IDENTIFIER_LENGTH = 63

/** Derives an identifier from a label: "Contact form" becomes "contact-form". */
export function toIdentifier(label: string): string {
  let v = label.trim().toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '')
  if (v.length > MAX_IDENTIFIER_LENGTH) v = v.slice(0, MAX_IDENTIFIER_LENGTH).replace(/-+$/, '')
  return v
}
