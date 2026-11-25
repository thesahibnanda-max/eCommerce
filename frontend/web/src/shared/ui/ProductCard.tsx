import { useQueryClient } from '@tanstack/react-query'
import clsx from 'clsx'
import { Link } from 'react-router'
import { prefetchProduct } from '@/shared/api/queries'
import type { Product } from '@/shared/api/types'
import { discountPercent } from '@/shared/lib/format'
import { isNewArrival, primaryVariant, productImage } from '@/shared/lib/product'
import { toneAt } from './doodles'
import { Price } from './Price'
import { Skeleton } from './Skeleton'
import { SmartImage } from './SmartImage'
import styles from './ProductCard.module.css'

type Props = { product: Product; priority?: boolean; index?: number }

export function ProductCard({ product, priority, index = 0 }: Props) {
  const client = useQueryClient()
  const variant = primaryVariant(product)
  const inStock = product.variants.some((v) => v.stock_units > 0)
  const off = variant ? discountPercent(variant.shown_price, variant.selling_price) : 0
  return (
    <Link
      to={`/p/${product.id}`}
      className={clsx(styles.card, styles[toneAt(index)])}
      onPointerDown={() => prefetchProduct(client, product.id)}
      state={{ product }}
      style={{ animationDelay: `${Math.min(index, 8) * 40}ms` }}
    >
      <div className={styles.media}>
        <SmartImage src={productImage(product)} alt={product.name} priority={priority ?? false} />
        {!inStock ? (
          <span className={clsx(styles.badge, styles.soldOut)}>sold out</span>
        ) : off >= 5 ? (
          <span className={clsx(styles.badge, styles.sale)}>-{off}%</span>
        ) : isNewArrival(product) ? (
          <span className={clsx(styles.badge, styles.fresh)}>new ✿</span>
        ) : null}
      </div>
      <div className={styles.body}>
        <span className={styles.name}>{product.name}</span>
        {variant && (
          <span className={styles.meta}>
            <Price
              selling={variant.selling_price}
              shown={variant.shown_price}
              currency={variant.currency}
            />
          </span>
        )}
      </div>
    </Link>
  )
}

export function ProductCardSkeleton() {
  return (
    <div className={clsx(styles.card, styles.skeleton)}>
      <Skeleton className={styles.skeletonMedia} />
      <div className={styles.body}>
        <Skeleton className={styles.skeletonLine} />
        <Skeleton className={styles.skeletonShort} />
      </div>
    </div>
  )
}
