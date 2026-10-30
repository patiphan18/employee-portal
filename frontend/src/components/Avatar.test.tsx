import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Avatar } from './Avatar'
import { CompletenessMeter } from './CompletenessMeter'

describe('Avatar', () => {
  const person = { first_name: 'Somchai', last_name: 'Jaidee' }

  it('renders the photo when there is one', () => {
    render(<Avatar profile={{ ...person, avatar_url: '/uploads/a.png' }} />)
    const img = screen.getByRole('img', { name: 'Somchai Jaidee' })
    expect(img).toHaveAttribute('src', '/uploads/a.png')
  })

  it('falls back to initials when there is no photo', () => {
    render(<Avatar profile={{ ...person, avatar_url: null }} />)
    expect(screen.getByText('SJ')).toBeInTheDocument()
    expect(screen.getByRole('img', { name: /no profile photo/i })).toBeInTheDocument()
  })

  it('falls back to initials when the image fails to load', () => {
    render(<Avatar profile={{ ...person, avatar_url: '/uploads/missing.png' }} />)
    fireEvent.error(screen.getByRole('img', { name: 'Somchai Jaidee' }))
    expect(screen.getByText('SJ')).toBeInTheDocument()
  })
})

describe('CompletenessMeter', () => {
  it('reports the percentage on an accessible progress bar', () => {
    render(<CompletenessMeter completeness={{ percent: 60, missing: ['avatar', 'address'] }} />)

    const bar = screen.getByRole('progressbar', { name: 'Profile completeness' })
    expect(bar).toHaveAttribute('aria-valuenow', '60')
    expect(screen.getByText('60%')).toBeInTheDocument()
  })

  it('lists what is still missing, in words', () => {
    render(<CompletenessMeter completeness={{ percent: 60, missing: ['avatar', 'skills'] }} />)
    expect(screen.getByText('Add a profile photo')).toBeInTheDocument()
    expect(screen.getByText('Select at least one skill')).toBeInTheDocument()
  })

  it('congratulates a complete profile instead of listing nothing', () => {
    render(<CompletenessMeter completeness={{ percent: 100, missing: [] }} />)
    expect(screen.getByText('Your profile is complete.')).toBeInTheDocument()
    expect(screen.queryByRole('list')).not.toBeInTheDocument()
  })

  it('handles an empty profile', () => {
    render(
      <CompletenessMeter
        completeness={{ percent: 0, missing: ['avatar', 'mobile', 'email', 'address', 'skills'] }}
      />,
    )
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '0')
    expect(screen.getAllByRole('listitem')).toHaveLength(5)
  })
})
