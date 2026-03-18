import api from './client'
import type { ApiResponse } from './types'

export interface Session {
  id: number
  ip_address: string
  user_agent: string
  current: boolean
  created_at: string
  expires_at: string
}

export const sessionsApi = {
  list() {
    return api.get<ApiResponse<Session[]>>('/users/me/sessions')
  },
  revoke(id: number) {
    return api.delete<ApiResponse<{ message: string }>>(`/users/me/sessions/${id}`)
  },
  revokeOthers() {
    return api.post<ApiResponse<{ message: string; revoked: number }>>('/users/me/sessions/revoke-others')
  },
  logout() {
    return api.post<ApiResponse<{ message: string }>>('/users/me/sessions/logout')
  },
}
