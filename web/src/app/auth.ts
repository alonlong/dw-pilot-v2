import { createContext, useContext } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api, ApiError, type Me, type Role } from '@/api'

export const meKey = ['me'] as const

/** The signed-in user, `null` when there is no session. */
export function useMe() {
  return useQuery<Me | null>({
    queryKey: meKey,
    queryFn: async () => {
      try {
        return await api.me()
      } catch (err) {
        if (err instanceof ApiError && err.code === 'unauthenticated') return null
        throw err
      }
    },
    staleTime: Infinity,
  })
}

/**
 * The signed-in user for pages under RequireAuth. Provided by RequireAuth, so it is never
 * null there — unlike useMe().data, which goes null the moment a session expires, before
 * RequireAuth has had the chance to redirect.
 */
export const CurrentUserContext = createContext<Me | null>(null)

export function useCurrentUser(): Me {
  const me = useContext(CurrentUserContext)
  if (!me) throw new Error('useCurrentUser must be used under RequireAuth')
  return me
}

export const roleLabel: Record<Role, string> = {
  admin: '管理员',
  manager: '项目经理',
  field: '现场',
  finance: '财务',
}

// UI gating only, mirroring docs/api.md "权限". The server enforces the same table; for a
// single report the server's `can` field is the authority, not these.
export const canCreateReport = (role: Role) => role === 'admin' || role === 'field'
export const canViewSummary = (role: Role) => role === 'admin' || role === 'manager' || role === 'finance'
