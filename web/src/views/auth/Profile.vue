<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { authApi } from '../../api/auth'
import { sessionsApi, type Session } from '../../api/sessions'
import { useAuthStore } from '../../stores/auth'
import { useNotificationStore } from '../../stores/notification'
import { useConfirm } from '../../composables/useConfirm'
const auth = useAuthStore()
const notify = useNotificationStore()
const { confirm } = useConfirm()

// Profile
const name = ref('')
const email = ref('')
const profileLoading = ref(false)
const twoFactorEnabled = ref(false)

onMounted(async () => {
  name.value = auth.user?.name || ''
  email.value = auth.user?.email || ''
  // Fetch fresh profile to get 2FA status
  try {
    const res = await authApi.me()
    twoFactorEnabled.value = res.data.data.two_factor_enabled
  } catch { /* ignore */ }
})

async function handleProfileUpdate() {
  if (!name.value.trim()) {
    notify.error('Name is required')
    return
  }
  profileLoading.value = true
  try {
    const res = await authApi.updateProfile({ name: name.value.trim() })
    auth.user = res.data.data
    localStorage.setItem('posta_user', JSON.stringify(res.data.data))
    twoFactorEnabled.value = res.data.data.two_factor_enabled
    notify.success('Profile updated successfully')
  } catch (e: any) {
    const message = e?.response?.data?.error?.message || 'Failed to update profile'
    notify.error(message)
  } finally {
    profileLoading.value = false
  }
}

// Password
const currentPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const passwordLoading = ref(false)

async function handlePasswordChange() {
  if (!currentPassword.value || !newPassword.value || !confirmPassword.value) {
    notify.error('Please fill in all fields')
    return
  }
  if (newPassword.value.length < 8) {
    notify.error('New password must be at least 8 characters')
    return
  }
  if (newPassword.value !== confirmPassword.value) {
    notify.error('New passwords do not match')
    return
  }
  passwordLoading.value = true
  try {
    await authApi.changePassword(currentPassword.value, newPassword.value)
    notify.success('Password changed successfully')
    currentPassword.value = ''
    newPassword.value = ''
    confirmPassword.value = ''
  } catch (e: any) {
    const message = e?.response?.data?.error?.message || 'Failed to change password'
    notify.error(message)
  } finally {
    passwordLoading.value = false
  }
}

// 2FA
const show2FASetup = ref(false)
const tfaSecret = ref('')
const tfaURL = ref('')
const tfaCode = ref('')
const tfaLoading = ref(false)
const tfaDisableCode = ref('')
const show2FADisable = ref(false)
const tfaDisableLoading = ref(false)

async function startSetup2FA() {
  tfaLoading.value = true
  try {
    const res = await authApi.setup2FA()
    tfaSecret.value = res.data.data.secret
    tfaURL.value = res.data.data.url
    show2FASetup.value = true
  } catch (e: any) {
    const message = e?.response?.data?.error?.message || 'Failed to setup 2FA'
    notify.error(message)
  } finally {
    tfaLoading.value = false
  }
}

async function verify2FA() {
  if (!tfaCode.value || tfaCode.value.length !== 6) {
    notify.error('Please enter a valid 6-digit code')
    return
  }
  tfaLoading.value = true
  try {
    await authApi.verify2FA(tfaCode.value)
    twoFactorEnabled.value = true
    show2FASetup.value = false
    tfaCode.value = ''
    tfaSecret.value = ''
    tfaURL.value = ''
    notify.success('Two-factor authentication enabled')
  } catch (e: any) {
    const message = e?.response?.data?.error?.message || 'Invalid code. Please try again.'
    notify.error(message)
  } finally {
    tfaLoading.value = false
  }
}

