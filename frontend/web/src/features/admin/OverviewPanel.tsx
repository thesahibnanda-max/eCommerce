import { useQuery } from '@tanstack/react-query'
import clsx from 'clsx'
import { adminOverview } from '@/shared/api/admin'
import { orderStatus, paymentStatus } from '@/shared/lib/orderStatus'
import { StatusBadge } from '@/shared/ui/StatusBadge'
import type { AdminContext } from './AdminPage'
import { QueryState } from './shared'
import styles from './AdminData.module.css'

export function OverviewPanel({ context }: { context: AdminContext }) {
  const { readKey, open } = context
  const query = useQuery({
    queryKey: ['admin', 'overview', readKey],
    queryFn: ({ signal }) => adminOverview(readKey, signal),
    enabled: readKey.length > 0,
    refetchInterval: 30_000,
  })
  const data = query.data
  return (
    <div className={styles.panel}>
      <QueryState
        isLoading={query.isLoading}
        error={query.error}
        empty={!data}
        emptyText="Enter the admin key to see the store at a glance."
      >
        {data && (
          <>
            <div className={styles.stats}>
              <button
                type="button"
                className={clsx(styles.stat, data.payments_awaiting_review > 0 && styles.statAlert)}
                onClick={() => open('payments')}
              >
                <span className={styles.statValue}>{data.payments_awaiting_review}</span>
                <span className={styles.statLabel}>Payments to review</span>
              </button>
              <button
                type="button"
                className={clsx(styles.stat, data.orders_to_fulfil > 0 && styles.statAlert)}
                onClick={() => open('orders')}
              >
                <span className={styles.statValue}>{data.orders_to_fulfil}</span>
                <span className={styles.statLabel}>Orders to ship / deliver</span>
              </button>
              <button type="button" className={styles.stat} onClick={() => open('users')}>
                <span className={styles.statValue}>{data.users}</span>
                <span className={styles.statLabel}>Customers</span>
              </button>
              <button type="button" className={styles.stat} onClick={() => open('catalog')}>
                <span className={styles.statValue}>{data.products}</span>
                <span className={styles.statLabel}>Products</span>
              </button>
              <button type="button" className={styles.stat} onClick={() => open('catalog')}>
                <span className={styles.statValue}>{data.images}</span>
                <span className={styles.statLabel}>Images</span>
              </button>
              <button type="button" className={styles.stat} onClick={() => open('carts')}>
                <span className={styles.statValue}>{data.carts}</span>
                <span className={styles.statLabel}>Active bags</span>
              </button>
            </div>
            <p className={styles.muted}>
              Payment provider:{' '}
              <strong>
                {data.payment_provider === 'SELF_UPI'
                  ? 'UPI (manual review)'
                  : data.payment_provider}
              </strong>
            </p>
            <div className={styles.split}>
              <section className={styles.detail}>
                <h3 className={styles.subTitle}>Orders by status</h3>
                <table className={styles.table}>
                  <tbody>
                    {Object.entries(data.orders_by_state)
                      .sort((a, b) => b[1] - a[1])
                      .map(([state, count]) => (
                        <tr key={state}>
                          <td>
                            <StatusBadge
                              label={orderStatus(state).label}
                              tone={orderStatus(state).tone}
                            />
                          </td>
                          <td className={styles.mono}>{state}</td>
                          <td>{count}</td>
                        </tr>
                      ))}
                  </tbody>
                </table>
              </section>
              <section className={styles.detail}>
                <h3 className={styles.subTitle}>Payments by status</h3>
                <table className={styles.table}>
                  <tbody>
                    {Object.entries(data.payments_by_state)
                      .sort((a, b) => b[1] - a[1])
                      .map(([state, count]) => (
                        <tr key={state}>
                          <td>
                            <StatusBadge
                              label={paymentStatus(state).label}
                              tone={paymentStatus(state).tone}
                            />
                          </td>
                          <td className={styles.mono}>{state}</td>
                          <td>{count}</td>
                        </tr>
                      ))}
                  </tbody>
                </table>
              </section>
            </div>
          </>
        )}
      </QueryState>
    </div>
  )
}
