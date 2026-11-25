import { QueryClient } from '@tanstack/react-query'
import { ApiError } from '@/shared/api/errors'

export function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 60_000,
        gcTime: 10 * 60_000,
        refetchOnWindowFocus: false,
        retry: (failures, error) => {
          if (error instanceof ApiError && error.status < 500 && error.status !== 408) return false
          return failures < 2
        },
      },
    },
  })
}
