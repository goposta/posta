<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { settingsApi } from '../../api/settings'
import type { AdminSetting } from '../../api/types'
import { useNotificationStore } from '../../stores/notification'

const notify = useNotificationStore()
const loading = ref(true)
const saving = ref(false)
const settings = ref<AdminSetting[]>([])

// Human-readable labels and descriptions for each setting key
const settingMeta: Record<string, { label: string; description: string; category: string }> = {
  registration_enabled: { label: 'User Registration', description: 'Allow new users to self-register.', category: 'General' },
  allowed_signup_domains: { label: 'Allowed Signup Domains', description: 'Restrict registration to specific email domains (comma-separated). Leave empty to allow all.', category: 'General' },
  maintenance_mode: { label: 'Maintenance Mode', description: 'Disable all email sending and show a maintenance banner.', category: 'General' },
  require_email_verification: { label: 'Require Email Verification', description: 'New users must verify their email before sending.', category: 'Security' },
  require_domain_verification: { label: 'Require Domain Verification', description: 'Users must verify domain ownership before sending.', category: 'Security' },
  two_factor_required: { label: 'Require Two-Factor Auth', description: 'Force all users to enable 2FA.', category: 'Security' },
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
}

const categories = ['General', 'Security', 'Limits', 'Retention']

function settingsByCategory(category: string) {
  return settings.value.filter(s => {
    const meta = settingMeta[s.key]
    return meta ? meta.category === category : category === 'General'
  })
}

// Track edited values separately
const editedValues = ref<Record<string, string>>({})

function getEditedValue(key: string, original: string): string {
  return key in editedValues.value ? editedValues.value[key] : original
}

function setEditedValue(key: string, value: string) {
  editedValues.value[key] = value
}

const hasChanges = computed(() => {
  return settings.value.some(s => {
    return s.key in editedValues.value && editedValues.value[s.key] !== s.value
  })
})

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

async function save() {
  saving.value = true
  try {
    const changedSettings = settings.value
      .filter(s => s.key in editedValues.value && editedValues.value[s.key] !== s.value)
      .map(s => ({
        key: s.key,
        value: editedValues.value[s.key],
        type: s.type,
      }))

    if (changedSettings.length === 0) {
      notify.success('No changes to save')
      saving.value = false
      return
    }

    const res = await settingsApi.updateAdminSettings(changedSettings)
    settings.value = res.data.data || []
    editedValues.value = {}
    notify.success('Settings saved')
  } catch {
    notify.error('Failed to save settings')
  } finally {
    saving.value = false
  }
}

function toggleBool(setting: AdminSetting) {
  const current = getEditedValue(setting.key, setting.value)
  setEditedValue(setting.key, current === 'true' ? 'false' : 'true')
}
</script>

<template>
  <div>
    <div class="page-header">
      <div>
        <h1>Platform Settings</h1>
        <p class="page-description">Configure platform-wide settings for all users.</p>
      </div>
      <button class="btn btn-primary" :disabled="saving || !hasChanges" @click="save">
        {{ saving ? 'Saving...' : 'Save Changes' }}
      </button>
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
              <!-- Boolean toggle -->
              <template v-if="setting.type === 'bool'">
                <button
                  :class="['toggle-btn', { active: getEditedValue(setting.key, setting.value) === 'true' }]"
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
                  @input="setEditedValue(setting.key, ($event.target as HTMLInputElement).value)"
                />
              </template>
              <!-- String input -->
              <template v-else>
                <input
                  type="text"
                  class="form-input setting-input-text"
                  :value="getEditedValue(setting.key, setting.value)"
                  @input="setEditedValue(setting.key, ($event.target as HTMLInputElement).value)"
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
}

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
