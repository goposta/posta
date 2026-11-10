import api from './client'
import type {
  ApiResponse,
  PaginatedResponse,
  ChatAgentPresence,
  ChatAnalytics,
  ChatSnippet,
  ChatVisitorContext,
  ChatBusinessHours,
  ChatHome,
  ChatTheme,
  ChatLauncher,
  ChatTeaser,
  ChatWidgetAsset,
  ChatConversation,
  ChatMessage,
  ChatOfflineMode,
  ChatPresenceStatus,
  ChatState,
  ChatSummary,
  ChatWidget,
  ChatWidgetSnippet,
  ChatWidgetStatus,
} from './types'

const WORKSPACE_KEY = 'posta_workspace_id'

function workspaceParam(): string {
  const ws = localStorage.getItem(WORKSPACE_KEY)
  return ws ? `workspace_id=${encodeURIComponent(ws)}` : ''
}

export interface ChatWidgetPayload {
  name?: string
  display_name?: string
  status?: ChatWidgetStatus
  allowed_origins?: string[]
  strict_origin?: boolean
  title?: string
  greeting?: string
  accent_color?: string
  position?: 'left' | 'right'
  avatar_url?: string
  show_agent_names?: boolean
  home?: ChatHome
  theme?: ChatTheme
  launcher?: ChatLauncher
  teaser?: ChatTeaser
  one_open_conversation?: boolean
  require_name?: boolean
  require_email?: boolean
  business_hours?: ChatBusinessHours
  offline_mode?: ChatOfflineMode
  offline_greeting?: string
  reply_from?: string
  reply_from_name?: string
  scan_enabled?: boolean
  flag_threshold?: number
  quarantine_threshold?: number
  reject_threshold?: number
  max_message_bytes?: number
  allow_attachments?: boolean
  auto_assign?: boolean
  max_open_per_agent?: number
  retention_days?: number
}

export interface ConversationListParams {
  page?: number
  size?: number
  state?: ChatState
  assigned?: 'me' | 'unassigned' | 'any'
  widget_id?: number
  q?: string
  quarantined?: boolean
}

export interface ChatMessagesParams {
  after?: number
  before?: number
  limit?: number
}

