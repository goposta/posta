import api from './client'
import type {
  ApiResponse,
  Workspace,
  WorkspaceInput,
  WorkspaceMember,
  WorkspaceInvitation,
  InviteMemberInput,
  WorkspaceRole,
  TransferResponse,
  WorkspaceDataExport,
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
  update(data: { name?: string; description?: string }) {
    return api.put<ApiResponse<Workspace>>('/workspaces/current', data)
  },
  delete() {
    return api.delete('/workspaces/current')
  },

  // Members
  listMembers() {
    return api.get<ApiResponse<WorkspaceMember[]>>('/workspaces/current/members')
  },
  updateMemberRole(memberUserId: number, role: WorkspaceRole) {
    return api.put<ApiResponse<{ message: string }>>(`/workspaces/current/members/${memberUserId}`, { role })
  },
  removeMember(memberUserId: number) {
    return api.delete(`/workspaces/current/members/${memberUserId}`)
  },

  // Invitations (workspace-scoped)
  invite(data: InviteMemberInput) {
    return api.post<ApiResponse<WorkspaceInvitation>>('/workspaces/current/invitations', data)
  },
  listInvitations() {
    return api.get<ApiResponse<WorkspaceInvitation[]>>('/workspaces/current/invitations')
  },
  cancelInvitation(invitationId: number) {
    return api.delete(`/workspaces/current/invitations/${invitationId}`)
  },

  // User-level invitation actions
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

  // Plan
  getPlan() {
    return api.get<ApiResponse<Plan | null>>('/workspaces/current/plan')
  },

  // Data Transfer
  transferData(resources: string[]) {
    return api.post<ApiResponse<TransferResponse>>('/workspaces/current/transfer', { resources })
  },

  // Data Export/Import
  exportData() {
    return api.get<ApiResponse<WorkspaceDataExport>>('/workspaces/current/data/export')
  },
  importData(data: WorkspaceDataExport) {
    return api.post<ApiResponse<{ message: string; imported_count: number }>>('/workspaces/current/data/import', data)
  },
}
