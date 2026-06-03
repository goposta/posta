import api from './client'
import type { PaginatedResponse, Event } from './types'

export const auditApi = {
  // Workspace audit log. The X-Posta-Workspace-Id header (injected by the
  // workspace store) scopes this to the active workspace. The endpoint always
  // returns audit-category events for the workspace.
  list(page = 0, size = 20) {
    return api.get<PaginatedResponse<Event>>('/workspaces/current/audit-log', {
      params: { page, size },
    })
  },

  // A single audit event for the active workspace.
  get(id: number) {
    return api.get<{ success: boolean; data: Event }>(`/workspaces/current/audit-log/${id}`)
  },
}
