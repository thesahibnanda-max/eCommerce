import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { adminListCarts } from '@/shared/api/admin'
import { formatPrice } from '@/shared/lib/format'
import { formatDateTime } from '@/shared/lib/orderStatus'
import type { AdminContext } from './AdminPage'
import { JsonView, Pager, QueryState } from './shared'
import styles from './AdminData.module.css'

const pageSize = 20

export function CartsPanel({ context }: { context: AdminContext }) {
  const { readKey, open } = context
  const [offset, setOffset] = useState(0)
  const list = useQuery({
    queryKey: ['admin', 'carts', readKey, offset],
    queryFn: ({ signal }) => adminListCarts(readKey, offset, pageSize, signal),
    enabled: readKey.length > 0,
  })
  const carts = list.data?.carts ?? []
  return (
    <div className={styles.panel}>
      <QueryState
        isLoading={list.isLoading}
        error={list.error}
        empty={carts.length === 0}
        emptyText="No one has anything in their bag right now."
      >
        <div className={styles.tableWrap}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th>Customer</th>
                <th>Items</th>
                <th>Value</th>
                <th>Updated</th>
              </tr>
            </thead>
            <tbody>
              {carts.map((cart) => (
                <tr key={cart.user_id}>
                  <td>
                    <button
                      type="button"
                      className={styles.linkButton}
                      onClick={() => open('users', cart.user_id)}
                    >
                      <span className={styles.mono}>{cart.user_id}</span>
                    </button>
                    <div className={styles.muted}>
                      {cart.lines
                        .map((line) => `${line.product_name ?? 'Deleted'} ×${line.quantity}`)
                        .join(', ')}
                    </div>
                  </td>
                  <td>{cart.item_count}</td>
                  <td>{formatPrice(cart.value, 'INR')}</td>
                  <td>{formatDateTime(cart.updated_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <JsonView value={carts} />
      </QueryState>
      <Pager offset={offset} limit={pageSize} total={list.data?.total ?? 0} onChange={setOffset} />
    </div>
  )
}
