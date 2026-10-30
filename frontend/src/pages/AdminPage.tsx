import { useCallback, useEffect, useState } from 'react'
import { api } from '../api/client'
import { useAuth } from '../auth/AuthContext'
import { RetentionSetting } from '../components/RetentionSetting'
import { SkillCatalog } from '../components/SkillCatalog'
import type { Skill, SkillInput } from '../types'

function Card({
  title,
  description,
  children,
}: {
  title: string
  description?: string
  children: React.ReactNode
}) {
  return (
    <section className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm sm:p-6">
      <h2 className="text-base font-semibold text-slate-900">{title}</h2>
      {description && <p className="mt-1 text-sm text-slate-500">{description}</p>}
      <div className="mt-4">{children}</div>
    </section>
  )
}

/**
 * The administrator's console. Administrators configure the system; they have no
 * employee profile, so nothing here touches /me.
 */
export function AdminPage() {
  const { email, logout } = useAuth()
  const [skills, setSkills] = useState<Skill[]>([])
  const [days, setDays] = useState(7)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const [catalog, settings] = await Promise.all([api.admin.skills(), api.admin.settings()])
      setSkills(catalog)
      setDays(settings.history_retention_days)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Could not load the configuration')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
  }, [load])

  // Each mutation lets the child surface its own error, then refreshes the list.
  const createSkill = useCallback(async (input: SkillInput) => {
    await api.admin.createSkill(input)
    setSkills(await api.admin.skills())
  }, [])

  const updateSkill = useCallback(async (id: string, input: SkillInput) => {
    await api.admin.updateSkill(id, input)
    setSkills(await api.admin.skills())
  }, [])

  const saveDays = useCallback(async (value: number) => {
    const saved = await api.admin.updateSettings({ history_retention_days: value })
    setDays(saved.history_retention_days)
  }, [])

  const activeCount = skills.filter((s) => s.active).length

  return (
    <div className="min-h-screen bg-slate-50">
      <header className="border-b border-slate-200 bg-white">
        <div className="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3 px-4 py-4 sm:px-6 lg:px-8">
          <div className="min-w-0">
            <span className="text-base font-semibold text-slate-900">Employee Portal</span>
            <span className="ml-2 rounded-full bg-brand-50 px-2 py-0.5 text-xs font-medium text-brand-700">
              Administration
            </span>
          </div>
          <div className="flex items-center gap-3">
            {email && <span className="hidden text-sm text-slate-500 sm:inline">{email}</span>}
            <button
              type="button"
              onClick={logout}
              className="rounded-lg px-3 py-2 text-sm font-medium text-slate-600 hover:bg-slate-100"
            >
              Sign out
            </button>
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-5xl space-y-6 px-4 py-6 sm:px-6 lg:px-8 lg:py-8">
        <div>
          <h1 className="text-xl font-bold text-slate-900">System configuration</h1>
          <p className="mt-1 text-sm text-slate-500">
            Administrators configure the portal and do not have an employee profile.
          </p>
        </div>

        {error && (
          <div className="rounded-xl bg-red-50 p-4">
            <p role="alert" className="text-sm text-red-700">
              {error}
            </p>
            <button
              type="button"
              onClick={() => void load()}
              className="mt-3 h-11 rounded-lg bg-brand-600 px-5 text-sm font-semibold text-white"
            >
              Try again
            </button>
          </div>
        )}

        {loading ? (
          <p className="text-sm text-slate-500">Loading the configuration…</p>
        ) : (
          <>
            <Card
              title="Skill catalog"
              description={`${activeCount} of ${skills.length} skills are offered to employees. Retiring a skill hides it from the picker but keeps it on the profiles that already have it.`}
            >
              <SkillCatalog skills={skills} onCreate={createSkill} onUpdate={updateSkill} />
            </Card>

            <Card title="History" description="Applies to every employee.">
              <RetentionSetting days={days} onSave={saveDays} />
            </Card>
          </>
        )}
      </main>
    </div>
  )
}
