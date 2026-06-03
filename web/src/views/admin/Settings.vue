<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { settingsApi } from '../../api/settings'
import type { AdminSetting } from '../../api/types'
import { useNotificationStore } from '../../stores/notification'
import { useConfirm } from '../../composables/useConfirm'

const notify = useNotificationStore()
const { confirm } = useConfirm()
const loading = ref(true)
const settings = ref<AdminSetting[]>([])

// Human-readable labels and descriptions for each setting key
const settingMeta: Record<string, { label: string; description: string; category: string }> = {
  registration_enabled: { label: 'User Registration', description: 'Allow new users to self-register.', category: 'General' },
  allowed_signup_domains: { label: 'Allowed Signup Domains', description: 'Restrict registration to specific email domains (comma-separated). Leave empty to allow all.', category: 'General' },
  maintenance_mode: { label: 'Maintenance Mode', description: 'Disable all email sending and show a maintenance banner.', category: 'General' },
  require_email_verification: { label: 'Require Email Verification', description: 'New users must verify their email before sending.', category: 'Security' },
  require_domain_verification: { label: 'Require Domain Verification', description: 'Users must verify domain ownership before sending.', category: 'Security' },
  two_factor_required: { label: 'Require Two-Factor Auth', description: 'Force all users to enable 2FA.', category: 'Security' },
  password_reset_enabled: { label: 'Self-Service Password Reset', description: 'Allow users to reset a forgotten password via an emailed link. Requires a configured system SMTP sender.', category: 'Security' },
  default_rate_limit_hourly: { label: 'Hourly Rate Limit', description: 'Default hourly send limit for users.', category: 'Limits' },
  default_rate_limit_daily: { label: 'Daily Rate Limit', description: 'Default daily send limit for users.', category: 'Limits' },
  max_batch_size: { label: 'Max Batch Size', description: 'Maximum recipients in a single batch send.', category: 'Limits' },
  max_attachment_size_mb: { label: 'Max Attachment Size (MB)', description: 'Maximum attachment size in megabytes.', category: 'Limits' },
  global_bounce_threshold: { label: 'Bounce Threshold', description: 'Auto-suppress a contact after this many bounces.', category: 'Limits' },
  login_rate_limit_count: { label: 'Login Rate Limit (attempts)', description: 'Max login attempts per IP within the login window.', category: 'Security' },
  login_rate_limit_window_minutes: { label: 'Login Rate Limit Window (minutes)', description: 'Time window for the login rate limit.', category: 'Security' },
  smtp_timeout_seconds: { label: 'SMTP Timeout (seconds)', description: 'Global SMTP connection timeout.', category: 'Limits' },
  retention_days: { label: 'Email Log Retention (days)', description: 'How long to keep email logs before cleanup.', category: 'Retention' },
  audit_log_retention_days: { label: 'Audit Log Retention (days)', description: 'How long to keep audit/event logs.', category: 'Retention' },
  webhook_delivery_retention_days: { label: 'Webhook Delivery Retention (days)', description: 'How long to keep webhook delivery logs.', category: 'Retention' },
  email_content_visibility: { label: 'Email Content Visibility', description: 'When enabled, email body content (HTML/Text) is visible in the dashboard. When disabled, content is redacted for privacy.', category: 'Privacy' },
  custom_headers_enabled: { label: 'Custom Email Headers', description: 'Allow users to include custom headers when sending emails. When disabled, custom headers are silently ignored.', category: 'Security' },
}

const categories = ['General', 'Security', 'Privacy', 'Limits', 'Retention']

function settingsByCategory(category: string) {
  // Only render known, documented settings. Unknown keys (e.g. the upgrade
  // framework's internal app.* bookkeeping rows) are never editable platform
  // settings and must not leak into the UI.
  return settings.value.filter(s => settingMeta[s.key]?.category === category)
}


const editedValues = ref<Record<string, string>>({})
const savingKeys = ref<Record<string, boolean>>({})
const savedKeys = ref<Record<string, boolean>>({})

// Debounce timers for free-text / number inputs (toggles save immediately).
const SAVE_DEBOUNCE_MS = 700
const timers: Record<string, ReturnType<typeof setTimeout>> = {}
const savedFlashTimers: Record<string, ReturnType<typeof setTimeout>> = {}

function getEditedValue(key: string, original: string): string {
  return key in editedValues.value ? editedValues.value[key] : original
}

function setEditedValue(key: string, value: string) {
  editedValues.value[key] = value
}

const anySaving = computed(() => Object.values(savingKeys.value).some(Boolean))

