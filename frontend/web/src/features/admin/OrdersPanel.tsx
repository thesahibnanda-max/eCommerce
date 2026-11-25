import { useInfiniteQuery, useQuery, useQueryClient } from '@tanstack/react-query'
import clsx from 'clsx'
import { useState } from 'react'
import {
  adminGetOrder,
  adminListOrders,
  adminSetTracking,
  adminTransitionOrder,
} from '@/shared/api/admin'
import type { Order } from '@/shared/api/types'
import { formatPrice } from '@/shared/lib/format'
import {
  formatDateTime,
  orderStates,
  orderStatus,
  paymentStatus,
  shortOrderID,
} from '@/shared/lib/orderStatus'
import { Button } from '@/shared/ui/Button'
import { Chip } from '@/shared/ui/Chip'
import { StatusBadge } from '@/shared/ui/StatusBadge'
import { TextField } from '@/shared/ui/TextField'
import { AddressLines } from '@/features/addresses/AddressForm'
import type { AdminContext } from './AdminPage'
import { Facts, JsonView, QueryState } from './shared'
import styles from './AdminData.module.css'

const destructive = new Set(['CANCELLED', 'FAILED', 'DITCHED', 'RETURNED', 'REFUNDED'])

export function OrderSummaryRow({
  order,
  active,
  onClick,
}: {
  order: Order
  active?: boolean
  onClick: () => void
}) {
  const status = orderStatus(order.state)
  const payment = order.payment ? paymentStatus(order.payment.state) : null
  return (
    <button
      type="button"
      className={clsx(styles.rowButton, active && styles.rowActive)}
      onClick={onClick}
    >
      <span className={styles.rowTop}>
        <span className={styles.rowTitle}>
          #{shortOrderID(order.id)} · {formatPrice(order.total_payable, order.currency)}
        </span>
        <span className={styles.badges}>
          <StatusBadge label={status.label} tone={status.tone} />
          {payment && <StatusBadge label={payment.label} tone={payment.tone} />}
        </span>
      </span>
      <span className={styles.muted}>
        {order.shipping_address.full_name} · {order.item_count} items ·{' '}
        {formatDateTime(order.created_at)}
      </span>
      {order.metadata?.needs_refund === true && (
        <span className={styles.flag}>Paid after it closed: refund needed</span>
      )}
    </button>
  )
}

export function OrdersPanel({ context }: { context: AdminContext }) {
  const { readKey, focus, open } = context
  const [states, setStates] = useState<string[]>([])
  const selected = focus.orders ?? null
  const list = useInfiniteQuery({
    queryKey: ['admin', 'orders', readKey, states.join(',')],
    queryFn: ({ pageParam, signal }) => adminListOrders(readKey, states, pageParam, signal),
    initialPageParam: '',
    getNextPageParam: (last) => last.next_cursor || undefined,
    enabled: readKey.length > 0,
    refetchInterval: 30_000,
  })
  const orders = list.data?.pages.flatMap((page) => page.orders) ?? []
  const toggle = (state: string) =>
    setStates((current) =>
      current.includes(state) ? current.filter((s) => s !== state) : [...current, state],
    )

  return (
    <div className={styles.panel}>
      <div className={styles.toolbar}>
        <Chip active={states.length === 0} onClick={() => setStates([])}>
          All
        </Chip>
        {orderStates.map((state) => (
          <Chip key={state} active={states.includes(state)} onClick={() => toggle(state)}>
            {orderStatus(state).label}
          </Chip>
        ))}
      </div>
      <div className={styles.split}>
        <QueryState
          isLoading={list.isLoading}
          error={list.error}
          empty={orders.length === 0}
          emptyText="No orders match these filters."
        >
          <ul className={styles.list}>
            {orders.map((order) => (
              <li key={order.id}>
                <OrderSummaryRow
                  order={order}
                  active={selected === order.id}
                  onClick={() => open('orders', order.id)}
                />
              </li>
            ))}
            {list.hasNextPage && (
              <Button
                variant="ghost"
                block
                loading={list.isFetchingNextPage}
                onClick={() => void list.fetchNextPage()}
              >
                Load more
              </Button>
            )}
          </ul>
        </QueryState>
        {selected ? (
          <OrderDetail context={context} id={selected} />
        ) : (
          <p className={styles.empty}>Pick an order to see everything about it.</p>
        )}
      </div>
    </div>
  )
}

