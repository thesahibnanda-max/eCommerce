import { useEffect } from 'react'
import type { Product } from '@/shared/api/types'
import { useInView } from '@/shared/hooks/useInView'
import { Button } from './Button'
import { ProductCard, ProductCardSkeleton } from './ProductCard'
import styles from './ProductGrid.module.css'

type Props = {
  products: Product[]
  loading: boolean
  hasMore: boolean
  loadingMore: boolean
  onLoadMore: () => void
  error?: string | null
  onRetry?: () => void
}

export function ProductGrid({
  products,
  loading,
  hasMore,
  loadingMore,
  onLoadMore,
  error,
  onRetry,
}: Props) {
  const [sentinel, inView] = useInView<HTMLDivElement>('600px')
  useEffect(() => {
    if (inView && hasMore && !loadingMore && !error) onLoadMore()
  }, [inView, hasMore, loadingMore, onLoadMore, error])

  if (loading) {
    return (
      <div className={styles.grid} aria-busy>
        {Array.from({ length: 8 }, (_, i) => (
          <ProductCardSkeleton key={i} />
        ))}
      </div>
    )
  }
  return (
    <>
      <div className={styles.grid}>
        {products.map((product, index) => (
          <ProductCard key={product.id} product={product} index={index} priority={index < 4} />
        ))}
        {loadingMore &&
          Array.from({ length: 2 }, (_, i) => <ProductCardSkeleton key={`more-${i}`} />)}
      </div>
      <div ref={sentinel} className={styles.sentinel} />
      {error && (
        <div className={styles.status}>
          <Button variant="ghost" onClick={onRetry}>
            Try again
          </Button>
        </div>
      )}
      {!hasMore && products.length > 0 && (
        <div className={styles.status}>✿ that's everything, for now ✿</div>
      )}
    </>
  )
}
