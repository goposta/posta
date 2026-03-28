import api from './client'
import type { ApiResponse, AuthResponse, UserProfile, Setup2FAResponse } from './types'

export const authApi = {
  login(email: string, password: string, twoFactorCode?: string) {
    const body: Record<string, string> = { email, password }
    if (twoFactorCode) body.two_factor_code = twoFactorCode
    return api.post<ApiResponse<AuthResponse>>('/auth/login', body)
  },
  me() {
    return api.get<ApiResponse<UserProfile>>('/users/me')
  },
  updateProfile(data: { name: string; require_verified_domain?: boolean }) {
    return api.put<ApiResponse<UserProfile>>('/users/me', data)
  },
  changePassword(currentPassword: string, newPassword: string) {
    return api.put<ApiResponse<{ message: string }>>('/users/me/password', {
      current_password: currentPassword,
      new_password: newPassword,
    })
  },
  setup2FA() {
    return api.post<ApiResponse<Setup2FAResponse>>('/users/me/2fa/setup')
  },
  verify2FA(code: string) {
    return api.post<ApiResponse<{ message: string }>>('/users/me/2fa/verify', { code })
  },
  disable2FA(code: string) {
    return api.post<ApiResponse<{ message: string }>>('/users/me/2fa/disable', { code })
  },
  requestAccountDeletion() {
    return api.post<ApiResponse<{ message: string; scheduled_deletion_at: string }>>('/users/me/delete')
  },
  cancelAccountDeletion() {
    return api.post<ApiResponse<{ message: string }>>('/users/me/cancel-deletion')
  },
  register(name: string, email: string, password: string) {
    return api.post<ApiResponse<AuthResponse>>('/auth/register', { name, email, password })
  },
  registrationStatus() {
    return api.get<ApiResponse<{ registration_enabled: boolean }>>('/auth/registration-status')
  },
}