function OrderDetail({ context, id }: { context: AdminContext; id: string }) {
  const { readKey, run, busy, open } = context
  const client = useQueryClient()
  const [reason, setReason] = useState('')
  const [confirming, setConfirming] = useState<string | null>(null)
  const query = useQuery({
    queryKey: ['admin', 'order', readKey, id],
    queryFn: ({ signal }) => adminGetOrder(readKey, id, signal),
    enabled: readKey.length > 0,
  })
  const order = query.data

  const move = async (to: string) => {
    if (destructive.has(to) && confirming !== to) {
      setConfirming(to)
      return
    }
    setConfirming(null)
    const result = await run(`Move order to ${orderStatus(to).label}`, (key) =>
      adminTransitionOrder(key, id, to, reason.trim()),
    )
    if (result) {
      setReason('')
      await client.invalidateQueries({ queryKey: ['admin'] })
    }
  }

  return (
    <section className={styles.detail}>
      <QueryState
        isLoading={query.isLoading}
        error={query.error}
        empty={!order}
        emptyText="Order not found."
      >
        {order && (
          <>
            <div className={styles.detailHead}>
              <h3 className={styles.detailTitle}>Order #{shortOrderID(order.id)}</h3>
              <div className={styles.badges}>
                <StatusBadge
                  label={orderStatus(order.state).label}
                  tone={orderStatus(order.state).tone}
                />
                {order.payment && (
                  <StatusBadge
                    label={`Payment: ${paymentStatus(order.payment.state).label}`}
                    tone={paymentStatus(order.payment.state).tone}
                  />
                )}
              </div>
            </div>
            {order.metadata?.needs_refund === true && (
              <p className={styles.flag}>
                Money arrived after this order had closed. Refund the customer or reopen manually.
              </p>
            )}
            <Facts
              rows={[
                ['Order', <span className={styles.mono}>{order.id}</span>],
                [
                  'Customer',
                  order.user_id ? (
                    <button
                      type="button"
                      className={styles.linkButton}
                      onClick={() => open('users', order.user_id)}
                    >
                      {order.user_id} (open)
                    </button>
                  ) : (
                    ''
                  ),
                ],
                ['Placed', formatDateTime(order.created_at)],
                ['Updated', formatDateTime(order.updated_at)],
                ['Subtotal', formatPrice(order.subtotal, order.currency)],
                ['Shipping', formatPrice(order.shipping_fee, order.currency)],
                ['Total', <strong>{formatPrice(order.total_payable, order.currency)}</strong>],
              ]}
            />
            {(order.allowed_transitions ?? []).length > 0 && (
              <div className={styles.decision}>
                <strong>Move this order</strong>
                <TextField
                  label="Reason (optional, included in customer emails)"
                  value={reason}
                  maxLength={300}
                  onChange={(event) => setReason(event.target.value)}
                />
                <div className={styles.actions}>
                  {(order.allowed_transitions ?? []).map((to) => (
                    <Button
                      key={to}
                      variant={
                        destructive.has(to) ? (confirming === to ? 'danger' : 'ghost') : 'accent'
                      }
                      disabled={busy}
                      onClick={() => void move(to)}
                    >
                      {confirming === to
                        ? `Confirm: ${orderStatus(to).label}`
                        : orderStatus(to).label}
                    </Button>
                  ))}
                </div>
              </div>
            )}
            <TrackingForm
              key={`${order.id}-${order.tracking?.url ?? ''}`}
              context={context}
              order={order}
            />
            <div>
              <h4 className={styles.subTitle}>Items</h4>
              <div className={styles.tableWrap}>
                <table className={styles.table}>
                  <thead>
                    <tr>
                      <th>Product</th>
                      <th>SKU</th>
                      <th>Qty</th>
                      <th>Price</th>
                      <th>Total</th>
                    </tr>
                  </thead>
                  <tbody>
                    {order.items.map((item) => (
                      <tr key={item.product_variant_id}>
                        <td>
                          {item.product_name}
                          {item.attributes && Object.keys(item.attributes).length > 0 && (
                            <div className={styles.muted}>
                              {Object.entries(item.attributes)
                                .map(([k, v]) => `${k}: ${v}`)
                                .join(', ')}
                            </div>
                          )}
                        </td>
                        <td className={styles.mono}>{item.sku}</td>
                        <td>{item.quantity}</td>
                        <td>{formatPrice(item.unit_price, order.currency)}</td>
                        <td>{formatPrice(item.line_total, order.currency)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
            <div>
              <h4 className={styles.subTitle}>Ship to</h4>
              <AddressLines address={order.shipping_address} />
            </div>
            {(order.payments ?? []).length > 0 && (
              <div>
                <h4 className={styles.subTitle}>Payments</h4>
                <ul className={styles.list}>
                  {(order.payments ?? []).map((payment) => (
                    <li key={payment.id}>
                      <button
                        type="button"
                        className={styles.rowButton}
                        onClick={() => open('payments', payment.id)}
                      >
                        <span className={styles.rowTop}>
                          <span className={styles.rowTitle}>
                            {formatPrice(payment.amount, payment.currency)}
                          </span>
                          <StatusBadge
                            label={paymentStatus(payment.state).label}
                            tone={paymentStatus(payment.state).tone}
                          />
                        </span>
                        <span className={styles.muted}>
                          {payment.provider} · {formatDateTime(payment.created_at)}
                          {payment.proof_transaction_id
                            ? ` · UTR ${payment.proof_transaction_id}`
                            : ''}
                        </span>
                      </button>
                    </li>
                  ))}
                </ul>
              </div>
            )}
            {order.history && (
              <div>
                <h4 className={styles.subTitle}>Status history</h4>
                <ol className={styles.events}>
                  {order.history.map((change) => (
                    <li key={`${change.to_state}-${change.created_at}`}>
                      <strong>{orderStatus(change.to_state).label}</strong>{' '}
                      <span className={styles.muted}>
                        by {change.actor.toLowerCase()} · {formatDateTime(change.created_at)}
                        {change.reason ? ` · ${change.reason}` : ''}
                      </span>
                    </li>
                  ))}
                </ol>
              </div>
            )}
            <JsonView value={order} />
          </>
        )}
      </QueryState>
    </section>
  )
}

function TrackingForm({ context, order }: { context: AdminContext; order: Order }) {
  const { run, busy } = context
  const client = useQueryClient()
  const [courier, setCourier] = useState(order.tracking?.courier ?? '')
  const [number, setNumber] = useState(order.tracking?.number ?? '')
  const [url, setURL] = useState(order.tracking?.url ?? '')

  const save = async (clear: boolean) => {
    const body = clear
      ? { courier: '', number: '', url: '' }
      : { courier: courier.trim(), number: number.trim(), url: url.trim() }
    const result = await run(clear ? 'Remove tracking' : 'Save tracking', (key) =>
      adminSetTracking(key, order.id, body),
    )
    if (result) await client.invalidateQueries({ queryKey: ['admin'] })
  }

  return (
    <div className={styles.decision}>
      <strong>Tracking for the customer</strong>
      <span className={styles.muted}>
        Shown as a "Track your order" button and included in shipping emails.
      </span>
      <TextField
        label="Courier"
        placeholder="Delhivery"
        value={courier}
        maxLength={60}
        onChange={(event) => setCourier(event.target.value)}
      />
      <TextField
        label="Tracking number"
        value={number}
        maxLength={60}
        onChange={(event) => setNumber(event.target.value)}
      />
      <TextField
        label="Tracking link"
        type="url"
        placeholder="https://"
        value={url}
        maxLength={512}
        onChange={(event) => setURL(event.target.value)}
      />
      <div className={styles.actions}>
        <Button
          variant="accent"
          disabled={busy || !courier.trim() || !number.trim() || !url.trim()}
          onClick={() => void save(false)}
        >
          Save tracking
        </Button>
        {order.tracking && (
          <Button variant="ghost" disabled={busy} onClick={() => void save(true)}>
            Remove
          </Button>
        )}
      </div>
    </div>
  )
}
