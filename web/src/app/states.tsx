import { errorMessage, type Status } from '@/api'
import { Alert, Badge, Button, Spinner, type BadgeTone } from '@/ui'

export function Loading({ label = '加载中' }: { label?: string }) {
  return (
    <div className="flex justify-center py-10">
      <Spinner label={label} showLabel />
    </div>
  )
}

export function LoadError({ title, error, onRetry }: { title: string; error: unknown; onRetry: () => void }) {
  return (
    <Alert
      tone="danger"
      title={title}
      action={
        <Button variant="secondary" size="sm" onClick={onRetry}>
          重试
        </Button>
      }
    >
      {errorMessage(error)}
    </Alert>
  )
}

const statusTone: Record<Status, BadgeTone> = {
  未提交: 'draft',
  已提交: 'submitted',
  已锁定: 'locked',
}

export function StatusBadge({ status }: { status: Status }) {
  return <Badge tone={statusTone[status] ?? 'neutral'}>{status}</Badge>
}

/** "迟交 N 天" when late; nothing otherwise. The number of days comes from daily.DaysLate via the API. */
export function LateBadge({ daysLate }: { daysLate: number }) {
  if (daysLate <= 0) return null
  return <Badge tone="late">{`迟交 ${daysLate} 天`}</Badge>
}
