import clsx from 'clsx'
import { House, Search, ShoppingBag, UserRound } from 'lucide-react'
import { NavLink } from 'react-router'
import { useCart } from '@/shared/cart/useCart'
import styles from './TabBar.module.css'

const tabs = [
  { to: '/', label: 'Home', icon: House, end: true, tone: styles.pink },
  { to: '/search', label: 'Search', icon: Search, end: false, tone: styles.lilac },
  { to: '/cart', label: 'Bag', icon: ShoppingBag, end: false, tone: styles.butter },
  { to: '/account', label: 'Me', icon: UserRound, end: false, tone: styles.mint },
]

export function TabBar() {
  const { cart } = useCart()
  return (
    <nav className={styles.bar} aria-label="Primary">
      <div className={styles.inner}>
        {tabs.map(({ to, label, icon: Icon, end, tone }) => (
          <NavLink
            key={to}
            to={to}
            end={end}
            className={({ isActive }) => clsx(styles.tab, tone, isActive && styles.active)}
          >
            <span className={styles.blob}>
              <Icon size={21} strokeWidth={2.2} />
              {to === '/cart' && cart.itemCount > 0 && (
                <span className={styles.dot}>{cart.itemCount > 9 ? '9+' : cart.itemCount}</span>
              )}
            </span>
            <span className={styles.label}>{label}</span>
          </NavLink>
        ))}
      </div>
    </nav>
  )
}
