import clsx from 'clsx'
import { useToastStore } from '@/shared/lib/toastStore'
import styles from './Toaster.module.css'

export function Toaster() {
  const toasts = useToastStore((state) => state.toasts)
  const dismiss = useToastStore((state) => state.dismiss)
  return (
    <div className={styles.stack} role="status" aria-live="polite">
      {toasts.map((item) => (
        <button
          key={item.id}
          type="button"
          className={clsx(
            styles.toast,
            item.tone === 'success' && styles.success,
            item.tone === 'error' && styles.error,
          )}
          onClick={() => dismiss(item.id)}
        >
          <span className={styles.icon} aria-hidden>
            {item.tone === 'success' ? '✿' : item.tone === 'error' ? '!' : '♡'}
          </span>
          {item.message}
        </button>
      ))}
    </div>
  )
}
