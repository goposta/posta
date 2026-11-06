import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { authApi } from '../api/auth'
import type { User, UserProfile } from '../api/types'

// The session is an HttpOnly cookie; `posta_user` only caches the profile so a reload renders before /users/me, it is not a credential.
const USER_KEY = 'posta_user'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | UserProfile | null>(JSON.parse(localStorage.getItem(USER_KEY) || 'null'))

  const isAuthenticated = computed(() => !!user.value)
  const isAdmin = computed(() => user.value?.role === 'admin')

  async function login(email: string, password: string, twoFactorCode?: string) {
    const res = await authApi.login(email, password, twoFactorCode)
    if (!res.data.success && (res.data.data as any)?.requires_2fa) {
      throw { requires2FA: true }
    }
    setUser(res.data.data.user)
    // Refresh to pick up extended profile fields (email_verified_at, etc).
    await fetchUser()
    return res.data.data
  }

  function setUser(u: User | UserProfile) {
    user.value = u
    localStorage.setItem(USER_KEY, JSON.stringify(u))
  }

  // Client state only: the 401 interceptor calls this, and logout() there would fire another request that 401s.
  function clearSession() {
    user.value = null
    localStorage.removeItem(USER_KEY)
    localStorage.removeItem('posta_workspace_id')
  }

  // Best-effort: an offline or already-expired session still logs out locally.
  async function logout() {
    try {
      await authApi.logout()
    } catch {
      /* the session is gone either way */
    }
    clearSession()
  }

  async function fetchUser() {
    try {
      const res = await authApi.me()
      setUser(res.data.data)
    } catch {
      clearSession()
    }
  }

  return { user, isAuthenticated, isAdmin, login, logout, clearSession, fetchUser, setUser }
})
