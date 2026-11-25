import { useCallback, useMemo, useRef } from 'react'
import { Link } from 'react-router'
import { errorMessage } from '@/shared/api/errors'
import { useCategories, useProductsFeed } from '@/shared/api/queries'
import { titleCase } from '@/shared/lib/format'
import { productImage } from '@/shared/lib/product'
import { Cloud, Flower, Heart, Sparkle, Star, Swirl, toneAt } from '@/shared/ui/doodles'
import { EmptyState } from '@/shared/ui/EmptyState'
import { ProductGrid } from '@/shared/ui/ProductGrid'
import { Hero } from './Hero'
import styles from './HomePage.module.css'

const ribbon = ['handpicked with love', 'made in India', 'packed with care', 'new drops weekly']

const moodIcons = [Flower, Heart, Star, Sparkle, Swirl, Cloud]

const perks = [
  {
    icon: Heart,
    tone: 'pink' as const,
    title: 'Handpicked',
    text: 'Every piece is chosen by a real human who loved it first.',
  },
  {
    icon: Flower,
    tone: 'mint' as const,
    title: 'Made in India',
    text: 'Crafted by makers and small studios across the country.',
  },
  {
    icon: Star,
    tone: 'butter' as const,
    title: 'Packed with care',
    text: 'Wrapped up neatly so it reaches you looking its best.',
  },
]

export function HomePage() {
  const feed = useProductsFeed()
  const categories = useCategories()
  const gridRef = useRef<HTMLElement>(null)
  const products = useMemo(
    () => feed.data?.pages.flatMap((page) => page.products) ?? [],
    [feed.data],
  )
  const heroImages = useMemo(
    () => products.map(productImage).filter(Boolean).slice(0, 3),
    [products],
  )
  const { fetchNextPage } = feed
  const loadMore = useCallback(() => void fetchNextPage(), [fetchNextPage])

  return (
    <div className={styles.page}>
      <Hero
        images={heroImages}
        onShop={() => gridRef.current?.scrollIntoView({ behavior: 'smooth', block: 'start' })}
      />

      <div className={styles.ribbon} aria-hidden>
        <div className={styles.track}>
          {[0, 1].map((copy) => (
            <span key={copy} className={styles.ribbonGroup}>
              {ribbon.map((text) => (
                <span key={text} className={styles.ribbonItem}>
                  {text} <span className={styles.ribbonMark}>✿</span>
                </span>
              ))}
            </span>
          ))}
        </div>
      </div>

      {categories.data && categories.data.length > 0 && (
        <section className={styles.section}>
          <h2 className={`${styles.heading} squiggle`}>Shop by mood</h2>
          <div className={styles.moods}>
            {categories.data.map((item, index) => {
              const Icon = moodIcons[index % moodIcons.length] ?? Flower
              const tone = toneAt(index)
              return (
                <Link
                  key={item.category}
                  to={`/c/${encodeURIComponent(item.category)}`}
                  className={`${styles.mood} ${styles[tone] ?? ''}`}
                >
                  <span className={styles.moodIcon}>
                    <Icon size={34} tone={tone} />
                  </span>
                  <span className={styles.moodName}>{titleCase(item.category)}</span>
                  <span className={styles.moodCount}>
                    {item.count} {item.count === 1 ? 'piece' : 'pieces'}
                  </span>
                </Link>
              )
            })}
          </div>
        </section>
      )}

      <section className={styles.perks} aria-label="Why you will love us">
        {perks.map(({ icon: Icon, tone, title, text }) => (
          <div key={title} className={`${styles.perk} ${styles[tone] ?? ''}`}>
            <span className={styles.perkIcon}>
              <Icon size={30} tone={tone} />
            </span>
            <div>
              <h3 className={styles.perkTitle}>{title}</h3>
              <p className={styles.perkText}>{text}</p>
            </div>
          </div>
        ))}
      </section>

      <section ref={gridRef} className={styles.section}>
        <div className={styles.headingRow}>
          <h2 className={`${styles.heading} squiggle`}>New arrivals</h2>
          {feed.data && (
            <span className={styles.count}>{feed.data.pages[0]?.total ?? 0} pieces</span>
          )}
        </div>
        {!feed.isPending && !feed.isError && products.length === 0 ? (
          <EmptyState
            mood="sleepy"
            title="Fresh drops are on the way"
            message="Check back soon, the shelves are being styled."
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
      </section>
    </div>
  )
}
