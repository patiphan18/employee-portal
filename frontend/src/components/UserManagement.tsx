import { useCallback, useEffect, useState, type FormEvent } from 'react'
import { api, ApiError } from '../api/client'
import { useAuth } from '../auth/AuthContext'
import { Modal } from './Modal'
import { Pagination } from './Pagination'
import type { AdminUser, AdminUserInput, AdminUserPage, Role } from '../types'

const blankUser: AdminUserInput = {
  first_name: '', last_name: '', email: '', mobile: '', address: '', role: 'employee', password: '',
}

export function UserManagement() {
  const { userID: actorID } = useAuth()
  const [result, setResult] = useState<AdminUserPage | null>(null)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [adding, setAdding] = useState(false)
  const [editing, setEditing] = useState<AdminUser | null>(null)
  const [deleting, setDeleting] = useState<AdminUser | null>(null)
  const [busy, setBusy] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)

  const load = useCallback(async (targetPage = page) => {
    setLoading(true)
    setError(null)
    try {
      const data = await api.admin.users(targetPage)
      if (targetPage > data.total_pages) {
        setPage(data.total_pages)
        const lastPage = await api.admin.users(data.total_pages)
        setResult(lastPage)
      } else {
        setPage(targetPage)
        setResult(data)
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Could not load users')
    } finally {
      setLoading(false)
    }
  }, [page])

  useEffect(() => { void load(1) }, [])

  async function saveUser(input: AdminUserInput, user?: AdminUser) {
    setBusy(true)
    setActionError(null)
    try {
      if (user) await api.admin.updateUser(user.id, input)
      else await api.admin.createUser(input)
      setAdding(false)
      setEditing(null)
      await load(user ? page : 1)
    } catch (e) {
      throw e
    } finally {
      setBusy(false)
    }
  }

  async function confirmDelete() {
    if (!deleting) return
    setBusy(true)
    setActionError(null)
    try {
      await api.admin.deleteUser(deleting.id)
      setDeleting(null)
      await load(page)
    } catch (e) {
      setActionError(e instanceof Error ? e.message : 'Could not delete user')
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="overflow-hidden rounded-2xl border border-slate-200 bg-white shadow-sm">
      <div className="flex flex-col gap-3 border-b border-slate-100 p-4 sm:flex-row sm:items-center sm:justify-between sm:p-5">
        <div>
          <h3 className="text-sm font-semibold text-slate-900">Portal accounts</h3>
          <p className="mt-1 text-xs text-slate-500">Manage login details, profile fields, and account roles.</p>
        </div>
        <button
          type="button"
          onClick={() => { setActionError(null); setAdding(true) }}
          className="h-11 rounded-lg bg-indigo-600 px-4 text-sm font-semibold text-white shadow-sm transition hover:bg-indigo-700"
        >
          <span aria-hidden="true" className="mr-2">+</span>Add user
        </button>
      </div>

      {error ? (
        <div className="p-5">
          <p role="alert" className="text-sm text-red-700">{error}</p>
          <button type="button" onClick={() => void load()} className="mt-3 h-10 rounded-lg border border-slate-200 px-4 text-sm font-medium text-slate-700">Try again</button>
        </div>
      ) : loading ? (
        <p className="p-6 text-sm text-slate-500">Loading users…</p>
      ) : result?.users.length ? (
        <>
          <div className="overflow-x-auto">
            <table className="w-full min-w-[760px] border-collapse text-left text-sm">
              <thead className="bg-slate-50 text-xs uppercase tracking-wide text-slate-500">
                <tr>
                  <th scope="col" className="px-5 py-3 font-semibold">User</th>
                  <th scope="col" className="px-4 py-3 font-semibold">Email</th>
                  <th scope="col" className="px-4 py-3 font-semibold">Mobile</th>
                  <th scope="col" className="px-4 py-3 font-semibold">Role</th>
                  <th scope="col" className="px-5 py-3 text-right font-semibold">Actions</th>
                </tr>
              </thead>
              <tbody>
                {result.users.map((user) => {
                  const isSelf = user.id === actorID
                  return (
                    <tr key={user.id} className="border-t border-slate-100 align-middle hover:bg-slate-50/70">
                      <td className="px-5 py-3.5">
                        <div className="flex items-center gap-3">
                          <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-indigo-50 text-xs font-semibold text-indigo-700">
                            {initials(user.first_name, user.last_name)}
                          </span>
                          <div className="min-w-0">
                            <p className="truncate font-semibold text-slate-800">{user.first_name} {user.last_name}</p>
                            <p className="text-xs text-slate-400">Added {dateLabel(user.created_at)}</p>
                          </div>
                        </div>
                      </td>
                      <td className="max-w-56 truncate px-4 py-3.5 text-slate-600">{user.email}</td>
                      <td className="px-4 py-3.5 text-slate-600">{user.mobile || '—'}</td>
                      <td className="px-4 py-3.5">
                        <span className={`inline-flex rounded-full px-2.5 py-1 text-xs font-medium ${user.role === 'admin' ? 'bg-violet-50 text-violet-700' : 'bg-emerald-50 text-emerald-700'}`}>
                          {user.role === 'admin' ? 'Admin' : 'Employee'}
                        </span>
                      </td>
                      <td className="px-5 py-3.5">
                        <div className="flex justify-end gap-2">
                          <button type="button" onClick={() => { setActionError(null); setEditing(user) }} className="h-9 rounded-lg border border-slate-200 px-3 text-xs font-semibold text-slate-700 hover:border-indigo-200 hover:text-indigo-700">Edit</button>
                          {!isSelf && <button type="button" onClick={() => { setActionError(null); setDeleting(user) }} className="h-9 rounded-lg border border-red-100 px-3 text-xs font-semibold text-red-600 hover:bg-red-50">Delete</button>}
                        </div>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
          <div className="px-4 pb-4 sm:px-5">
            <Pagination page={result.page} totalPages={result.total_pages} onChange={(next) => void load(next)} totalLabel={`${result.total} user${result.total === 1 ? '' : 's'}`} ariaLabel="User pages" />
          </div>
        </>
      ) : (
        <div className="p-10 text-center">
          <p className="text-sm font-medium text-slate-700">No users yet</p>
          <p className="mt-1 text-xs text-slate-500">Add a user to give them access to the portal.</p>
        </div>
      )}

      {adding && <UserFormModal title="Add user" initial={blankUser} busy={busy} onClose={() => setAdding(false)} onSave={(input) => saveUser(input)} />}
      {editing && (
        <UserFormModal
          title="Edit user"
          initial={{ first_name: editing.first_name, last_name: editing.last_name, email: editing.email, mobile: editing.mobile ?? '', address: editing.address ?? '', role: editing.role, password: '' }}
          busy={busy}
          disableRole={editing.id === actorID}
          onClose={() => setEditing(null)}
          onSave={(input) => saveUser(input, editing)}
        />
      )}
      {deleting && (
        <Modal
          title="Delete user?"
          onClose={() => setDeleting(null)}
          busy={busy}
          footer={<>
            <button type="button" disabled={busy} onClick={() => setDeleting(null)} className="h-10 rounded-lg border border-slate-200 px-4 text-sm font-medium text-slate-700 disabled:opacity-50">Cancel</button>
            <button type="button" disabled={busy} onClick={() => void confirmDelete()} className="h-10 rounded-lg bg-red-600 px-4 text-sm font-semibold text-white hover:bg-red-700 disabled:opacity-50">{busy ? 'Deleting…' : 'Delete user'}</button>
          </>}
        >
          <p className="text-sm leading-6 text-slate-600">Delete <strong className="text-slate-900">{deleting.first_name} {deleting.last_name}</strong> ({deleting.email})? Their profile, skills, and personal change history will also be removed. This cannot be undone.</p>
          {actionError && <p role="alert" className="mt-3 text-sm text-red-600">{actionError}</p>}
        </Modal>
      )}
    </section>
  )
}

function UserFormModal({
  title,
  initial,
  busy,
  disableRole = false,
  onClose,
  onSave,
}: {
  title: string
  initial: AdminUserInput
  busy: boolean
  disableRole?: boolean
  onClose: () => void
  onSave: (input: AdminUserInput) => Promise<void>
}) {
  const [form, setForm] = useState(initial)
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [formError, setFormError] = useState<string | null>(null)
  const creating = title === 'Add user'

  function set<K extends keyof AdminUserInput>(key: K, value: AdminUserInput[K]) {
    setForm((current) => ({ ...current, [key]: value }))
    setErrors((current) => ({ ...current, [key]: '' }))
  }

  async function submit(event: FormEvent) {
    event.preventDefault()
    if (busy) return
    setFormError(null)
    setErrors({})
    try {
      await onSave(form)
    } catch (e) {
      if (e instanceof ApiError && Object.keys(e.fieldErrors).length) setErrors(e.fieldErrors)
      else setFormError(e instanceof Error ? e.message : 'Could not save user')
    }
  }

  const fieldClass = 'mt-1 h-10 w-full rounded-lg border border-slate-300 px-3 text-sm text-slate-900 outline-none focus:border-indigo-500 focus:ring-2 focus:ring-indigo-100'

  return (
    <Modal
      title={title}
      onClose={onClose}
      busy={busy}
      wide
      footer={<>
        <button type="button" disabled={busy} onClick={onClose} className="h-10 rounded-lg border border-slate-200 px-4 text-sm font-medium text-slate-700">Cancel</button>
        <button type="submit" form="managed-user-form" disabled={busy} className="h-10 rounded-lg bg-indigo-600 px-4 text-sm font-semibold text-white hover:bg-indigo-700 disabled:opacity-50">{busy ? 'Saving…' : creating ? 'Add user' : 'Save changes'}</button>
      </>}
    >
      <form id="managed-user-form" onSubmit={submit} className="space-y-4">
        <div className="grid gap-3 sm:grid-cols-2">
          <Field label="First name" name="first_name" value={form.first_name} error={errors.first_name} disabled={busy} onChange={(value) => set('first_name', value)} className={fieldClass} />
          <Field label="Last name" name="last_name" value={form.last_name} error={errors.last_name} disabled={busy} onChange={(value) => set('last_name', value)} className={fieldClass} />
        </div>
        <div className="grid gap-3 sm:grid-cols-2">
          <Field label="Email" name="email" type="email" value={form.email} error={errors.email} disabled={busy} onChange={(value) => set('email', value)} className={fieldClass} />
          <label className="block text-sm font-medium text-slate-700">
            Role
            <select value={form.role} disabled={disableRole || busy} onChange={(e) => set('role', e.target.value as Role)} className={fieldClass}>
              <option value="employee">Employee</option><option value="admin">Admin</option>
            </select>
            {disableRole && <span className="mt-1 block text-xs font-normal text-slate-400">You cannot change your own role.</span>}
            {errors.role && <span className="mt-1 block text-xs font-normal text-red-600">{errors.role}</span>}
          </label>
        </div>
        <div className="grid gap-3 sm:grid-cols-2">
          <Field label="Mobile number" name="mobile" type="tel" value={form.mobile} error={errors.mobile} disabled={busy} onChange={(value) => set('mobile', value)} className={fieldClass} />
          <Field label={creating ? 'Password' : 'New password (optional)'} name="password" type="password" value={form.password} error={errors.password} disabled={busy} onChange={(value) => set('password', value)} className={fieldClass} />
        </div>
        <Field label="Address" name="address" value={form.address} error={errors.address} disabled={busy} onChange={(value) => set('address', value)} className={fieldClass} />
        {creating && <p className="-mt-2 text-xs text-slate-400">Use at least 8 characters. You can provide a temporary password for the user.</p>}
        {formError && <p role="alert" className="text-sm text-red-600">{formError}</p>}
      </form>
    </Modal>
  )
}

function Field({
  label, name, value, error, type = 'text', disabled = false, onChange, className,
}: {
  label: string
  name: string
  value: string
  error?: string
  type?: string
  disabled?: boolean
  onChange: (value: string) => void
  className: string
}) {
  return (
    <label className="block text-sm font-medium text-slate-700">
      {label}
      <input
        name={name}
        type={type}
        value={value}
        required={name === 'first_name' || name === 'last_name' || name === 'email' || (name === 'password' && label === 'Password')}
        aria-invalid={error ? true : undefined}
        disabled={disabled}
        onChange={(e) => onChange(e.target.value)}
        className={`${className} ${disabled ? 'cursor-not-allowed bg-slate-50 opacity-70' : ''} ${error ? 'border-red-400 focus:border-red-500 focus:ring-red-100' : ''}`}
      />
      {error && <span className="mt-1 block text-xs font-normal text-red-600">{error}</span>}
    </label>
  )
}

function initials(first: string, last: string) {
  return `${first.trim().charAt(0)}${last.trim().charAt(0)}`.toUpperCase() || '?'
}

function dateLabel(value: string) {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? 'recently' : new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' }).format(date)
}
