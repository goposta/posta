import api from './client'
import type { ApiResponse, PaginatedResponse, SubscriberListItem, Subscriber, FilterRule } from './types'

export const subscriberListsApi = {
  list(page = 0, size = 20) {
    return api.get<PaginatedResponse<SubscriberListItem>>('/users/me/subscriber-lists', { params: { page, size } })
  },
  get(id: number) {
    return api.get<ApiResponse<SubscriberListItem>>(`/users/me/subscriber-lists/${id}`)
  },
  create(data: { name: string; description: string; type: string; filter_rules?: FilterRule[] }) {
    return api.post<ApiResponse<SubscriberListItem>>('/users/me/subscriber-lists', data)
  },
  update(id: number, data: { name?: string; description?: string; filter_rules?: FilterRule[] }) {
    return api.put<ApiResponse<SubscriberListItem>>(`/users/me/subscriber-lists/${id}`, data)
  },
  delete(id: number) {
    return api.delete(`/users/me/subscriber-lists/${id}`)
  },
  listMembers(id: number, page = 0, size = 20) {
    return api.get<PaginatedResponse<Subscriber>>(`/users/me/subscriber-lists/${id}/members`, { params: { page, size } })
  },
  addMember(id: number, subscriberId: number) {
    return api.post<ApiResponse<null>>(`/users/me/subscriber-lists/${id}/members`, { subscriber_id: subscriberId })
  },
  removeMember(id: number, subscriberId: number) {
    return api.delete(`/users/me/subscriber-lists/${id}/members`, { data: { subscriber_id: subscriberId } })
  },
  previewSegment(filterRules: FilterRule[]) {
    return api.post<PaginatedResponse<Subscriber>>('/users/me/subscriber-lists/preview', { filter_rules: filterRules })
  },
}
