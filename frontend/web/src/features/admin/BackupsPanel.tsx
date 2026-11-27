import { useQuery, useQueryClient } from '@tanstack/react-query'
import { CloudUpload, ExternalLink, Sheet } from 'lucide-react'
import { adminRunSnapshot, adminSnapshotStatus } from '@/shared/api/admin'
import type { SnapshotResult } from '@/shared/api/types'
import { formatDateTime } from '@/shared/lib/orderStatus'
import { Button } from '@/shared/ui/Button'
import type { AdminContext } from './AdminPage'
import { QueryState } from './shared'
import styles from './AdminData.module.css'

export function BackupsPanel({ context }: { context: AdminContext }) {
  const { readKey, run, busy } = context
  const client = useQueryClient()
  const query = useQuery({
    queryKey: ['admin', 'snapshot', readKey],
    queryFn: ({ signal }) => adminSnapshotStatus(readKey, signal),
    enabled: readKey.length > 0,
    refetchInterval: (current) => (current.state.data?.running ? 3_000 : false),
  })
  const status = query.data
  const running = status?.running ?? false

  const snapshot = async () => {
    await run('Snapshot to Google Sheets', (key) => adminRunSnapshot(key))
    await client.invalidateQueries({ queryKey: ['admin', 'snapshot'] })
  }

  return (
    <div className={styles.panel}>
      <QueryState
        isLoading={query.isLoading}
        error={query.error}
        empty={!status}
        emptyText="Enter the admin key to manage backups."
      >
        {status && (
          <>
            <section className={styles.decision}>
              <div className={styles.rowTitle}>
                <Sheet size={18} /> Google Sheets snapshots
              </div>
              {status.configured ? (
                <p className={styles.muted}>
                  Every database is copied to its own spreadsheet, one tab per table, automatically
                  every {status.interval}. Passwords are redacted and images are left out.
                </p>
              ) : (
                <p className={styles.error}>
                  Not configured. Add GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET and
                  GOOGLE_REFRESH_TOKEN to the server environment and restart.
                </p>
              )}
              <div className={styles.actions}>
                <Button
                  variant="accent"
                  icon={<CloudUpload size={18} />}
                  onClick={() => void snapshot()}
                  loading={busy || running}
                  disabled={!status.configured || busy || running}
                >
                  {running ? 'Snapshot running…' : 'Snapshot everything to Google Sheets'}
                </Button>
              </div>
            </section>
            {status.last ? (
              <LastRun result={status.last} />
            ) : (
              status.configured && <p className={styles.muted}>No snapshot has run yet.</p>
            )}
          </>
        )}
      </QueryState>
    </div>
  )
}

function LastRun({ result }: { result: SnapshotResult }) {
  return (
    <section className={styles.panel}>
      <p className={styles.muted}>
        Last snapshot {formatDateTime(result.finished_at)}
        {result.failed > 0 ? ` · ${result.failed} failed` : ' · all databases saved'}
      </p>
      <table className={styles.table}>
        <thead>
          <tr>
            <th>Database</th>
            <th>Tabs</th>
            <th>Rows</th>
            <th>Sheet</th>
          </tr>
        </thead>
        <tbody>
          {result.databases.map((item) => (
            <tr key={item.database}>
              <td>{item.database}</td>
              <td>{item.error ? '—' : item.sheets}</td>
              <td>{item.error ? '—' : item.rows}</td>
              <td>
                {item.error ? (
                  <span className={styles.error}>{item.error}</span>
                ) : item.url ? (
                  <a href={item.url} target="_blank" rel="noopener noreferrer">
                    Open sheet <ExternalLink size={14} />
                  </a>
                ) : null}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  )
}
