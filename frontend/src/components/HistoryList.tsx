import { Pagination } from './Pagination'
import { absoluteTime, displayValue, fieldLabel, relativeTime } from '../lib/format'
import type { ProfileChange } from '../types'

interface Props {
  events: ProfileChange[]
  days: number
  page: number
  total: number
  totalPages: number
  loading: boolean
  error: string | null
  onChangePage: (page: number) => void
}

/**
 * The change history is read-only. It records what happened; nothing here replays
 * or rewrites it, which is what makes it worth trusting as an audit trail.
 */
export function HistoryList({
  events,
  days,
  page,
  total,
  totalPages,
  loading,
  error,
  onChangePage,
}: Props) {
  return (
    <div>

      {/* days is 0 until the first response arrives; saying "the last 0 days" for
          a frame is worse than saying nothing. */}
      {days > 0 && (
        <p className="mb-4 text-sm text-slate-500">
          Showing the last {days} day{days === 1 ? '' : 's'}, as set by your administrator.
        </p>
      )}

      {error && (
        <p role="alert" className="text-sm text-red-600">
          {error}
        </p>
      )}

      {loading ? (
        <p className="text-sm text-slate-500">Loading your history…</p>
      ) : events.length === 0 ? (
        <p className="text-sm text-slate-500">No changes in the last {days} days.</p>
      ) : (
        <>
          <ol className="divide-y divide-slate-100">
            {events.map((event) => (
              <li key={event.id} className="py-3">
                <p className="text-sm font-medium text-slate-900">
                  {fieldLabel(event.field)} changed
                </p>
                <p className="mt-1 break-words text-sm text-slate-500">
                  <span className="line-through decoration-slate-300">
                    {displayValue(event.old_value)}
                  </span>
                  <span aria-hidden="true" className="mx-2 text-slate-300">
                    →
                  </span>
                  <span className="text-slate-700">{displayValue(event.new_value)}</span>
                </p>
                <p className="mt-1 text-xs text-slate-400" title={absoluteTime(event.changed_at)}>
                  {relativeTime(event.changed_at)}
                </p>
              </li>
            ))}
          </ol>

          <Pagination
            page={page}
            totalPages={totalPages}
            onChange={onChangePage}
            totalLabel={`${total} change${total === 1 ? '' : 's'}`}
          />
        </>
      )}
    </div>
  )
}
