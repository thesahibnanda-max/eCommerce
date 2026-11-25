import { useQueryClient } from '@tanstack/react-query'
import { LogOut } from 'lucide-react'
import { useAuthStore, useIsSignedIn } from '@/shared/auth/authStore'
import { signOut } from '@/shared/auth/session'
import { firstName, useMe } from '@/shared/auth/useMe'
import { toast } from '@/shared/lib/toastStore'
import { Button } from '@/shared/ui/Button'
import { Mascot } from '@/shared/ui/doodles'
import { AddressBook } from '@/features/addresses/AddressBook'
import { OrdersList } from '@/features/orders/OrdersList'
import { AuthPanel } from './AuthPanel'
import { EmailVerification } from './EmailVerification'
import styles from './AccountPage.module.css'

export default function AccountPage() {
  const signedIn = useIsSignedIn()
  return signedIn ? <Profile /> : <AuthPanel />
}

function Profile() {
  const client = useQueryClient()
  const contact = useAuthStore((state) => state.contact)
  const { data: me } = useMe()
  const name = firstName(me?.name)
  return (
    <div className={styles.page}>
      <section className={styles.profile}>
        <div className={styles.avatar}>
          <Mascot size={52} mood="wink" />
        </div>
        <div className={styles.profileText}>
          <span className={styles.hello}>Welcome back</span>
          <h1 className={styles.contact}>{name || 'Lovely to see you'}</h1>
          {(me?.email || contact) && <span className={styles.muted}>{me?.email || contact}</span>}
        </div>
      </section>

      <EmailVerification />

      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>Your orders</h2>
        <OrdersList />
      </section>

      <section className={styles.section}>
        <h2 className={styles.sectionTitle}>Addresses</h2>
        <AddressBook />
      </section>

      <Button
        variant="ghost"
        size="lg"
        block
        icon={<LogOut size={18} />}
        onClick={() => {
          signOut(client)
          toast('Signed out. See you soon!', 'info')
        }}
      >
        Sign out
      </Button>
    </div>
  )
}
