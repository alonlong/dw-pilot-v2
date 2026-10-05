import type { ReactNode } from 'react'
import { NavLink, useNavigate } from 'react-router'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api, type Me } from '@/api'
import { Button, cn } from '@/ui'
import { canViewSummary, meKey, roleLabel } from './auth'

const navClass = ({ isActive }: { isActive: boolean }) =>
  cn(
    'rounded-md px-3 py-2 text-sm font-medium',
    isActive ? 'bg-primary text-primary-fg' : 'text-fg hover:bg-surface-muted',
  )

export function Layout({ me, children }: { me: Me; children: ReactNode }) {
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const logout = useMutation({
    mutationFn: api.logout,
    // Signed out either way: a failed logout still leaves this tab with nothing to show.
    onSettled: () => {
      queryClient.clear()
      queryClient.setQueryData(meKey, null)
      void navigate('/login', { replace: true })
    },
  })

  return (
    <div className="min-h-screen">
      <header className="border-b border-border bg-surface">
        <div className="mx-auto flex max-w-6xl flex-wrap items-center justify-between gap-4 px-6 py-3">
          <div className="flex items-center gap-6">
            <span className="text-base font-semibold text-fg">施工日报</span>
            <nav aria-label="主导航" className="flex items-center gap-1">
              <NavLink to="/reports" className={navClass}>
                日报台账
              </NavLink>
              {canViewSummary(me.role) && (
                <NavLink to="/summary" className={navClass}>
                  工时汇总
                </NavLink>
              )}
            </nav>
          </div>
          <div className="flex items-center gap-4 text-sm">
            <dl className="flex items-center gap-3">
              <div className="flex gap-1">
                <dt className="sr-only">用户</dt>
                <dd className="font-medium text-fg">{me.name}</dd>
              </div>
              <div className="flex gap-1">
                <dt className="sr-only">角色</dt>
                <dd className="text-muted">{roleLabel[me.role]}</dd>
              </div>
              <div className="flex gap-1">
                <dt className="sr-only">租户</dt>
                <dd className="text-muted">{me.tenantName}</dd>
              </div>
            </dl>
            <Button variant="secondary" size="sm" loading={logout.isPending} onClick={() => logout.mutate()}>
              登出
            </Button>
          </div>
        </div>
      </header>
      <main className="mx-auto max-w-6xl px-6 py-6">{children}</main>
    </div>
  )
}
