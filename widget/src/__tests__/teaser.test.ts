import { describe, expect, it } from 'vitest'
import { pathMatches } from '../teaser'
import { launcherSheet } from '../styles'

describe('pathMatches', () => {
  it('shows everywhere without patterns', () => {
    expect(pathMatches([], '/anything')).toBe(true)
    expect(pathMatches(undefined, '/')).toBe(true)
  })
  it('matches exact paths, ignoring a trailing slash', () => {
    expect(pathMatches(['/pricing'], '/pricing')).toBe(true)
    expect(pathMatches(['/pricing'], '/pricing/')).toBe(true)
    expect(pathMatches(['/pricing'], '/pricing/teams')).toBe(false)
    expect(pathMatches(['/'], '/')).toBe(true)
    expect(pathMatches(['/'], '/about')).toBe(false)
  })
  it('matches prefixes ending with *', () => {
    expect(pathMatches(['/docs/*'], '/docs/api')).toBe(true)
    expect(pathMatches(['/docs/*'], '/blog')).toBe(false)
  })
})

describe('launcherSheet', () => {
  it('moves the widget only for valid offsets', () => {
    expect(launcherSheet({ offset_x: 40, offset_y: 90 })).toBe('.pc:not(.preview){bottom:90px;right:40px}')
    expect(launcherSheet({ offset_x: 40 }, true)).toBe('.pc:not(.preview){bottom:20px;left:40px}')
    expect(launcherSheet({ offset_x: 9999 })).toBe('')
    expect(launcherSheet(undefined)).toBe('')
  })
})
