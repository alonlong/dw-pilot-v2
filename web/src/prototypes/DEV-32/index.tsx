import { useState, type FormEvent } from 'react'
import { Alert, Button, Field, Input } from '@/ui'

// Fixed sample copy for the prototype; no API calls.
const MESSAGES = {
  badCredentials: '邮箱或密码错误。',
  locked: '登录失败次数过多，账号已临时锁定。请 15 分钟后再试，或联系管理员。',
  network: '无法连接服务器，请检查网络后重试。',
}

type Props = { screen?: string; state?: string }

type Outcome = 'none' | 'bad_credentials' | 'locked' | 'network'

function initialOutcome(screen: string, state: string): Outcome {
  if (screen === 'S2') return state === 'error' ? 'network' : 'locked'
  if (state === 'error') return 'bad_credentials'
  return 'none'
}

function LoginCard({ screen, state }: { screen: string; state: string }) {
  const [email, setEmail] = useState(screen === 'S2' || state === 'error' ? 'zhang.san@example.com' : '')
  const [password, setPassword] = useState(screen === 'S2' || state === 'error' ? '••••••' : '')
  const [outcome, setOutcome] = useState<Outcome>(initialOutcome(screen, state))
  const [failures, setFailures] = useState(screen === 'S2' ? 5 : state === 'error' ? 1 : 0)
  const [pending, setPending] = useState(state === 'loading')

  // Simulates the server: every attempt fails; the 6th and later get 423.
  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    if (!email.trim() || !password) return
    setPending(true)
    window.setTimeout(() => {
      setPending(false)
      if (failures >= 5) {
        setOutcome('locked')
        return
      }
      setFailures(failures + 1)
      setOutcome('bad_credentials')
    }, 400)
  }

  return (
    <div className="flex min-h-screen items-center justify-center px-4">
      <main className="flex w-full max-w-sm flex-col gap-6 rounded-lg border border-border bg-surface p-8">
        <div className="flex flex-col gap-1">
          <h1 className="text-xl font-semibold text-fg">施工日报</h1>
          <p className="text-sm text-muted">用公司分配的账号登录。</p>
        </div>
        {outcome === 'bad_credentials' && (
          <Alert tone="danger" title="登录失败">
            {MESSAGES.badCredentials}
          </Alert>
        )}
        {outcome === 'locked' && (
          <Alert tone="warning" title="账号已临时锁定">
            {MESSAGES.locked}
          </Alert>
        )}
        {outcome === 'network' && (
          <Alert tone="danger" title="登录失败">
            {MESSAGES.network}
          </Alert>
        )}
        <form className="flex flex-col gap-4" onSubmit={onSubmit} noValidate>
          <Field label="邮箱" required>
            <Input type="email" autoComplete="username" value={email} onChange={(e) => setEmail(e.target.value)} />
          </Field>
          <Field label="密码" required>
            <Input
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </Field>
          <Button type="submit" loading={pending}>
            登录
          </Button>
        </form>
      </main>
    </div>
  )
}

export default function Prototype({ screen = 'S1', state = 'normal' }: Props) {
  return <LoginCard key={`${screen}-${state}`} screen={screen} state={state} />
}
