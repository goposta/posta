import { describe, expect, it } from 'vitest'
import { contrastText, opensNewTab, safeHref, sameOrigin } from '../links'

describe('safeHref', () => {
  it('allows https, http, mailto and tel only', () => {
    expect(safeHref('https://example.com/faq')).toBe('https://example.com/faq')
    expect(safeHref(' mailto:help@example.com ')).toBe('mailto:help@example.com')
    expect(safeHref('tel:+33123456789')).toBe('tel:+33123456789')
    for (const bad of ['javascript:alert(1)', 'JaVaScRiPt:alert(1)', 'data:text/html,x', '/faq', '//evil.example', '', undefined]) {
      expect(safeHref(bad)).toBeNull()
    }
  })
})

describe('links and tabs', () => {
  const origin = 'https://shop.example'
  it('keeps same-origin pages in the tab', () => {
    expect(sameOrigin('https://shop.example/help', origin)).toBe(true)
    expect(opensNewTab('https://shop.example/help', true, origin)).toBe(false)
  })
  it('opens other sites in a new tab when asked', () => {
    expect(opensNewTab('https://docs.example/faq', true, origin)).toBe(true)
    expect(opensNewTab('https://docs.example/faq', false, origin)).toBe(false)
  })
  it('never opens mailto or tel in a new tab', () => {
    expect(opensNewTab('mailto:a@b.c', true, origin)).toBe(false)
  })
})

describe('contrastText', () => {
  it('picks the readable text colour', () => {
    expect(contrastText('#0f172a')).toBe('#ffffff')
    expect(contrastText('#7c3aed')).toBe('#ffffff')
    expect(contrastText('#fde68a')).toBe('#111827')
    expect(contrastText('#ffffff')).toBe('#111827')
  })
})
