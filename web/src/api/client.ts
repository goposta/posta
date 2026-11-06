import axios from 'axios'
import { useAuthStore } from '../stores/auth'

const api = axios.create({
  baseURL: import.meta.env.VITE_API_URL || '/api/v1',
  headers: {
    'Content-Type': 'application/json',
  },
  // Same-origin requests send the cookie anyway; this covers a cross-origin VITE_API_URL in development.
  withCredentials: true,
})

api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401 && !error.response?.data?.data?.requires_2fa) {
      const auth = useAuthStore()
      const message = error.response?.data?.error?.message || 'Your session has expired'
      // clearSession, not logout: logout() calls the API, which would 401 in turn.
      auth.clearSession()
      window.location.href = `/login?error=${encodeURIComponent(message)}`
    }
    return Promise.reject(error)
  }
)

export default api
