import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { ApiError } from '../api/client'
import type { Profile } from '../types'

type Field = 'mobile' | 'email' | 'address'

interface Props {
  profile: Profile
  onSave: (patch: Record<Field, string>) => Promise<void>
}

interface Draft {
  mobile: string
  email: string
  address: string
}

function draftFrom(profile: Profile): Draft {
  return {
    mobile: profile.mobile ?? '',
    email: profile.email,
    address: profile.address ?? '',
  }
}

/**
 * The whole contact block in one form with a single Save.
 *
 * Every field is submitted, changed or not: the server compares them against what
 * it has and records a history entry only for the ones that actually moved, so the
 * client never has to work out what changed — and cannot get that answer wrong.
 */
export function ContactForm({ profile, onSave }: Props) {
  const initial = useMemo(() => draftFrom(profile), [profile])
  const [draft, setDraft] = useState<Draft>(initial)
  const [saving, setSaving] = useState(false)
  const [errors, setErrors] = useState<Partial<Record<Field, string>>>({})
  const [formError, setFormError] = useState<string | null>(null)

  // Adopt the server's version after a save, which may be normalised.
  useEffect(() => {
    setDraft(initial)
  }, [initial])

  const dirty =
    draft.mobile !== initial.mobile ||
    draft.email !== initial.email ||
    draft.address !== initial.address

  function set(field: Field, value: string) {
    setDraft((d) => ({ ...d, [field]: value }))
    // Clear the message for a field as soon as the employee edits it again.
    setErrors((e) => (e[field] ? { ...e, [field]: undefined } : e))
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    setSaving(true)
    setErrors({})
    setFormError(null)
    try {
      await onSave({ ...draft })
    } catch (err) {
      if (err instanceof ApiError && Object.keys(err.fieldErrors).length > 0) {
        setErrors(err.fieldErrors as Partial<Record<Field, string>>)
      } else {
        setFormError(err instanceof Error ? err.message : 'Could not save')
      }
    } finally {
      setSaving(false)
    }
  }

  return (
    <form onSubmit={submit} noValidate>
      <div className="space-y-4">
        <TextField
          field="mobile"
          label="Mobile number"
          type="tel"
          placeholder="0812345678"
          value={draft.mobile}
          error={errors.mobile}
          onChange={set}
        />
        <TextField
          field="email"
          label="Email"
          type="email"
          placeholder="you@example.com"
          value={draft.email}
          error={errors.email}
          onChange={set}
        />
        <TextField
          field="address"
          label="Address"
          multiline
          placeholder="99 Sukhumvit Rd, Bangkok 10110"
          value={draft.address}
          error={errors.address}
          onChange={set}
        />
      </div>

      {formError && (
        <p role="alert" className="mt-3 text-sm text-red-600">
          {formError}
        </p>
      )}

      <div className="mt-5 flex items-center gap-3">
        <button
          type="submit"
          disabled={!dirty || saving}
          className="h-11 w-full rounded-lg bg-brand-600 px-4 text-sm font-semibold text-white hover:bg-brand-700 disabled:cursor-not-allowed disabled:opacity-50 sm:w-auto"
        >
          {saving ? 'Saving…' : 'Save changes'}
        </button>
        {dirty && !saving && (
          <button
            type="button"
            onClick={() => {
              setDraft(initial)
              setErrors({})
              setFormError(null)
            }}
            className="h-11 shrink-0 rounded-lg px-3 text-sm font-medium text-slate-600 hover:bg-slate-100"
          >
            Reset
          </button>
        )}
      </div>
    </form>
  )
}

function TextField({
  field,
  label,
  value,
  error,
  onChange,
  type = 'text',
  placeholder,
  multiline = false,
}: {
  field: Field
  label: string
  value: string
  error?: string
  onChange: (field: Field, value: string) => void
  type?: 'text' | 'tel' | 'email'
  placeholder?: string
  multiline?: boolean
}) {
  const id = `contact-${field}`
  const errorId = `${id}-error`
  const shared = {
    id,
    value,
    placeholder,
    'aria-invalid': error ? true : undefined,
    'aria-describedby': error ? errorId : undefined,
    onChange: (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) =>
      onChange(field, e.target.value),
    className: `mt-1 w-full rounded-lg border px-3 text-slate-900 focus:outline-none focus:ring-2 ${
      error
        ? 'border-red-400 focus:border-red-500 focus:ring-red-500'
        : 'border-slate-300 focus:border-brand-500 focus:ring-brand-500'
    }`,
  }

  return (
    <div>
      <label htmlFor={id} className="text-sm font-medium text-slate-700">
        {label}
      </label>
      {multiline ? (
        <textarea {...shared} rows={3} className={`${shared.className} py-2`} />
      ) : (
        <input {...shared} type={type} className={`${shared.className} h-11`} />
      )}
      {error && (
        <p id={errorId} role="alert" className="mt-1 text-sm text-red-600">
          {error}
        </p>
      )}
    </div>
  )
}
