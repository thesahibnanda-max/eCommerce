import clsx from 'clsx'
import type { ButtonHTMLAttributes } from 'react'
import styles from './Chip.module.css'

type Props = ButtonHTMLAttributes<HTMLButtonElement> & { active?: boolean; count?: number }

export function Chip({ active, count, children, className, ...rest }: Props) {
  return (
    <button
      type="button"
      className={clsx(styles.chip, active && styles.active, className)}
      aria-pressed={active}
      {...rest}
    >
      {children}
      {count !== undefined && <span className={styles.count}>{count}</span>}
    </button>
  )
}
