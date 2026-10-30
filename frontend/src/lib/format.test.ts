import { describe, expect, it } from 'vitest'
import {
  absoluteTime,
  displayValue,
  fieldLabel,
  fullName,
  initials,
  relativeTime,
  sameSet,
} from './format'

describe('fieldLabel', () => {
  it('maps every audit field to a human label', () => {
    expect(fieldLabel('avatar')).toBe('Profile photo')
    expect(fieldLabel('mobile')).toBe('Mobile number')
    expect(fieldLabel('email')).toBe('Email')
    expect(fieldLabel('address')).toBe('Address')
    expect(fieldLabel('skills')).toBe('Skills')
  })
})

describe('displayValue', () => {
  it('stands in for blanks with an em dash', () => {
    expect(displayValue(null)).toBe('—')
    expect(displayValue('')).toBe('—')
    expect(displayValue('   ')).toBe('—')
  })

  it('passes real values through untouched', () => {
    expect(displayValue('0812345678')).toBe('0812345678')
  })
})

describe('initials', () => {
  it('takes the first letter of each name', () => {
    expect(initials({ first_name: 'Somchai', last_name: 'Jaidee' })).toBe('SJ')
  })

  it('handles a missing surname', () => {
    expect(initials({ first_name: 'Somchai', last_name: '' })).toBe('S')
  })

  it('falls back to a question mark when there is no name at all', () => {
    expect(initials({ first_name: '', last_name: '' })).toBe('?')
    expect(initials({ first_name: '  ', last_name: '  ' })).toBe('?')
  })
})

describe('fullName', () => {
  it('joins the parts that exist', () => {
    expect(fullName({ first_name: 'Somchai', last_name: 'Jaidee' })).toBe('Somchai Jaidee')
    expect(fullName({ first_name: 'Somchai', last_name: '' })).toBe('Somchai')
  })
})

describe('relativeTime', () => {
  const now = new Date('2026-10-30T12:00:00Z')

  it.each([
    ['2026-10-30T11:59:40Z', 'just now'],
    ['2026-10-30T11:59:00Z', '1 minute ago'],
    ['2026-10-30T11:30:00Z', '30 minutes ago'],
    ['2026-10-30T09:00:00Z', '3 hours ago'],
    ['2026-10-29T12:00:00Z', '1 day ago'],
    ['2026-10-24T12:00:00Z', '6 days ago'],
    ['2026-09-20T12:00:00Z', '1 month ago'],
    ['2024-10-30T12:00:00Z', '2 years ago'],
  ])('renders %s as %s', (iso, expected) => {
    expect(relativeTime(iso, now)).toBe(expected)
  })

  it('treats a future timestamp as just now rather than a negative duration', () => {
    expect(relativeTime('2026-10-30T13:00:00Z', now)).toBe('just now')
  })

  it('returns an empty string for an unparseable date', () => {
    expect(relativeTime('not-a-date', now)).toBe('')
  })
})

describe('absoluteTime', () => {
  it('formats a valid timestamp', () => {
    expect(absoluteTime('2026-10-30T12:00:00Z')).not.toBe('')
  })

  it('returns an empty string for junk', () => {
    expect(absoluteTime('nope')).toBe('')
  })
})

describe('sameSet', () => {
  it('ignores order', () => {
    expect(sameSet(['a', 'b'], ['b', 'a'])).toBe(true)
  })

  it('notices an addition or a removal', () => {
    expect(sameSet(['a'], ['a', 'b'])).toBe(false)
    expect(sameSet(['a', 'b'], ['a'])).toBe(false)
    expect(sameSet(['a'], ['b'])).toBe(false)
  })

  it('treats two empty lists as equal', () => {
    expect(sameSet([], [])).toBe(true)
  })
})
