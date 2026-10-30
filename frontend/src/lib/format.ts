import type { ChangeField, Profile } from '../types'

/** Human label for an audit field. */
export function fieldLabel(field: ChangeField): string {
  const labels: Record<ChangeField, string> = {
    avatar: 'Profile photo',
    mobile: 'Mobile number',
    email: 'Email',
    address: 'Address',
    skills: 'Skills',
  }
  return labels[field] ?? field
}

/** Renders an audit value, standing in for a blank with an em dash. */
export function displayValue(value: string | null): string {
  if (value === null || value.trim() === '') return '—'
  return value
}

export function initials(profile: Pick<Profile, 'first_name' | 'last_name'>): string {
  const first = profile.first_name?.trim()?.[0] ?? ''
  const last = profile.last_name?.trim()?.[0] ?? ''
  return (first + last).toUpperCase() || '?'
}

export function fullName(profile: Pick<Profile, 'first_name' | 'last_name'>): string {
  return [profile.first_name, profile.last_name].filter(Boolean).join(' ')
}

/**
 * A short relative time such as "3 hours ago". `now` is injectable so the tests do
 * not depend on the wall clock.
 */
export function relativeTime(iso: string, now: Date = new Date()): string {
  const then = new Date(iso)
  if (Number.isNaN(then.getTime())) return ''

  const seconds = Math.round((now.getTime() - then.getTime()) / 1000)
  if (seconds < 0) return 'just now'
  if (seconds < 60) return 'just now'

  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes} minute${minutes === 1 ? '' : 's'} ago`

  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours} hour${hours === 1 ? '' : 's'} ago`

  const days = Math.floor(hours / 24)
  if (days < 30) return `${days} day${days === 1 ? '' : 's'} ago`

  const months = Math.floor(days / 30)
  if (months < 12) return `${months} month${months === 1 ? '' : 's'} ago`

  const years = Math.floor(months / 12)
  return `${years} year${years === 1 ? '' : 's'} ago`
}

export function absoluteTime(iso: string): string {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return ''
  return date.toLocaleString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

/** True when two id lists hold the same set, ignoring order. */
export function sameSet(a: string[], b: string[]): boolean {
  if (a.length !== b.length) return false
  const set = new Set(a)
  return b.every((id) => set.has(id))
}

export const MISSING_LABELS: Record<string, string> = {
  avatar: 'Add a profile photo',
  mobile: 'Add your mobile number',
  email: 'Add your email',
  address: 'Add your address',
  skills: 'Select at least one skill',
}
