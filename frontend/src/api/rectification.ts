import request from '@/utils/request'

export function listRectTasks(params: Record<string, unknown>) {
  return request.get('/rectification-tasks', { params })
}

export function getRectTask(id: number) {
  return request.get(`/rectification-tasks/${id}`)
}

export function listRectTasksByInspection(inspectionId: number) {
  return request.get('/rectification-tasks/by-inspection/' + inspectionId)
}

export function registerRectTask(data: { inspection_id: number; item_id: number; assignee_id: number; deadline: string }) {
  return request.post('/rectification-tasks', data)
}

export function submitRectTask(id: number, data: { rectification_note: string; rectification_photo?: string }) {
  return request.post(`/rectification-tasks/${id}/submit`, data)
}

export function reviewRectTask(id: number, data: { approved: boolean; review_note?: string }) {
  return request.post(`/rectification-tasks/${id}/review`, data)
}
