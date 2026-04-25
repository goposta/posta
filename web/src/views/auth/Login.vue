<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { authApi } from '../../api/auth'
import { oauthApi } from '../../api/oauth'
import { useAuthStore } from '../../stores/auth'
import { useNotificationStore } from '../../stores/notification'
import { useThemeStore } from '../../stores/theme'
import type { OAuthProviderInfo } from '../../api/types'

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()
const notification = useNotificationStore()
const theme = useThemeStore()

const email = ref('')
const password = ref('')
const twoFactorCode = ref('')
const loading = ref(false)
const requires2FA = ref(false)
const registrationEnabled = ref(false)
const oauthProviders = ref<OAuthProviderInfo[]>([])
const ssoAvailable = ref(false)
const ssoMode = ref(false)
const loginError = ref(router.currentRoute.value.query.error as string || '')
const emailError = ref('')
const passwordError = ref('')
const codeError = ref('')
const showPassword = ref(false)

onMounted(async () => {
  try {
    const [regRes, oauthRes] = await Promise.all([
      authApi.registrationStatus(),
      oauthApi.providers(),
    ])
    registrationEnabled.value = regRes.data.data.registration_enabled
    oauthProviders.value = oauthRes.data.data.providers || []
    ssoAvailable.value = !!oauthRes.data.data.sso_available
  } catch { /* ignore */ }
})

function oauthLogin(slug: string) {
  // Full page navigation to the backend OAuth authorize endpoint
  window.location.href = `/api/v1/auth/oauth/${slug}/authorize`
}

const ssoChecking = ref(false)
function enterSSOMode() {
  ssoMode.value = true
  password.value = ''
}
function exitSSOMode() {
  ssoMode.value = false
}
async function submitSSO() {
  emailError.value = ''
  if (!email.value) {
    emailError.value = 'Enter your email to continue.'
    return
  }
  ssoChecking.value = true
  try {
    const res = await oauthApi.discoverSSO(email.value)
    const slug = res.data.data?.slug
    if (slug) {
      oauthLogin(slug)
      return
    }
    notification.info('No SSO configured for this email.')
  } catch (err: any) {
    if (err?.response?.status === 404) {
      notification.info('No SSO configured for this email.')
    } else {
      notification.error('SSO lookup failed.')
    }
  } finally {
    ssoChecking.value = false
  }
}

async function handleLogin() {
  emailError.value = ''
  passwordError.value = ''
  codeError.value = ''
  if (requires2FA.value) {
    if (!twoFactorCode.value) {
      codeError.value = 'Enter the 6-digit code from your authenticator app.'
      return
    }
  } else {
    if (!email.value) emailError.value = 'Enter your email address.'
    if (!password.value) passwordError.value = 'Enter your password.'
    if (emailError.value || passwordError.value) return
  }
  loading.value = true
  try {
    await auth.login(email.value, password.value, requires2FA.value ? twoFactorCode.value : undefined)
    const redirect = route.query.redirect
    const target = typeof redirect === 'string' && redirect.startsWith('/') ? redirect : '/'
    router.push(target)
  } catch (err: any) {
    if (err?.requires2FA) {
      requires2FA.value = true
      notification.info('Please enter your two-factor authentication code.')
      return
    }
    // Check for 2FA required response (401 with requires_2fa flag)
    if (err?.response?.status === 401 && err?.response?.data?.data?.requires_2fa) {
      requires2FA.value = true
      notification.info('Please enter your two-factor authentication code.')
      return
    }
    if (err?.response?.status === 429) {
      notification.error('Too many login attempts. Please try again later.')
      return
    }
    const message = err?.response?.data?.error?.message || err?.response?.data?.error || err?.message || 'Login failed.'
    notification.error(message)
  } finally {
    loading.value = false
  }
}

function resetLogin() {
  requires2FA.value = false
  twoFactorCode.value = ''
}
</script>

