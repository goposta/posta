import api from './client'
import type { ApiResponse, PaginatedResponse, Language, LanguageInput } from './types'

export const languagesApi = {
  list(page = 0, size = 100) {
    return api.get<PaginatedResponse<Language>>('/workspaces/current/languages', { params: { page, size } })
  },
  create(data: LanguageInput) {
    return api.post<ApiResponse<Language>>('/workspaces/current/languages', data)
  },
  update(id: number, data: Partial<LanguageInput>) {
    return api.put<ApiResponse<Language>>(`/workspaces/current/languages/${id}`, data)
  },
  delete(id: number) {
    return api.delete(`/workspaces/current/languages/${id}`)
  },
}
