import { useCallback, useEffect, useState } from 'react'
import { Link, Navigate, NavLink, Route, Routes, useLocation } from 'react-router-dom'
import { api } from '../api/client'
import { useAuth } from '../auth/AuthContext'
import { RetentionSetting } from '../components/RetentionSetting'
import { SkillCatalog } from '../components/SkillCatalog'
import { UserManagement } from '../components/UserManagement'
import type { Skill, SkillInput } from '../types'

const menu = [
  { label: 'Dashboard', path: '/admin', icon: 'home' },
  { label: 'Users', path: '/admin/users', icon: 'users' },
  { label: 'Skills', path: '/admin/skills', icon: 'skills' },
  { label: 'History retention', path: '/admin/settings', icon: 'history' },
] as const

function MenuIcon({ name }: { name: (typeof menu)[number]['icon'] }) {
  const common = { fill: 'none', stroke: 'currentColor', strokeWidth: 1.8, strokeLinecap: 'round' as const, strokeLinejoin: 'round' as const }
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24" className="h-5 w-5 shrink-0" {...common}>
      {name === 'home' && <><path d="m3 10 9-7 9 7" /><path d="M5 9v12h14V9M9 21v-7h6v7" /></>}
      {name === 'users' && <><circle cx="9" cy="8" r="3" /><path d="M3 20v-1a6 6 0 0 1 12 0v1H3ZM16 5.4a3 3 0 0 1 0 5.2M18 14a5 5 0 0 1 3 4.6v1h-3" /></>}
      {name === 'skills' && <><path d="m12 3 9 5-9 5-9-5 9-5Z" /><path d="m3 12 9 5 9-5M3 16l9 5 9-5" /></>}
      {name === 'history' && <><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2M3 5v5h5" /></>}
    </svg>
  )
}

function AdminSidebar({
  collapsed,
  mobileOpen,
  onClose,
}: {
  collapsed: boolean
  mobileOpen: boolean
  onClose: () => void
}) {
  return (
    <>
      {mobileOpen && (
        <button
          type="button"
          aria-label="Close navigation"
          className="fixed inset-0 z-30 bg-slate-950/50 lg:hidden"
          onClick={onClose}
        />
      )}
      <aside
        className={`fixed inset-y-0 left-0 z-40 flex w-64 flex-col border-r border-slate-200 bg-white text-slate-600 shadow-xl transition-all duration-200 lg:translate-x-0 ${
          collapsed ? 'lg:w-[76px]' : 'lg:w-64'
        } ${mobileOpen ? 'translate-x-0' : '-translate-x-full'}`}
      >
        <Link to="/admin" className="flex h-[72px] items-center gap-3 border-b border-slate-200 px-5" onClick={onClose}>
          <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-indigo-500 text-sm font-bold text-white">EP</span>
          <span className={`overflow-hidden whitespace-nowrap text-sm font-semibold tracking-wide text-slate-900 ${collapsed ? 'lg:hidden' : ''}`}>
            Employee Portal
          </span>
        </Link>

        <p className={`px-5 pb-2 pt-7 text-[10px] font-bold uppercase tracking-[0.18em] text-slate-400 ${collapsed ? 'lg:hidden' : ''}`}>
          Workspace
        </p>
        <nav aria-label="Admin navigation" className="space-y-1 px-3">
          {menu.map((item) => (
            <NavLink
              key={item.path}
              to={item.path}
              end={item.path === '/admin'}
              title={collapsed ? item.label : undefined}
              onClick={onClose}
              className={({ isActive }) =>
                `flex min-h-11 items-center gap-3 rounded-xl px-3 text-sm font-medium transition ${
                  isActive ? 'bg-indigo-50 text-indigo-700 ring-1 ring-inset ring-indigo-100' : 'text-slate-600 hover:bg-slate-50 hover:text-slate-900'
                }`
              }
            >
              <MenuIcon name={item.icon} />
              <span className={collapsed ? 'lg:hidden' : ''}>{item.label}</span>
              {item.path === '/admin' && <span className={`ml-auto h-1.5 w-1.5 rounded-full bg-indigo-500 ${collapsed ? 'lg:hidden' : ''}`} />}
            </NavLink>
          ))}
        </nav>

        <div className="mt-auto border-t border-slate-200 p-4">
          <div className={`rounded-xl bg-slate-50 p-3 ${collapsed ? 'lg:hidden' : ''}`}>
            <p className="text-xs font-medium text-slate-800">Admin workspace</p>
            <p className="mt-1 text-[11px] leading-4 text-slate-500">Manage accounts and portal settings.</p>
          </div>
        </div>
      </aside>
    </>
  )
}