onMounted(async () => {
  try {
    const res = await settingsApi.getAdminSettings()
    settings.value = res.data.data || []
  } catch {
    notify.error('Failed to load settings')
  } finally {
    loading.value = false
  }
})

// Persist a single setting if its working value differs from the committed one.
async function commit(key: string) {
  clearTimeout(timers[key])
  const setting = settings.value.find(s => s.key === key)
  if (!setting) return

  const value = getEditedValue(key, setting.value)
  if (value === setting.value) {
    delete editedValues.value[key]
    return
  }

  savingKeys.value[key] = true
  try {
    const res = await settingsApi.updateAdminSettings([{ key, value, type: setting.type }])
    // Adopt the server's canonical value (it may normalize) and clear the edit.
    const updated = (res.data.data || []).find(s => s.key === key)
    setting.value = updated ? updated.value : value
    delete editedValues.value[key]
    flashSaved(key)
  } catch {
    const label = settingMeta[key]?.label || key
    notify.error(`Failed to save “${label}”`)
    // Revert the field to the last committed value.
    delete editedValues.value[key]
  } finally {
    savingKeys.value[key] = false
  }
}

function flashSaved(key: string) {
  savedKeys.value[key] = true
  clearTimeout(savedFlashTimers[key])
  savedFlashTimers[key] = setTimeout(() => {
    savedKeys.value[key] = false
  }, 2000)
}

// Free-text / number input: track the value and debounce the save.
function onInput(key: string, event: Event) {
  const value = (event.target as HTMLInputElement).value
  setEditedValue(key, value)
  clearTimeout(timers[key])
  timers[key] = setTimeout(() => commit(key), SAVE_DEBOUNCE_MS)
}

// Toggles that change platform-wide behavior require an explicit confirmation
// before auto-save fires. Messages adapt to the direction of the change.
type ConfirmCopy = { title: string; message: string; confirmText: string; variant: 'danger' | 'warning' | 'info' }
const sensitiveToggles: Record<string, { enable: ConfirmCopy; disable: ConfirmCopy }> = {
  maintenance_mode: {
    enable: {
      title: 'Enable Maintenance Mode',
      message: 'This stops ALL email sending platform-wide and shows a maintenance banner to every user. Continue?',
      confirmText: 'Enable Maintenance Mode',
      variant: 'danger',
    },
    disable: {
      title: 'Disable Maintenance Mode',
      message: 'Email sending will resume for all users and the maintenance banner will be removed. Continue?',
      confirmText: 'Disable Maintenance Mode',
      variant: 'warning',
    },
  },
  two_factor_required: {
    enable: {
      title: 'Require Two-Factor Auth',
      message: 'All users will be forced to set up two-factor authentication on their next login. Continue?',
      confirmText: 'Require 2FA',
      variant: 'warning',
    },
    disable: {
      title: 'Stop Requiring Two-Factor Auth',
      message: 'Users will no longer be required to enable two-factor authentication. This lowers account security. Continue?',
      confirmText: 'Disable Requirement',
      variant: 'danger',
    },
  },
  custom_headers_enabled: {
    enable: {
      title: 'Allow Custom Email Headers',
      message: 'Users will be able to attach custom headers to outgoing emails. This can affect deliverability and may expose internal headers. Continue?',
      confirmText: 'Allow Custom Headers',
      variant: 'warning',
    },
    disable: {
      title: 'Disable Custom Email Headers',
      message: 'Custom headers will be silently ignored for all users. Continue?',
      confirmText: 'Disable Custom Headers',
      variant: 'warning',
    },
  },
  email_content_visibility: {
    enable: {
      title: 'Make Email Content Visible',
      message: 'Email body content (HTML/Text) will be visible in the dashboard for all users. Disabling redaction can expose sensitive recipient data. Continue?',
      confirmText: 'Make Content Visible',
      variant: 'danger',
    },
    disable: {
      title: 'Redact Email Content',
      message: 'Email body content will be redacted from the dashboard for privacy. Continue?',
      confirmText: 'Redact Content',
      variant: 'warning',
    },
  },
}

// Boolean toggle: confirm sensitive changes, then flip and persist immediately.
async function toggleBool(setting: AdminSetting) {
  const next = getEditedValue(setting.key, setting.value) === 'true' ? 'false' : 'true'

  const sensitive = sensitiveToggles[setting.key]
  if (sensitive) {
    const copy = next === 'true' ? sensitive.enable : sensitive.disable
    const ok = await confirm(copy)
    if (!ok) return // leave the toggle unchanged
  }

  setEditedValue(setting.key, next)
  commit(setting.key)
}
</script>

