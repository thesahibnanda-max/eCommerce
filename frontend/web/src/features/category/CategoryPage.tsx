import { ArrowLeft } from 'lucide-react'
import { useCallback, useMemo } from 'react'
import { useNavigate, useParams } from 'react-router'
import { errorMessage } from '@/shared/api/errors'
import { useCategoryFeed } from '@/shared/api/queries'
import { titleCase } from '@/shared/lib/format'
import { Flower, Sparkle, Sticker } from '@/shared/ui/doodles'
import { EmptyState } from '@/shared/ui/EmptyState'
import { ProductGrid } from '@/shared/ui/ProductGrid'
import styles from './CategoryPage.module.css'

export default function CategoryPage() {
  const { category = '' } = useParams()
  const navigate = useNavigate()
  const feed = useCategoryFeed(category)
  const products = useMemo(
    () => feed.data?.pages.flatMap((page) => page.products) ?? [],
    [feed.data],
  )
  const total = feed.data?.pages[0]?.total
  const { fetchNextPage } = feed
  const loadMore = useCallback(() => void fetchNextPage(), [fetchNextPage])

  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <button
          type="button"
          className={styles.back}
          onClick={() => navigate(-1)}
          aria-label="Go back"
        >
          <ArrowLeft size={20} />
        </button>
        <div>
          <span className={styles.eyebrow}>✿ collection</span>
          <h1 className={styles.title}>{titleCase(category)}</h1>
          {total !== undefined && (
            <span className={styles.count}>
              {total} {total === 1 ? 'piece' : 'pieces'}
            </span>
          )}
        </div>
        <Sticker className={styles.s1} tilt={12}>
          <Flower size={40} />
        </Sticker>
        <Sticker className={styles.s2} tilt={-10} delay={0.8}>
          <Sparkle size={28} />
        </Sticker>
      </header>
      {!feed.isPending && !feed.isError && products.length === 0 ? (
        <EmptyState
          mood="sleepy"
          tone="mint"
          title="Nothing here yet"
          message="This collection is still being curated."
        />
      ) : (
        <ProductGrid
          products={products}
          loading={feed.isPending}
          hasMore={feed.hasNextPage}
          loadingMore={feed.isFetchingNextPage}
          onLoadMore={loadMore}
          error={feed.isError ? errorMessage(feed.error) : null}
          onRetry={() => void feed.refetch()}
        />
      )}
    </div>
  )
}
