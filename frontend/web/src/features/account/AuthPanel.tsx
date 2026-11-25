import { useQueryClient } from '@tanstack/react-query'
import clsx from 'clsx'
import { useState, type FormEvent } from 'react'
import { errorMessage } from '@/shared/api/errors'
import { logIn, signUp, type Contact } from '@/shared/api/users'
import { completeSignIn } from '@/shared/auth/session'
import { toast } from '@/shared/lib/toastStore'
import { Button } from '@/shared/ui/Button'
import { TextField } from '@/shared/ui/TextField'
import { Mascot } from '@/shared/ui/doodles'
import styles from './AccountPage.module.css'

type Mode = 'login' | 'signup'
type Channel = 'email' | 'phone'

export function AuthPanel() {
  const client = useQueryClient()
  const [mode, setMode] = useState<Mode>('login')
  const [channel, setChannel] = useState<Channel>('email')
  const [name, setName] = useState('')
  const [contact, setContact] = useState('')
  const [email, setEmail] = useState('')
  const [phone, setPhone] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const submit = async (event: FormEvent) => {
    event.preventDefault()
    setError(null)
    setBusy(true)
    try {
      if (mode === 'signup') {
        const session = await signUp({
          name: name.trim(),
          email: email.trim(),
          phone_number: phone.trim(),
          password,
        })
        await completeSignIn(client, session, email.trim())
      } else {
        const value = contact.trim()
        const contactField: Contact =
          channel === 'email' ? { email: value } : { phone_number: value }
        const session = await logIn({ ...contactField, password })
        await completeSignIn(client, session, value)
      }
      toast(mode === 'signup' ? 'Welcome to Aesthetic Indians!' : 'Signed in', 'success')
    } catch (caught) {
      setError(errorMessage(caught))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className={styles.page}>
      <header className={styles.authHeader}>
        <Mascot
          size={92}
          mood={mode === 'login' ? 'happy' : 'love'}
          wave
          className={styles.authMascot}
        />
        <h1 className={styles.authTitle}>
          {mode === 'login' ? 'Welcome back' : 'Join the family'}
        </h1>
        <p className={styles.muted}>
          {mode === 'login'
            ? 'Sign in to sync your bag across devices.'
            : 'Create an account in under a minute.'}
        </p>
      </header>

      <div className={styles.segment} role="tablist" aria-label="Account mode">
        {(['login', 'signup'] as const).map((item) => (
          <button
            key={item}
            type="button"
            role="tab"
            aria-selected={mode === item}
            className={clsx(styles.segmentItem, mode === item && styles.segmentActive)}
            onClick={() => {
              setMode(item)
              setError(null)
            }}
          >
            {item === 'login' ? 'Sign in' : 'Create account'}
          </button>
        ))}
      </div>

      <form className={styles.form} onSubmit={submit} noValidate>
        {mode === 'signup' && (
          <TextField
            label="Name"
            autoComplete="name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
          />
        )}
        {mode === 'signup' ? (
          <>
            <TextField
              label="Email"
              type="email"
              inputMode="email"
              autoComplete="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              hint="we'll send a code here to verify it's you"
              required
            />
            <TextField
              label="Phone number"
              type="tel"
              inputMode="tel"
              autoComplete="tel"
              placeholder="+91 98765 43210"
              value={phone}
              onChange={(e) => setPhone(e.target.value)}
              hint="we'll use this for order updates"
              required
            />
          </>
        ) : (
          <>
            <div className={styles.channel}>
              {(['email', 'phone'] as const).map((item) => (
                <button
                  key={item}
                  type="button"
                  className={clsx(styles.channelItem, channel === item && styles.channelActive)}
                  onClick={() => {
                    setChannel(item)
                    setContact('')
                  }}
                >
                  {item === 'email' ? 'Email' : 'Phone'}
                </button>
              ))}
            </div>
            {channel === 'email' ? (
              <TextField
                label="Email"
                type="email"
                inputMode="email"
                autoComplete="email"
                value={contact}
                onChange={(e) => setContact(e.target.value)}
                required
              />
            ) : (
              <TextField
                label="Phone number"
                type="tel"
                inputMode="tel"
                autoComplete="tel"
                placeholder="+91 98765 43210"
                value={contact}
                onChange={(e) => setContact(e.target.value)}
                required
              />
            )}
          </>
        )}
        <TextField
          label="Password"
          type="password"
          autoComplete={mode === 'signup' ? 'new-password' : 'current-password'}
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          hint={
            mode === 'signup' ? '8+ characters with a letter, a number and a symbol' : undefined
          }
          required
        />
        {error && (
          <p className={styles.error} role="alert">
            {error}
          </p>
        )}
        <Button
          type="submit"
          variant="accent"
          size="lg"
          block
          loading={busy}
          disabled={
            !password ||
            (mode === 'signup' ? !name.trim() || !email.trim() || !phone.trim() : !contact.trim())
          }
        >
          {mode === 'login' ? 'Sign in' : 'Create account'}
        </Button>
      </form>
    </div>
  )
}
