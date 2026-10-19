import {
  Alert,
  Badge,
  Button,
  EmptyState,
  Field,
  Input,
  PageHeader,
  Spinner,
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeaderCell,
  TableRow,
} from '@/ui'
import { sampleSummary } from './data'

// Proposed formatHours: always one decimal place, keep thousands separators.
const hoursFmt = new Intl.NumberFormat('zh-CN', { minimumFractionDigits: 1, maximumFractionDigits: 1 })
const formatHours = (h: number) => hoursFmt.format(h)

type Props = { screen?: string; state?: string }

export default function Prototype({ state = 'normal' }: Props) {
  if (state === 'noPermission') {
    return (
      <>
        <PageHeader title="工时汇总" />
        <Alert tone="warning" title="无权限">
          你的角色（现场）不能查看工时汇总。需要时请联系管理员。
        </Alert>
      </>
    )
  }

  const filtered = state === 'empty'

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
          <Input type="date" defaultValue={filtered ? '2000-01-01' : ''} />
        </Field>
        <Field label="结束日期">
          <Input type="date" defaultValue={filtered ? '2000-01-01' : ''} />
        </Field>
        <div>
          <Button variant="ghost" disabled={!filtered}>
            清除日期
          </Button>
        </div>
      </form>

      {state === 'loading' ? (
        <div className="flex justify-center py-10">
          <Spinner label="正在加载汇总" showLabel />
        </div>
      ) : state === 'error' ? (
        <Alert
          tone="danger"
          title="汇总加载失败"
          action={
            <Button variant="secondary" size="sm">
              重试
            </Button>
          }
        >
          服务暂时不可用，请稍后重试。
        </Alert>
      ) : state === 'empty' ? (
        <EmptyState title="所选日期范围内没有已提交的日报" description="只有已提交或迟交的日报会计入汇总。" />
      ) : (
        <Table caption={`按工地汇总，共 ${sampleSummary.length} 个工地`}>
          <TableHead>
            <TableRow>
              <TableHeaderCell>工地</TableHeaderCell>
              <TableHeaderCell align="right">总工时（小时）</TableHeaderCell>
              <TableHeaderCell align="right">迟交份数</TableHeaderCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {sampleSummary.map((r) => (
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
