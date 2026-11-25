import { useQuery } from '@tanstack/react-query'
import clsx from 'clsx'
import { useState } from 'react'
import { adminGetUser, adminListUsers } from '@/shared/api/admin'
import { formatPrice } from '@/shared/lib/format'
import { formatDateTime } from '@/shared/lib/orderStatus'
import { AddressLines } from '@/features/addresses/AddressForm'
import type { AdminContext } from './AdminPage'
import { OrderSummaryRow } from './OrdersPanel'
import { Facts, JsonView, Pager, QueryState } from './shared'
import styles from './AdminData.module.css'

const pageSize = 20

export function UsersPanel({ context }: { context: AdminContext }) {
  const { readKey, focus, open } = context
  const [offset, setOffset] = useState(0)
  const selected = focus.users ?? null
  const list = useQuery({
    queryKey: ['admin', 'users', readKey, offset],
    queryFn: ({ signal }) => adminListUsers(readKey, offset, pageSize, signal),
    enabled: readKey.length > 0,
  })
  const users = list.data?.users ?? []
  return (
    <div className={styles.panel}>
      <div className={styles.split}>
        <div className={styles.panel}>
          <QueryState
            isLoading={list.isLoading}
            error={list.error}
            empty={users.length === 0}
            emptyText="No customers yet."
          >
            <ul className={styles.list}>
              {users.map((user) => (
                <li key={user.id}>
                  <button
                    type="button"
                    className={clsx(styles.rowButton, selected === user.id && styles.rowActive)}
                    onClick={() => open('users', user.id)}
                  >
                    <span className={styles.rowTop}>
                      <span className={styles.rowTitle}>{user.name || 'Unnamed'}</span>
                      <span className={styles.muted}>{formatDateTime(user.created_at)}</span>
                    </span>
                    <span className={styles.muted}>
                      {user.email} {user.is_email_validated ? '✓' : ''} · {user.phone_number}
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          </QueryState>
          <Pager
            offset={offset}
            limit={pageSize}
            total={list.data?.total ?? 0}
            onChange={setOffset}
          />
        </div>
        {selected ? (
          <UserDetail context={context} id={selected} />
        ) : (
          <p className={styles.empty}>
            Pick a customer to see their profile, addresses, bag and orders.
          </p>
        )}
      </div>
    </div>
  )
}

function UserDetail({ context, id }: { context: AdminContext; id: string }) {
  const { readKey, open } = context
  const query = useQuery({
    queryKey: ['admin', 'user', readKey, id],
    queryFn: ({ signal }) => adminGetUser(readKey, id, signal),
    enabled: readKey.length > 0,
  })
  const detail = query.data
  return (
    <section className={styles.detail}>
      <QueryState
        isLoading={query.isLoading}
        error={query.error}
        empty={!detail}
        emptyText="Customer not found."
      >
        {detail && (
          <>
            <h3 className={styles.detailTitle}>{detail.user.name || 'Unnamed customer'}</h3>
            <Facts
              rows={[
                ['ID', <span className={styles.mono}>{detail.user.id}</span>],
                [
                  'Email',
                  `${detail.user.email} ${detail.user.is_email_validated ? '(verified)' : '(not verified)'}`,
                ],
                [
                  'Phone',
                  `${detail.user.phone_number} ${detail.user.is_phone_number_validated ? '(verified)' : ''}`,
                ],
                ['Joined', formatDateTime(detail.user.created_at)],
                ['Orders', String(detail.orders.length)],
              ]}
            />
            <div>
              <h4 className={styles.subTitle}>Orders</h4>
              {detail.orders.length === 0 ? (
                <p className={styles.muted}>No orders yet.</p>
              ) : (
                <ul className={styles.list}>
                  {detail.orders.map((order) => (
                    <li key={order.id}>
                      <OrderSummaryRow order={order} onClick={() => open('orders', order.id)} />
                    </li>
                  ))}
                </ul>
              )}
            </div>
            <div>
              <h4 className={styles.subTitle}>
                Bag ({detail.cart.item_count} items, {formatPrice(detail.cart.value, 'INR')})
              </h4>
              {detail.cart.lines.length === 0 ? (
                <p className={styles.muted}>Empty bag.</p>
              ) : (
                <table className={styles.table}>
                  <tbody>
                    {detail.cart.lines.map((line) => (
                      <tr key={line.variant_id}>
                        <td>{line.exists ? line.product_name : 'Deleted product'}</td>
                        <td className={styles.mono}>{line.sku}</td>
                        <td>×{line.quantity}</td>
                        <td>{formatPrice(line.selling_price, 'INR')}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>
            <div>
              <h4 className={styles.subTitle}>Addresses</h4>
              {detail.addresses.length === 0 ? (
                <p className={styles.muted}>No saved addresses.</p>
              ) : (
                <ul className={styles.list}>
                  {detail.addresses.map((address) => (
                    <li key={address.id} className={styles.rowButton}>
                      <span className={styles.muted}>
                        {address.label || 'Address'} {address.is_default ? '· default' : ''}
                      </span>
                      <AddressLines address={address} />
                    </li>
                  ))}
                </ul>
              )}
            </div>
            <JsonView value={detail} />
          </>
        )}
      </QueryState>
    </section>
  )
}