async function disable2FA() {
  if (!tfaDisableCode.value || tfaDisableCode.value.length !== 6) {
    notify.error('Please enter a valid 6-digit code')
    return
  }
  tfaDisableLoading.value = true
  try {
    await authApi.disable2FA(tfaDisableCode.value)
    twoFactorEnabled.value = false
    show2FADisable.value = false
    tfaDisableCode.value = ''
    notify.success('Two-factor authentication disabled')
  } catch (e: any) {
    const message = e?.response?.data?.error?.message || 'Invalid code. Please try again.'
    notify.error(message)
  } finally {
    tfaDisableLoading.value = false
  }
}

function cancel2FASetup() {
  show2FASetup.value = false
  tfaCode.value = ''
  tfaSecret.value = ''
  tfaURL.value = ''
}

// Sessions
const sessions = ref<Session[]>([])
const sessionsLoading = ref(false)
const revokingSession = ref<number | null>(null)
const revokingOthers = ref(false)

async function loadSessions() {
  sessionsLoading.value = true
  try {
    const res = await sessionsApi.list()
    sessions.value = res.data.data || []
  } catch {
    // silently fail — sessions card still shows
  } finally {
    sessionsLoading.value = false
  }
}

async function revokeSession(s: Session) {
  const confirmed = await confirm({
    title: 'Revoke Session',
    message: `Force logout the session from ${s.ip_address}?`,
    confirmText: 'Revoke',
    variant: 'danger',
  })
  if (!confirmed) return

  revokingSession.value = s.id
  try {
    await sessionsApi.revoke(s.id)
    sessions.value = sessions.value.filter(x => x.id !== s.id)
    notify.success('Session revoked')
  } catch (e: any) {
    notify.error(e.response?.data?.error?.message || 'Failed to revoke session')
  } finally {
    revokingSession.value = null
  }
}

async function revokeOtherSessions() {
  const confirmed = await confirm({
    title: 'Revoke All Other Sessions',
    message: 'This will force logout all other devices and browsers. Continue?',
    confirmText: 'Revoke All Others',
    variant: 'danger',
  })
  if (!confirmed) return

  revokingOthers.value = true
  try {
    const res = await sessionsApi.revokeOthers()
    notify.success(res.data.data.message)
    await loadSessions()
  } catch (e: any) {
    notify.error(e.response?.data?.error?.message || 'Failed to revoke sessions')
  } finally {
    revokingOthers.value = false
  }
}

function parseUserAgent(ua: string): string {
  if (!ua) return 'Unknown'
  // Extract browser name
  if (ua.includes('Firefox')) return 'Firefox'
  if (ua.includes('Edg/')) return 'Edge'
  if (ua.includes('Chrome')) return 'Chrome'
  if (ua.includes('Safari')) return 'Safari'
  if (ua.includes('curl')) return 'curl'
  return ua.slice(0, 30) + (ua.length > 30 ? '...' : '')
}

function formatSessionDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString(undefined, {
    year: 'numeric', month: 'short', day: 'numeric',
    hour: '2-digit', minute: '2-digit',
  })
}

