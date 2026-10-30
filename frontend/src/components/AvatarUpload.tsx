import { useEffect, useRef, useState } from 'react'
import { ApiError } from '../api/client'
import { Avatar } from './Avatar'
import { Modal } from './Modal'
import type { Profile } from '../types'

const MAX_BYTES = 2 * 1024 * 1024
const ACCEPTED = ['image/jpeg', 'image/png', 'image/webp']

interface Props {
  profile: Profile
  onUpload: (file: File) => Promise<void>
}

/**
 * The avatar, with a dialog for changing it. Nothing is sent until the employee has
 * seen the new photo in place and pressed Save, so a mis-picked file costs a click
 * rather than an upload and an entry in their history.
 */
export function AvatarUpload({ profile, onUpload }: Props) {
  const [open, setOpen] = useState(false)

  return (
    <div className="flex flex-col items-center gap-3">
      <Avatar profile={profile} size="lg" />

      <button
        type="button"
        onClick={() => setOpen(true)}
        className="h-11 rounded-lg border border-slate-300 px-4 text-sm font-semibold text-slate-700 hover:bg-slate-50"
      >
        {profile.avatar_url ? 'Change photo' : 'Upload photo'}
      </button>

      {open && (
        <AvatarDialog profile={profile} onUpload={onUpload} onClose={() => setOpen(false)} />
      )}
    </div>
  )
}

function AvatarDialog({ profile, onUpload, onClose }: Props & { onClose: () => void }) {
  const inputRef = useRef<HTMLInputElement>(null)
  const [file, setFile] = useState<File | null>(null)
  const [preview, setPreview] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  // Object URLs are revoked when the preview is replaced or the dialog closes,
  // otherwise each pick leaks a blob for the life of the page.
  useEffect(() => {
    if (!file) {
      setPreview(null)
      return
    }
    const url = URL.createObjectURL(file)
    setPreview(url)
    return () => URL.revokeObjectURL(url)
  }, [file])

  function choose(picked: File | undefined) {
    setError(null)
    if (!picked) return

    // Checked again on the server, which sniffs the bytes rather than trusting this.
    if (!ACCEPTED.includes(picked.type)) {
      setError('Please choose a JPEG, PNG, or WebP image.')
      setFile(null)
      return
    }
    if (picked.size > MAX_BYTES) {
      setError('That image is larger than 2 MB.')
      setFile(null)
      return
    }
    setFile(picked)
  }

  async function save() {
    if (!file) return
    setSaving(true)
    setError(null)
    try {
      await onUpload(file)
      onClose()
    } catch (e) {
      setError(e instanceof ApiError ? (e.fieldError('file') ?? e.message) : 'Upload failed')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal
      title="Change profile photo"
      onClose={onClose}
      busy={saving}
      footer={
        <>
          <button
            type="button"
            onClick={onClose}
            disabled={saving}
            className="h-11 rounded-lg border border-slate-300 px-4 text-sm font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-60 sm:px-5"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={() => void save()}
            disabled={!file || saving}
            className="h-11 rounded-lg bg-brand-600 px-4 text-sm font-semibold text-white hover:bg-brand-700 disabled:cursor-not-allowed disabled:opacity-50 sm:px-5"
          >
            {saving ? 'Saving…' : 'Save photo'}
          </button>
        </>
      }
    >
      <div className="flex flex-col items-center gap-3">
        {preview ? (
          <>
            <img
              src={preview}
              alt="Preview of the photo you selected"
              className="h-32 w-32 rounded-full object-cover ring-2 ring-white shadow-sm"
            />
            <p className="text-xs text-slate-400">Preview — not saved yet</p>
          </>
        ) : (
          <>
            <Avatar profile={profile} size="lg" />
            <p className="text-xs text-slate-400">Your photo today</p>
          </>
        )}
      </div>

      <input
        ref={inputRef}
        type="file"
        accept={ACCEPTED.join(',')}
        className="sr-only"
        aria-label="Choose a photo"
        onChange={(e) => choose(e.target.files?.[0])}
      />

      <button
        type="button"
        onClick={() => inputRef.current?.click()}
        disabled={saving}
        className="mt-5 h-11 w-full rounded-lg border border-slate-300 text-sm font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-60"
      >
        {file ? 'Choose a different photo' : 'Choose a photo'}
      </button>

      {error && (
        <p role="alert" className="mt-3 text-center text-sm text-red-600">
          {error}
        </p>
      )}
      <p className="mt-2 text-center text-xs text-slate-400">
        JPEG, PNG, or WebP · up to 2 MB
      </p>
    </Modal>
  )
}