<template>
  <div>
    <div class="page-header">
      <div>
        <h1>Platform Settings</h1>
        <p class="page-description">Configure platform-wide settings for all users.</p>
      </div>
      <div class="autosave-status" :class="{ active: anySaving }">
        <template v-if="anySaving">
          <span class="autosave-spinner"></span>
          <span>Saving…</span>
        </template>
        <template v-else>
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"
            stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <path d="M20 6 9 17l-5-5" />
          </svg>
          <span>Changes save automatically</span>
        </template>
      </div>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <div v-else class="settings-grid">
      <div v-for="category in categories" :key="category" class="card">
        <div class="card-header"><h2>{{ category }}</h2></div>
        <div class="card-body">
          <div
            v-for="setting in settingsByCategory(category)"
            :key="setting.key"
            class="setting-row"
          >
            <div class="setting-info">
              <label class="setting-label">{{ settingMeta[setting.key]?.label || setting.key }}</label>
              <span class="setting-description">{{ settingMeta[setting.key]?.description || '' }}</span>
            </div>
            <div class="setting-control">
              <span class="setting-status" aria-live="polite">
                <span v-if="savingKeys[setting.key]" class="setting-status-saving">Saving…</span>
                <span v-else-if="savedKeys[setting.key]" class="setting-status-saved">Saved ✓</span>
              </span>
              <!-- Boolean toggle -->
              <template v-if="setting.type === 'bool'">
                <button
                  :class="['toggle-btn', { active: getEditedValue(setting.key, setting.value) === 'true' }]"
                  :disabled="savingKeys[setting.key]"
                  @click="toggleBool(setting)"
                >
                  <span class="toggle-slider"></span>
                </button>
              </template>
              <!-- Number input -->
              <template v-else-if="setting.type === 'int'">
                <input
                  type="number"
                  class="form-input setting-input-number"
                  :value="getEditedValue(setting.key, setting.value)"
                  @input="onInput(setting.key, $event)"
                  @blur="commit(setting.key)"
                  @keyup.enter="commit(setting.key)"
                />
              </template>
              <!-- String input -->
              <template v-else>
                <input
                  type="text"
                  class="form-input setting-input-text"
                  :value="getEditedValue(setting.key, setting.value)"
                  @input="onInput(setting.key, $event)"
                  @blur="commit(setting.key)"
                  @keyup.enter="commit(setting.key)"
                />
              </template>
            </div>
          </div>
          <div v-if="settingsByCategory(category).length === 0" class="empty-state">
            <p>No settings in this category.</p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.settings-grid {
  display: grid;
  gap: 24px;
  max-width: 720px;
}

.setting-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  padding: 14px 0;
  border-bottom: 1px solid var(--border-primary);
}

.setting-row:last-child {
  border-bottom: none;
}

.setting-info {
  flex: 1;
  min-width: 0;
}

.setting-label {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-primary);
  display: block;
}

.setting-description {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 2px;
  display: block;
}

.setting-control {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 10px;
}

/* Header auto-save status */
.autosave-status {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12.5px;
  color: var(--text-muted);
  white-space: nowrap;
}
.autosave-status.active { color: var(--primary-600); }
.autosave-spinner {
  width: 13px;
  height: 13px;
  border: 2px solid color-mix(in srgb, var(--primary-600) 30%, transparent);
  border-top-color: var(--primary-600);
  border-radius: 50%;
  animation: autosave-spin 0.6s linear infinite;
}
@keyframes autosave-spin { to { transform: rotate(360deg); } }

/* Per-row save status */
.setting-status {
  font-size: 11.5px;
  min-width: 56px;
  text-align: right;
}
.setting-status-saving { color: var(--text-muted); }
.setting-status-saved { color: var(--success-600); font-weight: 500; }

.setting-input-number {
  width: 100px;
  text-align: right;
}

.setting-input-text {
  width: 200px;
}

.toggle-btn {
  position: relative;
  width: 44px;
  height: 24px;
  border-radius: 12px;
  border: none;
  background: var(--border-primary);
  cursor: pointer;
  transition: background 0.2s;
  padding: 0;
  flex-shrink: 0;
}

.toggle-btn.active {
  background: var(--primary-600);
}

.toggle-slider {
  position: absolute;
  top: 2px;
  left: 2px;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: white;
  transition: transform 0.2s;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.15);
}

.toggle-btn.active .toggle-slider {
  transform: translateX(20px);
}
</style>
