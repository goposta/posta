import api from './client'
import type { ApiResponse, PaginatedResponse, Subscriber, BulkImportResult, SubscriberStatus } from './types'

export const subscribersApi = {
  list(page = 0, size = 20, search = '', status?: SubscriberStatus) {
    return api.get<PaginatedResponse<Subscriber>>('/workspaces/current/subscribers', {
      params: { page, size, search: search || undefined, status: status || undefined },
    })
  },
  get(id: number) {
    return api.get<ApiResponse<Subscriber>>(`/workspaces/current/subscribers/${id}`)
  },
  create(data: { email: string; name: string; custom_fields?: Record<string, any> }) {
    return api.post<ApiResponse<Subscriber>>('/workspaces/current/subscribers', data)
  },
  update(id: number, data: { email?: string; name?: string; status?: SubscriberStatus; custom_fields?: Record<string, any> }) {
    return api.put<ApiResponse<Subscriber>>(`/workspaces/current/subscribers/${id}`, data)
  },
  delete(id: number) {
    return api.delete(`/workspaces/current/subscribers/${id}`)
  },
  bulkImportJSON(subscribers: { email: string; name?: string; custom_fields?: Record<string, any> }[]) {
    return api.post<ApiResponse<BulkImportResult>>('/workspaces/current/subscribers/import/json', { subscribers })
  },
  bulkImportCSV(file: File, columnMapping?: Record<string, string>) {
    const formData = new FormData()
    formData.append('file', file)
    if (columnMapping) {
      formData.append('column_mapping', JSON.stringify(columnMapping))
    }
    return api.post<ApiResponse<BulkImportResult>>('/workspaces/current/subscribers/import/csv', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
  },
}
