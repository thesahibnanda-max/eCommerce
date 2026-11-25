import { useEffect, useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router'
import { errorMessage } from '@/shared/api/errors'
import { confirmProviderRedirect } from '@/shared/api/orders'
import { Button } from '@/shared/ui/Button'
import { EmptyState } from '@/shared/ui/EmptyState'

export default function PaymentReturnPage() {
  const location = useLocation()
  const navigate = useNavigate()
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    confirmProviderRedirect(location.search)
      .then((result) => {
        if (!cancelled) navigate(`/orders/${result.order_id}`, { replace: true })
      })
      .catch((caught: unknown) => {
        if (!cancelled) setError(errorMessage(caught))
      })
    return () => {
      cancelled = true
    }
  }, [location.search, navigate])

  if (error) {
    return (
      <EmptyState
        mood="surprised"
        title="We couldn't match that payment"
        message={`${error} Your order is safe, check its status in your account.`}
        action={
          <Link to="/account">
            <Button variant="accent">Your orders</Button>
          </Link>
        }
      />
    )
  }
  return (
    <EmptyState
      mood="happy"
      title="Confirming your payment…"
      message="Hang tight, this takes a moment."
    />
  )
}
