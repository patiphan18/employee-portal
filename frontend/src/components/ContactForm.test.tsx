import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { ApiError } from '../api/client'
import { ContactForm } from './ContactForm'
import type { Profile } from '../types'

const profile: Profile = {
  id: 'user-1',
  first_name: 'Somchai',
  last_name: 'Jaidee',
  email: 'somchai@example.com',
  mobile: '0812345678',
  address: '99 Sukhumvit Rd',
  avatar_url: null,
  role: 'employee',
  skills: [],
  completeness: { percent: 80, missing: ['avatar'] },
  updated_at: new Date().toISOString(),
}

function setup(onSave = vi.fn().mockResolvedValue(undefined), overrides: Partial<Profile> = {}) {
  const p = { ...profile, ...overrides }
  const view = render(<ContactForm profile={p} onSave={onSave} />)
  return { onSave, user: userEvent.setup(), view, profile: p }
}

const mobile = () => screen.getByLabelText('Mobile number')
const email = () => screen.getByLabelText('Email')
const address = () => screen.getByLabelText('Address')
const saveButton = () => screen.getByRole('button', { name: /save changes/i })

describe('ContactForm', () => {
  it('shows all three fields as editable inputs at once', () => {
    setup()
    expect(mobile()).toHaveValue('0812345678')
    expect(email()).toHaveValue('somchai@example.com')
    expect(address()).toHaveValue('99 Sukhumvit Rd')
  })

  it('renders empty fields rather than placeholders text', () => {
    setup(vi.fn(), { mobile: null, address: null })
    expect(mobile()).toHaveValue('')
    expect(address()).toHaveValue('')
  })

  it('has a single save button for the whole block', () => {
    setup()
    expect(screen.getAllByRole('button', { name: /save/i })).toHaveLength(1)
  })

  it('keeps Save disabled until something changes', async () => {
    const { user } = setup()
    expect(saveButton()).toBeDisabled()

    await user.type(mobile(), '9')
    expect(saveButton()).toBeEnabled()
  })

  it('submits every field, changed or not, so the server can do the comparison', async () => {
    const onSave = vi.fn().mockResolvedValue(undefined)
    const { user } = setup(onSave)

    await user.clear(mobile())
    await user.type(mobile(), '0899999999')
    await user.click(saveButton())

    expect(onSave).toHaveBeenCalledWith({
      mobile: '0899999999',
      email: 'somchai@example.com',
      address: '99 Sukhumvit Rd',
    })
  })

  it('saves several fields together in one call', async () => {
    const onSave = vi.fn().mockResolvedValue(undefined)
    const { user } = setup(onSave)

    await user.clear(mobile())
    await user.type(mobile(), '0899999999')
    await user.clear(address())
    await user.type(address(), 'New address')
    await user.click(saveButton())

    expect(onSave).toHaveBeenCalledTimes(1)
    expect(onSave).toHaveBeenCalledWith({
      mobile: '0899999999',
      email: 'somchai@example.com',
      address: 'New address',
    })
  })

  it('sends an empty string when a field is cleared', async () => {
    const onSave = vi.fn().mockResolvedValue(undefined)
    const { user } = setup(onSave)

    await user.clear(address())
    await user.click(saveButton())

    expect(onSave).toHaveBeenCalledWith(expect.objectContaining({ address: '' }))
  })

  it('resets every field back to the saved values', async () => {
    const { user } = setup()

    await user.clear(mobile())
    await user.type(mobile(), '0899999999')
    await user.clear(address())
    await user.type(address(), 'Somewhere else')
    await user.click(screen.getByRole('button', { name: 'Reset' }))

    expect(mobile()).toHaveValue('0812345678')
    expect(address()).toHaveValue('99 Sukhumvit Rd')
    expect(saveButton()).toBeDisabled()
  })

  it('shows each server error against its own field', async () => {
    const onSave = vi.fn().mockRejectedValue(
      new ApiError(422, 'validation failed', {
        mobile: 'invalid phone number',
        address: 'address must be at most 500 characters',
      }),
    )
    const { user } = setup(onSave)

    await user.type(mobile(), 'x')
    await user.click(saveButton())

    expect(await screen.findByText('invalid phone number')).toBeInTheDocument()
    expect(screen.getByText('address must be at most 500 characters')).toBeInTheDocument()
    // The email field was fine, so it carries no message.
    expect(email()).not.toHaveAttribute('aria-invalid')
  })

  it('marks a failing field as invalid for screen readers', async () => {
    const onSave = vi
      .fn()
      .mockRejectedValue(new ApiError(422, 'validation failed', { mobile: 'invalid phone number' }))
    const { user } = setup(onSave)

    await user.type(mobile(), 'x')
    await user.click(saveButton())

    await waitFor(() => expect(mobile()).toHaveAttribute('aria-invalid', 'true'))
  })

  it('clears a field error as soon as the employee edits that field', async () => {
    const onSave = vi
      .fn()
      .mockRejectedValue(new ApiError(422, 'validation failed', { mobile: 'invalid phone number' }))
    const { user } = setup(onSave)

    await user.type(mobile(), 'x')
    await user.click(saveButton())
    expect(await screen.findByText('invalid phone number')).toBeInTheDocument()

    await user.type(mobile(), '1')
    expect(screen.queryByText('invalid phone number')).not.toBeInTheDocument()
  })

  it('shows a whole-form error when the server names no field', async () => {
    const onSave = vi.fn().mockRejectedValue(new ApiError(409, 'email already in use'))
    const { user } = setup(onSave)

    await user.type(email(), 'x')
    await user.click(saveButton())

    expect(await screen.findByRole('alert')).toHaveTextContent('email already in use')
  })

  it('keeps what the employee typed after a failed save', async () => {
    const onSave = vi
      .fn()
      .mockRejectedValue(new ApiError(422, 'validation failed', { mobile: 'invalid phone number' }))
    const { user } = setup(onSave)

    await user.clear(mobile())
    await user.type(mobile(), 'not-a-phone')
    await user.click(saveButton())

    await waitFor(() => expect(screen.getByText('invalid phone number')).toBeInTheDocument())
    expect(mobile()).toHaveValue('not-a-phone')
  })

  it('adopts the server version after a successful save', async () => {
    const onSave = vi.fn().mockResolvedValue(undefined)
    const { view, user } = setup(onSave)

    await user.clear(mobile())
    await user.type(mobile(), '081-234-5678')
    await user.click(saveButton())

    // The server normalises the number and the parent re-renders with it.
    view.rerender(
      <ContactForm profile={{ ...profile, mobile: '0812345678' }} onSave={onSave} />,
    )
    await waitFor(() => expect(mobile()).toHaveValue('0812345678'))
    expect(saveButton()).toBeDisabled()
  })

  it('disables Save while the request is in flight', async () => {
    let release: () => void = () => {}
    const onSave = vi.fn().mockImplementation(
      () =>
        new Promise<void>((resolve) => {
          release = resolve
        }),
    )
    const { user } = setup(onSave)

    await user.type(mobile(), '9')
    await user.click(saveButton())

    expect(screen.getByRole('button', { name: 'Saving…' })).toBeDisabled()
    release()
    await waitFor(() => expect(screen.getByRole('button', { name: /save changes/i })).toBeInTheDocument())
  })
})
