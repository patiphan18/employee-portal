import { useCallback } from 'react'
import { useAuth } from '../auth/AuthContext'
import { AvatarUpload } from '../components/AvatarUpload'
import { CompletenessMeter } from '../components/CompletenessMeter'
import { ContactForm } from '../components/ContactForm'
import { HistoryList } from '../components/HistoryList'
import { SkillSelector } from '../components/SkillSelector'
import { useHistory } from '../hooks/useHistory'
import { useProfile } from '../hooks/useProfile'
import { fullName } from '../lib/format'

function Card({
  title,
  description,
  children,
  className = '',
}: {
  title: string
  description?: string
  children: React.ReactNode
  className?: string
}) {
  return (
    <section
      className={`rounded-2xl border border-slate-200 bg-white p-5 shadow-sm sm:p-6 ${className}`}
    >
      <h2 className="text-base font-semibold text-slate-900">{title}</h2>
      {description && <p className="mt-1 text-sm text-slate-500">{description}</p>}
      <div className="mt-4">{children}</div>
    </section>
  )
}

export function ProfilePage() {
  const { profile: initial, logout } = useAuth()

  const history = useHistory()
  // A profile change can add a history row, so the two views refresh together.
  const onChanged = useCallback(() => {
    void history.reload()
  }, [history])

  const { profile, skills, loading, error, reload, updateContact, uploadAvatar, setSkills } =
    useProfile(initial, onChanged)

  if (loading && !profile) {
    return (
      <main className="flex min-h-screen items-center justify-center bg-slate-50">
        <p className="text-sm text-slate-500">Loading your profile…</p>
      </main>
    )
  }

  if (error && !profile) {
    return (
      <main className="flex min-h-screen items-center justify-center bg-slate-50 px-4">
        <div className="text-center">
          <p role="alert" className="text-sm text-red-600">
            {error}
          </p>
          <button
            type="button"
            onClick={() => void reload()}
            className="mt-4 h-11 rounded-lg bg-brand-600 px-5 text-sm font-semibold text-white"
          >
            Try again
          </button>
        </div>
      </main>
    )
  }

  if (!profile) return null



  return (
    <div className="min-h-screen bg-slate-50">
      <header className="border-b border-slate-200 bg-white">
        <div className="mx-auto flex max-w-7xl items-center justify-between gap-4 px-4 py-4 sm:px-6 lg:px-8">
          <span className="text-base font-semibold text-slate-900">Employee Portal</span>
          <button
            type="button"
            onClick={logout}
            className="rounded-lg px-3 py-2 text-sm font-medium text-slate-600 hover:bg-slate-100"
          >
            Sign out
          </button>
        </div>
      </header>

      {/* Mobile-first: one column by default, two from md, sidebar + content from lg. */}
      <main className="mx-auto max-w-7xl px-4 py-6 sm:px-6 lg:grid lg:grid-cols-[320px_1fr] lg:gap-6 lg:px-8 lg:py-8">
        <div className="lg:sticky lg:top-8 lg:self-start">
          <section className="rounded-2xl border border-slate-200 bg-white p-5 shadow-sm sm:p-6">
            <AvatarUpload
              profile={profile}
              onUpload={uploadAvatar}
            />

            <div className="mt-5 text-center">
              {/* Read-only identity fields (FR-3.1). */}
              <h1 className="text-xl font-bold text-slate-900">{fullName(profile)}</h1>
              <p className="mt-1 text-xs text-slate-400">Name is managed by HR</p>
              {profile.role === 'admin' && (
                <span className="mt-2 inline-block rounded-full bg-brand-50 px-3 py-1 text-xs font-medium text-brand-700">
                  Administrator
                </span>
              )}
            </div>

            <hr className="my-5 border-slate-100" />
            <CompletenessMeter completeness={profile.completeness} />
          </section>
        </div>

        <div className="mt-6 space-y-6 md:grid md:grid-cols-2 md:gap-6 md:space-y-0 lg:mt-0 lg:grid-cols-1">
          <Card
            title="Contact information"
            description="Edit any of these and save them together."
          >
            <ContactForm profile={profile} onSave={updateContact} />
          </Card>

          <Card
            title="Skills"
            description="Pick the skills you work with. If one is missing, type it and add your own."
          >
            <SkillSelector
              catalog={skills}
              selected={profile.skills}
              onSave={setSkills}
            />
          </Card>

          <Card
            title="Change history"
            description="A read-only record of edits to your profile."
            className="md:col-span-2 lg:col-span-1"
          >
            <HistoryList
              events={history.events}
              days={history.days}
              page={history.page}
              total={history.total}
              totalPages={history.totalPages}
              loading={history.loading}
              error={history.error}
              onChangePage={history.setPage}
            />
          </Card>
        </div>
      </main>
    </div>
  )
}
