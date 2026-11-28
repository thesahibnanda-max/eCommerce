import { useQuery, useQueryClient } from '@tanstack/react-query'
import { CloudUpload, ExternalLink, Link2, Link2Off, Sheet } from 'lucide-react'
import { useEffect, useState } from 'react'
import {
  adminConnectGoogle,
  adminDisconnectGoogle,
  adminRunSnapshot,
  adminSnapshotStatus,
} from '@/shared/api/admin'
import { apiOrigin } from '@/shared/api/config'
import type { SnapshotResult, SnapshotStatus } from '@/shared/api/types'
import { formatDateTime } from '@/shared/lib/orderStatus'
import { Button } from '@/shared/ui/Button'
import type { AdminContext } from './AdminPage'
import { QueryState } from './shared'
import styles from './AdminData.module.css'

const callbackPath = '/v1/admin/google/callback'

type GoogleOutcome = { ok: boolean; text: string }

const reasons: Record<string, string> = {
  google_consent_denied: "Access wasn't allowed on Google's screen, so nothing changed.",
  google_no_refresh_token:
    "Google didn't send a long-lived token. Remove this app at myaccount.google.com/permissions, then connect again.",
  google_exchange_failed:
    'Google sign-in could not be finished. Check GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET and that the redirect URL below is registered on the OAuth client.',
}

function readOutcome(): GoogleOutcome | null {
  const params = new URLSearchParams(window.location.search)
  const result = params.get('google')
  if (!result) return null
  if (result === 'connected') return { ok: true, text: 'Google Drive connected.' }
  const reason = params.get('google_reason') ?? ''
  return { ok: false, text: reasons[reason] ?? `Google sign-in failed (${reason || 'unknown'}).` }
}

function clearOutcome() {
  const url = new URL(window.location.href)
  if (!url.searchParams.has('google')) return
  url.searchParams.delete('google')
  url.searchParams.delete('google_reason')
  window.history.replaceState(window.history.state, '', url)
}

function returnURL(): string {
  return `${window.location.origin}${window.location.pathname}?tab=backups`
}

export function BackupsPanel({ context }: { context: AdminContext }) {
  const { readKey, run, busy } = context
  const client = useQueryClient()
  const [outcome] = useState(readOutcome)
  useEffect(clearOutcome, [])
  const query = useQuery({
    queryKey: ['admin', 'snapshot', readKey],
    queryFn: ({ signal }) => adminSnapshotStatus(readKey, signal),
    enabled: readKey.length > 0,
    refetchInterval: (current) => (current.state.data?.running ? 3_000 : false),
  })
  const status = query.data
  const running = status?.running ?? false
  const connected = status?.google?.connected ?? false

  const snapshot = async () => {
    await run('Snapshot to Google Sheets', (key) => adminRunSnapshot(key))
    await client.invalidateQueries({ queryKey: ['admin', 'snapshot'] })
  }

  const connect = async () => {
    const authURL = await run('Connect Google Drive', (key) => adminConnectGoogle(key, returnURL()))
    if (typeof authURL === 'string') window.location.assign(authURL)
  }

  const disconnect = async () => {
    await run('Disconnect Google Drive', (key) => adminDisconnectGoogle(key))
    await client.invalidateQueries({ queryKey: ['admin', 'snapshot'] })
  }

  return (
    <div className={styles.panel}>
      {outcome && (
        <p className={outcome.ok ? styles.muted : styles.error} role="status">
          {outcome.text}
          {!readKey && ' Enter the admin key to continue.'}
        </p>
      )}
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
                  Not configured. Add GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET to the server
                  environment and restart.
                </p>
              )}
              {status.configured && <GoogleAccount status={status} />}
              <div className={styles.actions}>
                {status.configured && !connected && (
                  <Button
                    variant="accent"
                    icon={<Link2 size={18} />}
                    onClick={() => void connect()}
                    loading={busy}
                    disabled={busy}
                  >
                    Connect Google Drive
                  </Button>
                )}
                {connected && (
                  <Button
                    variant="accent"
                    icon={<CloudUpload size={18} />}
                    onClick={() => void snapshot()}
                    loading={busy || running}
                    disabled={busy || running}
                  >
                    {running ? 'Snapshot running…' : 'Snapshot everything to Google Sheets'}
                  </Button>
                )}
                {connected && (
                  <Button
                    variant="ghost"
                    icon={<Link2Off size={18} />}
                    onClick={() => void disconnect()}
                    disabled={busy || running}
                  >
                    Disconnect
                  </Button>
                )}
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

function GoogleAccount({ status }: { status: SnapshotStatus }) {
  const google = status.google
  if (google?.connected) {
    return (
      <p className={styles.muted}>
        Connected as <strong>{google.account || 'your Google account'}</strong>
        {google.connected_at && ` since ${formatDateTime(google.connected_at)}`}.
      </p>
    )
  }
  return (
    <p className={styles.muted}>
      Google Drive isn&apos;t connected yet. Connect it once and snapshots run on their own. The
      OAuth client (type Web application) must list this redirect URL:{' '}
      <code>{`${apiOrigin}${callbackPath}`}</code>
    </p>
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
