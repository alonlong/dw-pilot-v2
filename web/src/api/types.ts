// JSON shapes shared with the Go API. Mirrors docs/api.md field for field; change both together.

export type Role = 'admin' | 'manager' | 'field' | 'finance'

/** Same strings as daily.StatusDraft / StatusSubmitted / StatusLocked. */
export type Status = '未提交' | '已提交' | '已锁定'

export interface Me {
  id: number
  email: string
  name: string
  role: Role
  tenant: string
  tenantName: string
}

export interface Job {
  id: string
  name: string
  address: string
}

/** One row of GET /api/reports: daily.Row plus id and job name. */
export interface ReportRow {
  id: number
  jobId: string
  jobName: string
  businessDate: string
  status: Status
  totalHours: number
  daysLate: number
}

export interface CrewEntry {
  worker: string
  hours: number
}

export interface Note {
  id: number
  body: string
  authorName: string
  createdAt: string
}

export interface ReportPermissions {
  editCrew: boolean
  submit: boolean
  lock: boolean
  addNote: boolean
}

export interface ReportDetail {
  id: number
  jobId: string
  jobName: string
  businessDate: string
  status: Status
  submittedAt: string | null
  locked: boolean
  totalHours: number
  daysLate: number
  /** Sorted by worker. */
  crew: CrewEntry[]
  /** Newest first (daily.LatestNotes order). */
  notes: Note[]
  /** Computed by the server from the caller's role and the report's state. */
  can: ReportPermissions
}

export interface SummaryRow {
  jobId: string
  jobName: string
  totalHours: number
  lateCount: number
}

export type ErrorCode =
  | 'unauthenticated'
  | 'bad_credentials'
  | 'forbidden'
  | 'bad_origin'
  | 'not_found'
  | 'bad_request'
  | 'already_exists'
  | 'already_submitted'
  | 'locked'
  | 'negative_hours'
  | 'empty_note'
  | 'note_too_long'
  | 'tenant_outdated'

export interface ErrorBody {
  error: { code: string; message: string }
}

export interface ReportFilter {
  job?: string
  from?: string
  to?: string
}

export interface DateRange {
  from?: string
  to?: string
}
