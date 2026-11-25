import { useInfiniteQuery, useQuery, useQueryClient } from '@tanstack/react-query'
import clsx from 'clsx'
import { Check, X } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import {
  adminDecidePayment,
  adminGetPayment,
  adminListPayments,
  adminPaymentProof,
} from '@/shared/api/admin'
import { formatPrice } from '@/shared/lib/format'
import {
  formatDateTime,
  paymentStates,
  paymentStatus,
  shortOrderID,
} from '@/shared/lib/orderStatus'
import { Button } from '@/shared/ui/Button'
import { Chip } from '@/shared/ui/Chip'
import { StatusBadge } from '@/shared/ui/StatusBadge'
import { TextField } from '@/shared/ui/TextField'
import type { AdminContext } from './AdminPage'
import { Facts, JsonView, QueryState } from './shared'
import styles from './AdminData.module.css'

const approvable = new Set(['AWAITING_PAYMENT', 'VERIFYING', 'FAILED', 'EXPIRED'])
const rejectable = new Set(['AWAITING_PAYMENT', 'VERIFYING'])

export function PaymentsPanel({ context }: { context: AdminContext }) {
  const { readKey, focus, open } = context
  const [states, setStates] = useState<string[]>(['VERIFYING'])
  const [localSelected, setLocalSelected] = useState<string | null>(null)
  const selected = focus.payments ?? localSelected

  const list = useInfiniteQuery({
    queryKey: ['admin', 'payments', readKey, states.join(',')],
    queryFn: ({ pageParam, signal }) => adminListPayments(readKey, states, pageParam, signal),
    initialPageParam: '',
    getNextPageParam: (last) => last.next_cursor || undefined,
    enabled: readKey.length > 0,
    refetchInterval: 30_000,
  })
  const payments = list.data?.pages.flatMap((page) => page.payments) ?? []

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
        {paymentStates.map((state) => (
          <Chip key={state} active={states.includes(state)} onClick={() => toggle(state)}>
            {paymentStatus(state).label}
          </Chip>
        ))}
      </div>
      <div className={styles.split}>
        <QueryState
          isLoading={list.isLoading}
          error={list.error}
          empty={payments.length === 0}
          emptyText="No payments match these filters."
        >
          <ul className={styles.list}>
            {payments.map((payment) => {
              const status = paymentStatus(payment.state)
              return (
                <li key={payment.id}>
                  <button
                    type="button"
                    className={clsx(styles.rowButton, selected === payment.id && styles.rowActive)}
                    onClick={() => {
                      setLocalSelected(payment.id)
                      open('payments', payment.id)
                    }}
                  >
                    <span className={styles.rowTop}>
                      <span className={styles.rowTitle}>
                        {formatPrice(payment.amount, payment.currency)}
                      </span>
                      <StatusBadge label={status.label} tone={status.tone} />
                    </span>
                    <span className={styles.muted}>
                      Order #{shortOrderID(payment.order_id)} · {payment.provider} ·{' '}
                      {formatDateTime(payment.created_at)}
                    </span>
                    {payment.proof_transaction_id && (
                      <span className={styles.mono}>UTR {payment.proof_transaction_id}</span>
                    )}
                  </button>
                </li>
              )
            })}
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
          <PaymentDetail context={context} id={selected} />
        ) : (
          <p className={styles.empty}>Pick a payment to review its proof and decide.</p>
        )}
      </div>
    </div>
  )
}

