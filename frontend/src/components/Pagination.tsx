interface Props {
  page: number
  totalPages: number
  onChange: (page: number) => void
  /** Shown beside the controls when there is room. */
  totalLabel?: string
}

/** A gap in the number run, rendered as an ellipsis. */
const GAP = 'gap' as const
type Slot = number | typeof GAP

/**
 * Builds the run of page numbers: always the first and last page, the current page
 * with one neighbour either side, and an ellipsis wherever the sequence jumps.
 * With seven pages or fewer everything fits, so no gap is needed.
 */
export function pageSlots(page: number, totalPages: number): Slot[] {
  if (totalPages <= 7) {
    return Array.from({ length: totalPages }, (_, i) => i + 1)
  }

  const slots: Slot[] = [1]
  let start = Math.max(2, page - 1)
  let end = Math.min(totalPages - 1, page + 1)

  // Near either end there is no gap to absorb, so widen the run on the other side
  // and keep the control a steady width instead of collapsing to "1 2 … 10".
  if (page <= 3) {
    start = 2
    end = Math.min(totalPages - 1, 3)
  } else if (page >= totalPages - 2) {
    start = Math.max(2, totalPages - 2)
    end = totalPages - 1
  }

  if (start > 2) slots.push(GAP)
  for (let i = start; i <= end; i++) slots.push(i)
  if (end < totalPages - 1) slots.push(GAP)

  slots.push(totalPages)
  return slots
}

export function Pagination({ page, totalPages, onChange, totalLabel }: Props) {
  if (totalPages <= 1) return null

  const slots = pageSlots(page, totalPages)

  return (
    <nav
      aria-label="History pages"
      className="mt-4 flex flex-col gap-3 border-t border-slate-100 pt-4 sm:flex-row sm:items-center sm:justify-between"
    >
      <div className="flex items-center justify-center gap-1">
        <button
          type="button"
          onClick={() => onChange(page - 1)}
          disabled={page <= 1}
          aria-label="Previous page"
          className="flex h-10 min-w-[40px] items-center justify-center rounded-lg border border-slate-300 px-2 text-sm font-medium text-slate-600 hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-40"
        >
          <span aria-hidden="true">‹</span>
        </button>

        {slots.map((slot, i) =>
          slot === GAP ? (
            <span
              // Gaps have no page number of their own, so the index is the only key.
              key={`gap-${i}`}
              aria-hidden="true"
              className="flex h-10 w-8 items-center justify-center text-sm text-slate-400"
            >
              …
            </span>
          ) : (
            <button
              key={slot}
              type="button"
              onClick={() => onChange(slot)}
              aria-label={`Page ${slot}`}
              aria-current={slot === page ? 'page' : undefined}
              className={`h-10 min-w-[40px] rounded-lg px-2 text-sm font-medium transition ${
                slot === page
                  ? 'bg-brand-600 text-white'
                  : 'border border-slate-300 text-slate-700 hover:bg-slate-50'
              }`}
            >
              {slot}
            </button>
          ),
        )}

        <button
          type="button"
          onClick={() => onChange(page + 1)}
          disabled={page >= totalPages}
          aria-label="Next page"
          className="flex h-10 min-w-[40px] items-center justify-center rounded-lg border border-slate-300 px-2 text-sm font-medium text-slate-600 hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-40"
        >
          <span aria-hidden="true">›</span>
        </button>
      </div>

      {totalLabel && (
        <p aria-live="polite" className="text-center text-sm text-slate-500 sm:text-right">
          {totalLabel}
        </p>
      )}
    </nav>
  )
}
