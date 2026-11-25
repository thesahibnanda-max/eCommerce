import { Truck } from 'lucide-react'
import { useMemo, useState } from 'react'
import { Link, useLocation, useParams } from 'react-router'
import { errorMessage, isApiError } from '@/shared/api/errors'
import { useProduct, useRecommendations } from '@/shared/api/queries'
import type { Product, Variant } from '@/shared/api/types'
import { useCart } from '@/shared/cart/useCart'
import { capFor } from '@/shared/cart/localCartStore'
import { titleCase } from '@/shared/lib/format'
import { primaryVariant, productImages } from '@/shared/lib/product'
import { toast } from '@/shared/lib/toastStore'
import { Button } from '@/shared/ui/Button'
import { Heart, Sparkle, Sticker } from '@/shared/ui/doodles'
import { EmptyState } from '@/shared/ui/EmptyState'
import { Price } from '@/shared/ui/Price'
import { ProductCard } from '@/shared/ui/ProductCard'
import { QuantityStepper } from '@/shared/ui/QuantityStepper'
import { Skeleton } from '@/shared/ui/Skeleton'
import { Gallery } from './Gallery'
import {
  findVariant,
  initialSelection,
  isOptionAvailable,
  optionGroups,
  selectOption,
  splitGroups,
  type Selection,
} from './variants'
import styles from './ProductPage.module.css'

function placeholderFrom(state: unknown): Product | undefined {
  if (state && typeof state === 'object' && 'product' in state)
    return (state as { product: Product }).product
  return undefined
}

export default function ProductPage() {
  const { id = '' } = useParams()
  const location = useLocation()
  const query = useProduct(id, placeholderFrom(location.state))
  const product = query.data

  if (query.isError && !product) {
    const notFound =
      isApiError(query.error, 'product_not_found') || isApiError(query.error, 'id_invalid')
    return (
      <EmptyState
        mood={notFound ? 'sleepy' : 'surprised'}
        title={notFound ? 'This piece has moved on' : 'Could not load this piece'}
        message={notFound ? 'It may have sold out or been taken down.' : errorMessage(query.error)}
        action={
          <Link to="/">
            <Button variant="accent">Keep browsing</Button>
          </Link>
        }
      />
    )
  }
  if (!product) return <ProductSkeleton />
  return <ProductDetail key={product.id} product={product} />
}