function PaymentDetail({ context, id }: { context: AdminContext; id: string }) {
  const { readKey, run, busy, open } = context
  const client = useQueryClient()
  const [note, setNote] = useState('')
  const query = useQuery({
    queryKey: ['admin', 'payment', readKey, id],
    queryFn: ({ signal }) => adminGetPayment(readKey, id, signal),
    enabled: readKey.length > 0,
  })
  const payment = query.data
  const proof = useQuery({
    queryKey: ['admin', 'payment-proof', readKey, id, payment?.proof_transaction_id],
    queryFn: ({ signal }) => adminPaymentProof(readKey, id, signal),
    enabled: readKey.length > 0 && Boolean(payment?.has_proof_image),
    staleTime: Infinity,
  })

  const proofURL = useMemo(
    () => (proof.data ? URL.createObjectURL(proof.data) : null),
    [proof.data],
  )

  useEffect(
    () => () => {
      if (proofURL) URL.revokeObjectURL(proofURL)
    },
    [proofURL],
  )

  const decide = async (status: 'SUCCEEDED' | 'FAILED') => {
    const result = await run(status === 'SUCCEEDED' ? 'Approve payment' : 'Reject payment', (key) =>
      adminDecidePayment(key, id, status, note.trim()),
    )
    if (result) {
      setNote('')
      await client.invalidateQueries({ queryKey: ['admin'] })
    }
  }

  return (
    <section className={styles.detail}>
      <QueryState
        isLoading={query.isLoading}
        error={query.error}
        empty={!payment}
        emptyText="Payment not found."
      >
        {payment && (
          <>
            <div className={styles.detailHead}>
              <h3 className={styles.detailTitle}>
                {formatPrice(payment.amount, payment.currency)}
              </h3>
              <StatusBadge
                label={paymentStatus(payment.state).label}
                tone={paymentStatus(payment.state).tone}
              />
            </div>
            <Facts
              rows={[
                ['Payment', <span className={styles.mono}>{payment.id}</span>],
                [
                  'Order',
                  <button
                    type="button"
                    className={styles.linkButton}
                    onClick={() => open('orders', payment.order_id)}
                  >
                    #{shortOrderID(payment.order_id)} (open)
                  </button>,
                ],
                [
                  'Customer',
                  payment.user_id ? (
                    <button
                      type="button"
                      className={styles.linkButton}
                      onClick={() => open('users', payment.user_id)}
                    >
                      {payment.user_id} (open)
                    </button>
                  ) : (
                    ''
                  ),
                ],
                ['Provider', payment.provider],
                ['UTR / transaction', payment.proof_transaction_id ?? ''],
                [
                  'Proof sent',
                  payment.proof_submitted_at ? formatDateTime(payment.proof_submitted_at) : '',
                ],
                ['Gateway ref', payment.gateway_reference ?? ''],
                ['Gateway payment', payment.gateway_payment_id ?? ''],
                ['Failure', payment.failure_reason ?? ''],
                ['Created', formatDateTime(payment.created_at)],
                ['Updated', formatDateTime(payment.updated_at)],
              ]}
            />
            {payment.has_proof_image && (
              <div>
                <h4 className={styles.subTitle}>Payment screenshot</h4>
                {proofURL ? (
                  <a href={proofURL} target="_blank" rel="noreferrer">
                    <img
                      className={styles.proof}
                      src={proofURL}
                      alt="Customer payment screenshot"
                    />
                  </a>
                ) : (
                  <p className={styles.muted}>
                    {proof.error ? 'Could not load the screenshot.' : 'Loading screenshot…'}
                  </p>
                )}
              </div>
            )}
            {payment.provider === 'SELF_UPI' && approvable.has(payment.state) && (
              <div className={styles.decision}>
                <strong>Did this money reach the bank account?</strong>
                <span className={styles.muted}>
                  Check your UPI app for {formatPrice(payment.amount, payment.currency)}
                  {payment.proof_transaction_id
                    ? ` with reference ${payment.proof_transaction_id}`
                    : ''}
                  . The customer is emailed either way.
                </span>
                <TextField
                  label="Note (optional, shown to the customer if rejected)"
                  value={note}
                  maxLength={300}
                  onChange={(event) => setNote(event.target.value)}
                />
                <div className={styles.actions}>
                  <Button
                    variant="accent"
                    icon={<Check size={16} />}
                    disabled={busy}
                    onClick={() => void decide('SUCCEEDED')}
                  >
                    Approve, money received
                  </Button>
                  {rejectable.has(payment.state) && (
                    <Button
                      variant="danger"
                      icon={<X size={16} />}
                      disabled={busy}
                      onClick={() => void decide('FAILED')}
                    >
                      Reject
                    </Button>
                  )}
                </div>
              </div>
            )}
            {payment.events && payment.events.length > 0 && (
              <div>
                <h4 className={styles.subTitle}>History</h4>
                <ol className={styles.events}>
                  {payment.events.map((event, index) => (
                    <li key={`${event.kind}-${index}`}>
                      <strong>{event.kind.replaceAll('_', ' ')}</strong>{' '}
                      <span className={styles.muted}>
                        by {event.source.toLowerCase()} · {formatDateTime(event.created_at)}
                        {event.to_state ? ` · → ${event.to_state}` : ''}
                      </span>
                    </li>
                  ))}
                </ol>
              </div>
            )}
            <JsonView value={payment} />
          </>
        )}
      </QueryState>
    </section>
  )
}