export const chatWidgetsApi = {
  list(params: { page?: number; size?: number } = {}) {
    return api.get<PaginatedResponse<ChatWidget>>('/workspaces/current/chat-widgets', {
      params: { page: 0, size: 20, ...params },
    })
  },
  get(id: number) {
    return api.get<ApiResponse<ChatWidget>>(`/workspaces/current/chat-widgets/${id}`)
  },
  create(payload: ChatWidgetPayload) {
    return api.post<ApiResponse<ChatWidget>>('/workspaces/current/chat-widgets', payload)
  },
  update(id: number, payload: ChatWidgetPayload) {
    return api.put<ApiResponse<ChatWidget>>(`/workspaces/current/chat-widgets/${id}`, payload)
  },
  assets(id: number) {
    return api.get<ApiResponse<ChatWidgetAsset[]>>(`/workspaces/current/chat-widgets/${id}/assets`)
  },
  uploadAsset(id: number, file: File) {
    const form = new FormData()
    form.append('file', file)
    return api.post<ApiResponse<ChatWidgetAsset>>(`/workspaces/current/chat-widgets/${id}/assets`, form, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
  },
  deleteAsset(id: number, asset: string) {
    return api.delete<ApiResponse<void>>(`/workspaces/current/chat-widgets/${id}/assets/${asset}`)
  },
  delete(id: number) {
    return api.delete<ApiResponse<void>>(`/workspaces/current/chat-widgets/${id}`)
  },
  rotateKey(id: number) {
    return api.post<ApiResponse<ChatWidget>>(`/workspaces/current/chat-widgets/${id}/rotate-key`)
  },
  snippet(id: number) {
    return api.get<ApiResponse<ChatWidgetSnippet>>(`/workspaces/current/chat-widgets/${id}/snippet`)
  },
  rotateIdentitySecret(id: number) {
    return api.post<ApiResponse<{ secret: string }>>(`/workspaces/current/chat-widgets/${id}/rotate-identity-secret`)
  },
}

export interface ChatSnippetPayload {
  shortcut: string
  title: string
  body: string
}

export const chatSnippetsApi = {
  list() {
    return api.get<ApiResponse<ChatSnippet[]>>('/workspaces/current/chat-snippets')
  },
  create(payload: ChatSnippetPayload) {
    return api.post<ApiResponse<ChatSnippet>>('/workspaces/current/chat-snippets', payload)
  },
  update(id: number, payload: ChatSnippetPayload) {
    return api.put<ApiResponse<ChatSnippet>>(`/workspaces/current/chat-snippets/${id}`, payload)
  },
  delete(id: number) {
    return api.delete<void>(`/workspaces/current/chat-snippets/${id}`)
  },
}

export const chatAnalyticsApi = {
  get(params: { from?: string; to?: string; widget_id?: number } = {}) {
    return api.get<ApiResponse<ChatAnalytics>>('/workspaces/current/chat/analytics', { params })
  },
}

export const conversationsApi = {
  list(params: ConversationListParams = {}) {
    return api.get<PaginatedResponse<ChatConversation>>('/workspaces/current/conversations', {
      params: { page: 0, size: 30, ...params },
    })
  },
  get(uuid: string) {
    return api.get<ApiResponse<ChatConversation>>(`/workspaces/current/conversations/${uuid}`)
  },
  messages(uuid: string, params: ChatMessagesParams = {}) {
    return api.get<ApiResponse<ChatMessage[]>>(`/workspaces/current/conversations/${uuid}/messages`, {
      params,
    })
  },
  send(uuid: string, payload: { body: string; client_msg_id?: string }) {
    return api.post<ApiResponse<ChatMessage>>(`/workspaces/current/conversations/${uuid}/messages`, payload)
  },
  setState(uuid: string, state: ChatState) {
    return api.put<ApiResponse<ChatConversation>>(`/workspaces/current/conversations/${uuid}/state`, { state })
  },
  assign(uuid: string, userId: number | null) {
    return api.put<ApiResponse<ChatConversation>>(`/workspaces/current/conversations/${uuid}/assign`, {
      user_id: userId,
    })
  },
  visitor(uuid: string) {
    return api.get<ApiResponse<ChatVisitorContext>>(`/workspaces/current/conversations/${uuid}/visitor`)
  },
  ban(uuid: string) {
    return api.post<void>(`/workspaces/current/conversations/${uuid}/ban`)
  },
  unban(uuid: string) {
    return api.delete<void>(`/workspaces/current/conversations/${uuid}/ban`)
  },
  /** Owners and admins: erase the conversation, its messages and files. */
  remove(uuid: string) {
    return api.delete<void>(`/workspaces/current/conversations/${uuid}`)
  },
  upload(uuid: string, file: File, opts: { caption?: string; client_msg_id?: string } = {}) {
    const form = new FormData()
    form.append('file', file)
    if (opts.caption) form.append('caption', opts.caption)
    if (opts.client_msg_id) form.append('client_msg_id', opts.client_msg_id)
    return api.post<ApiResponse<ChatMessage>>(`/workspaces/current/conversations/${uuid}/attachments`, form, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
  },
  /** Fetches an attachment as a Blob: the route needs the workspace header, so no plain <img src>. */
  attachment(uuid: string, messageId: string, idx: number) {
    return api.get<Blob>(`/workspaces/current/conversations/${uuid}/messages/${messageId}/attachments/${idx}`, {
      responseType: 'blob',
    })
  },
  typing(uuid: string) {
    return api.post<void>(`/workspaces/current/conversations/${uuid}/typing`)
  },
  read(uuid: string, seq: number) {
    return api.post<ApiResponse<{ seq: number }>>(`/workspaces/current/conversations/${uuid}/read`, { seq })
  },
  presence() {
    return api.get<ApiResponse<ChatAgentPresence[]>>('/workspaces/current/chat/presence')
  },
  setPresence(status: ChatPresenceStatus) {
    return api.put<void>('/workspaces/current/chat/presence', { status })
  },
  /** Marks the agent offline with keepalive so it survives unload; the workspace is explicit because the stored one may already have changed. */
  goOffline(workspaceId: number) {
    const base = (import.meta.env.VITE_API_URL || '/api/v1') as string
    try {
      fetch(`${base}/workspaces/current/chat/presence`, {
        method: 'PUT',
        keepalive: true,
        credentials: 'include',
        headers: { 'Content-Type': 'application/json', 'X-Posta-Workspace-Id': String(workspaceId) },
        body: JSON.stringify({ status: 'offline' }),
      }).catch(() => {})
    } catch {
      // Best effort: the heartbeat expires on its own within 75 seconds.
    }
  },
  summary() {
    return api.get<ApiResponse<ChatSummary>>('/workspaces/current/chat/summary')
  },
  streamUrl() {
    const ws = workspaceParam()
    return `/api/v1/workspaces/current/chat-stream${ws ? `?${ws}` : ''}`
  },
}
