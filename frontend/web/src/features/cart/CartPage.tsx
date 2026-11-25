import { Trash2 } from 'lucide-react'
import { Link, useNavigate } from 'react-router'
import { useIsSignedIn } from '@/shared/auth/authStore'
import type { CartLineView } from '@/shared/cart/cartView'
import { useCart } from '@/shared/cart/useCart'
import { discountPercent, formatPrice } from '@/shared/lib/format'
import { Button } from '@/shared/ui/Button'
import { Mascot } from '@/shared/ui/doodles'
import { EmptyState } from '@/shared/ui/EmptyState'
import { Price } from '@/shared/ui/Price'
import { QuantityStepper } from '@/shared/ui/QuantityStepper'
import { Skeleton } from '@/shared/ui/Skeleton'
import { SmartImage } from '@/shared/ui/SmartImage'
import styles from './CartPage.module.css'

export default function CartPage() {
  const { cart, isLoading, isMutating, setQuantity, remove, clear } = useCart()
  const signedIn = useIsSignedIn()
  const navigate = useNavigate()

  if (isLoading) {
    return (
      <div className={styles.page} aria-busy>
        <h1 className={styles.title}>Your bag</h1>
        {Array.from({ length: 3 }, (_, i) => (
          <Skeleton key={i} style={{ height: 112, borderRadius: 'var(--radius-lg)' }} />
        ))}
      </div>
    )
  }

  if (cart.lines.length === 0) {
    return (
      <EmptyState
        mood="sleepy"
        tone="butter"
        title="Your bag is feeling empty"
        message="Find something you love and it will wait for you here."
        action={
          <Link to="/">
            <Button variant="accent">Start shopping</Button>
          </Link>
        }
      />
    )
  }

  const hasUnavailable = cart.lines.some((line) => !line.available)

  return (
    <div className={styles.page}>
      <div className={styles.titleRow}>
        <h1 className={styles.title}>
          Your bag <span className={styles.titleCount}>{cart.itemCount}</span>
        </h1>
        <button type="button" className={styles.clear} onClick={clear} disabled={isMutating}>
          <Trash2 size={15} /> Clear
        </button>
      </div>
      {!signedIn && (
        <Link to="/account" className={styles.notice}>
          Saved on this device. <strong>Sign in</strong> to keep your bag everywhere.
        </Link>
      )}

      <div className={styles.layout}>
        <ul className={styles.list}>
          {cart.lines.map((line) => (
            <CartRow
              key={line.variantId}
              line={line}
              busy={isMutating}
              onQuantity={(quantity) => setQuantity(line.variantId, quantity)}
              onRemove={() => remove(line.variantId)}
            />
          ))}
        </ul>

        <section className={styles.summary}>
          <div className={styles.summaryHead}>
            <Mascot size={64} mood="love" className={styles.summaryMascot} />
            <div>
              <h2 className={styles.summaryTitle}>Order summary</h2>
              <p className={styles.summaryNote}>great picks, honestly ♡</p>
            </div>
          </div>
          {cart.totals.map((total) => {
            const saved = total.shown_price - total.selling_price
            return (
              <div key={total.currency} className={styles.totalBlock}>
                <div className={styles.totalRow}>
                  <span>Subtotal</span>
                  <span>{formatPrice(total.shown_price, total.currency)}</span>
                </div>
                {saved > 0 && (
                  <div className={`${styles.totalRow} ${styles.saved}`}>
                    <span>
                      You save ({discountPercent(total.shown_price, total.selling_price)}%)
                    </span>
                    <span>-{formatPrice(saved, total.currency)}</span>
                  </div>
                )}
                <div className={`${styles.totalRow} ${styles.grand}`}>
                  <span>Total</span>
                  <span>{formatPrice(total.selling_price, total.currency)}</span>
                </div>
              </div>
            )
          })}
          {hasUnavailable && (
            <p className={styles.warning}>
              Some items are unavailable and are not counted in the total.
            </p>
          )}
          <Button
            variant="accent"
            size="lg"
            block
            onClick={() => navigate('/checkout')}
            disabled={cart.totals.length === 0}
          >
            Checkout
          </Button>
        </section>
      </div>
    </div>
  )
}

function CartRow({
  line,
  busy,
  onQuantity,
  onRemove,
}: {
  line: CartLineView
  busy: boolean
  onQuantity: (quantity: number) => void
  onRemove: () => void
}) {
  const attributes = Object.entries(line.attributes)
  const link = line.productId ? `/p/${line.productId}` : null
  const image = <SmartImage src={line.imageUrl} alt={line.name} ratio="1 / 1" />
  return (
    <li className={styles.row} data-unavailable={!line.available || undefined}>
      {link ? (
        <Link to={link} className={styles.thumb}>
          {image}
        </Link>
      ) : (
        <div className={styles.thumb}>{image}</div>
      )}
      <div className={styles.info}>
        <span className={styles.name}>{line.name}</span>
        {attributes.length > 0 && (
          <span className={styles.attrs}>
            {attributes.map(([key, value]) => `${key}: ${value}`).join(' · ')}
          </span>
        )}
        {line.currency && (
          <Price selling={line.sellingPrice} shown={line.shownPrice} currency={line.currency} />
        )}
        {!line.available && (
          <span className={styles.unavailable}>
            {line.stock === 0 ? 'Sold out' : `Only ${line.stock} left`}
          </span>
        )}
        <div className={styles.controls}>
          {line.maxQuantity > 0 ? (
            <QuantityStepper
              value={line.quantity}
              max={line.maxQuantity}
              label={line.name}
              disabled={busy}
              onChange={onQuantity}
            />
          ) : (
            <Button variant="danger" onClick={onRemove} disabled={busy}>
              Remove
            </Button>
          )}
        </div>
      </div>
    </li>
  )
}
