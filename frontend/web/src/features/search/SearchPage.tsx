import { SlidersHorizontal, Search as SearchIcon, X } from 'lucide-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useSearchParams } from 'react-router'
import { errorMessage } from '@/shared/api/errors'
import { useSearchFeed } from '@/shared/api/queries'
import type { SearchParams } from '@/shared/api/types'
import { useDebouncedValue } from '@/shared/hooks/useDebouncedValue'
import { titleCase } from '@/shared/lib/format'
import { Chip } from '@/shared/ui/Chip'
import { EmptyState } from '@/shared/ui/EmptyState'
import { ProductGrid } from '@/shared/ui/ProductGrid'
import { FilterPanel, FilterSheet } from './FilterSheet'
import { activeFilterCount, fromURL, toQuery, toURL } from './searchParams'
import styles from './SearchPage.module.css'

export default function SearchPage() {
  const [urlParams, setURLParams] = useSearchParams()
  const params = useMemo(() => fromURL(urlParams), [urlParams])
  const [text, setText] = useState(params.text)
  const debouncedText = useDebouncedValue(text, 300)
  const [filtersOpen, setFiltersOpen] = useState(false)

  const update = useCallback(
    (next: SearchParams) => setURLParams(toURL(next), { replace: true }),
    [setURLParams],
  )

  useEffect(() => {
    if (debouncedText !== params.text) update({ ...params, text: debouncedText })
  }, [debouncedText, params, update])

  const feed = useSearchFeed(toQuery(params))
  const products = useMemo(
    () => feed.data?.pages.flatMap((page) => page.products) ?? [],
    [feed.data],
  )
  const facets = feed.data?.pages[0]?.category_facets ?? []
  const total = feed.data?.pages[0]?.total ?? 0
  const suggestion = feed.data?.pages[0]?.suggestion ?? ''
  const suggested = feed.data?.pages[0]?.suggested_products ?? []
  const filterCount = activeFilterCount(params)
  const { fetchNextPage } = feed
  const loadMore = useCallback(() => void fetchNextPage(), [fetchNextPage])

  const toggleCategory = (category: string) => {
    const categories = params.categories.includes(category)
      ? params.categories.filter((c) => c !== category)
      : [...params.categories, category]
    update({ ...params, categories })
  }

  return (
    <div className={styles.page}>
      <aside className={styles.sidebar}>
        <FilterPanel params={params} onApply={update} />
      </aside>
      <div className={styles.results}>
        <div className={styles.heading}>
          <h1 className={styles.title}>
            {params.text ? 'Searching for treasures' : 'Shop everything'}
          </h1>
          <p className={styles.subtitle}>find your next little obsession ♡</p>
        </div>
        <div className={styles.searchRow}>
          <label className={styles.searchBox}>
            <SearchIcon size={18} className={styles.searchIcon} />
            <input
              type="search"
              enterKeyHint="search"
              placeholder="Search sarees, kurtas, decor..."
              value={text}
              onChange={(event) => setText(event.target.value)}
              aria-label="Search products"
            />
            {text && (
              <button
                type="button"
                className={styles.clear}
                onClick={() => setText('')}
                aria-label="Clear search"
              >
                <X size={16} />
              </button>
            )}
          </label>
          <button
            type="button"
            className={styles.filterButton}
            onClick={() => setFiltersOpen(true)}
            aria-label="Filters"
          >
            <SlidersHorizontal size={18} />
            {filterCount > 0 && <span className={styles.filterBadge}>{filterCount}</span>}
          </button>
        </div>

        {facets.length > 0 && (
          <div className={styles.rail}>
            {facets.map((facet) => (
              <Chip
                key={facet.category}
                active={params.categories.includes(facet.category)}
                count={facet.count}
                onClick={() => toggleCategory(facet.category)}
              >
                {titleCase(facet.category)}
              </Chip>
            ))}
          </div>
        )}

        {feed.data && (
          <p className={styles.total}>
            {total} {total === 1 ? 'result' : 'results'}
            {params.text && <> for "{params.text}"</>}
          </p>
        )}

        {!feed.isPending && !feed.isError && products.length === 0 ? (
          <div className={styles.noResults}>
            <EmptyState
              mood="surprised"
              tone="lilac"
              title={params.text ? `Couldn't find "${params.text}"` : "Couldn't find anything"}
              message={
                filterCount > 0
                  ? 'Try fewer filters or a different word.'
                  : 'Check the spelling or try a simpler word, like kurta or saree.'
              }
              action={
                <div className={styles.noResultsActions}>
                  {suggestion && (
                    <Chip onClick={() => setText(suggestion)}>
                      Did you mean <strong>{suggestion}</strong>?
                    </Chip>
                  )}
                  {filterCount > 0 && (
                    <Chip
                      onClick={() =>
                        update({
                          ...params,
                          categories: [],
                          minPrice: null,
                          maxPrice: null,
                          inStockOnly: false,
                        })
                      }
                    >
                      Clear filters
                    </Chip>
                  )}
                </div>
              }
            />
            {suggested.length > 0 && (
              <section className={styles.suggested}>
                <h2 className={styles.suggestedTitle}>You might like these</h2>
                <ProductGrid
                  products={suggested}
                  loading={false}
                  hasMore={false}
                  loadingMore={false}
                  onLoadMore={loadMore}
                />
              </section>
            )}
          </div>
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

      <FilterSheet
        open={filtersOpen}
        onOpenChange={setFiltersOpen}
        params={params}
        onApply={update}
      />
    </div>
  )
}
