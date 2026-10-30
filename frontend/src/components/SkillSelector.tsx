import { useEffect, useMemo, useState } from 'react'
import { ApiError } from '../api/client'
import { sameSet } from '../lib/format'
import type { ProfileSkill, Skill } from '../types'

interface Props {
  /** The active catalog, which an admin configures (FR-4). */
  catalog: Skill[]
  selected: ProfileSkill[]
  onSave: (ids: string[], custom: string[]) => Promise<void>
}

/** A draft entry: a catalog pick carries an id, a typed one carries only a name. */
interface Draft {
  id: string | null
  name: string
}

function toDrafts(selected: ProfileSkill[]): Draft[] {
  return selected.map((s) => (s.custom ? { id: null, name: s.name } : { id: s.id, name: s.name }))
}

function key(d: Draft): string {
  return d.id ?? `custom:${d.name.toLowerCase()}`
}

export function SkillSelector({ catalog, selected, onSave }: Props) {
  const initial = useMemo(() => toDrafts(selected), [selected])
  const [draft, setDraft] = useState<Draft[]>(initial)
  const [query, setQuery] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Re-sync when the server hands back a new selection, e.g. after an undo.
  useEffect(() => {
    setDraft(initial)
  }, [initial])

  const dirty = !sameSet(initial.map(key), draft.map(key))
  const chosen = new Set(draft.map(key))
  const trimmed = query.trim()

  // Skills the employee holds that are no longer offered stay visible (FR-4.3).
  const retired = selected.filter(
    (s) => !s.custom && !catalog.some((c) => c.id === s.id),
  )
  const offered: Skill[] = [
    ...catalog,
    ...retired.map((s) => ({ id: s.id, name: s.name, active: false, sort_order: 999 })),
  ]

  const matches = trimmed
    ? offered.filter((s) => s.name.toLowerCase().includes(trimmed.toLowerCase()))
    : offered

  // Offer to add the typed text only when nothing in the catalog matches it exactly
  // and the employee has not already added it.
  const exactExists =
    offered.some((s) => s.name.toLowerCase() === trimmed.toLowerCase()) ||
    draft.some((d) => d.name.toLowerCase() === trimmed.toLowerCase())
  const canAddCustom = trimmed !== '' && !exactExists && !trimmed.includes(',')

  function toggle(skill: Skill) {
    const k = skill.id
    setDraft((cur) =>
      cur.some((d) => d.id === k)
        ? cur.filter((d) => d.id !== k)
        : [...cur, { id: skill.id, name: skill.name }],
    )
  }

  function addCustom() {
    if (!canAddCustom) return
    setDraft((cur) => [...cur, { id: null, name: trimmed }])
    setQuery('')
  }

  function removeCustom(name: string) {
    setDraft((cur) => cur.filter((d) => !(d.id === null && d.name === name)))
  }

  async function save() {
    setSaving(true)
    setError(null)
    try {
      await onSave(
        draft.filter((d) => d.id !== null).map((d) => d.id as string),
        draft.filter((d) => d.id === null).map((d) => d.name),
      )
    } catch (e) {
      const field = e instanceof ApiError ? (e.fieldError('skills') ?? e.fieldError('custom_skills')) : undefined
      setError(field ?? (e instanceof Error ? e.message : 'Could not save'))
    } finally {
      setSaving(false)
    }
  }

  const customDrafts = draft.filter((d) => d.id === null)

  return (
    <div>
      <label htmlFor="skill-search" className="sr-only">
        Search skills
      </label>
      <input
        id="skill-search"
        type="text"
        value={query}
        placeholder="Search skills, or type your own"
        onChange={(e) => setQuery(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'Enter') {
            e.preventDefault()
            addCustom()
          }
        }}
        className="h-11 w-full rounded-lg border border-slate-300 px-3 focus:border-brand-500 focus:outline-none focus:ring-2 focus:ring-brand-500"
      />

      {canAddCustom && (
        <button
          type="button"
          onClick={addCustom}
          className="mt-2 w-full rounded-lg border border-dashed border-brand-400 px-4 py-3 text-left text-sm text-brand-700 hover:bg-brand-50"
        >
          Not in the list? Add <span className="font-semibold">“{trimmed}”</span> as your own
          skill
        </button>
      )}
      {trimmed !== '' && trimmed.includes(',') && (
        <p className="mt-2 text-sm text-amber-600">A skill name cannot contain a comma.</p>
      )}

      {customDrafts.length > 0 && (
        <div className="mt-4">
          <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-400">
            Your own skills
          </p>
          <div className="flex flex-wrap gap-2">
            {customDrafts.map((d) => (
              <span
                key={d.name}
                className="inline-flex min-h-[44px] items-center gap-2 rounded-full border border-brand-600 bg-brand-600 px-4 text-sm font-medium text-white"
              >
                {d.name}
                <button
                  type="button"
                  onClick={() => removeCustom(d.name)}
                  aria-label={`Remove ${d.name}`}
                  className="text-white/80 hover:text-white"
                >
                  ×
                </button>
              </span>
            ))}
          </div>
        </div>
      )}

      <div className="mt-4">
        {customDrafts.length > 0 && (
          <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-slate-400">
            From the catalog
          </p>
        )}
        {matches.length === 0 ? (
          <p className="text-sm text-slate-500">No skill matches “{trimmed}”.</p>
        ) : (
          <div className="flex flex-wrap gap-2">
            {matches.map((skill) => {
              const checked = chosen.has(skill.id)
              return (
                <label
                  key={skill.id}
                  className={`inline-flex min-h-[44px] cursor-pointer items-center rounded-full border px-4 text-sm font-medium transition ${
                    checked
                      ? 'border-brand-600 bg-brand-600 text-white'
                      : 'border-slate-300 bg-white text-slate-700 hover:border-brand-500 hover:text-brand-700'
                  }`}
                >
                  <input
                    type="checkbox"
                    className="sr-only"
                    checked={checked}
                    onChange={() => toggle(skill)}
                  />
                  {skill.name}
                  {!skill.active && <span className="ml-2 text-xs opacity-70">(retired)</span>}
                </label>
              )
            })}
          </div>
        )}
      </div>

      {error && (
        <p role="alert" className="mt-3 text-sm text-red-600">
          {error}
        </p>
      )}

      <div className="mt-4 flex items-center gap-3">
        <button
          type="button"
          onClick={() => void save()}
          disabled={!dirty || saving}
          className="h-11 w-full rounded-lg bg-brand-600 px-4 text-sm font-semibold text-white hover:bg-brand-700 disabled:cursor-not-allowed disabled:opacity-50 sm:w-auto"
        >
          {saving ? 'Saving…' : 'Save skills'}
        </button>
        {dirty && !saving && (
          <button
            type="button"
            onClick={() => setDraft(initial)}
            className="h-11 shrink-0 rounded-lg px-3 text-sm font-medium text-slate-600 hover:bg-slate-100"
          >
            Reset
          </button>
        )}
      </div>
    </div>
  )
}
