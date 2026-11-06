import api from './client'
import type { PaginatedResponse, Event } from './types'

export const auditApi = {
  // The X-Posta-Workspace-Id header scopes this to the active workspace; it always returns audit-category events.
  list(page = 0, size = 20) {
    return api.get<PaginatedResponse<Event>>('/workspaces/current/audit-log', {
      params: { page, size },
    })
  },

  get(id: number) {
    return api.get<{ success: boolean; data: Event }>(`/workspaces/current/audit-log/${id}`)
  },
}
