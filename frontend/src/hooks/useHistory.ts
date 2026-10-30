import { useCallback, useEffect, useState } from 'react'
import { api } from '../api/client'
import type { ProfileChange } from '../types'

export const PAGE_SIZE = 5

interface UseHistory {
  events: ProfileChange[]
  /** The window the server applied. Read-only here: an administrator sets it. */
  days: number
  page: number
  totalPages: number
  total: number
  setPage: (page: number) => void
  loading: boolean
  error: string | null
  reload: () => Promise<void>
}

export function useHistory(): UseHistory {
  const [days, setDays] = useState(0)
  const [page, setPage] = useState(1)
  const [events, setEvents] = useState<ProfileChange[]>([])
  const [total, setTotal] = useState(0)
  const [totalPages, setTotalPages] = useState(1)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async (which: number) => {
    setLoading(true)
    setError(null)
    try {
      const result = await api.history(which, PAGE_SIZE)
      setEvents(result.events)
      setTotal(result.total)
      setTotalPages(result.total_pages)
      // The window comes from the server's configuration, not from this client.
      setDays(result.days)
      setPage(result.page)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Could not load your change history')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load(page)
  }, [page, load])

  return {
    events,
    days,
    page,
    total,
    totalPages,
    setPage,
    loading,
    error,
    reload: () => load(page),
  }
}
