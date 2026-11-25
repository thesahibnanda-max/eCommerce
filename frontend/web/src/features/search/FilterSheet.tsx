import { useState } from 'react'
import { useCategories } from '@/shared/api/queries'
import type { SearchParams } from '@/shared/api/types'
import { titleCase } from '@/shared/lib/format'
import { Button } from '@/shared/ui/Button'
import { Chip } from '@/shared/ui/Chip'
import { Sheet } from '@/shared/ui/Sheet'
import { TextField } from '@/shared/ui/TextField'
import { sortLabels, sortOptions } from './searchParams'
import styles from './SearchPage.module.css'

type Props = {
  open: boolean
  onOpenChange: (open: boolean) => void
  params: SearchParams
  onApply: (params: SearchParams) => void
}

function parsePrice(value: string): number | null {
  if (value.trim() === '') return null
  const parsed = Number(value)
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : null
}

export function FilterSheet({ open, onOpenChange, params, onApply }: Props) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange} title="Filters">
      {open && <FilterBody params={params} onApply={onApply} onClose={() => onOpenChange(false)} />}
    </Sheet>
  )
}

export function FilterPanel({
  params,
  onApply,
}: {
  params: SearchParams
  onApply: (params: SearchParams) => void
}) {
  return (
    <div className={styles.panel}>
      <h2 className={styles.panelTitle}>Filters ✿</h2>
      <FilterBody
        key={JSON.stringify(params)}
        params={params}
        onApply={onApply}
        onClose={() => undefined}
      />
    </div>
  )
}

function FilterBody({
  params,
  onApply,
  onClose,
}: {
  params: SearchParams
  onApply: (params: SearchParams) => void
  onClose: () => void
}) {
  const categories = useCategories()
  const [draft, setDraft] = useState(params)
  const [min, setMin] = useState(params.minPrice === null ? '' : String(params.minPrice))
  const [max, setMax] = useState(params.maxPrice === null ? '' : String(params.maxPrice))

  const apply = () => {
    let minPrice = parsePrice(min)
    let maxPrice = parsePrice(max)
    if (minPrice !== null && maxPrice !== null && minPrice > maxPrice)
      [minPrice, maxPrice] = [maxPrice, minPrice]
    onApply({ ...draft, minPrice, maxPrice })
    onClose()
  }

  const reset = () => {
    onApply({
      text: params.text,
      categories: [],
      minPrice: null,
      maxPrice: null,
      inStockOnly: false,
      sort: 'relevance',
    })
    onClose()
  }

  return (
    <>
      <div className={styles.sheetBody}>
        <section className={styles.group}>
          <h3>Sort by</h3>
          <div className={styles.wrap}>
            {sortOptions.map((sort) => (
              <Chip
                key={sort}
                active={draft.sort === sort}
                onClick={() => setDraft({ ...draft, sort })}
              >
                {sortLabels[sort]}
              </Chip>
            ))}
          </div>
        </section>

        {categories.data && categories.data.length > 0 && (
          <section className={styles.group}>
            <h3>Categories</h3>
            <div className={styles.wrap}>
              {categories.data.map((item) => {
                const active = draft.categories.includes(item.category)
                return (
                  <Chip
                    key={item.category}
                    active={active}
                    count={item.count}
                    onClick={() =>
                      setDraft({
                        ...draft,
                        categories: active
                          ? draft.categories.filter((c) => c !== item.category)
                          : [...draft.categories, item.category],
                      })
                    }
                  >
                    {titleCase(item.category)}
                  </Chip>
                )
              })}
            </div>
          </section>
        )}

        <section className={styles.group}>
          <h3>Price (₹)</h3>
          <div className={styles.priceRow}>
            <TextField
              label="Min"
              inputMode="numeric"
              placeholder="0"
              value={min}
              onChange={(e) => setMin(e.target.value)}
            />
            <TextField
              label="Max"
              inputMode="numeric"
              placeholder="Any"
              value={max}
              onChange={(e) => setMax(e.target.value)}
            />
          </div>
        </section>

        <section className={styles.group}>
          <label className={styles.toggle}>
            <span>
              <strong>In stock only</strong>
              <small>Hide pieces that are sold out</small>
            </span>
            <input
              type="checkbox"
              checked={draft.inStockOnly}
              onChange={(e) => setDraft({ ...draft, inStockOnly: e.target.checked })}
            />
          </label>
        </section>
      </div>
      <div className={styles.sheetFooter}>
        <Button variant="ghost" onClick={reset}>
          Reset
        </Button>
        <Button variant="accent" block onClick={apply}>
          Show results
        </Button>
      </div>
    </>
  )
}