// Load sessions on mount
onMounted(() => { loadSessions() })
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Profile</h1>
    </div>

    <div class="profile-grid">
      <!-- My Profile -->
      <div class="card">
        <div class="card-header"><h2>My Profile</h2></div>
        <div class="card-body">
          <form @submit.prevent="handleProfileUpdate" class="profile-form">
            <div class="form-group">
              <label class="form-label" for="profile-name">Name</label>
              <input id="profile-name" v-model="name" type="text" class="form-input" placeholder="Your name" required />
            </div>
            <div class="form-group">
              <label class="form-label" for="profile-email">Email</label>
              <input id="profile-email" :value="email" type="email" class="form-input" disabled />
              <small class="form-hint">Email cannot be changed</small>
            </div>
            <button type="submit" class="btn btn-primary" :disabled="profileLoading">
              {{ profileLoading ? 'Saving...' : 'Save Changes' }}
            </button>
          </form>
        </div>
      </div>

      <!-- Two-Factor Authentication -->
      <div class="card">
        <div class="card-header">
          <h2>Two-Factor Authentication</h2>
          <span v-if="twoFactorEnabled" class="badge badge-success">Enabled</span>
          <span v-else class="badge badge-secondary">Disabled</span>
        </div>
        <div class="card-body">
          <!-- 2FA Not Enabled -->
          <template v-if="!twoFactorEnabled && !show2FASetup">
            <p class="tfa-description">Add an extra layer of security to your account by requiring a code from your authenticator app.</p>
            <button class="btn btn-primary" :disabled="tfaLoading" @click="startSetup2FA">
              {{ tfaLoading ? 'Setting up...' : 'Enable 2FA' }}
            </button>
          </template>

          <!-- 2FA Setup Flow -->
          <template v-if="show2FASetup">
            <div class="tfa-setup">
              <p class="tfa-description">Scan this QR code with your authenticator app (Google Authenticator, Authy, etc.):</p>
              <div class="tfa-qr">
                <img :src="`https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=${encodeURIComponent(tfaURL)}`" alt="QR Code" width="200" height="200" />
              </div>
              <div class="tfa-secret-group">
                <label class="form-label">Or enter this secret manually:</label>
                <code class="tfa-secret">{{ tfaSecret }}</code>
              </div>
              <form @submit.prevent="verify2FA" class="profile-form" style="margin-top: 16px">
                <div class="form-group">
                  <label class="form-label" for="tfa-code">Verification Code</label>
                  <input
                    id="tfa-code"
                    v-model="tfaCode"
                    type="text"
                    class="form-input totp-input"
                    placeholder="000000"
                    maxlength="6"
                    inputmode="numeric"
                    autocomplete="one-time-code"
                  />
                  <small class="form-hint">Enter the 6-digit code from your authenticator app to verify setup</small>
                </div>
                <div style="display: flex; gap: 8px">
                  <button type="submit" class="btn btn-primary" :disabled="tfaLoading">
                    {{ tfaLoading ? 'Verifying...' : 'Verify & Enable' }}
                  </button>
                  <button type="button" class="btn btn-secondary" @click="cancel2FASetup">Cancel</button>
                </div>
              </form>
            </div>
          </template>

          <!-- 2FA Enabled -->
          <template v-if="twoFactorEnabled && !show2FADisable">
            <p class="tfa-description">Two-factor authentication is currently enabled on your account.</p>
            <button class="btn btn-danger" @click="show2FADisable = true">Disable 2FA</button>
          </template>

          <!-- 2FA Disable Flow -->
          <template v-if="show2FADisable">
            <form @submit.prevent="disable2FA" class="profile-form">
              <p class="tfa-description">Enter a code from your authenticator app to confirm disabling 2FA.</p>
              <div class="form-group">
                <label class="form-label" for="tfa-disable-code">Authentication Code</label>
                <input
                  id="tfa-disable-code"
                  v-model="tfaDisableCode"
                  type="text"
                  class="form-input totp-input"
                  placeholder="000000"
                  maxlength="6"
                  inputmode="numeric"
                  autocomplete="one-time-code"
                />
              </div>
              <div style="display: flex; gap: 8px">
                <button type="submit" class="btn btn-danger" :disabled="tfaDisableLoading">
                  {{ tfaDisableLoading ? 'Disabling...' : 'Confirm Disable' }}
                </button>
                <button type="button" class="btn btn-secondary" @click="show2FADisable = false; tfaDisableCode = ''">Cancel</button>
              </div>
            </form>
          </template>
        </div>
      </div>

      <!-- Active Sessions -->
      <div class="card">
        <div class="card-header">
          <h2>Active Sessions</h2>
          <button
            v-if="sessions.length > 1"
            class="btn btn-danger btn-sm"
            :disabled="revokingOthers"
            @click="revokeOtherSessions"
          >
            {{ revokingOthers ? 'Revoking...' : 'Revoke All Others' }}
          </button>
        </div>
        <div class="card-body">
          <p class="tfa-description">
            These are the devices and browsers currently logged in to your account.
          </p>

          <div v-if="sessionsLoading" style="text-align: center; padding: 20px 0">
            <div class="spinner"></div>
          </div>

          <div v-else-if="sessions.length === 0" class="text-muted" style="text-align: center; padding: 16px 0">
            No active sessions found.
          </div>

          <div v-else class="session-list">
            <div v-for="s in sessions" :key="s.id" class="session-item" :class="{ 'session-current': s.current }">
              <div class="session-info">
                <div class="session-browser">
                  {{ parseUserAgent(s.user_agent) }}
                  <span v-if="s.current" class="badge badge-success" style="margin-left: 6px">Current</span>
                </div>
                <div class="session-meta">
                  {{ s.ip_address }} &middot; Created {{ formatSessionDate(s.created_at) }} &middot; Expires {{ formatSessionDate(s.expires_at) }}
                </div>
              </div>
              <button
                v-if="!s.current"
                class="btn btn-danger btn-sm"
                :disabled="revokingSession === s.id"
                @click="revokeSession(s)"
              >
                {{ revokingSession === s.id ? 'Revoking...' : 'Revoke' }}
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- Change Password -->
      <div class="card">
        <div class="card-header"><h2>Change Password</h2></div>
        <div class="card-body">
          <form @submit.prevent="handlePasswordChange" class="profile-form">
            <div class="form-group">
              <label class="form-label" for="current-password">Current Password</label>
              <input id="current-password" v-model="currentPassword" type="password" class="form-input" placeholder="Enter current password" required autocomplete="current-password" />
            </div>
            <div class="form-group">
              <label class="form-label" for="new-password">New Password</label>
              <input id="new-password" v-model="newPassword" type="password" class="form-input" placeholder="Minimum 8 characters" required minlength="8" autocomplete="new-password" />
            </div>
            <div class="form-group">
              <label class="form-label" for="confirm-password">Confirm New Password</label>
              <input id="confirm-password" v-model="confirmPassword" type="password" class="form-input" placeholder="Re-enter new password" required minlength="8" autocomplete="new-password" />
            </div>
            <button type="submit" class="btn btn-primary" :disabled="passwordLoading">
              {{ passwordLoading ? 'Updating...' : 'Change Password' }}
            </button>
          </form>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.profile-grid {
  display: grid;
  gap: 24px;
  max-width: 640px;
}

