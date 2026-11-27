import clsx from 'clsx'
import { KeyRound, ShieldAlert } from 'lucide-react'
import { useState } from 'react'
import { errorMessage } from '@/shared/api/errors'
import { useDebouncedValue } from '@/shared/hooks/useDebouncedValue'
import { CartsPanel } from './CartsPanel'
import { BackupsPanel } from './BackupsPanel'
import { CatalogPanel } from './CatalogPanel'
import { ImagesPanel } from './ImagesPanel'
import { OrdersPanel } from './OrdersPanel'
import { OverviewPanel } from './OverviewPanel'
import { PaymentsPanel } from './PaymentsPanel'
import { ProductsPanel } from './ProductsPanel'
import { UsersPanel } from './UsersPanel'
import styles from './AdminPage.module.css'

export type LogEntry = { id: number; ok: boolean; title: string; detail: string }

export type AdminTab =
  | 'overview'
  | 'payments'
  | 'orders'
  | 'users'
  | 'carts'
  | 'catalog'
  | 'products'
  | 'images'
  | 'backups'

const tabs: { id: AdminTab; label: string }[] = [
  { id: 'overview', label: 'Overview' },
  { id: 'payments', label: 'Payments' },
  { id: 'orders', label: 'Orders' },
  { id: 'users', label: 'Customers' },
  { id: 'carts', label: 'Bags' },
  { id: 'catalog', label: 'Catalog' },
  { id: 'products', label: 'Edit products' },
  { id: 'images', label: 'Upload images' },
  { id: 'backups', label: 'Backups' },
]

export type AdminContext = {
  adminKey: string
  readKey: string
  run: (title: string, action: (key: string) => Promise<unknown>) => Promise<unknown>
  busy: boolean
  focus: Partial<Record<AdminTab, string>>
  open: (tab: AdminTab, id?: string) => void
}

let nextLog = 1

export default function AdminPage() {
  const [adminKey, setAdminKey] = useState('')
  const [tab, setTab] = useState<AdminTab>('overview')
  const [focus, setFocus] = useState<Partial<Record<AdminTab, string>>>({})
  const readKey = useDebouncedValue(adminKey.trim(), 400)
  const [log, setLog] = useState<LogEntry[]>([])
  const [busy, setBusy] = useState(false)

  const run: AdminContext['run'] = async (title, action) => {
    const key = adminKey.trim()
    if (!key) {
      setLog((items) =>
        [{ id: nextLog++, ok: false, title, detail: 'Enter the admin key first.' }, ...items].slice(
          0,
          30,
        ),
      )
      return undefined
    }
    setBusy(true)
    try {
      const result = await action(key)
      setLog((items) =>
        [
          { id: nextLog++, ok: true, title, detail: JSON.stringify(result, null, 2) },
          ...items,
        ].slice(0, 30),
      )
      return result
    } catch (caught) {
      setLog((items) =>
        [{ id: nextLog++, ok: false, title, detail: errorMessage(caught) }, ...items].slice(0, 30),
      )
      return undefined
    } finally {
      setBusy(false)
    }
  }

  const open: AdminContext['open'] = (target, id) => {
    setTab(target)
    if (id) setFocus((current) => ({ ...current, [target]: id }))
  }

  const context: AdminContext = { adminKey, readKey, run, busy, focus, open }

  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <ShieldAlert size={22} className={styles.headerIcon} />
        <div>
          <h1 className={styles.title}>Admin console</h1>
          <p className={styles.muted}>
            The key lives only in this tab and is sent with every request.
          </p>
        </div>
      </header>

      <label className={styles.keyField}>
        <KeyRound size={18} />
        <input
          type="password"
          autoComplete="off"
          spellCheck={false}
          placeholder="Admin key"
          value={adminKey}
          onChange={(event) => setAdminKey(event.target.value)}
          aria-label="Admin key"
        />
      </label>

      <div className={styles.tabs} role="tablist">
        {tabs.map((item) => (
          <button
            key={item.id}
            type="button"
            role="tab"
            aria-selected={tab === item.id}
            className={clsx(styles.tab, tab === item.id && styles.tabActive)}
            onClick={() => setTab(item.id)}
          >
            {item.label}
          </button>
        ))}
      </div>

      {tab === 'overview' && <OverviewPanel context={context} />}
      {tab === 'payments' && <PaymentsPanel context={context} />}
      {tab === 'orders' && <OrdersPanel context={context} />}
      {tab === 'users' && <UsersPanel context={context} />}
      {tab === 'carts' && <CartsPanel context={context} />}
      {tab === 'catalog' && <CatalogPanel context={context} />}
      {tab === 'products' && <ProductsPanel context={context} />}
      {tab === 'images' && <ImagesPanel context={context} />}
      {tab === 'backups' && <BackupsPanel context={context} />}

      {log.length > 0 && (
        <section className={styles.log}>
          <h2 className={styles.sectionTitle}>Activity</h2>
          {log.map((entry) => (
            <details
              key={entry.id}
              className={clsx(styles.logEntry, entry.ok ? styles.logOk : styles.logError)}
              open={entry.id === log[0]?.id}
            >
              <summary>
                {entry.ok ? 'Done' : 'Failed'}: {entry.title}
              </summary>
              <pre>{entry.detail}</pre>
            </details>
          ))}
        </section>
      )}
    </div>
  )
}
