import api from './client'
import type { ApiResponse, PaginatedResponse, SmtpServer, SmtpServerInput } from './types'

export const smtpApi = {
  list(page = 0, size = 20) {
    return api.get<PaginatedResponse<SmtpServer>>('/workspaces/current/smtp-servers', { params: { page, size } })
  },
  get(id: number) {
    return api.get<ApiResponse<SmtpServer>>(`/workspaces/current/smtp-servers/${id}`)
  },
  create(data: SmtpServerInput) {
    return api.post<ApiResponse<SmtpServer>>('/workspaces/current/smtp-servers', data)
  },
  update(id: number, data: Partial<SmtpServerInput>) {
    return api.put<ApiResponse<SmtpServer>>(`/workspaces/current/smtp-servers/${id}`, data)
  },
  delete(id: number) {
    return api.delete(`/workspaces/current/smtp-servers/${id}`)
  },
  test(id: number) {
    return api.post(`/workspaces/current/smtp-servers/${id}/test`)
  },
}
