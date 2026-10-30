import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { Pagination, pageSlots } from './Pagination'

describe('pageSlots', () => {
  it('lists every page when they all fit', () => {
    expect(pageSlots(1, 1)).toEqual([1])
    expect(pageSlots(3, 7)).toEqual([1, 2, 3, 4, 5, 6, 7])
  })

  it('puts a gap after the first page when the current page is far along', () => {
    expect(pageSlots(8, 10)).toEqual([1, 'gap', 8, 9, 10])
  })

  it('puts a gap before the last page when the current page is near the start', () => {
    expect(pageSlots(2, 10)).toEqual([1, 2, 3, 'gap', 10])
  })

  it('puts a gap on both sides in the middle of a long run', () => {
    expect(pageSlots(5, 10)).toEqual([1, 'gap', 4, 5, 6, 'gap', 10])
  })

  it('keeps a steady run of numbers at either end', () => {
    // Without widening, page 1 would collapse to "1 2 … 10".
    expect(pageSlots(1, 10)).toEqual([1, 2, 3, 'gap', 10])
    expect(pageSlots(10, 10)).toEqual([1, 'gap', 8, 9, 10])
  })

  it('always includes the first and last page', () => {
    for (const page of [1, 2, 5, 9, 10]) {
      const slots = pageSlots(page, 10)
      expect(slots[0]).toBe(1)
      expect(slots[slots.length - 1]).toBe(10)
    }
  })

  it('never repeats a page number', () => {
    for (const page of [1, 2, 3, 8, 9, 10]) {
      const numbers = pageSlots(page, 10).filter((s): s is number => s !== 'gap')
      expect(new Set(numbers).size).toBe(numbers.length)
    }
  })
})

function setup(page: number, totalPages: number, onChange = vi.fn()) {
  render(
    <Pagination page={page} totalPages={totalPages} onChange={onChange} totalLabel="46 changes" />,
  )
  return { onChange, user: userEvent.setup() }
}

describe('Pagination', () => {
  it('renders nothing when there is only one page', () => {
    const { container } = render(<Pagination page={1} totalPages={1} onChange={vi.fn()} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('shows a numbered button per page', () => {
    setup(1, 3)
    expect(screen.getByRole('button', { name: 'Page 1' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Page 2' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Page 3' })).toBeInTheDocument()
  })

  it('marks the current page for assistive tech', () => {
    setup(2, 3)
    expect(screen.getByRole('button', { name: 'Page 2' })).toHaveAttribute('aria-current', 'page')
    expect(screen.getByRole('button', { name: 'Page 1' })).not.toHaveAttribute('aria-current')
  })

  it('jumps straight to a page number', async () => {
    const { onChange, user } = setup(1, 5)
    await user.click(screen.getByRole('button', { name: 'Page 4' }))
    expect(onChange).toHaveBeenCalledWith(4)
  })

  it('steps with the arrows', async () => {
    const { onChange, user } = setup(2, 5)
    await user.click(screen.getByRole('button', { name: 'Next page' }))
    expect(onChange).toHaveBeenCalledWith(3)

    await user.click(screen.getByRole('button', { name: 'Previous page' }))
    expect(onChange).toHaveBeenCalledWith(1)
  })

  it('disables Previous on the first page and Next on the last', () => {
    const { container } = render(<Pagination page={1} totalPages={3} onChange={vi.fn()} />)
    expect(screen.getByRole('button', { name: 'Previous page' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Next page' })).toBeEnabled()
    container.remove()

    render(<Pagination page={3} totalPages={3} onChange={vi.fn()} />)
    expect(screen.getByRole('button', { name: 'Next page' })).toBeDisabled()
  })

  it('collapses a long run with an ellipsis', () => {
    setup(10, 25)
    expect(screen.getByRole('button', { name: 'Page 1' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Page 25' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Page 15' })).not.toBeInTheDocument()
    // Only a handful of buttons, however many pages there are.
    expect(screen.getAllByRole('button').length).toBeLessThan(10)
  })

  it('shows the total alongside the controls', () => {
    setup(1, 3)
    expect(screen.getByText('46 changes')).toBeInTheDocument()
  })
})
