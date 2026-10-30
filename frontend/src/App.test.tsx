import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { App } from './App'
import { AuthProvider } from './auth/AuthContext'
import type { Profile, Role } from './types'

const profile: Profile = {
  id: 'user-1',
  first_name: 'Somchai',
  last_name: 'Jaidee',
  email: 'somchai@example.com',
  mobile: '0812345678',
  address: null,
  avatar_url: null,
  role: 'employee',
  skills: [],
  completeness: { percent: 40, missing: ['avatar', 'address', 'skills'] },
  updated_at: new Date().toISOString(),
}

/**
 * Routes fetches by path so the whole app can be mounted. `role` decides what
 * /auth/me reports, which is how a restored session picks its landing page.
 */
function mockApi(role: Role | null) {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string) => {
      const json = (status: number, body: unknown) =>
        ({ status, ok: status < 300, text: async () => JSON.stringify(body) }) as Response

      if (url.includes('/auth/me')) {
        if (!role) return json(401, { error: 'invalid or expired token' })
        return json(200, { user_id: 'u', email: `${role}@example.com`, role })
      }
      if (url.includes('/admin/skills')) return json(200, { skills: [] })
      if (url.includes('/admin/settings')) return json(200, { history_retention_days: 7 })
      if (url.includes('/me/history')) return json(200, { days: 7, events: [] })
      if (url.endsWith('/skills')) return json(200, { skills: [] })
      if (url.includes('/me')) return json(200, profile)
      return json(404, { error: 'not found' })
    }),
  )
}

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <AuthProvider>
        <App />
      </AuthProvider>
    </MemoryRouter>,
  )
}

describe('routing and role separation', () => {
  beforeEach(() => {
    sessionStorage.clear()
  })

  it('sends an anonymous visitor to the login page', async () => {
    mockApi(null)
    renderAt('/')
    expect(await screen.findByRole('button', { name: 'Sign in' })).toBeInTheDocument()
  })

  it('sends an anonymous visitor away from /admin', async () => {
    mockApi(null)
    renderAt('/admin')
    expect(await screen.findByRole('button', { name: 'Sign in' })).toBeInTheDocument()
    expect(screen.queryByText('System configuration')).not.toBeInTheDocument()
  })

  it('shows the profile page to an employee', async () => {
    sessionStorage.setItem('employee-portal.token', 'tok')
    mockApi('employee')
    renderAt('/')
    expect(await screen.findByText('Contact information')).toBeInTheDocument()
  })

  it('redirects an employee away from the admin console', async () => {
    sessionStorage.setItem('employee-portal.token', 'tok')
    mockApi('employee')
    renderAt('/admin')

    expect(await screen.findByText('Contact information')).toBeInTheDocument()
    expect(screen.queryByText('System configuration')).not.toBeInTheDocument()
  })

  it('shows the admin console to an administrator', async () => {
    sessionStorage.setItem('employee-portal.token', 'tok')
    mockApi('admin')
    renderAt('/admin')
    expect(await screen.findByText('System configuration')).toBeInTheDocument()
  })

  it('redirects an administrator away from the employee profile', async () => {
    sessionStorage.setItem('employee-portal.token', 'tok')
    mockApi('admin')
    renderAt('/')

    expect(await screen.findByText('System configuration')).toBeInTheDocument()
    // An admin must never be shown profile UI.
    expect(screen.queryByText('Contact information')).not.toBeInTheDocument()
  })

  it('never requests the employee profile for an administrator', async () => {
    sessionStorage.setItem('employee-portal.token', 'tok')
    mockApi('admin')
    renderAt('/admin')
    await screen.findByText('System configuration')

    const calls = (fetch as unknown as ReturnType<typeof vi.fn>).mock.calls.map(
      (c) => c[0] as string,
    )
    // /auth/me is token introspection, which both roles use; it is not the profile.
    const profileCalls = calls.filter(
      (u) => !u.includes('/auth/me') && (/\/me(\?|$)/.test(u) || u.includes('/me/')),
    )
    expect(profileCalls).toEqual([])
  })

  it('sends an unknown path to the role home', async () => {
    sessionStorage.setItem('employee-portal.token', 'tok')
    mockApi('admin')
    renderAt('/nope')
    expect(await screen.findByText('System configuration')).toBeInTheDocument()
  })

  it('drops a session whose token the server rejects', async () => {
    sessionStorage.setItem('employee-portal.token', 'stale')
    mockApi(null)
    renderAt('/admin')

    expect(await screen.findByRole('button', { name: 'Sign in' })).toBeInTheDocument()
    await waitFor(() => expect(sessionStorage.getItem('employee-portal.token')).toBeNull())
  })
})
