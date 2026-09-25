// 整改任务状态枚举（与后端 backend/internal/constants/rectification.go 保持一致）
export const RectTaskStatus = {
  PENDING: 'pending',
  SUBMITTED: 'submitted',
  REJECTED: 'rejected',
  APPROVED: 'approved',
} as const

export const RectTaskStatusText: Record<string, string> = {
  [RectTaskStatus.PENDING]: '待整改',
  [RectTaskStatus.SUBMITTED]: '待复查',
  [RectTaskStatus.REJECTED]: '已退回',
  [RectTaskStatus.APPROVED]: '复查通过',
}

export const RectTaskStatusColor: Record<string, string> = {
  [RectTaskStatus.PENDING]: 'orange',
  [RectTaskStatus.SUBMITTED]: 'blue',
  [RectTaskStatus.REJECTED]: 'volcano',
  [RectTaskStatus.APPROVED]: 'green',
}

export const RectHistoryActionText: Record<string, string> = {
  register: '登记任务',
  submit: '提交整改',
  reject: '复查退回',
  approve: '复查通过',
}

export const RectHistoryActionColor: Record<string, string> = {
  register: 'default',
  submit: 'blue',
  reject: 'red',
  approve: 'green',
}
