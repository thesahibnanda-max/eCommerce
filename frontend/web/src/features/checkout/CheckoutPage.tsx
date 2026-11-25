import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import clsx from 'clsx'
import { AlertTriangle, Plus, QrCode, Truck } from 'lucide-react'
import { useState } from 'react'
import { Link, useNavigate } from 'react-router'
import { errorMessage, isApiError } from '@/shared/api/errors'
import { fetchQuote, placeOrder } from '@/shared/api/orders'
import { queryKeys } from '@/shared/api/queries'
import { useIsSignedIn } from '@/shared/auth/authStore'
import { formatPrice } from '@/shared/lib/format'
import { toast } from '@/shared/lib/toastStore'
import { Button } from '@/shared/ui/Button'
import { EmptyState } from '@/shared/ui/EmptyState'
import { Skeleton } from '@/shared/ui/Skeleton'
import { SmartImage } from '@/shared/ui/SmartImage'
import { AddressForm, AddressLines } from '@/features/addresses/AddressForm'
import { useAddresses, useSaveAddress } from '@/features/addresses/AddressBook'
import styles from './CheckoutPage.module.css'

function newKey(): string {
  return typeof crypto !== 'undefined' && 'randomUUID' in crypto
    ? crypto.randomUUID()
    : `${Date.now()}-${Math.random().toString(36).slice(2)}`
}

export default function CheckoutPage() {
  const signedIn = useIsSignedIn()
  if (!signedIn) {
    return (
      <EmptyState
        mood="wink"
        tone="lilac"
        title="Sign in to check out"
        message="Your bag is saved. Sign in so we can deliver it to you."
        action={
          <Link to="/account">
            <Button variant="accent">Sign in</Button>
          </Link>
        }
      />
    )
  }
  return <Checkout />
}

