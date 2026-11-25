import type { ReactNode } from 'react'
import { errorMessage, isApiError } from '@/shared/api/errors'
import { Button } from '@/shared/ui/Button'
import styles from './AdminData.module.css'

export function JsonView({ value, label = 'Raw data' }: { value: unknown; label?: string }) {
  return (
    <details className={styles.json}>
      <summary>{label}</summary>
      <pre>{JSON.stringify(value, null, 2)}</pre>
    </details>
  )
}

export function QueryState({
  isLoading,
  error,
  empty,
  emptyText,
  children,
}: {
  isLoading: boolean
  error: unknown
  empty: boolean
  emptyText: string
  children: ReactNode
}) {
  if (isLoading) return <p className={styles.muted}>Loading…</p>
  if (error) {
    return (
      <p className={styles.error}>
        {isApiError(error, 'admin_unauthorized') || isApiError(error, 'unauthorized')
          ? 'The admin key is missing or wrong.'
          : errorMessage(error)}
      </p>
    )
  }
  if (empty) return <p className={styles.empty}>{emptyText}</p>
  return <>{children}</>
}

export function Pager({
  offset,
  limit,
  total,
  onChange,
}: {
  offset: number
  limit: number
  total: number
  onChange: (offset: number) => void
}) {
  if (total <= limit) return null
  return (
    <div className={styles.pager}>
      <Button
        variant="ghost"
        disabled={offset === 0}
        onClick={() => onChange(Math.max(0, offset - limit))}
      >
        Previous
      </Button>
      <span className={styles.muted}>
        {offset + 1}–{Math.min(offset + limit, total)} of {total}
      </span>
      <Button
        variant="ghost"
        disabled={offset + limit >= total}
        onClick={() => onChange(offset + limit)}
      >
        Next
      </Button>
    </div>
  )
}

export function Facts({ rows }: { rows: [string, ReactNode][] }) {
  return (
    <dl className={styles.facts}>
      {rows.map(([label, value]) => (
        <div key={label} style={{ display: 'contents' }}>
          <dt>{label}</dt>
          <dd>{value === '' || value === undefined || value === null ? '—' : value}</dd>
        </div>
      ))}
    </dl>
  )
}
