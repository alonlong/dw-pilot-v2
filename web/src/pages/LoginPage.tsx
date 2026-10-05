import { useState, type FormEvent } from 'react'
import { Navigate, useLocation, useNavigate } from 'react-router'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api, errorMessage } from '@/api'
import { meKey, useMe } from '@/app/auth'
import { Loading } from '@/app/states'
import { Alert, Button, Field, Input } from '@/ui'

function redirectTarget(state: unknown): string {
  const from = (state as { from?: unknown } | null)?.from
  // Only same-app paths; never bounce to /login itself.
  return typeof from === 'string' && from.startsWith('/') && !from.startsWith('//') && !from.startsWith('/login')
    ? from
    : '/reports'
}

export function LoginPage() {
  const me = useMe()
  const location = useLocation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [missing, setMissing] = useState<{ email?: string; password?: string }>({})

  const login = useMutation({
    mutationFn: () => api.login(email.trim(), password),
    onSuccess: (user) => {
      queryClient.setQueryData(meKey, user)
      void navigate(redirectTarget(location.state), { replace: true })
    },
  })

  if (me.isPending) return <Loading label="正在确认登录状态" />
  if (me.data) return <Navigate to={redirectTarget(location.state)} replace />

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    const m = {
      email: email.trim() ? undefined : '请输入邮箱',
      password: password ? undefined : '请输入密码',
    }
    setMissing(m)
    if (m.email || m.password) return
    login.mutate()
  }

  return (
    <div className="flex min-h-screen items-center justify-center px-4">
      <main className="flex w-full max-w-sm flex-col gap-6 rounded-lg border border-border bg-surface p-8">
        <div className="flex flex-col gap-1">
          <h1 className="text-xl font-semibold text-fg">施工日报</h1>
          <p className="text-sm text-muted">用公司分配的账号登录。</p>
        </div>
        {me.isError && <Alert tone="warning">{errorMessage(me.error)}</Alert>}
        {login.isError && (
          <Alert tone="danger" title="登录失败">
            {errorMessage(login.error)}
          </Alert>
        )}
        <form className="flex flex-col gap-4" onSubmit={onSubmit} noValidate>
          <Field label="邮箱" error={missing.email} required>
            <Input type="email" autoComplete="username" value={email} onChange={(e) => setEmail(e.target.value)} />
          </Field>
          <Field label="密码" error={missing.password} required>
            <Input
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </Field>
          <Button type="submit" loading={login.isPending}>
            登录
          </Button>
        </form>
      </main>
    </div>
  )
}
