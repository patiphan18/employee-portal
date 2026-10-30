import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { ApiError } from '../api/client'
import { Modal } from './Modal'
import { Pagination } from './Pagination'
import type { Skill, SkillInput } from '../types'

/**
 * Rows per page. The catalog is paged in the browser rather than on the server:
 * unlike the audit log it is small and bounded by what administrators create, and
 * the page already holds the whole list to show the active count. If it ever grows
 * large this becomes a store-layer change, exactly as the history paging was.
 */
export const CATALOG_PAGE_SIZE = 8

interface Props {
  skills: Skill[]
  onCreate: (input: SkillInput) => Promise<void>
  onUpdate: (id: string, input: SkillInput) => Promise<void>
}

export function SkillCatalog({ skills, onCreate, onUpdate }: Props) {
  const [page, setPage] = useState(1)
  const [adding, setAdding] = useState(false)

  const totalPages = Math.max(1, Math.ceil(skills.length / CATALOG_PAGE_SIZE))

  // Deleting is not possible, but retiring the last row of the last page can still
  // leave the current page past the end after a refresh.
  useEffect(() => {
    if (page > totalPages) setPage(totalPages)
  }, [page, totalPages])

  const visible = useMemo(
    () => skills.slice((page - 1) * CATALOG_PAGE_SIZE, page * CATALOG_PAGE_SIZE),
    [skills, page],
  )

  return (
    <div>
      <div className="mb-4 flex justify-end">
        <button
          type="button"
          onClick={() => setAdding(true)}
          className="h-11 rounded-lg bg-brand-600 px-4 text-sm font-semibold text-white hover:bg-brand-700"
        >
          Add skill
        </button>
      </div>

      {skills.length === 0 ? (
        <p className="text-sm text-slate-500">The catalog is empty.</p>
      ) : (
        <>
          {/* The wrapper scrolls instead of the page, so a narrow screen never gets
              a horizontal scrollbar on the whole document. */}
          <div className="-mx-5 overflow-x-auto sm:mx-0">
            <table className="w-full min-w-[22rem] border-collapse text-left text-sm">
              <thead>
                <tr className="border-b border-slate-200">
                  <th scope="col" className="px-5 py-2 font-medium text-slate-500 sm:px-3">
                    Skill
                  </th>
                  <th scope="col" className="px-3 py-2 font-medium text-slate-500">
                    Status
                  </th>
                  <th scope="col" className="px-5 py-2 text-right font-medium text-slate-500 sm:px-3">
                    Actions
                  </th>
                </tr>
              </thead>
              <tbody>
                {visible.map((skill) => (
                  <SkillRow key={skill.id} skill={skill} onUpdate={onUpdate} />
                ))}
              </tbody>
            </table>
          </div>

          <Pagination
            page={page}
            totalPages={totalPages}
            onChange={setPage}
            totalLabel={`${skills.length} skill${skills.length === 1 ? '' : 's'}`}
          />
        </>
      )}

      {adding && <AddSkillModal onCreate={onCreate} onClose={() => setAdding(false)} />}
    </div>
  )
}

