import clsx from 'clsx'
import { Search, ShoppingBag, UserRound } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { Link, NavLink, useNavigate } from 'react-router'
import { useCart } from '@/shared/cart/useCart'
import { Logo } from './Logo'
import styles from './TopBar.module.css'

const links = [
  { to: '/', label: 'Home', end: true },
  { to: '/search', label: 'Shop all', end: false },
  { to: '/account', label: 'Account', end: false },
]

export function TopBar() {
  const { cart } = useCart()
  const navigate = useNavigate()
  const [query, setQuery] = useState('')

  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const text = query.trim()
    navigate(text === '' ? '/search' : `/search?q=${encodeURIComponent(text)}`)
  }

  return (
    <header className={styles.bar}>
      <div className={styles.inner}>
        <Link to="/" className={styles.brand} aria-label="Aesthetic Indians home">
          <span className={styles.mark}>
            <Logo />
          </span>
          <span className={styles.name}>
            Aesthetic <span className={styles.accent}>Indians</span>
          </span>
        </Link>

        <nav className={styles.links} aria-label="Main">
          {links.map(({ to, label, end }) => (
            <NavLink
              key={to}
              to={to}
              end={end}
              className={({ isActive }) => clsx(styles.link, isActive && styles.linkActive)}
            >
              {label}
            </NavLink>
          ))}
        </nav>

        <form className={styles.search} role="search" onSubmit={submit}>
          <Search size={18} aria-hidden />
          <input
            type="search"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search cute things..."
            aria-label="Search products"
            enterKeyHint="search"
          />
        </form>

        <div className={styles.actions}>
          <Link
            to="/account"
            className={clsx(styles.iconButton, styles.desktopOnly)}
            aria-label="Account"
          >
            <UserRound size={20} />
          </Link>
          <Link to="/cart" className={styles.cart} aria-label={`Bag, ${cart.itemCount} items`}>
            <ShoppingBag size={20} />
            {cart.itemCount > 0 && (
              <span key={cart.itemCount} className={styles.badge}>
                {cart.itemCount > 99 ? '99+' : cart.itemCount}
              </span>
            )}
          </Link>
        </div>
      </div>
    </header>
  )
}
