import { ApiError } from './client'
import type { ErrorCode } from './types'

const messages: Record<ErrorCode | 'network' | 'unknown', string> = {
  unauthenticated: '登录已过期，请重新登录。',
  bad_credentials: '邮箱或密码错误。',
  forbidden: '你的角色没有权限执行这个操作。',
  bad_origin: '请求来源不被允许，请刷新页面后重试。',
  not_found: '找不到这份数据，可能已被删除或不属于当前租户。',
  bad_request: '提交的内容格式不正确，请检查后重试。',
  already_exists: '这个工地在这一天已经有日报了。',
  already_submitted: '这份日报已经提交过，不能重复提交。',
  locked: '这份日报已锁定，不能再修改工时或提交。',
  negative_hours: '工时不能为负数。',
  empty_note: '备注不能为空。',
  note_too_long: '备注不能超过 500 个字。',
  tenant_outdated: '系统正在升级数据，请稍后再试。',
  network: '无法连接服务器，请检查网络后重试。',
  unknown: '服务器返回了意外的响应，请稍后重试。',
}

/** The Chinese message shown for a failed call, chosen by error.code. */
export function errorMessage(err: unknown): string {
  if (err instanceof ApiError) {
    return (messages as Record<string, string>)[err.code] ?? `操作失败（${err.code}）。`
  }
  return messages.unknown
}

export function isApiError(err: unknown, code: ErrorCode): boolean {
  return err instanceof ApiError && err.code === code
}
