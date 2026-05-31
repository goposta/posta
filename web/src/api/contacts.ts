import api from './client'
import type { ApiResponse, PaginatedResponse, Contact } from './types'

export const contactsApi = {
  list(page = 0, size = 20, search = '') {
    return api.get<PaginatedResponse<Contact>>('/workspaces/current/contacts', { params: { page, size, search: search || undefined } })
  },
  get(id: number) {
    return api.get<ApiResponse<Contact>>(`/workspaces/current/contacts/${id}`)
  },
}
