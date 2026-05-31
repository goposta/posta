import api from './client'
import type { PaginatedResponse, Event } from './types'

export const auditApi = {
  // Workspace audit log. The X-Posta-Workspace-Id header (injected by the
  // workspace store) scopes this to the active workspace.
  list(page = 0, size = 20, category?: string) {
    const params: Record<string, any> = { page, size }
    if (category) params.category = category
    return api.get<PaginatedResponse<Event>>('/workspaces/current/audit-log', { params })
  },
}
