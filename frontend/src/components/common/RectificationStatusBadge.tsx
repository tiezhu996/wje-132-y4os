import { Space, Tag } from 'antd'
import { WarningOutlined } from '@ant-design/icons'
import { RectificationStatusColor, RectificationStatusText } from '@/constants/rectification'

// RectificationStatusBadge 整改任务状态徽章：逾期任务附带醒目红色「已逾期」标记。
export default function RectificationStatusBadge({ status, overdue }: { status: string; overdue?: boolean }) {
  return (
    <Space size={4}>
      <Tag color={RectificationStatusColor[status] || 'default'}>{RectificationStatusText[status] || status}</Tag>
      {overdue && (
        <Tag icon={<WarningOutlined />} color="red">
          已逾期
        </Tag>
      )}
    </Space>
  )
}
