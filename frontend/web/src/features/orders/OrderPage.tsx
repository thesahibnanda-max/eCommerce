import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import clsx from 'clsx'
import {
  ArrowLeft,
  Check,
  CircleX,
  Copy,
  ExternalLink,
  PackageCheck,
  ShieldCheck,
  Smartphone,
  Truck,
  Upload,
} from 'lucide-react'
import { useEffect, useMemo, useState, type FormEvent } from 'react'
import { Link, useParams } from 'react-router'
import { errorMessage } from '@/shared/api/errors'
import { cancelOrder, getOrder, payOrder, submitPaymentProof } from '@/shared/api/orders'
import { queryKeys } from '@/shared/api/queries'
import type { Order, OrderPayment } from '@/shared/api/types'
import { useIsSignedIn } from '@/shared/auth/authStore'
import { formatPrice } from '@/shared/lib/format'
import { formatDateTime, orderStatus, paymentStatus, shortOrderID } from '@/shared/lib/orderStatus'
import { toast } from '@/shared/lib/toastStore'
import { Button } from '@/shared/ui/Button'
import { EmptyState } from '@/shared/ui/EmptyState'
import { Skeleton } from '@/shared/ui/Skeleton'
import { SmartImage } from '@/shared/ui/SmartImage'
import { StatusBadge } from '@/shared/ui/StatusBadge'
import { TextField } from '@/shared/ui/TextField'
import { AddressLines } from '@/features/addresses/AddressForm'
import styles from './Orders.module.css'

const pollMs = 15_000
const maxProofBytes = 5 * 1024 * 1024

function isLive(order: Order | undefined): boolean {
  if (!order) return false
  const paymentOpen =
    order.payment && ['CREATED', 'AWAITING_PAYMENT', 'VERIFYING'].includes(order.payment.state)
  return Boolean(paymentOpen) || order.state === 'CREATED' || order.state === 'PROCESSING'
}

export default function OrderPage() {
  const { id = '' } = useParams()
  const signedIn = useIsSignedIn()
  const query = useQuery({
    queryKey: queryKeys.order(id),
    queryFn: ({ signal }) => getOrder(id, signal),
    enabled: signedIn && id.length > 0,
    refetchInterval: (state) => (isLive(state.state.data) ? pollMs : false),
  })

  if (!signedIn) {
    return (
      <EmptyState
        mood="wink"
        title="Sign in to see this order"
        action={
          <Link to="/account">
            <Button variant="accent">Sign in</Button>
          </Link>
        }
      />
    )
  }
  if (query.isLoading) {
    return (
      <div className={styles.page} aria-busy>
        <Skeleton style={{ height: 120, borderRadius: 'var(--radius-xl)' }} />
        <Skeleton style={{ height: 320, borderRadius: 'var(--radius-xl)' }} />
      </div>
    )
  }
  if (query.error || !query.data) {
    return (
      <EmptyState
        mood="surprised"
        title="We couldn't find that order"
        message={query.error ? errorMessage(query.error) : undefined}
        action={
          <Link to="/account">
            <Button variant="accent">Your orders</Button>
          </Link>
        }
      />
    )
  }
  return <OrderDetail order={query.data} />
}

