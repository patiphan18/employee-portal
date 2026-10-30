import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { ApiError } from '../api/client'
import { CATALOG_PAGE_SIZE, SkillCatalog } from './SkillCatalog'
import { RetentionSetting } from './RetentionSetting'
import type { Skill } from '../types'

const skill = (id: string, name: string, active = true): Skill => ({
  id,
  name,
  active,
  sort_order: 10,
})

const many = (n: number) => Array.from({ length: n }, (_, i) => skill(`s-${i}`, `Skill ${i}`))

function setup(
  skills: Skill[] = [skill('s-go', 'Go'), skill('s-react', 'React')],
  onCreate = vi.fn().mockResolvedValue(undefined),
  onUpdate = vi.fn().mockResolvedValue(undefined),
) {
  render(<SkillCatalog skills={skills} onCreate={onCreate} onUpdate={onUpdate} />)
  return { onCreate, onUpdate, user: userEvent.setup() }
}

const rows = () => within(screen.getByRole('table')).getAllByRole('row').slice(1) // minus header

// The action buttons carry the skill name in an sr-only span, so a plain text query
// matches more than the row. Read the first column instead.
const skillNames = () =>
  rows().map((r) => within(r).getAllByRole('cell')[0].textContent?.trim())

describe('SkillCatalog table', () => {
  it('renders the catalog as a table with a row per skill', () => {
    setup()
    expect(screen.getByRole('table')).toBeInTheDocument()
    expect(rows()).toHaveLength(2)
    expect(screen.getByRole('columnheader', { name: 'Skill' })).toBeInTheDocument()
    expect(screen.getByRole('columnheader', { name: 'Status' })).toBeInTheDocument()
  })

  it('shows the status of each skill', () => {
    setup([skill('s-go', 'Go'), skill('s-old', 'Flash', false)])
    expect(screen.getByText('Active')).toBeInTheDocument()
    expect(screen.getByText('Retired')).toBeInTheDocument()
  })

  it('offers Restore for a retired skill and Retire for an active one', () => {
    setup([skill('s-old', 'Flash', false)])
    expect(screen.getByRole('button', { name: /restore flash/i })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /retire flash/i })).not.toBeInTheDocument()
  })

  it('shows an empty catalog message instead of an empty table', () => {
    setup([])
    expect(screen.getByText('The catalog is empty.')).toBeInTheDocument()
    expect(screen.queryByRole('table')).not.toBeInTheDocument()
  })

  it('retires a skill', async () => {
    const onUpdate = vi.fn().mockResolvedValue(undefined)
    const { user } = setup(undefined, undefined, onUpdate)
    await user.click(screen.getByRole('button', { name: /retire go/i }))
    expect(onUpdate).toHaveBeenCalledWith('s-go', { active: false })
  })

  it('restores a retired skill', async () => {
    const onUpdate = vi.fn().mockResolvedValue(undefined)
    const { user } = setup([skill('s-old', 'Flash', false)], undefined, onUpdate)
    await user.click(screen.getByRole('button', { name: /restore flash/i }))
    expect(onUpdate).toHaveBeenCalledWith('s-old', { active: true })
  })

  it('renames a skill in its row', async () => {
    const onUpdate = vi.fn().mockResolvedValue(undefined)
    const { user } = setup(undefined, undefined, onUpdate)

    await user.click(screen.getByRole('button', { name: /rename go/i }))
    const input = screen.getByLabelText('Name for Go')
    await user.clear(input)
    await user.type(input, 'Golang')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(onUpdate).toHaveBeenCalledWith('s-go', { name: 'Golang' })
  })

  it('discards a rename on Cancel', async () => {
    const onUpdate = vi.fn()
    const { user } = setup(undefined, undefined, onUpdate)

    await user.click(screen.getByRole('button', { name: /rename go/i }))
    await user.clear(screen.getByLabelText('Name for Go'))
    await user.type(screen.getByLabelText('Name for Go'), 'Nope')
    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(onUpdate).not.toHaveBeenCalled()
    expect(skillNames()).toContain('Go')
  })

  it('shows a server error against the row being renamed', async () => {
    const onUpdate = vi
      .fn()
      .mockRejectedValue(new ApiError(422, 'validation failed', { name: 'name must not contain a comma' }))
    const { user } = setup(undefined, undefined, onUpdate)

    await user.click(screen.getByRole('button', { name: /rename go/i }))
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('name must not contain a comma')
  })
})

describe('SkillCatalog pagination', () => {
  it('shows no pager when everything fits on one page', () => {
    setup(many(CATALOG_PAGE_SIZE))
    expect(rows()).toHaveLength(CATALOG_PAGE_SIZE)
    expect(screen.queryByRole('navigation', { name: 'History pages' })).not.toBeInTheDocument()
  })

  it('splits a long catalog into pages', () => {
    setup(many(20))
    expect(rows()).toHaveLength(CATALOG_PAGE_SIZE)
    expect(screen.getByRole('navigation', { name: 'History pages' })).toBeInTheDocument()
    expect(screen.getByText('20 skills')).toBeInTheDocument()
  })

  it('shows the next slice of skills on page 2', async () => {
    const { user } = setup(many(20))
    expect(skillNames()).toContain('Skill 0')

    await user.click(screen.getByRole('button', { name: 'Page 2' }))

    expect(skillNames()).not.toContain('Skill 0')
    expect(skillNames()).toContain(`Skill ${CATALOG_PAGE_SIZE}`)
  })

  it('puts the remainder on the last page', async () => {
    const { user } = setup(many(20))
    await user.click(screen.getByRole('button', { name: 'Page 3' }))
    expect(rows()).toHaveLength(20 - CATALOG_PAGE_SIZE * 2)
  })
})

