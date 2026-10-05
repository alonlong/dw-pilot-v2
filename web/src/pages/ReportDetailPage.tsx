import { useState, type FormEvent } from 'react'
import { Link, useParams } from 'react-router'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, errorMessage, isApiError, type CrewEntry, type ReportDetail } from '@/api'
import { roleLabel, useCurrentUser } from '@/app/auth'
import { formatHours, formatTime } from '@/app/format'
import { LateBadge, LoadError, Loading, StatusBadge } from '@/app/states'
import {
  Alert,
  Button,
  Dialog,
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
  Textarea,
} from '@/ui'

/** Same limit as daily.MaxNoteRunes; checked here only to warn early, the server decides. */
const MAX_NOTE_RUNES = 500

const backLink = (
  <Link to="/reports" className="text-sm font-medium text-primary underline">
    返回日报台账
  </Link>
)

export function ReportDetailPage() {
  const { id: rawId } = useParams()
  const id = Number(rawId)
  const valid = Number.isInteger(id) && id > 0
  const report = useQuery({ queryKey: ['report', id], queryFn: () => api.report(id), enabled: valid })

  if (!valid || isApiError(report.error, 'not_found')) {
    return <EmptyState title="找不到这份日报" description="它可能不存在，或不属于当前租户。" action={backLink} />
  }
  if (report.isPending) return <Loading label="正在加载日报" />
  if (report.isError) {
    return <LoadError title="日报加载失败" error={report.error} onRetry={() => void report.refetch()} />
  }
  return <ReportView report={report.data} />
}

type Pending = 'submit' | 'lock' | null

function useReportMutation<A>(id: number, fn: (arg: A) => Promise<ReportDetail>, onDone?: () => void) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: fn,
    onSuccess: (detail) => {
      queryClient.setQueryData(['report', id], detail)
      void queryClient.invalidateQueries({ queryKey: ['reports'] })
      void queryClient.invalidateQueries({ queryKey: ['summary'] })
      onDone?.()
    },
  })
}

function ReportView({ report }: { report: ReportDetail }) {
  const me = useCurrentUser()
  const [confirm, setConfirm] = useState<Pending>(null)
  const submit = useReportMutation(report.id, () => api.submit(report.id), () => setConfirm(null))
  const lock = useReportMutation(report.id, () => api.lock(report.id), () => setConfirm(null))
  const action = confirm === 'lock' ? lock : submit

  const readOnlyReason = report.locked
    ? '这份日报已锁定：任何角色都不能再修改工时或提交，仍可追加备注。'
    : !report.can.editCrew
      ? `你的角色（${roleLabel[me.role]}）只能查看工时，不能修改。`
      : null

  return (
    <>
      <div className="pb-2">{backLink}</div>
      <PageHeader
        title={`${report.jobName} · ${report.businessDate}`}
        description={
          <div className="flex flex-wrap items-center gap-2">
            <StatusBadge status={report.status} />
            <LateBadge daysLate={report.daysLate} />
            <span>
              {report.submittedAt ? `提交于 ${formatTime(report.submittedAt)}` : '尚未提交'} · 工地编号{' '}
              <span className="font-mono">{report.jobId}</span>
            </span>
          </div>
        }
        actions={
          <>
            {report.can.submit && <Button onClick={() => setConfirm('submit')}>提交日报</Button>}
            {report.can.lock && (
              <Button variant="secondary" onClick={() => setConfirm('lock')}>
                锁定
              </Button>
            )}
          </>
        }
      />

      {readOnlyReason && (
        <Alert tone="info" title={report.locked ? '只读：已锁定' : '只读'} className="mb-4">
          {readOnlyReason}
        </Alert>
      )}

      <div className="grid gap-6 lg:grid-cols-3">
        <section aria-labelledby="crew-heading" className="flex flex-col gap-3 lg:col-span-2">
          <div className="flex items-baseline justify-between">
            <h2 id="crew-heading" className="text-lg font-semibold text-fg">
              工时
            </h2>
            <span className="text-sm text-muted">
              合计 <span className="font-semibold text-fg tabular-nums">{formatHours(report.totalHours)}</span> 小时
            </span>
          </div>
          {report.can.editCrew ? (
            // Keyed on the saved crew so a successful save (or a refetch) resets the draft.
            <CrewEditor key={JSON.stringify(report.crew)} report={report} />
          ) : (
            <CrewTable crew={report.crew} />
          )}
        </section>

        <NotesSection report={report} />
      </div>

      <Dialog
        open={confirm !== null}
        onOpenChange={(open) => {
          if (!open) {
            setConfirm(null)
            action.reset()
          }
        }}
        title={confirm === 'lock' ? '锁定这份日报？' : '提交这份日报？'}
        description={
          confirm === 'lock'
            ? '锁定后任何人都不能再修改工时或提交，仍可追加备注。锁定不能撤销。'
            : '提交后不能撤回；业务日期之后提交会记为迟交。'
        }
        footer={
          <>
            <Button variant="secondary" onClick={() => setConfirm(null)}>
              取消
            </Button>
            <Button loading={action.isPending} onClick={() => action.mutate(undefined)}>
              {confirm === 'lock' ? '确认锁定' : '确认提交'}
            </Button>
          </>
        }
      >
        {action.isError && <Alert tone="danger">{errorMessage(action.error)}</Alert>}
      </Dialog>
    </>
  )
}

