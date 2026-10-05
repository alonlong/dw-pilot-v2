import { MutationCache, QueryCache, QueryClient } from '@tanstack/react-query'
import { ApiError } from '@/api'
import { meKey } from './auth'

/**
 * One QueryClient per app (and per test). Any call that comes back `unauthenticated`
 * clears the cached user, which sends RequireAuth to /login — so an expired session is
 * handled in one place instead of in every page.
 */
export function createQueryClient(opts: { retry?: boolean } = {}): QueryClient {
  const onError = (err: unknown) => {
    if (err instanceof ApiError && err.code === 'unauthenticated') client.setQueryData(meKey, null)
  }
  const client: QueryClient = new QueryClient({
    queryCache: new QueryCache({ onError }),
    mutationCache: new MutationCache({ onError }),
    defaultOptions: {
      queries: {
        refetchOnWindowFocus: false,
        // A 4xx will not change on retry; a dropped connection might.
        retry: (count, err) =>
          opts.retry !== false && count < 1 && !(err instanceof ApiError && err.status >= 400 && err.status < 500),
      },
    },
  })
  return client
}
