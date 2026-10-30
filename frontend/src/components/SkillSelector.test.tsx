import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { ApiError } from '../api/client'
import { SkillSelector } from './SkillSelector'
import type { ProfileSkill, Skill } from '../types'

const skill = (id: string, name: string, active = true): Skill => ({
  id,
  name,
  active,
  sort_order: 10,
})

const picked = (id: string, name: string): ProfileSkill => ({ id, name, custom: false })
const own = (name: string): ProfileSkill => ({ id: '', name, custom: true })

const catalog = [skill('s-go', 'Go'), skill('s-react', 'React'), skill('s-cicd', 'CI/CD')]

function setup(
  selected: ProfileSkill[] = [],
  onSave = vi.fn().mockResolvedValue(undefined),
) {
  render(<SkillSelector catalog={catalog} selected={selected} onSave={onSave} />)
  return { onSave, user: userEvent.setup() }
}

describe('SkillSelector', () => {
  it('lists the catalog as a flat set of chips', () => {
    setup()
    expect(screen.getAllByRole('checkbox')).toHaveLength(3)
    expect(screen.getByRole('checkbox', { name: /^Go$/ })).toBeInTheDocument()
  })

  it('pre-checks the skills already on the profile', () => {
    setup([picked('s-go', 'Go')])
    expect(screen.getByRole('checkbox', { name: /^Go$/ })).toBeChecked()
    expect(screen.getByRole('checkbox', { name: /^React$/ })).not.toBeChecked()
  })

  it('keeps Save disabled until the selection changes', async () => {
    const { user } = setup([picked('s-go', 'Go')])
    const save = screen.getByRole('button', { name: 'Save skills' })
    expect(save).toBeDisabled()

    await user.click(screen.getByRole('checkbox', { name: /^React$/ }))
    expect(save).toBeEnabled()

    await user.click(screen.getByRole('checkbox', { name: /^React$/ }))
    expect(save).toBeDisabled()
  })

  it('submits catalog ids and no custom skills', async () => {
    const onSave = vi.fn().mockResolvedValue(undefined)
    const { user } = setup([], onSave)

    await user.click(screen.getByRole('checkbox', { name: /^Go$/ }))
    await user.click(screen.getByRole('button', { name: 'Save skills' }))

    expect(onSave).toHaveBeenCalledWith(['s-go'], [])
  })

  // --- search ---

  it('filters the catalog as the employee types', async () => {
    const { user } = setup()
    await user.type(screen.getByLabelText('Search skills'), 'rea')

    expect(screen.getByRole('checkbox', { name: /^React$/ })).toBeInTheDocument()
    expect(screen.queryByRole('checkbox', { name: /^Go$/ })).not.toBeInTheDocument()
  })

  it('matches case-insensitively', async () => {
    const { user } = setup()
    await user.type(screen.getByLabelText('Search skills'), 'GO')
    expect(screen.getByRole('checkbox', { name: /^Go$/ })).toBeInTheDocument()
  })

  // --- adding your own skill (FR-3.10) ---

  it('offers to add the typed text when nothing matches', async () => {
    const { user } = setup()
    await user.type(screen.getByLabelText('Search skills'), 'Elixir')
    expect(screen.getByRole('button', { name: /add .Elixir. as your own skill/i })).toBeInTheDocument()
  })

  it('does not offer to add a skill the catalog already has', async () => {
    const { user } = setup()
    await user.type(screen.getByLabelText('Search skills'), 'Go')
    expect(screen.queryByRole('button', { name: /as your own skill/i })).not.toBeInTheDocument()
  })

  it('ignores case when deciding the catalog already has it', async () => {
    const { user } = setup()
    await user.type(screen.getByLabelText('Search skills'), 'react')
    expect(screen.queryByRole('button', { name: /as your own skill/i })).not.toBeInTheDocument()
  })

  it('adds the typed skill and submits it separately', async () => {
    const onSave = vi.fn().mockResolvedValue(undefined)
    const { user } = setup([], onSave)

    await user.type(screen.getByLabelText('Search skills'), 'Elixir')
    await user.click(screen.getByRole('button', { name: /as your own skill/i }))

    // It appears under the employee's own skills and the search box is cleared.
    expect(screen.getByText('Your own skills')).toBeInTheDocument()
    expect(screen.getByLabelText('Search skills')).toHaveValue('')

    await user.click(screen.getByRole('button', { name: 'Save skills' }))
    expect(onSave).toHaveBeenCalledWith([], ['Elixir'])
  })

  it('adds the typed skill on Enter', async () => {
    const onSave = vi.fn().mockResolvedValue(undefined)
    const { user } = setup([], onSave)

    await user.type(screen.getByLabelText('Search skills'), 'Elixir{Enter}')
    await user.click(screen.getByRole('button', { name: 'Save skills' }))

    expect(onSave).toHaveBeenCalledWith([], ['Elixir'])
  })

  it('refuses a comma, which the audit log would split', async () => {
    const { user } = setup()
    await user.type(screen.getByLabelText('Search skills'), 'React, Native')

    expect(screen.getByText(/cannot contain a comma/i)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /as your own skill/i })).not.toBeInTheDocument()
  })

  it('shows existing custom skills and can remove one', async () => {
    const onSave = vi.fn().mockResolvedValue(undefined)
    const { user } = setup([own('Elixir'), picked('s-go', 'Go')], onSave)

    expect(screen.getByText('Elixir')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Remove Elixir' }))
    await user.click(screen.getByRole('button', { name: 'Save skills' }))

    expect(onSave).toHaveBeenCalledWith(['s-go'], [])
  })

  it('submits both kinds together', async () => {
    const onSave = vi.fn().mockResolvedValue(undefined)
    const { user } = setup([own('Elixir')], onSave)

    await user.click(screen.getByRole('checkbox', { name: /^Go$/ }))
    await user.click(screen.getByRole('button', { name: 'Save skills' }))

    expect(onSave).toHaveBeenCalledWith(['s-go'], ['Elixir'])
  })

  it('will not add the same custom skill twice', async () => {
    const { user } = setup([own('Elixir')])
    await user.type(screen.getByLabelText('Search skills'), 'elixir')
    expect(screen.queryByRole('button', { name: /as your own skill/i })).not.toBeInTheDocument()
  })

  it('says so when a search matches nothing', async () => {
    const { user } = setup()
    await user.type(screen.getByLabelText('Search skills'), 'zzz')
    expect(screen.getByText(/No skill matches/i)).toBeInTheDocument()
  })

  // --- other behaviour ---

  it('shows a retired skill the employee still holds, marked as such', () => {
    setup([picked('s-flash', 'Flash')])
    expect(screen.getByRole('checkbox', { name: /Flash/ })).toBeChecked()
    expect(screen.getByText('(retired)')).toBeInTheDocument()
  })

  it('resets the draft back to the saved selection', async () => {
    const { user } = setup([picked('s-go', 'Go')])

    await user.click(screen.getByRole('checkbox', { name: /^React$/ }))
    await user.type(screen.getByLabelText('Search skills'), 'Elixir{Enter}')
    await user.click(screen.getByRole('button', { name: 'Reset' }))

    expect(screen.getByRole('checkbox', { name: /^React$/ })).not.toBeChecked()
    expect(screen.getByRole('checkbox', { name: /^Go$/ })).toBeChecked()
    expect(screen.queryByText('Your own skills')).not.toBeInTheDocument()
  })

  it('surfaces a server validation error', async () => {
    const onSave = vi
      .fn()
      .mockRejectedValue(
        new ApiError(422, 'validation failed', { custom_skills: 'name must not contain a comma' }),
      )
    const { user } = setup([], onSave)

    await user.click(screen.getByRole('checkbox', { name: /^Go$/ }))
    await user.click(screen.getByRole('button', { name: 'Save skills' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('name must not contain a comma')
  })

  it('re-syncs when the server returns a different selection', async () => {
    const onSave = vi.fn().mockResolvedValue(undefined)
    const { rerender } = render(
      <SkillSelector catalog={catalog} selected={[picked('s-go', 'Go')]} onSave={onSave} />,
    )
    expect(screen.getByRole('checkbox', { name: /^Go$/ })).toBeChecked()

    rerender(
      <SkillSelector catalog={catalog} selected={[picked('s-react', 'React')]} onSave={onSave} />,
    )

    await waitFor(() => {
      expect(screen.getByRole('checkbox', { name: /^Go$/ })).not.toBeChecked()
      expect(screen.getByRole('checkbox', { name: /^React$/ })).toBeChecked()
    })
  })
})
