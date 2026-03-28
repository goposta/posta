import api from './client'
import type {
  ApiResponse,
  OAuthProviderInfo,
  OAuthLinkedAccount,
  OAuthProviderAdmin,
  OAuthProviderInput,
  WorkspaceSSOConfig,
} from './types'

export const oauthApi = {
  // Public
  providers() {
    return api.get<ApiResponse<{ providers: OAuthProviderInfo[] }>>('/auth/oauth/providers')
  },

  // Linked accounts (authenticated user)
  linkedAccounts() {
    return api.get<ApiResponse<OAuthLinkedAccount[]>>('/users/me/oauth')
  },
  unlink(providerID: number) {
    return api.delete<ApiResponse<{ message: string }>>(`/users/me/oauth/${providerID}`)
  },

  // Admin: provider management
  adminList() {
    return api.get<ApiResponse<OAuthProviderAdmin[]>>('/admin/oauth/providers')
  },
  adminCreate(data: OAuthProviderInput) {
    return api.post<ApiResponse<OAuthProviderAdmin>>('/admin/oauth/providers', data)
  },
  adminUpdate(id: number, data: Partial<OAuthProviderInput> & { enabled?: boolean }) {
    return api.put<ApiResponse<OAuthProviderAdmin>>(`/admin/oauth/providers/${id}`, data)
  },
  adminDelete(id: number) {
    return api.delete(`/admin/oauth/providers/${id}`)
  },

  // Workspace SSO
  getSSO() {
    return api.get<ApiResponse<WorkspaceSSOConfig | null>>('/workspaces/current/sso')
  },
  setSSO(data: { provider_id: number; enforce_sso: boolean; auto_provision: boolean; allowed_domains: string }) {
    return api.put<ApiResponse<{ message: string }>>('/workspaces/current/sso', data)
  },
  deleteSSO() {
    return api.delete<ApiResponse<{ message: string }>>('/workspaces/current/sso')
  },
}
