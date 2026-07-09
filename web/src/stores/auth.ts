import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { authApi } from '../api/auth'
import type { User, UserProfile } from '../api/types'

// The session lives in an HttpOnly cookie that the browser attaches to every
// same-origin request, so there is no token here to hold. `posta_user` caches the
// profile only so a reload can render before /users/me resolves — it is not a
// credential. If the cookie is missing or expired the first API call 401s and the
// response interceptor in api/client.ts sends us to /login.
const USER_KEY = 'posta_user'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | UserProfile | null>(JSON.parse(localStorage.getItem(USER_KEY) || 'null'))

  const isAuthenticated = computed(() => !!user.value)
  const isAdmin = computed(() => user.value?.role === 'admin')

  async function login(email: string, password: string, twoFactorCode?: string) {
    const res = await authApi.login(email, password, twoFactorCode)
    // Check if 2FA is required
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

  // clearSession drops client state only. The 401 interceptor uses this: calling
  // logout() there would fire another request that 401s in turn.
  function clearSession() {
    user.value = null
    localStorage.removeItem(USER_KEY)
    // Clear workspace state
    localStorage.removeItem('posta_workspace_id')
  }

  // logout revokes the session server-side (blacklists its jti and expires the
  // cookie), then clears client state. Best-effort: an offline or already-expired
  // session still logs out locally.
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
