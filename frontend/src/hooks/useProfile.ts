import { useCallback, useEffect, useState } from 'react'
import { api } from '../api/client'
import type { Profile, Skill } from '../types'

interface UseProfile {
  profile: Profile | null
  skills: Skill[]
  loading: boolean
  error: string | null
  reload: () => Promise<void>
  updateContact: (patch: Partial<Record<'mobile' | 'email' | 'address', string>>) => Promise<void>
  uploadAvatar: (file: File) => Promise<void>
  setSkills: (ids: string[], custom: string[]) => Promise<void>
}

/**
 * Owns the server state for the profile page. Mutations return the refreshed profile,
 * so nothing here is optimistic: the server's normalised values win.
 */
export function useProfile(initial: Profile | null, onChanged?: () => void): UseProfile {
  const [profile, setProfile] = useState<Profile | null>(initial)
  const [skills, setSkills] = useState<Skill[]>([])
  const [loading, setLoading] = useState(initial === null)
  const [error, setError] = useState<string | null>(null)

  const reload = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const [p, catalog] = await Promise.all([api.profile(), api.skills()])
      setProfile(p)
      setSkills(catalog)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Could not load your profile')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void reload()
  }, [reload])

  // Each mutation lets the caller handle the error, so a field can show it inline.
  const apply = useCallback(
    async (fn: () => Promise<Profile>) => {
      const updated = await fn()
      setProfile(updated)
      onChanged?.()
    },
    [onChanged],
  )

  return {
    profile,
    skills,
    loading,
    error,
    reload,
    updateContact: (patch) => apply(() => api.updateContact(patch)),
    uploadAvatar: (file) => apply(() => api.uploadAvatar(file)),
    setSkills: (ids, custom) => apply(() => api.setSkills(ids, custom)),
  }
}