function SkillRow({
  skill,
  onUpdate,
}: {
  skill: Skill
  onUpdate: (id: string, input: SkillInput) => Promise<void>
}) {
  const [editing, setEditing] = useState(false)
  const [name, setName] = useState(skill.name)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function run(input: SkillInput, after?: () => void) {
    setBusy(true)
    setError(null)
    try {
      await onUpdate(skill.id, input)
      after?.()
    } catch (e) {
      setError(e instanceof ApiError ? (e.fieldError('name') ?? e.message) : 'Could not save')
    } finally {
      setBusy(false)
    }
  }

  function cancel() {
    setName(skill.name)
    setError(null)
    setEditing(false)
  }

  return (
    <tr className="border-b border-slate-100 align-middle">
      <td className="px-5 py-3 sm:px-3">
        {editing ? (
          <input
            aria-label={`Name for ${skill.name}`}
            value={name}
            autoFocus
            onChange={(e) => setName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') void run({ name }, () => setEditing(false))
              if (e.key === 'Escape') cancel()
            }}
            className="h-10 w-full min-w-[8rem] rounded-lg border border-slate-300 px-2 focus:border-brand-500 focus:outline-none focus:ring-2 focus:ring-brand-500"
          />
        ) : (
          <span className="font-medium text-slate-900">{skill.name}</span>
        )}
        {error && (
          <p role="alert" className="mt-1 text-sm text-red-600">
            {error}
          </p>
        )}
      </td>

      <td className="px-3 py-3">
        <span
          className={`inline-block rounded-full px-2 py-0.5 text-xs font-medium ${
            skill.active ? 'bg-emerald-50 text-emerald-700' : 'bg-slate-100 text-slate-500'
          }`}
        >
          {skill.active ? 'Active' : 'Retired'}
        </span>
      </td>

      <td className="px-5 py-3 text-right sm:px-3">
        <div className="flex justify-end gap-1">
          {editing ? (
            <>
              <button
                type="button"
                disabled={busy}
                onClick={() => void run({ name }, () => setEditing(false))}
                className="min-h-[40px] rounded-lg bg-brand-600 px-3 text-sm font-semibold text-white hover:bg-brand-700 disabled:opacity-60"
              >
                {busy ? 'Saving…' : 'Save'}
              </button>
              <button
                type="button"
                disabled={busy}
                onClick={cancel}
                className="min-h-[40px] rounded-lg border border-slate-300 px-3 text-sm font-medium text-slate-700 hover:bg-slate-50 disabled:opacity-60"
              >
                Cancel
              </button>
            </>
          ) : (
            <>
              <button
                type="button"
                disabled={busy}
                onClick={() => setEditing(true)}
                className="min-h-[40px] rounded-lg px-3 text-sm font-medium text-brand-600 hover:bg-brand-50 disabled:opacity-60"
              >
                Rename
                <span className="sr-only"> {skill.name}</span>
              </button>
              <button
                type="button"
                disabled={busy}
                onClick={() => void run({ active: !skill.active })}
                className="min-h-[40px] rounded-lg border border-slate-300 px-3 text-sm font-medium text-slate-700 hover:bg-slate-50 disabled:opacity-60"
              >
                {skill.active ? 'Retire' : 'Restore'}
                <span className="sr-only"> {skill.name}</span>
              </button>
            </>
          )}
        </div>
      </td>
    </tr>
  )
}

function AddSkillModal({
  onCreate,
  onClose,
}: {
  onCreate: (input: SkillInput) => Promise<void>
  onClose: () => void
}) {
  const [name, setName] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (name.trim() === '') return
    setBusy(true)
    setError(null)
    try {
      await onCreate({ name })
      onClose()
    } catch (err) {
      setError(err instanceof ApiError ? (err.fieldError('name') ?? err.message) : 'Could not add')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      title="Add a skill"
      onClose={onClose}
      busy={busy}
      footer={
        <>
          <button
            type="button"
            onClick={onClose}
            disabled={busy}
            className="h-11 rounded-lg border border-slate-300 px-4 text-sm font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-60 sm:px-5"
          >
            Cancel
          </button>
          <button
            type="submit"
            form="add-skill-form"
            disabled={busy || name.trim() === ''}
            className="h-11 rounded-lg bg-brand-600 px-4 text-sm font-semibold text-white hover:bg-brand-700 disabled:cursor-not-allowed disabled:opacity-50 sm:px-5"
          >
            {busy ? 'Adding…' : 'Add skill'}
          </button>
        </>
      }
    >
      <form id="add-skill-form" onSubmit={submit} noValidate>
        <label htmlFor="new-skill-name" className="text-sm font-medium text-slate-700">
          Skill name
        </label>
        <input
          id="new-skill-name"
          value={name}
          autoFocus
          placeholder="e.g. Rust"
          aria-invalid={error ? true : undefined}
          aria-describedby={error ? 'new-skill-error' : undefined}
          onChange={(e) => {
            setName(e.target.value)
            setError(null)
          }}
          className="mt-1 h-11 w-full rounded-lg border border-slate-300 px-3 focus:border-brand-500 focus:outline-none focus:ring-2 focus:ring-brand-500"
        />
        {error && (
          <p id="new-skill-error" role="alert" className="mt-2 text-sm text-red-600">
            {error}
          </p>
        )}
        <p className="mt-2 text-xs text-slate-400">
          Offered to every employee once added. Commas are not allowed.
        </p>
      </form>
    </Modal>
  )
}
