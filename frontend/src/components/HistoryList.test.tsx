import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { HistoryList } from './HistoryList'
import type { ProfileChange } from '../types'

const event = (overrides: Partial<ProfileChange> = {}): ProfileChange => ({
  id: 'evt-1',
  user_id: 'user-1',
  field: 'mobile',
  old_value: '0812345678',
  new_value: '0899999999',
  changed_by: 'user-1',
  changed_at: new Date().toISOString(),
  ...overrides,
})

const manyEvents = (n: number) =>
  Array.from({ length: n }, (_, i) => event({ id: `evt-${i}` }))

function setup(overrides: Partial<Parameters<typeof HistoryList>[0]> = {}) {
  const onChangePage = overrides.onChangePage ?? vi.fn()
  render(
    <HistoryList
      events={[event()]}
      days={7}
      page={1}
      total={1}
      totalPages={1}
      loading={false}
      error={null}
      onChangePage={onChangePage}
      {...overrides}
    />,
  )
  return { onChangePage, user: userEvent.setup() }
}

describe('HistoryList', () => {
  it('renders an event as a readable before/after row', () => {
    setup()
    expect(screen.getByText('Mobile number changed')).toBeInTheDocument()
    expect(screen.getByText('0812345678')).toBeInTheDocument()
    expect(screen.getByText('0899999999')).toBeInTheDocument()
  })

  it('shows an em dash where a value was blank', () => {
    setup({ events: [event({ old_value: null })] })
    expect(screen.getByText('—')).toBeInTheDocument()
  })



  it('shows an empty state naming the window', () => {
    setup({ events: [], days: 30, total: 0 })
    expect(screen.getByText('No changes in the last 30 days.')).toBeInTheDocument()
  })

  // The window is a policy an administrator sets; employees only see what it is.
  it('states the window as text, with no way to change it', () => {
    setup({ days: 30 })
    expect(screen.getByText(/Showing the last 30 days, as set by your administrator/)).toBeInTheDocument()
    expect(screen.queryByRole('group', { name: 'History period' })).not.toBeInTheDocument()
    for (const label of ['7 days', '30 days', '90 days']) {
      expect(screen.queryByRole('button', { name: label })).not.toBeInTheDocument()
    }
  })

  it('says "day" rather than "days" for a one-day window', () => {
    setup({ days: 1 })
    expect(screen.getByText(/Showing the last 1 day,/)).toBeInTheDocument()
  })

  // The window is unknown until the first response lands.
  it('says nothing about the window before it is known', () => {
    setup({ days: 0, loading: true })
    expect(screen.queryByText(/Showing the last/)).not.toBeInTheDocument()
  })

  it('shows a loading state', () => {
    setup({ loading: true })
    expect(screen.getByText('Loading your history…')).toBeInTheDocument()
  })

  it('shows a load error', () => {
    setup({ error: 'Could not load your change history' })
    expect(screen.getByRole('alert')).toHaveTextContent('Could not load your change history')
  })

  // --- the log is read-only ---

  it('offers no way to undo a change', () => {
    setup({ events: manyEvents(3), total: 3 })
    expect(screen.queryByRole('button', { name: /undo/i })).not.toBeInTheDocument()
  })

  // --- pagination (FR-5.8) ---

  it('hides the pager when everything fits on one page', () => {
    setup({ events: manyEvents(5), total: 5, totalPages: 1 })
    expect(screen.queryByRole('navigation', { name: 'History pages' })).not.toBeInTheDocument()
  })

  it('shows the pager and passes paging through once there is more than one page', async () => {
    const onChangePage = vi.fn()
    const { user } = setup({
      events: manyEvents(5),
      total: 46,
      totalPages: 10,
      page: 2,
      onChangePage,
    })

    expect(screen.getByRole('navigation', { name: 'History pages' })).toBeInTheDocument()
    expect(screen.getByText('46 changes')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Page 2' })).toHaveAttribute('aria-current', 'page')

    await user.click(screen.getByRole('button', { name: 'Page 3' }))
    expect(onChangePage).toHaveBeenCalledWith(3)
  })

  it('renders only the page it was given, however long the log is', () => {
    setup({ events: manyEvents(5), total: 500, totalPages: 100 })
    expect(screen.getAllByRole('listitem')).toHaveLength(5)
  })







  it('shows no pager while loading', () => {
    setup({ loading: true, events: manyEvents(20), total: 45, totalPages: 3 })
    expect(screen.queryByRole('navigation', { name: 'History pages' })).not.toBeInTheDocument()
  })
})