function OrderDetail({ order }: { order: Order }) {
  const client = useQueryClient()
  const status = orderStatus(order.state)
  const [confirmCancel, setConfirmCancel] = useState(false)
  const cancel = useMutation({
    mutationFn: () => cancelOrder(order.id),
    onSuccess: (updated) => {
      client.setQueryData(queryKeys.order(order.id), updated)
      void client.invalidateQueries({ queryKey: queryKeys.orders })
      toast('Order cancelled', 'info')
    },
    onError: (error) => toast(errorMessage(error), 'error'),
  })

  return (
    <div className={styles.page}>
      <Link to="/account" className={styles.back}>
        <ArrowLeft size={16} /> Your orders
      </Link>
      <header className={styles.header}>
        <div>
          <h1 className={styles.title}>Order #{shortOrderID(order.id)}</h1>
          <p className={styles.muted}>Placed {formatDateTime(order.created_at)}</p>
        </div>
        <div className={styles.badges}>
          <StatusBadge label={status.label} tone={status.tone} />
          {order.payment && (
            <StatusBadge
              label={`Payment: ${paymentStatus(order.payment.state).label}`}
              tone={paymentStatus(order.payment.state).tone}
            />
          )}
        </div>
      </header>
      {status.hint && <p className={styles.hint}>{status.hint}</p>}

      <div className={styles.layout}>
        <div className={styles.column}>
          {order.closed ? (
            <section className={clsx(styles.card, styles.closedCard)}>
              <h2 className={styles.sectionTitle}>
                <CircleX size={18} /> {status.label}
              </h2>
              <p>{status.hint}</p>
            </section>
          ) : (
            <OrderTracker order={order} />
          )}
          <PaymentPanel order={order} />
          <section className={styles.card}>
            <h2 className={styles.sectionTitle}>Items</h2>
            <ul className={styles.items}>
              {order.items.map((item) => (
                <li key={item.product_variant_id} className={styles.item}>
                  <div className={styles.thumb}>
                    <SmartImage src={item.image_url} alt={item.product_name} ratio="1 / 1" />
                  </div>
                  <div className={styles.itemText}>
                    <Link to={`/p/${item.product_id}`} className={styles.itemName}>
                      {item.product_name}
                    </Link>
                    <span className={styles.muted}>
                      {formatPrice(item.unit_price, order.currency)} × {item.quantity}
                      {item.attributes &&
                        Object.keys(item.attributes).length > 0 &&
                        ` · ${Object.values(item.attributes).join(' / ')}`}
                    </span>
                  </div>
                  <span className={styles.price}>
                    {formatPrice(item.line_total, order.currency)}
                  </span>
                </li>
              ))}
            </ul>
            <div className={styles.totals}>
              <div className={styles.row}>
                <span>Subtotal</span>
                <span>{formatPrice(order.subtotal, order.currency)}</span>
              </div>
              <div className={styles.row}>
                <span>Shipping</span>
                <span>
                  {order.shipping_fee === 0
                    ? 'Free'
                    : formatPrice(order.shipping_fee, order.currency)}
                </span>
              </div>
              {order.discount > 0 && (
                <div className={styles.row}>
                  <span>Discount</span>
                  <span>-{formatPrice(order.discount, order.currency)}</span>
                </div>
              )}
              <div className={clsx(styles.row, styles.grand)}>
                <span>Total</span>
                <span>{formatPrice(order.total_payable, order.currency)}</span>
              </div>
            </div>
          </section>
        </div>
        <div className={styles.column}>
          {order.tracking && <TrackingCard order={order} />}
          <section className={styles.card}>
            <h2 className={styles.sectionTitle}>Delivery address</h2>
            <AddressLines address={order.shipping_address} />
          </section>
          {order.history && order.history.length > 0 && (
            <section className={styles.card}>
              <h2 className={styles.sectionTitle}>Timeline</h2>
              <ol className={styles.timeline}>
                {[...order.history].reverse().map((change) => (
                  <li key={`${change.to_state}-${change.created_at}`}>
                    <strong>{orderStatus(change.to_state).label}</strong>
                    <span className={styles.muted}>{formatDateTime(change.created_at)}</span>
                    {change.reason && <span className={styles.muted}>{change.reason}</span>}
                  </li>
                ))}
              </ol>
            </section>
          )}
          {order.can_cancel && (
            <Button
              variant={confirmCancel ? 'danger' : 'ghost'}
              block
              loading={cancel.isPending}
              onClick={() => (confirmCancel ? cancel.mutate() : setConfirmCancel(true))}
            >
              {confirmCancel ? 'Tap again to cancel this order' : 'Cancel order'}
            </Button>
          )}
        </div>
      </div>
    </div>
  )
}

function OrderTracker({ order }: { order: Order }) {
  return (
    <section className={styles.card} aria-label="Order progress">
      <h2 className={styles.sectionTitle}>
        <PackageCheck size={18} /> Order progress
      </h2>
      <ol className={styles.tracker}>
        {order.steps.map((step) => (
          <li
            key={step.key}
            className={clsx(
              styles.trackerStep,
              step.done && styles.trackerDone,
              step.current && styles.trackerCurrent,
            )}
            aria-current={step.current ? 'step' : undefined}
          >
            <span className={styles.trackerDot} aria-hidden>
              {step.done && <Check size={12} strokeWidth={3} />}
            </span>
            <div className={styles.trackerText}>
              <strong>{step.label}</strong>
              {step.at && <span className={styles.muted}>{formatDateTime(step.at)}</span>}
            </div>
          </li>
        ))}
      </ol>
    </section>
  )
}

