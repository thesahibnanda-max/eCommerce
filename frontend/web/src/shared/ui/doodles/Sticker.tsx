import clsx from 'clsx'
import type { CSSProperties, ReactNode } from 'react'
import styles from './Sticker.module.css'

type Props = {
  children: ReactNode
  tilt?: number
  delay?: number
  float?: boolean
  className?: string
  style?: CSSProperties
}

export function Sticker({ children, tilt = 0, delay = 0, float = true, className, style }: Props) {
  const vars = { '--tilt': `${tilt}deg`, '--delay': `${delay}s`, ...style } as CSSProperties
  return (
    <span
      className={clsx(styles.sticker, float && styles.float, className)}
      style={vars}
      aria-hidden
    >
      {children}
    </span>
  )
}
