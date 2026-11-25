import clsx from 'clsx'
import type { ButtonHTMLAttributes, ReactNode } from 'react'
import styles from './Button.module.css'

type Props = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: 'primary' | 'accent' | 'ghost' | 'plain' | 'danger'
  size?: 'md' | 'lg'
  block?: boolean
  loading?: boolean
  icon?: ReactNode
}

export function Button({
  variant = 'primary',
  size = 'md',
  block,
  loading,
  icon,
  children,
  className,
  disabled,
  type,
  ...rest
}: Props) {
  return (
    <button
      type={type ?? 'button'}
      className={clsx(
        styles.button,
        styles[variant],
        size === 'lg' && styles.lg,
        block && styles.block,
        className,
      )}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      {...rest}
    >
      {loading ? <span className={styles.spinner} aria-hidden /> : icon}
      {children}
    </button>
  )
}
