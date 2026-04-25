import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { authApi } from '../api/auth'
import type { User, UserProfile, AuthResponse } from '../api/types'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('posta_token') || '')
  const user = ref<User | UserProfile | null>(JSON.parse(localStorage.getItem('posta_user') || 'null'))

  const isAuthenticated = computed(() => !!token.value)
  const isAdmin = computed(() => user.value?.role === 'admin')

  async function login(email: string, password: string, twoFactorCode?: string) {
    const res = await authApi.login(email, password, twoFactorCode)
    // Check if 2FA is required
    if (!res.data.success && (res.data.data as any)?.requires_2fa) {
      throw { requires2FA: true }
    }
    setAuth(res.data.data)
    // Refresh to pick up extended profile fields (email_verified_at, etc).
    await fetchUser()
    return res.data.data
  }

  function setAuth(data: AuthResponse) {
    token.value = data.token
    user.value = data.user
    localStorage.setItem('posta_token', data.token)
    localStorage.setItem('posta_user', JSON.stringify(data.user))
  }

  function logout() {
    token.value = ''
    user.value = null
    localStorage.removeItem('posta_token')
    localStorage.removeItem('posta_user')
    // Clear workspace state
    localStorage.removeItem('posta_workspace_id')
  }

  async function fetchUser() {
    try {
      const res = await authApi.me()
      user.value = res.data.data
      localStorage.setItem('posta_user', JSON.stringify(res.data.data))
    } catch {
      logout()
    }
  }

  return { token, user, isAuthenticated, isAdmin, login, logout, fetchUser }
})
