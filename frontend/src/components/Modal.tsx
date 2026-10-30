import { useEffect, useId, type ReactNode } from 'react'
import { createPortal } from 'react-dom'

interface Props {
  title: string
  onClose: () => void
  /** While true, Escape and backdrop clicks are ignored so a request is not orphaned. */
  busy?: boolean
  children: ReactNode
  /** Rendered right-aligned at the foot of the dialog. */
  footer?: ReactNode
}

/**
 * A dialog rendered into <body>.
 *
 * The portal is not cosmetic: dialogs are opened from inside cards that sit in a
 * sticky column, and a sticky element creates its own stacking context — a z-index
 * applied inside it cannot lift the dialog above the rest of the page.
 */
export function Modal({ title, onClose, busy = false, children, footer }: Props) {
  const titleId = useId()

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape' && !busy) onClose()
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [onClose, busy])

  return createPortal(
    <div
      className="fixed inset-0 z-50 flex items-end justify-center bg-slate-900/50 p-4 sm:items-center"
      onClick={(e) => {
        if (e.target === e.currentTarget && !busy) onClose()
      }}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        className="w-full max-w-sm rounded-2xl bg-white p-5 shadow-xl sm:p-6"
      >
        <h2 id={titleId} className="text-base font-semibold text-slate-900">
          {title}
        </h2>

        <div className="mt-5">{children}</div>

        {footer && (
          <div className="mt-5 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            {footer}
          </div>
        )}
      </div>
    </div>,
    document.body,
  )
}
