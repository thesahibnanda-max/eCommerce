import { useInfiniteQuery } from '@tanstack/react-query'
import { ChevronRight, Package } from 'lucide-react'
import { Link } from 'react-router'
import { errorMessage } from '@/shared/api/errors'
import { listOrders } from '@/shared/api/orders'
import { queryKeys } from '@/shared/api/queries'
import { formatPrice } from '@/shared/lib/format'
import { formatDateTime, orderStatus, paymentStatus, shortOrderID } from '@/shared/lib/orderStatus'
import { Button } from '@/shared/ui/Button'
import { Skeleton } from '@/shared/ui/Skeleton'
import { StatusBadge } from '@/shared/ui/StatusBadge'
import styles from './Orders.module.css'

export function OrdersList() {
  const query = useInfiniteQuery({
    queryKey: queryKeys.orders,
    queryFn: ({ pageParam, signal }) => listOrders(pageParam, signal),
    initialPageParam: '',
    getNextPageParam: (last) => last.next_cursor || undefined,
    staleTime: 15_000,
  })

  if (query.isLoading) return <Skeleton style={{ height: 96, borderRadius: 'var(--radius-lg)' }} />
  if (query.error) return <p className={styles.error}>{errorMessage(query.error)}</p>

  const orders = query.data?.pages.flatMap((page) => page.orders) ?? []
  if (orders.length === 0) {
    return (
      <div className={styles.emptyOrders}>
        <Package size={20} />
        <span>No orders yet. Your future favourites will show up here.</span>
      </div>
    )
  }

  return (
    <div className={styles.column}>
      <ul className={styles.orderList}>
        {orders.map((order) => {
          const status = orderStatus(order.state)
          const payment = order.payment ? paymentStatus(order.payment.state) : null
          return (
            <li key={order.id}>
              <Link to={`/orders/${order.id}`} className={styles.orderRow}>
                <div className={styles.orderMain}>
                  <span className={styles.orderID}>#{shortOrderID(order.id)}</span>
                  <span className={styles.muted}>
                    {formatDateTime(order.created_at)} · {order.item_count}{' '}
                    {order.item_count === 1 ? 'item' : 'items'} ·{' '}
                    {formatPrice(order.total_payable, order.currency)}
                  </span>
                  <div className={styles.badges}>
                    <StatusBadge label={status.label} tone={status.tone} title="Order status" />
                    {payment && (
                      <StatusBadge
                        label={`Payment: ${payment.label}`}
                        tone={payment.tone}
                        title="Payment status"
                      />
                    )}
                  </div>
                </div>
                <ChevronRight size={18} className={styles.chevron} />
              </Link>
            </li>
          )
        })}
      </ul>
      {query.hasNextPage && (
        <Button
          variant="ghost"
          block
          loading={query.isFetchingNextPage}
          onClick={() => void query.fetchNextPage()}
        >
          Show older orders
        </Button>
      )}
    </div>
  )
}
