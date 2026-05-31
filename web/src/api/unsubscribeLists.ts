import api from './client'
import type { ApiResponse, PaginatedResponse, UnsubscribeListItem, UnsubscribeListInput } from './types'

export const unsubscribeListsApi = {
  list(page = 0, size = 20) {
    return api.get<PaginatedResponse<UnsubscribeListItem>>('/workspaces/current/unsubscribe-lists', { params: { page, size } })
  },
  get(id: number) {
    return api.get<ApiResponse<UnsubscribeListItem>>(`/workspaces/current/unsubscribe-lists/${id}`)
  },
  create(data: UnsubscribeListInput) {
    return api.post<ApiResponse<UnsubscribeListItem>>('/workspaces/current/unsubscribe-lists', data)
  },
  update(id: number, data: Partial<UnsubscribeListInput>) {
    return api.put<ApiResponse<UnsubscribeListItem>>(`/workspaces/current/unsubscribe-lists/${id}`, data)
  },
  delete(id: number) {
    return api.delete(`/workspaces/current/unsubscribe-lists/${id}`)
  },
}
