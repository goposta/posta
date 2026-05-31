import api from './client'
import type { ApiResponse, PaginatedResponse, Campaign, CampaignMessage, CampaignAnalyticsData } from './types'

export const campaignsApi = {
  list(page = 0, size = 20, status?: string) {
    return api.get<PaginatedResponse<Campaign>>('/workspaces/current/campaigns', { params: { page, size, status } })
  },
  get(id: number) {
    return api.get<ApiResponse<Campaign>>(`/workspaces/current/campaigns/${id}`)
  },
  create(data: {
    name: string
    subject: string
    from_email: string
    from_name?: string
    template_id: number
    template_version_id?: number
    language?: string
    template_data?: Record<string, any>
    list_id: number
    send_rate?: number
    scheduled_at?: string
  }) {
    return api.post<ApiResponse<Campaign>>('/workspaces/current/campaigns', data)
  },
  update(id: number, data: Partial<{
    name: string
    subject: string
    from_email: string
    from_name: string
    template_id: number
    template_version_id: number
    language: string
    template_data: Record<string, any>
    list_id: number
    send_rate: number
    scheduled_at: string
  }>) {
    return api.put<ApiResponse<Campaign>>(`/workspaces/current/campaigns/${id}`, data)
  },
  delete(id: number) {
    return api.delete(`/workspaces/current/campaigns/${id}`)
  },
  send(id: number) {
    return api.post<ApiResponse<Campaign>>(`/workspaces/current/campaigns/${id}/send`)
  },
  pause(id: number) {
    return api.post<ApiResponse<Campaign>>(`/workspaces/current/campaigns/${id}/pause`)
  },
  resume(id: number) {
    return api.post<ApiResponse<Campaign>>(`/workspaces/current/campaigns/${id}/resume`)
  },
  cancel(id: number) {
    return api.post<ApiResponse<Campaign>>(`/workspaces/current/campaigns/${id}/cancel`)
  },
  listMessages(id: number, page = 0, size = 20, status?: string) {
    return api.get<PaginatedResponse<CampaignMessage>>(`/workspaces/current/campaigns/${id}/messages`, { params: { page, size, status } })
  },
  analytics(id: number) {
    return api.get<ApiResponse<CampaignAnalyticsData>>(`/workspaces/current/campaigns/${id}/analytics`)
  },
}
