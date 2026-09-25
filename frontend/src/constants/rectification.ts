// 整改任务状态枚举（与后端 backend/internal/constants/rectification.go 保持一致）
export const RectificationStatus = {
  PENDING: 'pending',
  SUBMITTED: 'submitted',
  RETURNED: 'returned',
  APPROVED: 'approved',
} as const

export const RectificationStatusText: Record<string, string> = {
  [RectificationStatus.PENDING]: '待整改',
  [RectificationStatus.SUBMITTED]: '待复查',
  [RectificationStatus.RETURNED]: '已退回',
  [RectificationStatus.APPROVED]: '复查通过',
}

export const RectificationStatusColor: Record<string, string> = {
  [RectificationStatus.PENDING]: 'orange',
  [RectificationStatus.SUBMITTED]: 'processing',
  [RectificationStatus.RETURNED]: 'red',
  [RectificationStatus.APPROVED]: 'green',
}

// 整改办理记录动作（与后端 RectificationAction 保持一致）
export const RectificationActionText: Record<string, string> = {
  assign: '登记整改',
  submit: '提交整改',
  review_pass: '复查通过',
  review_return: '复查退回',
}

export const RectificationFilterOptions = [
  { label: '待整改', value: 'pending' },
  { label: '已逾期', value: 'overdue' },
]

export const RectificationStatusOptions = Object.entries(RectificationStatusText).map(([value, label]) => ({ label, value }))
