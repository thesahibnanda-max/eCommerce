import { useQueryClient } from '@tanstack/react-query'
import { MailCheck } from 'lucide-react'
import { useEffect, useState, type FormEvent } from 'react'
import { errorMessage, isApiError } from '@/shared/api/errors'
import { queryKeys } from '@/shared/api/queries'
import { startEmailValidation, validateEmail } from '@/shared/api/users'
import { useMe } from '@/shared/auth/useMe'
import { toast } from '@/shared/lib/toastStore'
import { Button } from '@/shared/ui/Button'
import styles from './AccountPage.module.css'

const cooldownSeconds = 60

export function EmailVerification() {
  const client = useQueryClient()
  const me = useMe()
  const [state, setState] = useState<'idle' | 'sent' | 'verified'>('idle')
  const [otp, setOtp] = useState('')
  const [sending, setSending] = useState(false)
  const [verifying, setVerifying] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [cooldown, setCooldown] = useState(0)

  useEffect(() => {
    if (cooldown <= 0) return
    const timer = window.setTimeout(() => setCooldown((c) => c - 1), 1000)
    return () => window.clearTimeout(timer)
  }, [cooldown])

  const markVerified = () => {
    setState('verified')
    void client.invalidateQueries({ queryKey: queryKeys.me })
  }

  const send = async () => {
    if (sending) return
    setError(null)
    setSending(true)
    try {
      const result = await startEmailValidation()
      setState('sent')
      setCooldown(cooldownSeconds)
      toast(`Code sent to ${result.email}`, 'success')
    } catch (caught) {
      if (isApiError(caught, 'email_already_validated')) markVerified()
      else if (isApiError(caught, 'otp_resend_too_soon')) {
        setState('sent')
        setCooldown(caught.retryAfter ?? cooldownSeconds)
        setError(errorMessage(caught))
      } else setError(errorMessage(caught))
    } finally {
      setSending(false)
    }
  }

  const verify = async (event: FormEvent) => {
    event.preventDefault()
    if (verifying) return
    setError(null)
    setVerifying(true)
    try {
      const result = await validateEmail(otp.trim())
      if (result.is_validated) {
        markVerified()
        toast('Email verified', 'success')
      }
    } catch (caught) {
      setError(errorMessage(caught))
    } finally {
      setVerifying(false)
    }
  }

  if (me.isLoading) return null

  if (state === 'verified' || me.data?.is_email_validated) {
    return (
      <section className={styles.card}>
        <MailCheck size={20} className={styles.verifiedIcon} />
        <p>Your email is verified.</p>
      </section>
    )
  }

  return (
    <section className={styles.card}>
      <div className={styles.cardHead}>
        <MailCheck size={20} className={styles.cardIcon} />
        <div>
          <h2 className={styles.cardTitle}>Verify your email</h2>
          <p className={styles.muted}>We will send a one-time code to your inbox.</p>
        </div>
      </div>
      {state === 'sent' && (
        <form className={styles.otpForm} onSubmit={verify}>
          <input
            className={styles.otp}
            inputMode="numeric"
            autoComplete="one-time-code"
            maxLength={12}
            placeholder="••••••"
            value={otp}
            onChange={(e) => setOtp(e.target.value.replace(/\D/g, ''))}
            aria-label="One-time code"
          />
          <Button
            type="submit"
            variant="accent"
            loading={verifying}
            disabled={otp.length < 4 || sending}
          >
            Verify
          </Button>
        </form>
      )}
      {error && (
        <p className={styles.error} role="alert">
          {error}
        </p>
      )}
      <Button
        variant={state === 'sent' ? 'plain' : 'primary'}
        onClick={send}
        loading={sending}
        disabled={cooldown > 0 || verifying}
      >
        {sending
          ? 'Sending code…'
          : state === 'idle'
            ? 'Send code'
            : cooldown > 0
              ? `Resend in ${cooldown}s`
              : 'Resend code'}
      </Button>
    </section>
  )
}
