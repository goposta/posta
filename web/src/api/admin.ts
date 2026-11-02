import api from './client'
import type {
  ApiResponse, PaginatedResponse, User, ApiKey, Email, Event, AdminMetrics,
  UserDetailMetrics, CronJob, AdminWorkspace, UpdateInfo,
  Domain, AdminDomainRow, AdminDomainDetail, AdminDomainVerifyResult,
  AdminWorkspaceRow, AdminWorkspaceDetail,
} from './types'
import type { Announcement, AnnouncementInput } from './inbox'

export const adminApi = {
  // ---- Announcements (platform-wide in-app notices) ----
  listAnnouncements(page = 0, size = 20) {
    return api.get<PaginatedResponse<Announcement>>('/admin/announcements', { params: { page, size } })
  },
  createAnnouncement(input: AnnouncementInput) {
    return api.post<ApiResponse<Announcement>>('/admin/announcements', input)
  },
  retractAnnouncement(id: number) {
    return api.delete<ApiResponse<{ message: string }>>(`/admin/announcements/${id}`)
  },
  listUsers(page = 0, size = 20, search = '') {
    return api.get<PaginatedResponse<User>>('/admin/users', { params: { page, size, search: search || undefined } })
  },
  // ---- Domains (platform-wide) ----
  listDomains(page = 0, size = 20, search = '', status = '', workspace = 0) {
    return api.get<PaginatedResponse<AdminDomainRow>>('/admin/domains', {
      params: {
        page,
        size,
        search: search || undefined,
        status: status || undefined,
        workspace: workspace || undefined,
      },
    })
  },
  getDomain(id: number) {
    return api.get<ApiResponse<AdminDomainDetail>>(`/admin/domains/${id}`)
  },
  verifyDomain(id: number) {
    return api.post<ApiResponse<AdminDomainVerifyResult>>(`/admin/domains/${id}/verify`)
  },
  setDomainVerification(id: number, ownershipVerified: boolean, reason: string) {
    return api.put<ApiResponse<Domain>>(`/admin/domains/${id}/verification`, {
      ownership_verified: ownershipVerified,
      reason,
    })
  },

  // ---- Workspaces (platform-wide) ----
  listWorkspaces(page = 0, size = 20, search = '', plan = 0, owner = 0) {
    return api.get<PaginatedResponse<AdminWorkspaceRow>>('/admin/workspaces', {
      params: {
        page,
        size,
        search: search || undefined,
        plan: plan || undefined,
        owner: owner || undefined,
      },
    })
  },
  getWorkspace(id: number) {
    return api.get<ApiResponse<AdminWorkspaceDetail>>(`/admin/workspaces/${id}`)
  },
  setWorkspaceHostedForms(id: number, enabled: boolean) {
    return api.put<ApiResponse<AdminWorkspaceRow>>(`/admin/workspaces/${id}/hosted-forms`, { enabled })
  },

  createUser(name: string, email: string, password: string, role: string) {
    return api.post<ApiResponse<User>>('/admin/users', { name, email, password, role })
  },
  updateUser(id: number, data: { role?: string; active?: boolean; email_verified?: boolean }) {
    return api.put<ApiResponse<User>>(`/admin/users/${id}`, data)
  },
  deleteUser(id: number) {
    return api.delete(`/admin/users/${id}`)
  },
  forceDeleteUser(id: number) {
    return api.delete(`/admin/users/${id}/force`)
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
  listEvents(page = 0, size = 20, category?: string, search = '') {
    const params: Record<string, any> = { page, size }
    if (category) params.category = category
    if (search) params.search = search
    return api.get<PaginatedResponse<Event>>('/admin/events', { params })
  },
  getEvent(id: number) {
    return api.get<ApiResponse<Event>>(`/admin/events/${id}`)
  },
  listJobs() {
    return api.get<ApiResponse<CronJob[]>>('/admin/jobs')
  },
  getUpdate() {
    return api.get<ApiResponse<UpdateInfo>>('/admin/update')
  },
  dismissUpdate(version: string) {
    return api.post<ApiResponse<{ message: string }>>('/admin/update/dismiss', { version })
  },
}
