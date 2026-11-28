import { describe, expect, it } from 'vitest'
import { backText, namesText, personalise, strings, languages, resolveLanguage } from '../i18n'

const en = strings('en')

describe('personalise', () => {
  it('puts in the first name, or drops the placeholder', () => {
    expect(personalise('Hi {name} 👋', 'Jane')).toBe('Hi Jane 👋')
    expect(personalise('Hi {name} 👋')).toBe('Hi 👋')
    expect(personalise('Hello {name}!')).toBe('Hello!')
    expect(personalise('Hi! How can we help?', 'Jane')).toBe('Hi! How can we help?')
    expect(personalise(undefined)).toBe('')
  })
})

describe('backText', () => {
  const now = new Date(2026, 10, 10, 18, 0)
  it('says a time for later today', () => {
    expect(backText(en, new Date(2026, 10, 10, 20, 0).toISOString(), now, 'en-GB')).toBe("We're away. Back at 20:00")
  })
  it('names the weekday within the week', () => {
    expect(backText(en, new Date(2026, 10, 11, 9, 0).toISOString(), now, 'en-GB')).toBe("We're away. Back Wednesday at 09:00")
  })
  it('falls back when the date is unreadable', () => {
    expect(backText(en, 'not a date', now)).toBe(en.away)
  })
})

describe('namesText', () => {
  it('lists the agents', () => {
    expect(namesText(en, ['Jane', 'Bob'])).toBe('Jane, Bob and the team')
  })
})

describe('languages', () => {
  it('lets data-lang win, then the widget setting, then the browser', () => {
    expect(resolveLanguage('de', 'fr', ['es'])).toBe('de')
    expect(resolveLanguage(undefined, 'fr', ['es'])).toBe('fr')
    expect(resolveLanguage(undefined, 'auto', ['ja', 'pt-BR', 'en'])).toBe('pt')
    expect(resolveLanguage('xx', 'auto', ['ja'])).toBe('en')
    expect(resolveLanguage(undefined, undefined, ['ln-CD'])).toBe('ln')
  })

  it('keeps every placeholder in every language', () => {
    for (const code of languages) {
      const t = strings(code)
      expect(t.lang).toBe(code)
      for (const [key, text] of Object.entries(strings('en'))) {
        const want = (text.match(/\{\w+\}/g) ?? []).sort()
        const got = ((t as Record<string, string>)[key].match(/\{\w+\}/g) ?? []).sort()
        expect(got, `${code}.${key}`).toEqual(want)
        expect((t as Record<string, string>)[key].trim(), `${code}.${key}`).not.toBe('')
      }
    }
  })
})
