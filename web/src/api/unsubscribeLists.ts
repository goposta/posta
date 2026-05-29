import api from './client'
import type { ApiResponse, PaginatedResponse, UnsubscribeListItem, UnsubscribeListInput } from './types'

export const unsubscribeListsApi = {
  list(page = 0, size = 20) {
    return api.get<PaginatedResponse<UnsubscribeListItem>>('/users/me/unsubscribe-lists', { params: { page, size } })
  },
  get(id: number) {
    return api.get<ApiResponse<UnsubscribeListItem>>(`/users/me/unsubscribe-lists/${id}`)
  },
  create(data: UnsubscribeListInput) {
    return api.post<ApiResponse<UnsubscribeListItem>>('/users/me/unsubscribe-lists', data)
  },
  update(id: number, data: Partial<UnsubscribeListInput>) {
    return api.put<ApiResponse<UnsubscribeListItem>>(`/users/me/unsubscribe-lists/${id}`, data)
  },
  delete(id: number) {
    return api.delete(`/users/me/unsubscribe-lists/${id}`)
  },
}