<template>
  <div class="auth-page">
    <div class="auth-card">
      <div class="auth-header">
        <div class="auth-wordmark" aria-label="Posta">Posta<span class="auth-wordmark-dot">.</span></div>
        <h1 class="auth-title">
          {{ requires2FA ? 'Two-factor authentication' : (ssoMode ? 'Continue with SSO' : 'Sign in to your account') }}
        </h1>
        <p v-if="requires2FA" class="auth-subtitle">Enter the code from your authenticator app.</p>
        <p v-else-if="ssoMode" class="auth-subtitle">We'll redirect you to your identity provider.</p>
      </div>

      <form class="auth-form" @submit.prevent="ssoMode ? submitSSO() : handleLogin()">
        <template v-if="!requires2FA">
          <div class="form-group">
            <label class="form-label" for="email">Email</label>
            <input
              id="email"
              v-model="email"
              type="email"
              class="form-input"
              :class="{ 'form-input-error': emailError }"
              placeholder="you@example.com"
              autocomplete="email"
              @input="emailError = ''"
            />
            <small v-if="emailError" class="form-error">{{ emailError }}</small>
          </div>
          <div v-if="!ssoMode" class="form-group">
            <label class="form-label" for="password">Password</label>
            <div class="password-wrap">
              <input
                id="password"
                v-model="password"
                :type="showPassword ? 'text' : 'password'"
                class="form-input"
                :class="{ 'form-input-error': passwordError }"
                placeholder="Enter your password"
                autocomplete="current-password"
                @input="passwordError = ''"
              />
              <button
                type="button"
                class="password-toggle"
                :aria-label="showPassword ? 'Hide password' : 'Show password'"
                :title="showPassword ? 'Hide password' : 'Show password'"
                @click="showPassword = !showPassword"
              >
                <svg v-if="showPassword" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17.94 17.94A10.07 10.07 0 0112 20c-7 0-11-8-11-8a18.45 18.45 0 015.06-5.94M9.9 4.24A9.12 9.12 0 0112 4c7 0 11 8 11 8a18.5 18.5 0 01-2.16 3.19m-6.72-1.07a3 3 0 11-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
                <svg v-else width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
              </button>
            </div>
            <small v-if="passwordError" class="form-error">{{ passwordError }}</small>
          </div>
        </template>
        <template v-else>
          <div class="form-group">
            <label class="form-label" for="2fa-code">Authentication code</label>
            <input
              id="2fa-code"
              v-model="twoFactorCode"
              type="text"
              class="form-input totp-input"
              :class="{ 'form-input-error': codeError }"
              placeholder="000000"
              maxlength="6"
              inputmode="numeric"
              autocomplete="one-time-code"
              autofocus
              @input="codeError = ''"
            />
            <small v-if="codeError" class="form-error">{{ codeError }}</small>
          </div>
        </template>
        <button v-if="!ssoMode" type="submit" class="btn btn-primary auth-btn" :disabled="loading">
          <span v-if="loading" class="spinner"></span>
          {{ loading ? 'Signing in...' : 'Sign in' }}
        </button>
        <button v-if="ssoMode" type="submit" class="btn btn-primary auth-btn" :disabled="ssoChecking">
          <span v-if="ssoChecking" class="spinner"></span>
          {{ ssoChecking ? 'Checking...' : 'Continue' }}
        </button>
        <button v-if="!requires2FA && !ssoMode && ssoAvailable" type="button" class="btn btn-secondary auth-btn" style="margin-top: 8px" @click="enterSSOMode">
          Continue with SSO
        </button>
        <button v-if="ssoMode" type="button" class="btn btn-secondary auth-btn" style="margin-top: 8px" @click="exitSSOMode">
          Back to password sign in
        </button>
        <button v-if="requires2FA" type="button" class="btn btn-secondary auth-btn" style="margin-top: 8px" @click="resetLogin">
          Back to Login
        </button>
      </form>

      <!-- OAuth Providers -->
      <div v-if="oauthProviders.length > 0 && !requires2FA && !ssoMode" class="oauth-section">
        <div class="oauth-divider">
          <span>or</span>
        </div>
        <div class="oauth-buttons">
          <button
            v-for="p in oauthProviders"
            :key="p.slug"
            class="btn btn-secondary auth-btn oauth-btn"
            @click="oauthLogin(p.slug)"
          >
            <svg v-if="p.type === 'google'" width="18" height="18" viewBox="0 0 24 24"><path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 01-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z" fill="#4285F4"/><path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853"/><path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18A10.97 10.97 0 001 12c0 1.77.42 3.45 1.18 4.93l3.66-2.84z" fill="#FBBC05"/><path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335"/></svg>
            <svg v-else width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0110 0v4"/></svg>
            Sign in with {{ p.name }}
          </button>
        </div>
      </div>

      <!-- Login error from redirect (OAuth failure, session revoked, etc.) -->
      <div v-if="loginError" class="oauth-error-msg">
        {{ loginError.replace(/_/g, ' ') }}
      </div>

      <div class="auth-footer">
        <template v-if="registrationEnabled">
          <span>Don't have an account?</span>
          <router-link to="/register">Sign up</router-link>
        </template>
        <span v-else>Contact your administrator for an account.</span>
      </div>
    </div>

    <button class="theme-btn" @click="theme.toggle()" :title="theme.isDark ? 'Light mode' : 'Dark mode'">
      <svg v-if="theme.isDark" width="18" height="18" viewBox="0 0 16 16" fill="none"><circle cx="8" cy="8" r="3" stroke="currentColor" stroke-width="1.5"/><path d="M8 1v2M8 13v2M1 8h2M13 8h2M3.05 3.05l1.41 1.41M11.54 11.54l1.41 1.41M3.05 12.95l1.41-1.41M11.54 4.46l1.41-1.41" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>
      <svg v-else width="18" height="18" viewBox="0 0 16 16" fill="none"><path d="M14 9.5A6.5 6.5 0 016.5 2 6.5 6.5 0 1014 9.5z" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>
    </button>
  </div>