function ProductDetail({ product }: { product: Product }) {
  const groups = useMemo(() => optionGroups(product), [product])
  const { choices, details } = useMemo(() => splitGroups(groups), [groups])
  const [selection, setSelection] = useState<Selection>(() => initialSelection(product))
  const [skuVariant, setSkuVariant] = useState<Variant | undefined>(() => primaryVariant(product))
  const variant = groups.length > 0 ? findVariant(product, selection) : skuVariant
  const [requested, setQuantity] = useState(1)
  const cart = useCart()
  const [adding, setAdding] = useState(false)
  const images = productImages(product, variant)
  const max = variant ? capFor(variant.stock_units) : 0
  const quantity = Math.max(1, Math.min(requested, max || 1))

  const add = async () => {
    if (!variant) return
    setAdding(true)
    const ok = await cart.add(product, variant, quantity)
    setAdding(false)
    if (ok) toast(`Added ${product.name} to your bag`, 'success')
  }

  return (
    <article className={styles.page}>
      <div className={styles.galleryCol}>
        <Gallery images={images} alt={product.name} />
        <Sticker className={styles.galleryHeart} tilt={-14}>
          <Heart size={36} />
        </Sticker>
        <Sticker className={styles.gallerySparkle} tilt={10} delay={0.9}>
          <Sparkle size={30} />
        </Sticker>
      </div>

      <div className={styles.info}>
        <header className={styles.header}>
          {product.categories && product.categories.length > 0 && (
            <div className={styles.categories}>
              {product.categories.map((category) => (
                <Link
                  key={category}
                  to={`/c/${encodeURIComponent(category)}`}
                  className={styles.category}
                >
                  {titleCase(category)}
                </Link>
              ))}
            </div>
          )}
          <h1 className={styles.name}>{product.name}</h1>
          {variant ? (
            <Price
              selling={variant.selling_price}
              shown={variant.shown_price}
              currency={variant.currency}
              large
            />
          ) : (
            <span className={styles.muted}>Choose your options</span>
          )}
          {variant && <StockNote stock={variant.stock_units} />}
        </header>

        {choices.map((group) => (
          <section key={group.name} className={styles.options}>
            <h2 className={styles.optionTitle}>
              {titleCase(group.name)}
              {selection[group.name] && (
                <span className={styles.optionValue}>{selection[group.name]}</span>
              )}
            </h2>
            <div className={styles.optionList}>
              {group.values.map((value) => {
                const available = isOptionAvailable(product, selection, group.name, value)
                const active = selection[group.name] === value
                return (
                  <button
                    key={value}
                    type="button"
                    className={styles.option}
                    data-active={active || undefined}
                    data-unavailable={!available || undefined}
                    aria-pressed={active}
                    onClick={() =>
                      setSelection(selectOption(product, selection, group.name, value))
                    }
                  >
                    {value}
                  </button>
                )
              })}
            </div>
          </section>
        ))}

        {groups.length === 0 && product.variants.length > 1 && (
          <section className={styles.options}>
            <h2 className={styles.optionTitle}>Option</h2>
            <div className={styles.optionList}>
              {product.variants.map((item) => (
                <button
                  key={item.id}
                  type="button"
                  className={styles.option}
                  data-active={skuVariant?.id === item.id || undefined}
                  data-unavailable={item.stock_units === 0 || undefined}
                  aria-pressed={skuVariant?.id === item.id}
                  onClick={() => setSkuVariant(item)}
                >
                  {item.sku}
                </button>
              ))}
            </div>
          </section>
        )}

        <div className={styles.actionBar}>
          <div className={styles.actionInner}>
            {variant && max > 0 && (
              <QuantityStepper
                value={quantity}
                max={max}
                label={product.name}
                onChange={(next) => setQuantity(Math.max(1, Math.min(next, max)))}
              />
            )}
            <Button
              variant="accent"
              size="lg"
              block
              onClick={add}
              loading={adding}
              disabled={!variant || max === 0}
            >
              {!variant ? 'Select options' : max === 0 ? 'Sold out' : 'Add to bag ♡'}
            </Button>
          </div>
        </div>

        <p className={styles.promise}>
          <Truck size={18} /> Packed with care and shipped across India
        </p>

        {details.length > 0 && (
          <section className={styles.details}>
            <h2 className={styles.optionTitle}>Details</h2>
            <dl className={styles.detailList}>
              {details.map((item) => (
                <div key={item.name} className={styles.detailRow}>
                  <dt>{titleCase(item.name)}</dt>
                  <dd>{item.value}</dd>
                </div>
              ))}
            </dl>
          </section>
        )}

        {product.description && (
          <section className={styles.description}>
            <h2 className={styles.optionTitle}>The story</h2>
            <p>{product.description}</p>
          </section>
        )}
      </div>

      <Recommendations productID={product.id} />
    </article>
  )
}

function StockNote({ stock }: { stock: number }) {
  if (stock === 0) return <span className={styles.soldOut}>Sold out</span>
  if (stock <= 5) return <span className={styles.low}>Only {stock} left</span>
  return <span className={styles.inStock}>In stock, ready to ship</span>
}

function Recommendations({ productID }: { productID: string }) {
  const query = useRecommendations(productID)
  if (!query.data || query.data.length === 0) return null
  return (
    <section className={styles.recs}>
      <h2 className={`${styles.recsTitle} squiggle`}>You might also love</h2>
      <div className={styles.recsRail}>
        {query.data.map((item, index) => (
          <div key={item.id} className={styles.recsItem}>
            <ProductCard product={item} index={index + 1} />
          </div>
        ))}
      </div>
    </section>
  )
}

function ProductSkeleton() {
  return (
    <div className={styles.page} aria-busy>
      <div className={styles.galleryCol}>
        <Skeleton style={{ aspectRatio: '1 / 1', borderRadius: 'var(--radius-xl)' }} />
      </div>
      <div className={styles.info}>
        <Skeleton style={{ height: 28, width: '70%' }} />
        <Skeleton style={{ height: 22, width: '40%' }} />
        <Skeleton style={{ height: 44, width: '100%' }} />
      </div>
    </div>
  )
}
