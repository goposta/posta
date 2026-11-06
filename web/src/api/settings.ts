import api from './client'
import type { ApiResponse, AdminSetting, AdminSettingInput, UserSettings, WorkspaceSettings } from './types'

export const settingsApi = {
  getAdminSettings() {
    return api.get<ApiResponse<AdminSetting[]>>('/admin/settings')
  },
  updateAdminSettings(settings: AdminSettingInput[]) {
    return api.put<ApiResponse<AdminSetting[]>>('/admin/settings', { settings })
  },

  getUserSettings() {
    return api.get<ApiResponse<UserSettings>>('/users/me/settings')
  },
  updateUserSettings(data: Partial<Omit<UserSettings, 'id' | 'user_id' | 'created_at' | 'updated_at'>>) {
    return api.put<ApiResponse<UserSettings>>('/users/me/settings', data)
  },

  getWorkspaceSettings() {
    return api.get<ApiResponse<WorkspaceSettings>>('/workspaces/current/settings')
  },
  updateWorkspaceSettings(
    data: Partial<Omit<WorkspaceSettings, 'id' | 'workspace_id' | 'created_at' | 'updated_at'>>
  ) {
    return api.put<ApiResponse<WorkspaceSettings>>('/workspaces/current/settings', data)
  },
}