</template>

<style scoped>
.auth-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg-secondary);
  padding: 20px;
  position: relative;
}

.auth-card {
  width: 100%;
  max-width: 400px;
  background: var(--bg-primary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-xl);
  box-shadow: var(--shadow-lg);
}

.auth-header { text-align: center; padding: 36px 32px 0; }

.auth-wordmark {
  font-size: 32px;
  font-weight: 800;
  letter-spacing: -1px;
  color: var(--text-primary);
  margin-bottom: 24px;
  line-height: 1;
}
.auth-wordmark-dot {
  color: var(--primary-500);
  margin-left: 1px;
}

.auth-title {
  font-size: 22px;
  font-weight: 600;
  color: var(--text-primary);
  letter-spacing: -0.2px;
  margin: 0 0 8px;
}

.auth-subtitle { font-size: 14px; color: var(--text-muted); margin: 0; }

.password-wrap { position: relative; }
.password-wrap .form-input { padding-right: 40px; }
.password-toggle {
  position: absolute;
  top: 0;
  right: 0;
  bottom: 0;
  width: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
}
.password-toggle:hover { color: var(--text-primary); }

.form-input-error { border-color: var(--danger-500, #ef4444); }
.form-input-error:focus { border-color: var(--danger-500, #ef4444); }

.form-error {
  display: block;
  font-size: 12px;
  color: var(--danger-600, #dc2626);
  margin-top: 6px;
}

.auth-form { padding: 28px 32px 20px; }

.auth-btn {
  width: 100%;
  padding: 11px 18px;
  font-size: 15px;
  margin-top: 4px;
}

.totp-input {
  font-size: 24px;
  text-align: center;
  letter-spacing: 8px;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
}

.auth-footer {
  text-align: center;
  padding: 0 32px 28px;
  font-size: 14px;
  color: var(--text-muted);
  display: flex;
  gap: 6px;
  justify-content: center;
}
.auth-footer a { color: var(--primary-500); font-weight: 500; }

.theme-btn {
  position: fixed;
  top: 20px;
  right: 20px;
  background: var(--bg-primary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius);
  padding: 10px;
  cursor: pointer;
  color: var(--text-tertiary);
  display: flex;
  align-items: center;
  transition: all var(--transition);
  box-shadow: var(--shadow-sm);
}
.theme-btn:hover { color: var(--text-primary); border-color: var(--border-input); }

.oauth-section { padding: 0 32px 20px; }

.oauth-divider {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}
.oauth-divider::before, .oauth-divider::after {
  content: '';
  flex: 1;
  height: 1px;
  background: var(--border-primary);
}
.oauth-divider span {
  font-size: 12px;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.oauth-buttons { display: flex; flex-direction: column; gap: 8px; }

.oauth-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  font-weight: 500;
}

.oauth-error-msg {
  padding: 10px 32px 16px;
  font-size: 13px;
  color: var(--danger-600);
  text-align: center;
}
</style>