.profile-form {
  display: grid;
  gap: 1rem;
}

.form-hint {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 4px;
}

.tfa-description {
  font-size: 13px;
  color: var(--text-secondary);
  margin-bottom: 16px;
}

.tfa-setup {
  display: grid;
  gap: 16px;
}

.tfa-qr {
  display: flex;
  justify-content: center;
  padding: 16px;
  background: #fff;
  border-radius: var(--radius);
  border: 1px solid var(--border-primary);
  width: fit-content;
}

.tfa-secret-group {
  display: grid;
  gap: 6px;
}

.tfa-secret {
  display: block;
  padding: 10px 14px;
  background: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius);
  font-size: 14px;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  letter-spacing: 2px;
  word-break: break-all;
  user-select: all;
}

.totp-input {
  font-size: 20px;
  text-align: center;
  letter-spacing: 8px;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  max-width: 220px;
}

.session-list {
  display: grid;
  gap: 8px;
}

.session-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 14px;
  border: 1px solid var(--border-primary);
  border-radius: var(--radius);
  background: var(--bg-secondary);
}

.session-current {
  border-color: var(--primary-300);
  background: var(--primary-50, rgba(147, 51, 234, 0.04));
}

.session-info {
  min-width: 0;
}

.session-browser {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-primary);
  display: flex;
  align-items: center;
}

.session-meta {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 2px;
}
</style>
