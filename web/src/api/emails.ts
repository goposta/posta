import api from './client'
import type { ApiResponse, PaginatedResponse, Email } from './types'

export interface RetryResponse {
  id: string
  status: string
}

export interface EmailPreviewRequest {
  template: string
  language?: string
  template_data?: Record<string, any>
}

export interface EmailPreviewResponse {
  subject: string
  html: string
  text: string
}

export const emailsApi = {
  list(page = 0, size = 20) {
    return api.get<PaginatedResponse<Email>>('/workspaces/current/emails', { params: { page, size } })
  },
  get(uuid: string) {
    return api.get<ApiResponse<Email>>(`/workspaces/current/emails/${uuid}`)
  },
  retry(uuid: string) {
    return api.post<ApiResponse<RetryResponse>>(`/workspaces/current/emails/${uuid}/retry`)
  },
  preview(data: EmailPreviewRequest) {
    return api.post<ApiResponse<EmailPreviewResponse>>('/workspaces/current/emails/preview', data)
  },
}
