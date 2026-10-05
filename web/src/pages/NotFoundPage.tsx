import { Link } from 'react-router'
import { EmptyState } from '@/ui'

export function NotFoundPage() {
  return (
    <EmptyState
      title="页面不存在"
      description="地址可能输错了。"
      action={
        <Link to="/reports" className="text-sm font-medium text-primary underline">
          返回日报台账
        </Link>
      }
    />
  )
}
