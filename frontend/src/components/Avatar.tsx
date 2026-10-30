import { useState } from 'react'
import { initials } from '../lib/format'
import type { Profile } from '../types'

interface Props {
  profile: Pick<Profile, 'first_name' | 'last_name' | 'avatar_url'>
  size?: 'sm' | 'lg'
  className?: string
}

const sizes = {
  sm: 'h-10 w-10 text-sm',
  lg: 'h-28 w-28 text-3xl sm:h-32 sm:w-32',
}

/** The photo, or the employee's initials on a tinted disc when there is none. */
export function Avatar({ profile, size = 'lg', className = '' }: Props) {
  const [failed, setFailed] = useState(false)
  const label = initials(profile)
  const classes = `${sizes[size]} ${className} rounded-full object-cover ring-2 ring-white shadow-sm`

  if (profile.avatar_url && !failed) {
    return (
      <img
        src={profile.avatar_url}
        alt={`${profile.first_name} ${profile.last_name}`}
        className={classes}
        onError={() => setFailed(true)}
      />
    )
  }

  return (
    <div
      className={`${classes} flex items-center justify-center bg-brand-100 font-semibold text-brand-700`}
      role="img"
      aria-label={`${profile.first_name} ${profile.last_name}, no profile photo`}
    >
      {label}
    </div>
  )
}