function TrackingCard({ order }: { order: Order }) {
  const tracking = order.tracking
  const [copied, setCopied] = useState(false)
  if (!tracking) return null
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(tracking.number)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 2000)
    } catch {
      toast('Copy failed, please select the number manually', 'error')
    }
  }
  return (
    <section className={clsx(styles.card, styles.trackingCard)}>
      <h2 className={styles.sectionTitle}>
        <Truck size={18} /> Track your order
      </h2>
      <p className={styles.muted}>Shipped with {tracking.courier}</p>
      <div className={styles.upiID}>
        <code>{tracking.number}</code>
        <button type="button" className={styles.copy} onClick={() => void copy()}>
          {copied ? <Check size={14} /> : <Copy size={14} />} {copied ? 'Copied' : 'Copy'}
        </button>
      </div>
      <a href={tracking.url} target="_blank" rel="noopener noreferrer" className={styles.fullLink}>
        <Button variant="accent" block icon={<ExternalLink size={16} />}>
          Track on {tracking.courier}
        </Button>
      </a>
    </section>
  )
}

function PaymentPanel({ order }: { order: Order }) {
  const client = useQueryClient()
  const payment = order.payment
  const pay = useMutation({
    mutationFn: () => payOrder(order.id),
    onSuccess: (updated) => client.setQueryData(queryKeys.order(order.id), updated),
    onError: (error) => toast(errorMessage(error), 'error'),
  })

  if (!payment || (order.can_pay && payment.state !== 'AWAITING_PAYMENT')) {
    if (!order.can_pay) return null
    return (
      <section className={styles.card}>
        <h2 className={styles.sectionTitle}>Payment</h2>
        <p className={styles.muted}>Your order is reserved. Start the payment to confirm it.</p>
        <Button
          variant="accent"
          size="lg"
          block
          loading={pay.isPending}
          onClick={() => pay.mutate()}
        >
          Pay {formatPrice(order.total_payable, order.currency)}
        </Button>
      </section>
    )
  }

  if (payment.state === 'AWAITING_PAYMENT') {
    return (
      <section className={clsx(styles.card, styles.payCard)}>
        <h2 className={styles.sectionTitle}>Complete your payment</h2>
        {payment.action?.type === 'REDIRECT' && payment.action.redirect_url ? (
          <a href={payment.action.redirect_url} className={styles.fullLink}>
            <Button variant="accent" size="lg" block icon={<ExternalLink size={18} />}>
              Continue to secure payment
            </Button>
          </a>
        ) : payment.action?.type === 'SHOW_QR' ? (
          <UPIInstructions order={order} payment={payment} />
        ) : (
          <p className={styles.muted}>Loading payment details…</p>
        )}
        {order.can_submit_proof && <ProofForm order={order} />}
      </section>
    )
  }

  if (payment.state === 'VERIFYING') {
    return (
      <section className={clsx(styles.card, styles.reviewCard)}>
        <h2 className={styles.sectionTitle}>
          <ShieldCheck size={18} /> We're checking your payment
        </h2>
        <p>
          Thanks! We'll confirm it shortly and email you as soon as it's done. This page updates on
          its own.
        </p>
        {payment.proof_transaction_id && (
          <p className={styles.muted}>
            Transaction ID <strong>{payment.proof_transaction_id}</strong>
          </p>
        )}
        {order.can_submit_proof && (
          <details className={styles.resubmit}>
            <summary>Sent the wrong screenshot? Upload again</summary>
            <ProofForm order={order} />
          </details>
        )}
      </section>
    )
  }

  if (payment.state === 'SUCCEEDED') {
    return (
      <section className={clsx(styles.card, styles.paidCard)}>
        <h2 className={styles.sectionTitle}>
          <Check size={18} /> Payment received
        </h2>
        <p>
          We received {formatPrice(payment.amount, payment.currency)}. A confirmation email is on
          its way.
        </p>
      </section>
    )
  }

  return (
    <section className={clsx(styles.card, styles.failedCard)}>
      <h2 className={styles.sectionTitle}>{paymentStatus(payment.state).label}</h2>
      <p>{payment.failure_reason || paymentStatus(payment.state).hint}</p>
      <p className={styles.muted}>
        If money left your account, reply to the email we sent and we'll sort it out.
      </p>
    </section>
  )
}

