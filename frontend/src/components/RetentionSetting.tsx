import { useEffect, useState } from 'react'
import { ApiError } from '../api/client'

const MIN_DAYS = 1
const MAX_DAYS = 365

interface Props {
  days: number
  onSave: (days: number) => Promise<void>
}

/** Controls how far back employees can see their own change history (FR-5.4). */
export function RetentionSetting({ days, onSave }: Props) {
  const [draft, setDraft] = useState(String(days))
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [saved, setSaved] = useState(false)

  // Re-sync when the server reports a different value, e.g. after a clamp.
  useEffect(() => {
    setDraft(String(days))
  }, [days])

  const parsed = Number(draft)
  const valid = Number.isInteger(parsed) && parsed >= MIN_DAYS && parsed <= MAX_DAYS
  const dirty = parsed !== days

  async function save() {
    setSaving(true)
    setError(null)
    setSaved(false)
    try {
      await onSave(parsed)
      setSaved(true)
    } catch (e) {
      setError(
        e instanceof ApiError
          ? (e.fieldError('history_retention_days') ?? e.message)
          : 'Could not save',
      )
    } finally {
      setSaving(false)
    }
  }

  return (
    <div>
      <label htmlFor="retention" className="block text-sm font-medium text-slate-700">
        History retention (days)
      </label>
      <p className="mt-1 text-sm text-slate-500">
        How far back employees see their own profile changes by default.
      </p>

      <div className="mt-3 flex flex-col gap-2 sm:flex-row sm:items-start">
        <input
          id="retention"
          type="number"
          inputMode="numeric"
          min={MIN_DAYS}
          max={MAX_DAYS}
          value={draft}
          aria-invalid={!valid || undefined}
          aria-describedby={error ? 'retention-error' : undefined}
          onChange={(e) => {
            setDraft(e.target.value)
            setSaved(false)
          }}
          className="h-11 w-full rounded-lg border border-slate-300 px-3 focus:border-brand-500 focus:outline-none focus:ring-2 focus:ring-brand-500 sm:w-32"
        />
        <button
          type="button"
          onClick={() => void save()}
          disabled={!valid || !dirty || saving}
          className="h-11 rounded-lg bg-brand-600 px-5 text-sm font-semibold text-white hover:bg-brand-700 disabled:cursor-not-allowed disabled:opacity-50"
        >
          {saving ? 'Saving…' : 'Save'}
        </button>
      </div>

      {!valid && (
        <p className="mt-2 text-sm text-amber-600">
          Enter a whole number between {MIN_DAYS} and {MAX_DAYS}.
        </p>
      )}
      {error && (
        <p id="retention-error" role="alert" className="mt-2 text-sm text-red-600">
          {error}
        </p>
      )}
      {saved && !dirty && (
        <p role="status" className="mt-2 text-sm text-emerald-600">
          Saved. Employees now see {days} days by default.
        </p>
      )}
    </div>
  )
}