describe('SkillCatalog add dialog', () => {
  it('shows no dialog until Add skill is pressed', () => {
    setup()
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('Skill name')).not.toBeInTheDocument()
  })

  it('opens a dialog from the Add skill button', async () => {
    const { user } = setup()
    await user.click(screen.getByRole('button', { name: 'Add skill' }))

    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(screen.getByText('Add a skill')).toBeInTheDocument()
  })

  it('creates the skill and closes', async () => {
    const onCreate = vi.fn().mockResolvedValue(undefined)
    const { user } = setup(undefined, onCreate)

    await user.click(screen.getByRole('button', { name: 'Add skill' }))
    await user.type(screen.getByLabelText('Skill name'), 'Rust')
    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Add skill' }))

    expect(onCreate).toHaveBeenCalledWith({ name: 'Rust' })
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  })

  it('keeps the submit button disabled until a name is typed', async () => {
    const { user } = setup()
    await user.click(screen.getByRole('button', { name: 'Add skill' }))

    const dialog = within(screen.getByRole('dialog'))
    expect(dialog.getByRole('button', { name: 'Add skill' })).toBeDisabled()

    await user.type(screen.getByLabelText('Skill name'), 'Rust')
    expect(dialog.getByRole('button', { name: 'Add skill' })).toBeEnabled()
  })

  it('creates nothing on Cancel', async () => {
    const onCreate = vi.fn()
    const { user } = setup(undefined, onCreate)

    await user.click(screen.getByRole('button', { name: 'Add skill' }))
    await user.type(screen.getByLabelText('Skill name'), 'Rust')
    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(onCreate).not.toHaveBeenCalled()
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('closes on Escape', async () => {
    const { user } = setup()
    await user.click(screen.getByRole('button', { name: 'Add skill' }))
    await user.keyboard('{Escape}')
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  })

  it('keeps the dialog open and reports a duplicate name', async () => {
    const onCreate = vi.fn().mockRejectedValue(new ApiError(409, 'already exists'))
    const { user } = setup(undefined, onCreate)

    await user.click(screen.getByRole('button', { name: 'Add skill' }))
    await user.type(screen.getByLabelText('Skill name'), 'Go')
    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Add skill' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('already exists')
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    // What was typed survives so it can be corrected.
    expect(screen.getByLabelText('Skill name')).toHaveValue('Go')
  })

  it('submits on Enter', async () => {
    const onCreate = vi.fn().mockResolvedValue(undefined)
    const { user } = setup(undefined, onCreate)

    await user.click(screen.getByRole('button', { name: 'Add skill' }))
    await user.type(screen.getByLabelText('Skill name'), 'Rust{Enter}')

    expect(onCreate).toHaveBeenCalledWith({ name: 'Rust' })
  })
})

describe('RetentionSetting', () => {
  function setupRetention(days = 7, onSave = vi.fn().mockResolvedValue(undefined)) {
    render(<RetentionSetting days={days} onSave={onSave} />)
    return { onSave, user: userEvent.setup() }
  }

  it('shows the current value', () => {
    setupRetention(30)
    expect(screen.getByLabelText('History retention (days)')).toHaveValue(30)
  })

  it('keeps Save disabled until the value changes', async () => {
    const { user } = setupRetention(7)
    const save = screen.getByRole('button', { name: 'Save' })
    expect(save).toBeDisabled()

    const input = screen.getByLabelText('History retention (days)')
    await user.clear(input)
    await user.type(input, '30')
    expect(save).toBeEnabled()
  })

  it('saves the new window', async () => {
    const { onSave, user } = setupRetention(7)
    const input = screen.getByLabelText('History retention (days)')
    await user.clear(input)
    await user.type(input, '30')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(onSave).toHaveBeenCalledWith(30)
  })

  it.each([['0'], ['400'], ['-5'], ['abc']])('refuses to save %s', async (value) => {
    const { onSave, user } = setupRetention(7)
    const input = screen.getByLabelText('History retention (days)')
    await user.clear(input)
    await user.type(input, value)

    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()
    expect(onSave).not.toHaveBeenCalled()
  })

  it('explains why an out-of-range value is refused', async () => {
    const { user } = setupRetention(7)
    const input = screen.getByLabelText('History retention (days)')
    await user.clear(input)
    await user.type(input, '999')
    expect(screen.getByText(/between 1 and 365/i)).toBeInTheDocument()
  })

  it('surfaces a server rejection', async () => {
    const onSave = vi
      .fn()
      .mockRejectedValue(
        new ApiError(422, 'validation failed', {
          history_retention_days: 'must be between 1 and 365',
        }),
      )
    const { user } = setupRetention(7, onSave)
    const input = screen.getByLabelText('History retention (days)')
    await user.clear(input)
    await user.type(input, '30')
    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('must be between 1 and 365')
  })
})
