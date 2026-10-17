import { useState, type FormEvent } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, errorMessage, type Job, type ReportFilter } from '@/api'
import { canCreateReport, useCurrentUser } from '@/app/auth'
import { formatHours } from '@/app/format'
import { LateBadge, LoadError, Loading, StatusBadge } from '@/app/states'
import {
  Alert,
  Button,
  Dialog,
  EmptyState,
  Field,
  Input,
  PageHeader,
  Select,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeaderCell,
  TableRow,
} from '@/ui'

export function ReportsPage() {
  const me = useCurrentUser()
  const [params, setParams] = useSearchParams()
  const filter: ReportFilter = {
    job: params.get('job') ?? undefined,
    from: params.get('from') ?? undefined,
    to: params.get('to') ?? undefined,
  }
  const rangeError = filter.from && filter.to && filter.from > filter.to ? '结束日期不能早于开始日期' : undefined
  const filtered = Boolean(filter.job || filter.from || filter.to)
  const [creating, setCreating] = useState(false)

  const jobs = useQuery({ queryKey: ['jobs'], queryFn: api.jobs })
  const reports = useQuery({
    queryKey: ['reports', filter],
    queryFn: () => api.reports(filter),
    enabled: !rangeError,
  })

  const setFilter = (key: keyof ReportFilter, value: string) => {
    const next = new URLSearchParams(params)
    if (value) next.set(key, value)
    else next.delete(key)
    setParams(next, { replace: true })
  }

  const allowCreate = canCreateReport(me.role)
  const createButton = allowCreate && (
    <Button onClick={() => setCreating(true)} disabled={!jobs.data}>
      新建日报
    </Button>
  )

  return (
    <>
      <PageHeader title="日报台账" description="按业务日期排列的全部日报。" actions={createButton} />

      <form
        role="search"
        aria-label="筛选日报"
        className="mb-4 grid gap-4 rounded-md border border-border bg-surface-muted p-4 sm:grid-cols-4 sm:items-end"
        onSubmit={(e) => e.preventDefault()}
      >
        <Field label="工地" hint={jobs.isError ? '工地列表加载失败' : undefined}>
          <Select
            value={filter.job ?? ''}
            onChange={(e) => setFilter('job', e.target.value)}
            placeholder="全部工地"
            options={(jobs.data ?? []).map((j) => ({ value: j.id, label: `${j.name}（${j.id}）` }))}
          />
        </Field>
        <Field label="开始日期">
          <Input type="date" value={filter.from ?? ''} onChange={(e) => setFilter('from', e.target.value)} />
        </Field>
        <Field label="结束日期" error={rangeError}>
          <Input type="date" value={filter.to ?? ''} onChange={(e) => setFilter('to', e.target.value)} />
        </Field>
        <div>
          <Button variant="ghost" disabled={!filtered} onClick={() => setParams({}, { replace: true })}>
            清除筛选
          </Button>
        </div>
      </form>

      {rangeError ? null : reports.isPending ? (
        <Loading label="正在加载日报" />
      ) : reports.isError ? (
        <LoadError title="日报加载失败" error={reports.error} onRetry={() => void reports.refetch()} />
      ) : reports.data.length === 0 ? (
        filtered ? (
          <EmptyState
            title="没有符合筛选条件的日报"
            description="换一个工地或日期范围试试。"
            action={
              <Button variant="secondary" onClick={() => setParams({}, { replace: true })}>
                清除筛选
              </Button>
            }
          />
        ) : (
          <EmptyState
            title="还没有日报"
            description={allowCreate ? '新建第一份日报，开始记录工时。' : '现场人员新建日报后会显示在这里。'}
            action={createButton}
          />
        )
      ) : (
        <Table caption={`日报台账，共 ${reports.data.length} 份`}>
          <TableHead>
            <TableRow>
              <TableHeaderCell>工地</TableHeaderCell>
              <TableHeaderCell>业务日期</TableHeaderCell>
              <TableHeaderCell>状态</TableHeaderCell>
              <TableHeaderCell align="right">工时合计</TableHeaderCell>
              <TableHeaderCell>迟交天数</TableHeaderCell>
              <TableHeaderCell align="right">备注</TableHeaderCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {reports.data.map((r) => (
              <TableRow key={r.id}>
                <TableCell>
                  <span className="font-medium">{r.jobName}</span>{' '}
                  <span className="font-mono text-xs text-muted">{r.jobId}</span>
                </TableCell>
                <TableCell>
                  <Link
                    to={`/reports/${r.id}`}
                    className="font-medium text-primary underline-offset-2 hover:underline"
                    aria-label={`查看 ${r.jobName} ${r.businessDate} 的日报`}
                  >
                    {r.businessDate}
                  </Link>
                </TableCell>
                <TableCell>
                  <StatusBadge status={r.status} />
                </TableCell>
                <TableCell align="right">{formatHours(r.totalHours)}</TableCell>
                <TableCell>
                  {r.daysLate > 0 ? <LateBadge daysLate={r.daysLate} /> : <span className="text-muted">0</span>}
                </TableCell>
                <TableCell align="right">
                  {r.noteCount ? r.noteCount : <span className="text-muted">0</span>}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}

      {allowCreate && jobs.data && (
        <CreateReportDialog open={creating} onOpenChange={setCreating} jobs={jobs.data} defaultJob={filter.job} />
      )}
    </>
  )
}

function CreateReportDialog({
  open,
  onOpenChange,
  jobs,
  defaultJob,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  jobs: Job[]
  defaultJob?: string
}) {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [jobId, setJobId] = useState(defaultJob ?? '')
  const [date, setDate] = useState('')
  const [missing, setMissing] = useState<{ job?: string; date?: string }>({})

  const create = useMutation({
    mutationFn: () => api.createReport(jobId, date),
    onSuccess: (detail) => {
      queryClient.setQueryData(['report', detail.id], detail)
      void queryClient.invalidateQueries({ queryKey: ['reports'] })
      onOpenChange(false)
      void navigate(`/reports/${detail.id}`)
    },
  })

  const close = (next: boolean) => {
    if (!next) {
      create.reset()
      setMissing({})
    }
    onOpenChange(next)
  }

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    const m = { job: jobId ? undefined : '请选择工地', date: date ? undefined : '请选择业务日期' }
    setMissing(m)
    if (m.job || m.date) return
    create.mutate()
  }

  return (
    <Dialog open={open} onOpenChange={close} title="新建日报" description="每个工地每天只能有一份日报。">
      <form id="create-report" className="flex flex-col gap-4" onSubmit={onSubmit} noValidate>
        {create.isError && <Alert tone="danger">{errorMessage(create.error)}</Alert>}
        <Field label="工地" error={missing.job} required>
          <Select
            value={jobId}
            onChange={(e) => setJobId(e.target.value)}
            placeholder="请选择工地"
            options={jobs.map((j) => ({ value: j.id, label: `${j.name}（${j.id}）` }))}
          />
        </Field>
        <Field label="业务日期" error={missing.date} required>
          <Input type="date" value={date} onChange={(e) => setDate(e.target.value)} />
        </Field>
        <div className="flex justify-end gap-2">
          <Button variant="secondary" onClick={() => close(false)}>
            取消
          </Button>
          <Button type="submit" loading={create.isPending}>
            创建
          </Button>
        </div>
      </form>
    </Dialog>
  )
}
