import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, describe, expect, it, vi } from 'vitest'
import { ApiError } from '../api/client'
import { AvatarUpload } from './AvatarUpload'
import type { Profile } from '../types'

const profile: Profile = {
  id: 'user-1',
  first_name: 'Somchai',
  last_name: 'Jaidee',
  email: 'somchai@example.com',
  mobile: '0812345678',
  address: null,
  avatar_url: null,
  role: 'employee',
  skills: [],
  completeness: { percent: 40, missing: [] },
  updated_at: new Date().toISOString(),
}

// jsdom does not implement object URLs.
beforeAll(() => {
  URL.createObjectURL = vi.fn(() => 'blob:preview')
  URL.revokeObjectURL = vi.fn()
})

function pngFile(name = 'me.png', size = 1024) {
  const file = new File([new Uint8Array(size)], name, { type: 'image/png' })
  Object.defineProperty(file, 'size', { value: size })
  return file
}

function setup(onUpload = vi.fn().mockResolvedValue(undefined), p: Partial<Profile> = {}) {
  render(<AvatarUpload profile={{ ...profile, ...p }} onUpload={onUpload} />)
  return { onUpload, user: userEvent.setup() }
}

const openDialog = async (user: ReturnType<typeof userEvent.setup>) => {
  await user.click(screen.getByRole('button', { name: /upload photo|change photo/i }))
  return screen.getByRole('dialog')
}

describe('AvatarUpload', () => {
  it('shows no dialog until the button is pressed', () => {
    setup()
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('opens a dialog for changing the photo', async () => {
    const { user } = setup()
    await openDialog(user)
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(screen.getByText('Change profile photo')).toBeInTheDocument()
  })

  it('does not upload until Save is pressed', async () => {
    const { onUpload, user } = setup()
    await openDialog(user)
    await user.upload(screen.getByLabelText('Choose a photo'), pngFile())

    // The file is chosen and previewed, but nothing has been sent.
    expect(screen.getByAltText('Preview of the photo you selected')).toBeInTheDocument()
    expect(screen.getByText('Preview — not saved yet')).toBeInTheDocument()
    expect(onUpload).not.toHaveBeenCalled()
  })

  it('uploads the chosen file on Save and closes', async () => {
    const onUpload = vi.fn().mockResolvedValue(undefined)
    const { user } = setup(onUpload)
    await openDialog(user)

    const file = pngFile()
    await user.upload(screen.getByLabelText('Choose a photo'), file)
    await user.click(screen.getByRole('button', { name: 'Save photo' }))

    expect(onUpload).toHaveBeenCalledWith(file)
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  })

  it('keeps Save disabled until a file is chosen', async () => {
    const { user } = setup()
    await openDialog(user)
    expect(screen.getByRole('button', { name: 'Save photo' })).toBeDisabled()

    await user.upload(screen.getByLabelText('Choose a photo'), pngFile())
    expect(screen.getByRole('button', { name: 'Save photo' })).toBeEnabled()
  })

  it('discards the choice on Cancel without uploading', async () => {
    const { onUpload, user } = setup()
    await openDialog(user)
    await user.upload(screen.getByLabelText('Choose a photo'), pngFile())
    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(onUpload).not.toHaveBeenCalled()
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('closes on Escape', async () => {
    const { user } = setup()
    await openDialog(user)
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  })

  it('rejects a file that is too large, before uploading', async () => {
    const { onUpload, user } = setup()
    await openDialog(user)
    await user.upload(screen.getByLabelText('Choose a photo'), pngFile('big.png', 3 * 1024 * 1024))

    expect(await screen.findByRole('alert')).toHaveTextContent('larger than 2 MB')
    expect(screen.getByRole('button', { name: 'Save photo' })).toBeDisabled()
    expect(onUpload).not.toHaveBeenCalled()
  })

  // The accept attribute filters the picker, so this drives the change event
  // directly to prove the guard behind it holds too.
  it('rejects a file that is not an accepted image', async () => {
    const { onUpload, user } = setup()
    await openDialog(user)

    const gif = new File(['x'], 'a.gif', { type: 'image/gif' })
    fireEvent.change(screen.getByLabelText('Choose a photo'), { target: { files: [gif] } })

    expect(await screen.findByRole('alert')).toHaveTextContent('JPEG, PNG, or WebP')
    expect(screen.getByRole('button', { name: 'Save photo' })).toBeDisabled()
    expect(onUpload).not.toHaveBeenCalled()
  })

  it('keeps the dialog open and shows the error when the server refuses', async () => {
    const onUpload = vi
      .fn()
      .mockRejectedValue(new ApiError(422, 'validation failed', { file: 'unsupported image type' }))
    const { user } = setup(onUpload)
    await openDialog(user)

    await user.upload(screen.getByLabelText('Choose a photo'), pngFile())
    await user.click(screen.getByRole('button', { name: 'Save photo' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('unsupported image type')
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('offers to swap the file once one is chosen', async () => {
    const { user } = setup()
    await openDialog(user)
    expect(screen.getByRole('button', { name: 'Choose a photo' })).toBeInTheDocument()

    await user.upload(screen.getByLabelText('Choose a photo'), pngFile())
    expect(screen.getByRole('button', { name: 'Choose a different photo' })).toBeInTheDocument()
  })
})
