import { useSearchParams } from 'react-router'
import { useQuery } from '@tanstack/react-query'
import { api, type DateRange } from '@/api'
import { canViewSummary, roleLabel, useCurrentUser } from '@/app/auth'
import { formatHours } from '@/app/format'
import { LoadError, Loading } from '@/app/states'
import {
  Alert,
  Badge,
  Button,
  EmptyState,
  Field,
  Input,
  PageHeader,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeaderCell,
  TableRow,
} from '@/ui'

export function SummaryPage() {
  const me = useCurrentUser()
  if (!canViewSummary(me.role)) {
    return (
      <>
        <PageHeader title="工时汇总" />
        <Alert tone="warning" title="无权限">
          {`你的角色（${roleLabel[me.role]}）不能查看工时汇总。需要时请联系管理员。`}
        </Alert>
      </>
    )
  }
  return <Summary />
}

function Summary() {
  const [params, setParams] = useSearchParams()
  const range: DateRange = { from: params.get('from') ?? undefined, to: params.get('to') ?? undefined }
  const rangeError = range.from && range.to && range.from > range.to ? '结束日期不能早于开始日期' : undefined
  const filtered = Boolean(range.from || range.to)
  const summary = useQuery({
    queryKey: ['summary', range],
    queryFn: () => api.summary(range),
    enabled: !rangeError,
  })

  const setRange = (key: keyof DateRange, value: string) => {
    const next = new URLSearchParams(params)
    if (value) next.set(key, value)
    else next.delete(key)
    setParams(next, { replace: true })
  }

  return (
    <>
      <PageHeader title="工时汇总" description="按工地统计已提交（含已锁定）日报的总工时和迟交份数；未提交的日报不计入。" />

      <form
        role="search"
        aria-label="汇总日期范围"
        className="mb-4 grid gap-4 rounded-md border border-border bg-surface-muted p-4 sm:grid-cols-3 sm:items-end"
        onSubmit={(e) => e.preventDefault()}
      >
        <Field label="开始日期">
          <Input type="date" value={range.from ?? ''} onChange={(e) => setRange('from', e.target.value)} />
        </Field>
        <Field label="结束日期" error={rangeError}>
          <Input type="date" value={range.to ?? ''} onChange={(e) => setRange('to', e.target.value)} />
        </Field>
        <div>
          <Button variant="ghost" disabled={!filtered} onClick={() => setParams({}, { replace: true })}>
            清除日期
          </Button>
        </div>
      </form>

      {rangeError ? null : summary.isPending ? (
        <Loading label="正在加载汇总" />
      ) : summary.isError ? (
        <LoadError title="汇总加载失败" error={summary.error} onRetry={() => void summary.refetch()} />
      ) : summary.data.length === 0 ? (
        <EmptyState
          title={filtered ? '这个日期范围内没有可汇总的日报' : '还没有可汇总的日报'}
          description="只有已提交或迟交的日报会计入汇总。"
        />
      ) : (
        <Table caption={`按工地汇总，共 ${summary.data.length} 个工地`}>
          <TableHead>
            <TableRow>
              <TableHeaderCell>工地</TableHeaderCell>
              <TableHeaderCell align="right">总工时（小时）</TableHeaderCell>
              <TableHeaderCell align="right">迟交份数</TableHeaderCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {summary.data.map((r) => (
              <TableRow key={r.jobId}>
                <TableCell>
                  <span className="font-medium">{r.jobName}</span>{' '}
                  <span className="font-mono text-xs text-muted">{r.jobId}</span>
                </TableCell>
                <TableCell align="right">{formatHours(r.totalHours)}</TableCell>
                <TableCell align="right">
                  {r.lateCount > 0 ? <Badge tone="late">{`迟交 ${r.lateCount} 份`}</Badge> : <span className="text-muted">0</span>}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}
    </>
  )
}
