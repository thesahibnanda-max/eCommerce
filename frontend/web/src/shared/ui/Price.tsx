import clsx from 'clsx'
import { discountPercent, formatPrice } from '@/shared/lib/format'
import styles from './Price.module.css'

type Props = { selling: number; shown: number; currency: string; large?: boolean }

export function Price({ selling, shown, currency, large }: Props) {
  const off = discountPercent(shown, selling)
  return (
    <span className={clsx(styles.price, large && styles.large)}>
      <span className={styles.selling}>{formatPrice(selling, currency)}</span>
      {off > 0 && (
        <>
          <span className={styles.shown}>{formatPrice(shown, currency)}</span>
          <span className={styles.off}>{off}% off</span>
        </>
      )}
    </span>
  )
}
