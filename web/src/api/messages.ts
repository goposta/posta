import api from './client'
import type {
  ApiResponse,
  PaginatedResponse,
  Form,
  FormSnippet,
  Message,
  MessageFilterRule,
  MessageReply,
  MessageStats,
} from './types'

const WORKSPACE_KEY = 'posta_workspace_id'

function workspaceParam(): string {
  const ws = localStorage.getItem(WORKSPACE_KEY)
  return ws ? `workspace_id=${encodeURIComponent(ws)}` : ''
}

export interface FormPayload {
  name?: string
  slug?: string
  description?: string
  status?: string
  allowed_origins?: string[]
  strict_origin?: boolean
  redirect_url?: string
  max_body_bytes?: number
  max_fields?: number
  allow_attachments?: boolean
  honeypot_field?: string
  require_nonce?: boolean
  min_fill_seconds?: number
  scan_enabled?: boolean
  flag_threshold?: number
  quarantine_threshold?: number
  reject_threshold?: number
  notify_enabled?: boolean
  notify_emails?: string[]
  notify_mode?: string
  notify_on_flagged?: boolean
  reply_from?: string
  reply_from_name?: string
  retention_days?: number
}

export interface MessageListParams {
  page?: number
  size?: number
  form_id?: number
  status?: string
  state?: string
  unread?: boolean
  q?: string
  after?: string
  before?: string
}

export interface FilterPayload {
  form_id?: number | null
  kind?: string
  pattern?: string
  action?: string
  score?: number
  fields?: string[]
  case_sensitive?: boolean
  enabled?: boolean
  note?: string
}

export interface FilterTestResult {
  scanned: number
  matched: number
  samples: Array<{
    message_uuid: string
    sender_email: string
    subject: string
    excerpt: string
  }>
}

export interface MessageAnalytics {
  daily: Array<{ day: string; total: number; spam: number }>
  total: number
  spam: number
}

export const formsApi = {
  list(params: { page?: number; size?: number } = {}) {
    return api.get<PaginatedResponse<Form>>('/workspaces/current/forms', {
      params: { page: 0, size: 20, ...params },
    })
  },
  get(id: number) {
    return api.get<ApiResponse<Form>>(`/workspaces/current/forms/${id}`)
  },
  create(payload: FormPayload) {
    return api.post<ApiResponse<Form>>('/workspaces/current/forms', payload)
  },
  update(id: number, payload: FormPayload) {
    return api.put<ApiResponse<Form>>(`/workspaces/current/forms/${id}`, payload)
  },
  delete(id: number) {
    return api.delete<ApiResponse<void>>(`/workspaces/current/forms/${id}`)
  },
  rotateKey(id: number) {
    return api.post<ApiResponse<Form>>(`/workspaces/current/forms/${id}/rotate-key`)
  },
  snippet(id: number) {
    return api.get<ApiResponse<FormSnippet>>(`/workspaces/current/forms/${id}/snippet`)
  },
}

export const messagesApi = {
  list(params: MessageListParams = {}) {
    return api.get<PaginatedResponse<Message>>('/workspaces/current/messages', {
      params: { page: 0, size: 20, ...params },
    })
  },
  get(uuid: string) {
    return api.get<ApiResponse<Message>>(`/workspaces/current/messages/${uuid}`)
  },
  delete(uuid: string) {
    return api.delete<ApiResponse<void>>(`/workspaces/current/messages/${uuid}`)
  },
  reply(uuid: string, payload: { subject?: string; html?: string; text?: string }) {
    return api.post<ApiResponse<MessageReply>>(`/workspaces/current/messages/${uuid}/reply`, payload)
  },
  setState(uuid: string, payload: { state: string; read?: boolean }) {
    return api.put<ApiResponse<Message>>(`/workspaces/current/messages/${uuid}/state`, payload)
  },
  assign(uuid: string, userId: number | null) {
    return api.put<ApiResponse<Message>>(`/workspaces/current/messages/${uuid}/assign`, {
      user_id: userId,
    })
  },
  markSpam(uuid: string, payload: { create_filter?: boolean; pattern?: string; kind?: string } = {}) {
    return api.post<ApiResponse<Message>>(`/workspaces/current/messages/${uuid}/spam`, payload)
  },
  markNotSpam(uuid: string) {
    return api.post<ApiResponse<Message>>(`/workspaces/current/messages/${uuid}/not-spam`)
  },
  stats() {
    return api.get<ApiResponse<MessageStats>>('/workspaces/current/messages/stats')
  },
  analytics(days = 30) {
    return api.get<ApiResponse<MessageAnalytics>>('/workspaces/current/messages/analytics', {
      params: { days },
    })
  },
  attachmentUrl(uuid: string, idx: number) {
    const ws = workspaceParam()
    return `/api/v1/workspaces/current/messages/${uuid}/attachments/${idx}${ws ? `?${ws}` : ''}`
  },
  streamUrl() {
    const ws = workspaceParam()
    return `/api/v1/workspaces/current/message-stream${ws ? `?${ws}` : ''}`
  },
}

export const messageFiltersApi = {
  list(params: { page?: number; size?: number } = {}) {
    return api.get<PaginatedResponse<MessageFilterRule>>('/workspaces/current/message-filters', {
      params: { page: 0, size: 50, ...params },
    })
  },
  create(payload: FilterPayload) {
    return api.post<ApiResponse<MessageFilterRule>>('/workspaces/current/message-filters', payload)
  },
  update(id: number, payload: FilterPayload) {
    return api.put<ApiResponse<MessageFilterRule>>(`/workspaces/current/message-filters/${id}`, payload)
  },
  delete(id: number) {
    return api.delete<ApiResponse<void>>(`/workspaces/current/message-filters/${id}`)
  },
  test(payload: { kind: string; pattern: string; case_sensitive?: boolean; limit?: number }) {
    return api.post<ApiResponse<FilterTestResult>>('/workspaces/current/message-filters/test', payload)
  },
}
