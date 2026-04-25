import api from './client'
import type { ApiResponse, PaginatedResponse, InboundEmail } from './types'

export interface InboundListParams {
  page?: number
  size?: number
  status?: string
  source?: string
  sender?: string
  q?: string
}

export interface InboundRetryResponse {
  id: string
  status: string
}

export const inboundApi = {
  list(params: InboundListParams = {}) {
    return api.get<PaginatedResponse<InboundEmail>>('/users/me/inbound-emails', {
      params: { page: 0, size: 20, ...params },
    })
  },
  get(uuid: string) {
    return api.get<ApiResponse<InboundEmail>>(`/users/me/inbound-emails/${uuid}`)
  },
  delete(uuid: string) {
    return api.delete<ApiResponse<void>>(`/users/me/inbound-emails/${uuid}`)
  },
  retry(uuid: string) {
    return api.post<ApiResponse<InboundRetryResponse>>(`/users/me/inbound-emails/${uuid}/retry`)
  },
  rawUrl(uuid: string) {
    return `/api/v1/users/me/inbound-emails/${uuid}/raw`
  },
  attachmentUrl(uuid: string, idx: number) {
    return `/api/v1/users/me/inbound-emails/${uuid}/attachments/${idx}`
  },
  streamUrl(token?: string) {
    const t = token ?? localStorage.getItem('posta_token') ?? ''
    const qs = t ? `?token=${encodeURIComponent(t)}` : ''
    return `/api/v1/users/me/inbound-stream${qs}`
  },
}
