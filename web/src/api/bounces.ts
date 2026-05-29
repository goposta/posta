import api from './client'
import type { PaginatedResponse, Bounce, Suppression, ApiResponse } from './types'

export const bouncesApi = {
  list(page = 0, size = 20) {
    return api.get<PaginatedResponse<Bounce>>('/users/me/bounces', { params: { page, size } })
  },
  create(data: { email_id: number; recipient: string; type: string; reason: string }) {
    return api.post<ApiResponse<Bounce>>('/users/me/bounces', data)
  },
}

export const suppressionsApi = {
  list(page = 0, size = 20, listId?: number) {
    const params: Record<string, number> = { page, size }
    if (listId) params.list_id = listId
    return api.get<PaginatedResponse<Suppression>>('/users/me/suppressions', { params })
  },
  create(data: { email: string; reason: string; list_id?: number }) {
    return api.post<ApiResponse<Suppression>>('/users/me/suppressions', data)
  },
  delete(email: string, listId?: number) {
    const body: Record<string, string | number> = { email }
    if (listId) body.list_id = listId
    return api.delete('/users/me/suppressions', { data: body })
  },
}
