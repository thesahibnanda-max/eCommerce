import { Link } from 'react-router'
import { Button } from '@/shared/ui/Button'
import { EmptyState } from '@/shared/ui/EmptyState'

export default function NotFoundPage() {
  return (
    <EmptyState
      mood="surprised"
      tone="sky"
      sign="404"
      title="Oops, I got lost"
      message="This page wandered off somewhere. Let's get you back to the cute stuff."
      action={
        <Link to="/">
          <Button>Back to home</Button>
        </Link>
      }
    />
  )
}
