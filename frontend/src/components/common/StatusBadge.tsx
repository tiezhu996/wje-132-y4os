import { Tag } from 'antd'
import { IncidentStatusText } from '@/constants/incident'
import { RectTaskStatusText } from '@/constants/rectification'

const statusColor: Record<string, string> = {
  reported: 'red',
  investigating: 'orange',
  resolved: 'blue',
  closed: 'green',
  scheduled: 'default',
  in_progress: 'processing',
  completed: 'green',
  failed: 'red',
  pending: 'orange',
  submitted: 'blue',
  rejected: 'volcano',
  approved: 'green',
  expired: 'default',
}

export default function StatusBadge({ status }: { status: string }) {
  const text = IncidentStatusText[status] || RectTaskStatusText[status] || status
  return <Tag color={statusColor[status] || 'default'}>{text}</Tag>
}
