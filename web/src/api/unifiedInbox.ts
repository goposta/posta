import api from './client'
import type { PaginatedResponse } from './types'

export type InboxSource = 'form' | 'inbound' | 'chat'
export type InboxState = 'open' | 'closed' | 'spam'

/** One row of the unified inbox: a form message, an inbound email or a chat. */
export interface InboxItem {
  source: InboxSource
  /** UUID of the message, inbound email or conversation. */
  id: string
  title: string
  from_name: string
  from_email: string
  preview: string
  state: InboxState
  assigned_to_id?: number | null
  assigned_to_name?: string
  unread: boolean
  last_activity_at: string
  created_at: string
}

export interface InboxListParams {
  page?: number
  size?: number
  source?: InboxSource
  state?: InboxState
  assigned?: 'me' | 'unassigned' | 'any'
  q?: string
}

export const unifiedInboxApi = {
  /** 404 when the server runs none of forms, inbound or chat. */
  list(params: InboxListParams = {}) {
    return api.get<PaginatedResponse<InboxItem>>('/workspaces/current/inbox', {
      params: { page: 0, size: 30, ...params },
    })
  },
}

/** The dashboard route that opens an inbox item in its own view. */
export function inboxItemRoute(item: InboxItem): string {
  switch (item.source) {
    case 'form':
      return `/messages/${item.id}`
    case 'inbound':
      return `/inbound-emails/${item.id}`
    default:
      return `/chat/${item.id}`
  }
}