function Checkout() {
  const client = useQueryClient()
  const navigate = useNavigate()
  const [idempotencyKey] = useState(newKey)
  const [chosen, setChosen] = useState<string | null>(null)
  const [adding, setAdding] = useState(false)
  const quote = useQuery({ queryKey: queryKeys.quote, queryFn: ({ signal }) => fetchQuote(signal) })
  const addresses = useAddresses()
  const save = useSaveAddress((address) => {
    setChosen(address.id)
    setAdding(false)
  })

  const list = addresses.data ?? []
  const selected = chosen ?? list.find((address) => address.is_default)?.id ?? list[0]?.id ?? null

  const order = useMutation({
    mutationFn: () => placeOrder(selected ?? '', idempotencyKey),
    onSuccess: (placed) => {
      void client.invalidateQueries({ queryKey: queryKeys.cart })
      void client.invalidateQueries({ queryKey: queryKeys.orders })
      client.removeQueries({ queryKey: queryKeys.quote })
      client.setQueryData(queryKeys.order(placed.id), placed)
      navigate(`/orders/${placed.id}`, { replace: true })
    },
    onError: (error) => {
      if (isApiError(error, 'out_of_stock') || isApiError(error, 'item_unavailable')) {
        void quote.refetch()
      }
      toast(errorMessage(error), 'error')
    },
  })

  if (quote.isLoading || addresses.isLoading) {
    return (
      <div className={styles.page} aria-busy>
        <h1 className={styles.title}>Checkout</h1>
        <Skeleton style={{ height: 180, borderRadius: 'var(--radius-xl)' }} />
        <Skeleton style={{ height: 220, borderRadius: 'var(--radius-xl)' }} />
      </div>
    )
  }

  if (quote.error) {
    return (
      <EmptyState
        mood="surprised"
        title="Checkout couldn't load"
        message={errorMessage(quote.error)}
        action={
          <Button variant="accent" onClick={() => void quote.refetch()}>
            Try again
          </Button>
        }
      />
    )
  }

  const summary = quote.data
  if (!summary || summary.lines.length === 0) {
    return (
      <EmptyState
        mood="sleepy"
        tone="butter"
        title="Your bag is empty"
        message="Add something you love, then come back to check out."
        action={
          <Link to="/">
            <Button variant="accent">Start shopping</Button>
          </Link>
        }
      />
    )
  }

  const toFree = summary.free_shipping_above - summary.subtotal
  const showForm = adding || (addresses.isSuccess && list.length === 0)
  const canPlace = summary.ready && selected !== null && !showForm

  return (
    <div className={styles.page}>
      <h1 className={styles.title}>Checkout</h1>
      <div className={styles.layout}>
        <section className={styles.card}>
          <h2 className={styles.sectionTitle}>
            <Truck size={18} /> Deliver to
          </h2>
          {list.length > 0 && (
            <div className={styles.addresses} role="radiogroup" aria-label="Delivery address">
              {list.map((address) => (
                <label
                  key={address.id}
                  className={clsx(styles.address, selected === address.id && styles.addressActive)}
                >
                  <input
                    type="radio"
                    name="address"
                    checked={selected === address.id}
                    onChange={() => setChosen(address.id)}
                  />
                  <div>
                    {address.label && <span className={styles.addressLabel}>{address.label}</span>}
                    <AddressLines address={address} />
                  </div>
                </label>
              ))}
            </div>
          )}
          {showForm ? (
            <AddressForm
              submitLabel="Save and use this address"
              busy={save.isPending}
              error={save.error ? errorMessage(save.error) : null}
              onSubmit={(input) =>
                save.mutate({
                  input: { ...input, is_default: input.is_default || list.length === 0 },
                })
              }
              onCancel={list.length > 0 ? () => setAdding(false) : undefined}
            />
          ) : (
            <Button variant="ghost" icon={<Plus size={16} />} onClick={() => setAdding(true)}>
              Add a new address
            </Button>
          )}
        </section>

        <section className={styles.card}>
          <h2 className={styles.sectionTitle}>Order summary</h2>
          <ul className={styles.lines}>
            {summary.lines.map((line) => (
              <li
                key={line.variant_id}
                className={clsx(styles.line, !line.available && styles.lineOff)}
              >
                <div className={styles.thumb}>
                  <SmartImage src={line.image_url} alt={line.product_name} ratio="1 / 1" />
                </div>
                <div className={styles.lineText}>
                  <span className={styles.lineName}>{line.product_name || 'Unavailable item'}</span>
                  <span className={styles.muted}>
                    Qty {line.quantity}
                    {line.attributes &&
                      Object.keys(line.attributes).length > 0 &&
                      ` · ${Object.values(line.attributes).join(' / ')}`}
                  </span>
                </div>
                <span className={styles.linePrice}>
                  {formatPrice(line.line_total, summary.currency)}
                </span>
              </li>
            ))}
          </ul>
          <div className={styles.totals}>
            <div className={styles.row}>
              <span>Subtotal</span>
              <span>{formatPrice(summary.subtotal, summary.currency)}</span>
            </div>
            <div className={styles.row}>
              <span>Shipping</span>
              <span>
                {summary.shipping_fee === 0
                  ? 'Free'
                  : formatPrice(summary.shipping_fee, summary.currency)}
              </span>
            </div>
            {summary.shipping_fee > 0 && toFree > 0 && (
              <p className={styles.hint}>
                Add {formatPrice(toFree, summary.currency)} more for free shipping.
              </p>
            )}
            <div className={clsx(styles.row, styles.grand)}>
              <span>Total</span>
              <span>{formatPrice(summary.total_payable, summary.currency)}</span>
            </div>
          </div>
          {summary.problems.length > 0 && (
            <div className={styles.problems} role="alert">
              <AlertTriangle size={16} />
              <div>
                {summary.problems.map((problem) => (
                  <p key={problem}>{problem}</p>
                ))}
                <Link to="/cart">Fix your bag</Link>
              </div>
            </div>
          )}
          <div className={styles.method}>
            <QrCode size={18} />
            <span>
              {summary.payment_provider === 'SELF_UPI'
                ? 'Pay by UPI. You will see a QR code and our UPI ID right after you place the order.'
                : 'You will be taken to our secure payment page after you place the order.'}
            </span>
          </div>
          <Button
            variant="accent"
            size="lg"
            block
            loading={order.isPending}
            disabled={!canPlace}
            onClick={() => order.mutate()}
          >
            Place order · {formatPrice(summary.total_payable, summary.currency)}
          </Button>
          {!selected && <p className={styles.hint}>Add a delivery address to continue.</p>}
        </section>
      </div>
    </div>
  )
}
