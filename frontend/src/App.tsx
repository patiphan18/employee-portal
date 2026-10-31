import { Navigate, Route, Routes } from 'react-router-dom'
import { useAuth } from './auth/AuthContext'
import { LoginPage } from './pages/LoginPage'
import { ProfilePage } from './pages/ProfilePage'
import { AdminPage } from './pages/AdminPage'
import type { Role } from './types'

function FullPageMessage({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-screen items-center justify-center bg-slate-50 px-4">
      <p className="text-sm text-slate-500">{children}</p>
    </div>
  )
}

/** Where a signed-in session belongs. The two roles have disjoint surfaces. */
function homeFor(role: Role | null): string {
  return role === 'admin' ? '/admin' : '/'
}

/**
 * Guards a route by role. An unauthenticated visitor goes to the login page; a
 * signed-in user who lands on the other role's route is sent to their own home
 * rather than shown an error, because arriving here is a wrong turn, not a failure.
 */
function RequireRole({ allow, children }: { allow: Role; children: React.ReactNode }) {
  const { token, role, ready } = useAuth()

  if (!ready) return <FullPageMessage>Loading…</FullPageMessage>
  if (!token) return <Navigate to="/login" replace />
  // The role arrives with the session; until it does, keep the spinner up.
  if (role === null) return <FullPageMessage>Loading…</FullPageMessage>
  if (role !== allow) return <Navigate to={homeFor(role)} replace />

  return <>{children}</>
}

export function App() {
  const { token, role, ready } = useAuth()

  // Hold the first paint until a stored session has been checked, so a reload does
  // not flash the login screen or route to the wrong home.
  if (!ready) return <FullPageMessage>Loading…</FullPageMessage>

  return (
    <Routes>
      <Route
        path="/login"
        element={token ? <Navigate to={homeFor(role)} replace /> : <LoginPage />}
      />
      <Route
        path="/"
        element={
          <RequireRole allow="employee">
            <ProfilePage />
          </RequireRole>
        }
      />
      <Route
        path="/admin/*"
        element={
          <RequireRole allow="admin">
            <AdminPage />
          </RequireRole>
        }
      />
      {/* Unknown paths go to the right home rather than a dead end. */}
      <Route path="*" element={<Navigate to={token ? homeFor(role) : '/login'} replace />} />
    </Routes>
  )
}