function UPIInstructions({ order, payment }: { order: Order; payment: OrderPayment }) {
  const action = payment.action
  const [copied, setCopied] = useState(false)
  if (!action) return null
  const copy = async () => {
    if (!action.upi_id) return
    try {
      await navigator.clipboard.writeText(action.upi_id)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 2000)
    } catch {
      toast('Copy failed, please select the UPI ID manually', 'error')
    }
  }
  return (
    <div className={styles.upi}>
      <div className={styles.amount}>
        <span className={styles.muted}>Amount to pay</span>
        <strong>{formatPrice(payment.amount, payment.currency)}</strong>
      </div>
      {action.qr_image_data_uri && (
        <img className={styles.qr} src={action.qr_image_data_uri} alt="UPI QR code to pay" />
      )}
      {action.upi_id && (
        <div className={styles.upiID}>
          <span className={styles.muted}>UPI ID</span>
          <code>{action.upi_id}</code>
          <button type="button" className={styles.copy} onClick={() => void copy()}>
            {copied ? <Check size={14} /> : <Copy size={14} />} {copied ? 'Copied' : 'Copy'}
          </button>
        </div>
      )}
      {action.upi_intent_url && (
        <a href={action.upi_intent_url} className={styles.fullLink}>
          <Button variant="primary" block icon={<Smartphone size={18} />}>
            Pay with a UPI app
          </Button>
        </a>
      )}
      <ol className={styles.steps}>
        <li>
          Scan the QR or pay to the UPI ID above, exactly{' '}
          {formatPrice(order.total_payable, order.currency)}.
        </li>
        <li>Note the 12-digit UPI reference (UTR) from your payment app.</li>
        <li>Upload the payment screenshot below so we can confirm it.</li>
      </ol>
    </div>
  )
}

function ProofForm({ order }: { order: Order }) {
  const client = useQueryClient()
  const [transactionID, setTransactionID] = useState('')
  const [file, setFile] = useState<File | null>(null)
  const [error, setError] = useState<string | null>(null)
  const preview = useMemo(() => (file ? URL.createObjectURL(file) : null), [file])

  useEffect(
    () => () => {
      if (preview) URL.revokeObjectURL(preview)
    },
    [preview],
  )

  const submit = useMutation({
    mutationFn: () => submitPaymentProof(order.id, transactionID.trim(), file as File),
    onSuccess: (updated) => {
      client.setQueryData(queryKeys.order(order.id), updated)
      void client.invalidateQueries({ queryKey: queryKeys.orders })
      toast('Got it! We are checking your payment.', 'success')
    },
    onError: (caught) => setError(errorMessage(caught)),
  })

  const onSubmit = (event: FormEvent) => {
    event.preventDefault()
    setError(null)
    if (!/^[A-Za-z0-9]{6,35}$/.test(transactionID.trim())) {
      setError('Enter the UPI reference (UTR) from your payment app, letters and digits only.')
      return
    }
    if (!file) {
      setError('Attach a screenshot of the payment.')
      return
    }
    if (file.size > maxProofBytes) {
      setError('The screenshot must be 5 MB or smaller.')
      return
    }
    submit.mutate()
  }

  return (
    <form className={styles.proof} onSubmit={onSubmit} noValidate>
      <h3 className={styles.proofTitle}>I've paid, here is the proof</h3>
      <TextField
        label="UPI reference / transaction ID"
        inputMode="text"
        autoComplete="off"
        placeholder="e.g. 412345678901"
        value={transactionID}
        onChange={(event) => setTransactionID(event.target.value)}
      />
      <label className={styles.file}>
        <Upload size={16} />
        <span>{file ? file.name : 'Choose payment screenshot'}</span>
        <input
          type="file"
          accept="image/jpeg,image/png,image/webp"
          onChange={(event) => setFile(event.target.files?.[0] ?? null)}
        />
      </label>
      {preview && (
        <img className={styles.preview} src={preview} alt="Selected payment screenshot" />
      )}
      {error && <p className={styles.error}>{error}</p>}
      <Button type="submit" variant="accent" block loading={submit.isPending}>
        Submit payment proof
      </Button>
    </form>
  )
}
