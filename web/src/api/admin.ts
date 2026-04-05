import api from './client'
import type { ApiResponse, PaginatedResponse, User, ApiKey, Email, Event, AdminMetrics, UserDetailMetrics, CronJob, AdminWorkspace } from './types'

export const adminApi = {
  listUsers(page = 0, size = 20) {
    return api.get<PaginatedResponse<User>>('/admin/users', { params: { page, size } })
  },
  createUser(name: string, email: string, password: string, role: string) {
    return api.post<ApiResponse<User>>('/admin/users', { name, email, password, role })
  },
  updateUser(id: number, data: { role?: string; active?: boolean }) {
    return api.put<ApiResponse<User>>(`/admin/users/${id}`, data)
  },
  deleteUser(id: number) {
    return api.delete(`/admin/users/${id}`)
  },
  cancelUserDeletion(id: number) {
    return api.post<ApiResponse<{ message: string }>>(`/admin/users/${id}/cancel-deletion`)
  },
  getUserMetrics(id: number) {
    return api.get<ApiResponse<UserDetailMetrics>>(`/admin/users/${id}/metrics`)
  },
  disable2FA(id: number) {
    return api.delete(`/admin/users/${id}/2fa`)
  },
  revokeUserSessions(id: number) {
    return api.post<ApiResponse<{ message: string; revoked: number }>>(`/admin/users/${id}/revoke-sessions`)
  },
  getUserWorkspaces(id: number) {
    return api.get<ApiResponse<AdminWorkspace[]>>(`/admin/users/${id}/workspaces`)
  },
  listApiKeys(page = 0, size = 20) {
    return api.get<PaginatedResponse<ApiKey>>('/admin/api-keys', { params: { page, size } })
  },
  revokeApiKey(id: number) {
    return api.delete(`/admin/api-keys/${id}`)
  },
  listEmails(page = 0, size = 20) {
    return api.get<PaginatedResponse<Email>>('/admin/emails', { params: { page, size } })
  },
  getMetrics() {
    return api.get<ApiResponse<AdminMetrics>>('/admin/metrics')
  },
  listEvents(page = 0, size = 20, category?: string) {
    const params: Record<string, any> = { page, size }
    if (category) params.category = category
    return api.get<PaginatedResponse<Event>>('/admin/events', { params })
  },
  listJobs() {
    return api.get<ApiResponse<CronJob[]>>('/admin/jobs')
  },
}
