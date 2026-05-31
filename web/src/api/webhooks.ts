import api from './client'
import type { ApiResponse, PaginatedResponse, Webhook, WebhookDelivery, WebhookInput } from './types'

export const webhooksApi = {
  list(page = 0, size = 20) {
    return api.get<PaginatedResponse<Webhook>>('/workspaces/current/webhooks', { params: { page, size } })
  },
  create(data: WebhookInput) {
    return api.post<ApiResponse<Webhook>>('/workspaces/current/webhooks', data)
  },
  delete(id: number) {
    return api.delete(`/workspaces/current/webhooks/${id}`)
  },
}

export const webhookDeliveriesApi = {
  list(page = 0, size = 20) {
    return api.get<PaginatedResponse<WebhookDelivery>>('/workspaces/current/webhook-deliveries', { params: { page, size } })
  },
}
