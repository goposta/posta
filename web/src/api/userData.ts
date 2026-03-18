import api from './client'
import type { ApiResponse, UserDataExport, GDPRDeleteResult } from './types'

export const userDataApi = {
  exportAll() {
    return api.get<ApiResponse<UserDataExport>>('/users/me/data/export')
  },
  importAll(data: UserDataExport) {
    return api.post<ApiResponse<{ message: string; imported_count: number }>>('/users/me/data/import', data)
  },
  deleteContacts(email?: string) {
    return api.post<ApiResponse<GDPRDeleteResult>>('/users/me/gdpr/delete-contacts', { email: email || '' })
  },
  deleteEmailLogs(olderThanDays: number) {
    return api.post<ApiResponse<GDPRDeleteResult>>('/users/me/gdpr/delete-email-logs', { older_than_days: olderThanDays })
  },
}
