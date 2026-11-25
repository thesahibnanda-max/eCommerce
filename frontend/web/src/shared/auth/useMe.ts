import { useQuery } from '@tanstack/react-query'
import { queryKeys } from '@/shared/api/queries'
import { fetchMe } from '@/shared/api/users'
import { useIsSignedIn } from './authStore'

export function useMe() {
  const signedIn = useIsSignedIn()
  return useQuery({
    queryKey: queryKeys.me,
    queryFn: ({ signal }) => fetchMe(signal),
    enabled: signedIn,
    staleTime: 5 * 60_000,
  })
}

export function firstName(name: string | undefined): string {
  return (name ?? '').trim().split(/\s+/)[0] ?? ''
}
