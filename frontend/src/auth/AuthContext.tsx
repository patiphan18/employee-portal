import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react'
import { api, configureClient } from '../api/client'
import type { Profile, Role } from '../types'

const TOKEN_KEY = 'employee-portal.token'

interface AuthState {
  token: string | null
  /** Null until the session has been identified. */
  role: Role | null
  email: string | null
  /** Always null for administrators, who have no employee profile. */
  profile: Profile | null
  ready: boolean
  login: (email: string, password: string) => Promise<Role>
  logout: () => void
  setProfile: (p: Profile) => void
}

const AuthContext = createContext<AuthState | null>(null)

/** Reads the stored token. sessionStorage can throw in locked-down browsers. */
function readStoredToken(): string | null {
  try {
    return sessionStorage.getItem(TOKEN_KEY)
  } catch {
    return null
  }
}

function writeStoredToken(token: string | null) {
  try {
    if (token) sessionStorage.setItem(TOKEN_KEY, token)
    else sessionStorage.removeItem(TOKEN_KEY)
  } catch {
    // A session that cannot be persisted still works for this tab.
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(readStoredToken)
  const [role, setRole] = useState<Role | null>(null)
  const [email, setEmail] = useState<string | null>(null)
  const [profile, setProfile] = useState<Profile | null>(null)
  const [ready, setReady] = useState(false)

  const logout = useCallback(() => {
    setToken(null)
    setRole(null)
    setEmail(null)
    setProfile(null)
    writeStoredToken(null)
  }, [])

  // The client needs a live view of the token, so it reads through a getter rather
  // than capturing the value at mount.
  useEffect(() => {
    configureClient(() => readStoredToken(), logout)
  }, [logout])

  // Restore a session left in sessionStorage by a page reload. The role decides
  // where the router sends the user, so it has to be known before the first paint.
  useEffect(() => {
    let cancelled = false
    if (!token) {
      setReady(true)
      return
    }
    api
      .tokenInfo()
      .then(async (info) => {
        if (cancelled) return
        setRole(info.role)
        setEmail(info.email)
        // Only employees have a profile to load.
        if (info.role === 'employee') {
          const p = await api.profile()
          if (!cancelled) setProfile(p)
        }
      })
      .catch(() => {
        // An expired token has already triggered logout inside the client.
      })
      .finally(() => {
        if (!cancelled) setReady(true)
      })
    return () => {
      cancelled = true
    }
    // Runs once on mount: later token changes come from login, which sets everything.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const login = useCallback(async (loginEmail: string, password: string): Promise<Role> => {
    const result = await api.login(loginEmail, password)
    writeStoredToken(result.token)
    setToken(result.token)
    setRole(result.role)
    setEmail(result.email)
    setProfile(result.profile)
    return result.role
  }, [])

  const value = useMemo<AuthState>(
    () => ({ token, role, email, profile, ready, login, logout, setProfile }),
    [token, role, email, profile, ready, login, logout],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used inside an AuthProvider')
  return ctx
}
