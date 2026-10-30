import { MISSING_LABELS } from '../lib/format'
import type { Completeness } from '../types'

export function CompletenessMeter({ completeness }: { completeness: Completeness }) {
  const { percent, missing } = completeness
  const complete = percent === 100

  return (
    <div>
      <div className="flex items-baseline justify-between">
        <h3 className="text-sm font-semibold text-slate-700">Profile completeness</h3>
        <span className="text-sm font-semibold tabular-nums text-slate-900">{percent}%</span>
      </div>

      <div
        role="progressbar"
        aria-valuenow={percent}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-label="Profile completeness"
        className="mt-2 h-2 w-full overflow-hidden rounded-full bg-slate-200"
      >
        <div
          className={`h-full rounded-full transition-all duration-500 ${
            complete ? 'bg-emerald-500' : 'bg-brand-500'
          }`}
          style={{ width: `${percent}%` }}
        />
      </div>

      {complete ? (
        <p className="mt-3 text-sm text-emerald-600">Your profile is complete.</p>
      ) : (
        <ul className="mt-3 space-y-1">
          {missing.map((key) => (
            <li key={key} className="flex items-start gap-2 text-sm text-slate-500">
              <span aria-hidden="true" className="mt-[2px] text-slate-300">
                ○
              </span>
              {MISSING_LABELS[key] ?? key}
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
