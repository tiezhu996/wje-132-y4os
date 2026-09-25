import request from '@/utils/request'

export function listRectificationTasks(params: Record<string, unknown>) {
  return request.get('/rectification-tasks', { params })
}

export function getRectificationTask(id: number) {
  return request.get(`/rectification-tasks/${id}`)
}

export function assignRectificationTask(id: number, data: { assignee_id: number; deadline: string }) {
  return request.post(`/rectification-tasks/${id}/assign`, data)
}

export function submitRectificationTask(id: number, data: { note: string }) {
  return request.post(`/rectification-tasks/${id}/submit`, data)
}

export function reviewRectificationTask(id: number, data: { passed: boolean; note?: string }) {
  return request.post(`/rectification-tasks/${id}/review`, data)
}
