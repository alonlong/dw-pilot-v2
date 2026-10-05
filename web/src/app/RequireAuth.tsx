import { Navigate, Outlet, useLocation } from 'react-router'
import { errorMessage } from '@/api'
import { Alert, Button, Spinner } from '@/ui'
import { CurrentUserContext, useMe } from './auth'
import { Layout } from './Layout'

/** Gate for every page except /login: no session → /login, remembering where the user was going. */
export function RequireAuth() {
  const me = useMe()
  const location = useLocation()

  if (me.isPending) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <Spinner label="正在确认登录状态" showLabel />
      </div>
    )
  }
  if (me.isError) {
    return (
      <div className="mx-auto max-w-lg p-6">
        <Alert
          tone="danger"
          title="无法确认登录状态"
          action={
            <Button variant="secondary" size="sm" onClick={() => void me.refetch()}>
              重试
            </Button>
          }
        >
          {errorMessage(me.error)}
        </Alert>
      </div>
    )
  }
  if (!me.data) {
    return <Navigate to="/login" replace state={{ from: location.pathname + location.search }} />
  }
  return (
    <CurrentUserContext.Provider value={me.data}>
      <Layout me={me.data}>
        <Outlet />
      </Layout>
    </CurrentUserContext.Provider>
  )
}
