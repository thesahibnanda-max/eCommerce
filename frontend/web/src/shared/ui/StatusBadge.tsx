import clsx from 'clsx'
import type { StatusTone } from '@/shared/lib/orderStatus'
import styles from './StatusBadge.module.css'

export function StatusBadge({
  label,
  tone,
  title,
}: {
  label: string
  tone: StatusTone
  title?: string
}) {
  return (
    <span className={clsx(styles.badge, styles[tone])} title={title}>
      <span className={styles.dot} aria-hidden />
      {label}
    </span>
  )
}
