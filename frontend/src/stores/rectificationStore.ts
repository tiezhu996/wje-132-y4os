import { create } from 'zustand'
import { listRectificationTasks } from '@/api/rectification'
import type { RectificationTask } from '@/types'

interface RectificationState {
  list: RectificationTask[]
  total: number
  fetchList: (params?: Record<string, unknown>) => Promise<void>
}

export const useRectificationStore = create<RectificationState>((set) => ({
  list: [],
  total: 0,
  async fetchList(params = {}) {
    const res: any = await listRectificationTasks(params)
    set({ list: res.data.list, total: res.data.total })
  },
}))
