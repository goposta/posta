import api from './client'
import type {
  ApiResponse,
  PaginatedResponse,
  Form,
  FormAppearance,
  FormField,
  FormMode,
  FormSnippet,
  Message,
  MessageFilterRule,
  MessagePriority,
  MessageReply,
  MessageStats,
  SubmissionQuery,
  SubmissionSummary,
  TagCount,
  FormRule,
  FormRuleInput,
  RuleTestResult,
  Team,
  TeamInput,
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
  mode?: FormMode
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
  redact_pii_in_webhooks?: boolean
  reply_from?: string
  reply_from_name?: string
  retention_days?: number
  fields?: FormField[]
  unknown_fields?: 'keep' | 'drop' | 'reject'
  error_redirect_url?: string
  confirmation_message?: string
  closed_message?: string
  close_at?: string
  max_responses?: number
  allow_another_response?: boolean
  appearance?: FormAppearance
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
  tag?: string
  priority?: string
  team_id?: number
  mine?: boolean
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
  updateTags(uuid: string, payload: { add?: string[]; remove?: string[] }) {
    return api.put<ApiResponse<Message>>(`/workspaces/current/messages/${uuid}/tags`, payload)
  },
  setPriority(uuid: string, priority: MessagePriority) {
    return api.put<ApiResponse<Message>>(`/workspaces/current/messages/${uuid}/priority`, { priority })
  },
  tags() {
    return api.get<ApiResponse<TagCount[]>>('/workspaces/current/message-tags')
  },
  setTeam(uuid: string, teamId: number | null) {
    return api.put<ApiResponse<Message>>(`/workspaces/current/messages/${uuid}/team`, { team_id: teamId })
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

// Submission filters become flat query parameters: field.<key>=v for
// equality, field.<key>[gte]=n and field.<key>[lte]=n for ranges.
function submissionParams(formId: number, query: SubmissionQuery): Record<string, string | number> {
  const params: Record<string, string | number> = { form_id: formId }
  for (const k of ['page', 'size', 'status', 'q', 'after', 'before', 'tag', 'priority'] as const) {
    const v = query[k]
    if (v !== undefined && v !== '') params[k] = v
  }
  for (const f of query.filters || []) {
    params[f.op === 'eq' ? `field.${f.key}` : `field.${f.key}[${f.op}]`] = f.value
  }
  return params
}

export const formSubmissionsApi = {
  list(formId: number, query: SubmissionQuery = {}) {
    return api.get<PaginatedResponse<Message>>('/workspaces/current/form-submissions', {
      params: submissionParams(formId, { page: 0, size: 25, ...query }),
    })
  },
  summary(formId: number, query: SubmissionQuery = {}) {
    return api.get<ApiResponse<SubmissionSummary>>('/workspaces/current/form-submissions/summary', {
      params: submissionParams(formId, query),
    })
  },
  // A browser download cannot send the workspace header, so the workspace
  // travels as a query parameter (WorkspaceFromQueryOrHeader).
  exportUrl(formId: number, format: 'csv' | 'json', query: SubmissionQuery = {}) {
    const { page: _page, size: _size, ...rest } = query
    const params = new URLSearchParams()
    for (const [k, v] of Object.entries(submissionParams(formId, rest))) params.set(k, String(v))
    params.set('format', format)
    const ws = workspaceParam()
    return `/api/v1/workspaces/current/form-submissions/export?${params.toString()}${ws ? '&' + ws : ''}`
  },
}

// Routing rules belong to a form and run in order.
export const formRulesApi = {
  list(formId: number) {
    return api.get<ApiResponse<FormRule[]>>(`/workspaces/current/forms/${formId}/rules`)
  },
  create(formId: number, rule: FormRuleInput) {
    return api.post<ApiResponse<FormRule>>(`/workspaces/current/forms/${formId}/rules`, rule)
  },
  update(formId: number, ruleId: number, rule: FormRuleInput) {
    return api.put<ApiResponse<FormRule>>(`/workspaces/current/forms/${formId}/rules/${ruleId}`, rule)
  },
  delete(formId: number, ruleId: number) {
    return api.delete(`/workspaces/current/forms/${formId}/rules/${ruleId}`)
  },
  reorder(formId: number, ids: number[]) {
    return api.put<ApiResponse<FormRule[]>>(`/workspaces/current/forms/${formId}/rules/order`, { ids })
  },
  test(formId: number, rules?: FormRuleInput[], limit = 100) {
    return api.post<ApiResponse<RuleTestResult>>(`/workspaces/current/forms/${formId}/rules/test`, { rules, limit })
  },
}

export const teamsApi = {
  list() {
    return api.get<ApiResponse<Team[]>>('/workspaces/current/teams')
  },
  create(team: TeamInput) {
    return api.post<ApiResponse<Team>>('/workspaces/current/teams', team)
  },
  update(id: number, team: TeamInput) {
    return api.put<ApiResponse<Team>>(`/workspaces/current/teams/${id}`, team)
  },
  delete(id: number) {
    return api.delete(`/workspaces/current/teams/${id}`)
  },
}