export function AdminPage() {
  const { email, logout } = useAuth()
  const location = useLocation()
  const [collapsed, setCollapsed] = useState(false)
  const [mobileOpen, setMobileOpen] = useState(false)
  const current = menu.find((item) => item.path === location.pathname)?.label ?? 'Dashboard'

  useEffect(() => setMobileOpen(false), [location.pathname])

  return (
    <div className="min-h-screen bg-slate-100">
      <AdminSidebar collapsed={collapsed} mobileOpen={mobileOpen} onClose={() => setMobileOpen(false)} />
      <div className={`min-h-screen transition-all duration-200 ${collapsed ? 'lg:pl-[76px]' : 'lg:pl-64'}`}>
        <header className="sticky top-0 z-20 flex h-[72px] items-center justify-between border-b border-slate-200 bg-white/95 px-4 backdrop-blur sm:px-6 lg:px-8">
          <div className="flex min-w-0 items-center gap-3">
            <button
              type="button"
              aria-label="Open navigation"
              onClick={() => setMobileOpen(true)}
              className="flex h-10 w-10 items-center justify-center rounded-lg border border-slate-200 text-slate-600 hover:bg-slate-50 lg:hidden"
            >
              <span aria-hidden="true" className="text-xl leading-none">☰</span>
            </button>
            <button
              type="button"
              aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
              onClick={() => setCollapsed((value) => !value)}
              className="hidden h-10 w-10 items-center justify-center rounded-lg text-slate-500 hover:bg-slate-100 lg:flex"
            >
              <span aria-hidden="true" className="text-lg">☰</span>
            </button>
            <div className="min-w-0">
              <p className="text-xs text-slate-400">Admin / <span className="text-slate-500">{current}</span></p>
              <h1 className="truncate text-sm font-semibold text-slate-900 sm:text-base">{current}</h1>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <span className="hidden max-w-56 truncate text-sm text-slate-500 md:inline">{email}</span>
            <button
              type="button"
              onClick={logout}
              className="h-10 rounded-lg border border-slate-200 px-3 text-sm font-medium text-slate-600 hover:bg-slate-50"
            >
              Sign out
            </button>
          </div>
        </header>

        <main className="mx-auto w-full max-w-[1440px] p-4 sm:p-6 lg:p-8">
          <Routes>
            <Route index element={<DashboardPanel />} />
            <Route path="users" element={<UsersPanel />} />
            <Route path="skills" element={<SkillsPanel />} />
            <Route path="settings" element={<SettingsPanel />} />
            <Route path="*" element={<Navigate to="/admin" replace />} />
          </Routes>
        </main>
      </div>
    </div>
  )
}

function DashboardPanel() {
  return (
    <section className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm sm:p-8">
      <p className="text-xs font-semibold uppercase tracking-[0.16em] text-indigo-600">Overview</p>
      <h2 className="mt-2 text-2xl font-bold tracking-tight text-slate-900">Welcome to your admin dashboard</h2>
      <p className="mt-2 max-w-xl text-sm leading-6 text-slate-500">
        Your workspace is ready. Choose a section from the sidebar to manage employee accounts, skills, or history settings.
      </p>
      <div className="mt-7 grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        {menu.slice(1).map((item) => (
          <Link key={item.path} to={item.path} className="rounded-xl border border-slate-200 p-4 transition hover:border-indigo-200 hover:bg-indigo-50/40">
            <span className="text-sm font-semibold text-slate-900">{item.label}</span>
            <p className="mt-1 text-xs text-slate-500">Open {item.label.toLowerCase()} management</p>
          </Link>
        ))}
      </div>
    </section>
  )
}

function SectionHeading({ title, description }: { title: string; description: string }) {
  return (
    <div className="mb-6">
      <h2 className="text-xl font-bold tracking-tight text-slate-900">{title}</h2>
      <p className="mt-1 text-sm text-slate-500">{description}</p>
    </div>
  )
}

function LoadError({ message, onRetry }: { message: string; onRetry: () => void }) {
  return (
    <div className="rounded-xl border border-red-100 bg-red-50 p-4">
      <p role="alert" className="text-sm text-red-700">{message}</p>
      <button type="button" onClick={onRetry} className="mt-3 h-10 rounded-lg bg-white px-4 text-sm font-semibold text-red-700 shadow-sm">Try again</button>
    </div>
  )
}

function SkillsPanel() {
  const [skills, setSkills] = useState<Skill[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const load = useCallback(async () => {
    setLoading(true); setError(null)
    try { setSkills(await api.admin.skills()) }
    catch (e) { setError(e instanceof Error ? e.message : 'Could not load skills') }
    finally { setLoading(false) }
  }, [])
  useEffect(() => { void load() }, [load])
  const create = useCallback(async (input: SkillInput) => {
    await api.admin.createSkill(input)
    setSkills(await api.admin.skills())
  }, [])
  const update = useCallback(async (id: string, input: SkillInput) => {
    await api.admin.updateSkill(id, input)
    setSkills(await api.admin.skills())
  }, [])

  return <>
    <SectionHeading title="Skills" description="Maintain the catalog employees can choose from." />
    {error ? <LoadError message={error} onRetry={() => void load()} /> : loading ? <p className="rounded-xl bg-white p-6 text-sm text-slate-500">Loading skills…</p> : (
      <section className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm sm:p-6">
        <p className="mb-4 text-sm text-slate-500">{skills.filter((s) => s.active).length} of {skills.length} skills are active.</p>
        <SkillCatalog skills={skills} onCreate={create} onUpdate={update} />
      </section>
    )}
  </>
}

function SettingsPanel() {
  const [days, setDays] = useState(7)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const load = useCallback(async () => {
    setLoading(true); setError(null)
    try { setDays((await api.admin.settings()).history_retention_days) }
    catch (e) { setError(e instanceof Error ? e.message : 'Could not load settings') }
    finally { setLoading(false) }
  }, [])
  useEffect(() => { void load() }, [load])
  const saveDays = useCallback(async (value: number) => {
    setDays((await api.admin.updateSettings({ history_retention_days: value })).history_retention_days)
  }, [])

  return <>
    <SectionHeading title="History retention" description="Set how far back employees can view their profile change history." />
    {error ? <LoadError message={error} onRetry={() => void load()} /> : loading ? <p className="rounded-xl bg-white p-6 text-sm text-slate-500">Loading settings…</p> : (
      <section className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm sm:p-6">
        <RetentionSetting days={days} onSave={saveDays} />
      </section>
    )}
  </>
}

function UsersPanel() {
  return <>
    <SectionHeading title="Users" description="Create, update, and remove portal accounts." />
    <UserManagement />
  </>
}