function CrewTable({ crew }: { crew: CrewEntry[] }) {
  if (crew.length === 0) return <EmptyState title="没有工时记录" description="这份日报没有登记任何工人的工时。" />
  return (
    <Table caption="工人工时">
      <TableHead>
        <TableRow>
          <TableHeaderCell>工人</TableHeaderCell>
          <TableHeaderCell align="right">工时（小时）</TableHeaderCell>
        </TableRow>
      </TableHead>
      <TableBody>
        {crew.map((c) => (
          <TableRow key={c.worker}>
            <TableCell>{c.worker}</TableCell>
            <TableCell align="right">{formatHours(c.hours)}</TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

interface DraftRow {
  key: number
  worker: string
  hours: string
}

let nextKey = 1
const toDraft = (crew: CrewEntry[]): DraftRow[] =>
  crew.map((c) => ({ key: nextKey++, worker: c.worker, hours: String(c.hours) }))

function validate(rows: DraftRow[]): { errors: Map<number, string>; crew: CrewEntry[] } {
  const errors = new Map<number, string>()
  const seen = new Set<string>()
  const crew: CrewEntry[] = []
  for (const r of rows) {
    const worker = r.worker.trim()
    const hours = Number(r.hours)
    if (!worker) errors.set(r.key, '请填写工人姓名')
    else if (seen.has(worker)) errors.set(r.key, '工人重复')
    else if (r.hours.trim() === '' || !Number.isFinite(hours)) errors.set(r.key, '请填写工时')
    else if (hours < 0) errors.set(r.key, '工时不能为负数')
    seen.add(worker)
    crew.push({ worker, hours })
  }
  return { errors, crew }
}

function CrewEditor({ report }: { report: ReportDetail }) {
  const [rows, setRows] = useState<DraftRow[]>(() => toDraft(report.crew))
  const [showErrors, setShowErrors] = useState(false)
  const save = useReportMutation(report.id, (crew: CrewEntry[]) => api.updateCrew(report.id, crew))
  const { errors, crew } = validate(rows)
  const dirty = JSON.stringify(crew) !== JSON.stringify(report.crew)

  const update = (key: number, patch: Partial<DraftRow>) =>
    setRows((rs) => rs.map((r) => (r.key === key ? { ...r, ...patch } : r)))

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    setShowErrors(true)
    if (errors.size > 0) return
    save.mutate(crew)
  }

  return (
    <form className="flex flex-col gap-3" onSubmit={onSubmit} noValidate aria-label="编辑工时">
      {save.isError && <Alert tone="danger">{errorMessage(save.error)}</Alert>}
      {rows.length === 0 ? (
        <EmptyState title="还没有工时" description="添加工人并填写工时，然后保存。" />
      ) : (
        <Table caption="编辑工人工时">
          <TableHead>
            <TableRow>
              <TableHeaderCell>工人</TableHeaderCell>
              <TableHeaderCell>工时（小时）</TableHeaderCell>
              <TableHeaderCell>
                <span className="sr-only">操作</span>
              </TableHeaderCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {rows.map((r, i) => {
              const error = showErrors ? errors.get(r.key) : undefined
              return (
                <TableRow key={r.key}>
                  <TableCell>
                    <Input
                      aria-label={`第 ${i + 1} 行工人`}
                      value={r.worker}
                      invalid={Boolean(error)}
                      aria-describedby={error ? `crew-error-${r.key}` : undefined}
                      onChange={(e) => update(r.key, { worker: e.target.value })}
                    />
                    {error && (
                      <p id={`crew-error-${r.key}`} className="pt-1 text-xs text-danger">
                        {error}
                      </p>
                    )}
                  </TableCell>
                  <TableCell>
                    <Input
                      aria-label={`第 ${i + 1} 行工时`}
                      type="number"
                      min={0}
                      step={0.5}
                      inputMode="decimal"
                      value={r.hours}
                      invalid={Boolean(error)}
                      aria-describedby={error ? `crew-error-${r.key}` : undefined}
                      onChange={(e) => update(r.key, { hours: e.target.value })}
                    />
                  </TableCell>
                  <TableCell align="right">
                    <Button
                      variant="ghost"
                      size="sm"
                      aria-label={`删除第 ${i + 1} 行`}
                      onClick={() => setRows((rs) => rs.filter((x) => x.key !== r.key))}
                    >
                      删除
                    </Button>
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      )}
      <div className="flex flex-wrap items-center justify-between gap-2">
        <Button
          variant="secondary"
          onClick={() => setRows((rs) => [...rs, { key: nextKey++, worker: '', hours: '' }])}
        >
          添加工人
        </Button>
        <div className="flex items-center gap-2">
          {dirty && <span className="text-sm text-muted">有未保存的修改，合计在保存后更新</span>}
          <Button type="submit" loading={save.isPending} disabled={!dirty}>
            保存工时
          </Button>
        </div>
      </div>
    </form>
  )
}

function NotesSection({ report }: { report: ReportDetail }) {
  const [body, setBody] = useState('')
  const [error, setError] = useState<string>()
  const add = useReportMutation(report.id, (text: string) => api.addNote(report.id, text), () => setBody(''))
  const length = [...body].length

  const onSubmit = (e: FormEvent) => {
    e.preventDefault()
    if (!body.trim()) return setError('备注不能为空')
    if (length > MAX_NOTE_RUNES) return setError(`备注不能超过 ${MAX_NOTE_RUNES} 个字`)
    setError(undefined)
    add.mutate(body)
  }

  return (
    <section aria-labelledby="notes-heading" className="flex flex-col gap-3">
      <h2 id="notes-heading" className="text-lg font-semibold text-fg">
        备注
      </h2>
      {report.can.addNote && (
        <form className="flex flex-col gap-2" onSubmit={onSubmit} noValidate>
          {add.isError && <Alert tone="danger">{errorMessage(add.error)}</Alert>}
          <Field label="追加备注" error={error} hint={`${length} / ${MAX_NOTE_RUNES}`}>
            <Textarea value={body} onChange={(e) => setBody(e.target.value)} />
          </Field>
          <div className="flex justify-end">
            <Button type="submit" size="sm" loading={add.isPending}>
              追加
            </Button>
          </div>
        </form>
      )}
      {report.notes.length === 0 ? (
        <EmptyState title="还没有备注" />
      ) : (
        <ol aria-label="备注列表（新的在前）" className="flex flex-col gap-2">
          {report.notes.map((n) => (
            <li key={n.id} className="rounded-md border border-border bg-surface p-3">
              <p className="text-sm whitespace-pre-wrap text-fg">{n.body}</p>
              <p className="pt-1 text-xs text-muted">
                {n.authorName} · <time dateTime={n.createdAt}>{formatTime(n.createdAt)}</time>
              </p>
            </li>
          ))}
        </ol>
      )}
    </section>
  )
}
