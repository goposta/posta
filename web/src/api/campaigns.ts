import api from './client'
import type { ApiResponse, PaginatedResponse, Campaign, CampaignMessage, CampaignAnalyticsData } from './types'

export const campaignsApi = {
  list(page = 0, size = 20, status?: string) {
    return api.get<PaginatedResponse<Campaign>>('/users/me/campaigns', { params: { page, size, status } })
  },
  get(id: number) {
    return api.get<ApiResponse<Campaign>>(`/users/me/campaigns/${id}`)
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
    return api.post<ApiResponse<Campaign>>('/users/me/campaigns', data)
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
    return api.put<ApiResponse<Campaign>>(`/users/me/campaigns/${id}`, data)
  },
  delete(id: number) {
    return api.delete(`/users/me/campaigns/${id}`)
  },
  send(id: number) {
    return api.post<ApiResponse<Campaign>>(`/users/me/campaigns/${id}/send`)
  },
  pause(id: number) {
    return api.post<ApiResponse<Campaign>>(`/users/me/campaigns/${id}/pause`)
  },
  resume(id: number) {
    return api.post<ApiResponse<Campaign>>(`/users/me/campaigns/${id}/resume`)
  },
  cancel(id: number) {
    return api.post<ApiResponse<Campaign>>(`/users/me/campaigns/${id}/cancel`)
  },
  listMessages(id: number, page = 0, size = 20, status?: string) {
    return api.get<PaginatedResponse<CampaignMessage>>(`/users/me/campaigns/${id}/messages`, { params: { page, size, status } })
  },
  analytics(id: number) {
    return api.get<ApiResponse<CampaignAnalyticsData>>(`/users/me/campaigns/${id}/analytics`)
  },
}
