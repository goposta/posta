import api from './client'
import type { ApiResponse, PaginatedResponse, SharedServer, SharedServerInput } from './types'

export const serversApi = {
  list(page = 0, size = 20, search = '') {
    return api.get<PaginatedResponse<SharedServer>>('/admin/servers', { params: { page, size, search: search || undefined } })
  },
  get(id: number) {
    return api.get<ApiResponse<SharedServer>>(`/admin/servers/${id}`)
  },
  create(data: SharedServerInput) {
    return api.post<ApiResponse<SharedServer>>('/admin/servers', data)
  },
  update(id: number, data: Partial<SharedServerInput>) {
    return api.put<ApiResponse<SharedServer>>(`/admin/servers/${id}`, data)
  },
  delete(id: number) {
    return api.delete(`/admin/servers/${id}`)
  },
  enable(id: number) {
    return api.post<ApiResponse<SharedServer>>(`/admin/servers/${id}/enable`)
  },
  disable(id: number) {
    return api.post<ApiResponse<SharedServer>>(`/admin/servers/${id}/disable`)
  },
  test(id: number) {
    return api.post<ApiResponse<{ success: boolean; message: string }>>(`/admin/servers/${id}/test`)
  },
}
