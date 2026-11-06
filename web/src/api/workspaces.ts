import api from './client'
import type {
  ApiResponse,
  Workspace,
  WorkspaceInput,
  WorkspaceMember,
  WorkspaceInvitation,
  InviteMemberInput,
  WorkspaceRole,
  WorkspaceDataExport,
  GDPRDeleteResult,
  Plan,
} from './types'

export const workspaceApi = {
  list() {
    return api.get<ApiResponse<Workspace[]>>('/workspaces')
  },
  create(data: WorkspaceInput) {
    return api.post<ApiResponse<Workspace>>('/workspaces', data)
  },
  getCurrent() {
    return api.get<ApiResponse<Workspace>>('/workspaces/current')
  },
  update(data: { display_name?: string; description?: string }) {
    return api.put<ApiResponse<Workspace>>('/workspaces/current', data)
  },
  delete() {
    return api.delete('/workspaces/current')
  },

  listMembers() {
    return api.get<ApiResponse<WorkspaceMember[]>>('/workspaces/current/members')
  },
  updateMemberRole(memberUserId: number, role: WorkspaceRole) {
    return api.put<ApiResponse<{ message: string }>>(`/workspaces/current/members/${memberUserId}`, { role })
  },
  removeMember(memberUserId: number) {
    return api.delete(`/workspaces/current/members/${memberUserId}`)
  },

  invite(data: InviteMemberInput) {
    return api.post<ApiResponse<WorkspaceInvitation>>('/workspaces/current/invitations', data)
  },
  listInvitations() {
    return api.get<ApiResponse<WorkspaceInvitation[]>>('/workspaces/current/invitations')
  },
  cancelInvitation(invitationId: number) {
    return api.delete(`/workspaces/current/invitations/${invitationId}`)
  },

  myInvitations() {
    return api.get<ApiResponse<WorkspaceInvitation[]>>('/invitations')
  },
  acceptInvitationByToken(token: string) {
    return api.post<ApiResponse<{ message: string; workspace_id: number }>>('/invitations/accept', { token })
  },
  declineInvitationByToken(token: string) {
    return api.post<ApiResponse<{ message: string }>>('/invitations/decline', { token })
  },
  acceptInvitationById(id: number) {
    return api.post<ApiResponse<{ message: string; workspace_id: number }>>(`/invitations/${id}/accept`)
  },
  declineInvitationById(id: number) {
    return api.post<ApiResponse<{ message: string }>>(`/invitations/${id}/decline`)
  },

  getPlan() {
    return api.get<ApiResponse<Plan | null>>('/workspaces/current/plan')
  },

  exportData() {
    return api.get<ApiResponse<WorkspaceDataExport>>('/workspaces/current/data/export')
  },
  importData(data: WorkspaceDataExport) {
    return api.post<ApiResponse<{ message: string; imported_count: number }>>('/workspaces/current/data/import', data)
  },

  deleteContacts(email?: string) {
    return api.post<ApiResponse<GDPRDeleteResult>>('/workspaces/current/gdpr/delete-contacts', { email: email || '' })
  },
  deleteEmailLogs(olderThanDays: number) {
    return api.post<ApiResponse<GDPRDeleteResult>>('/workspaces/current/gdpr/delete-email-logs', { older_than_days: olderThanDays })
  },
  deleteMessages(email: string) {
    return api.post<ApiResponse<GDPRDeleteResult>>('/workspaces/current/gdpr/delete-messages', { email })
  },
}
