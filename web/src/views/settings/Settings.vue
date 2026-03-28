<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { settingsApi } from '../../api/settings'
import { authApi } from '../../api/auth'
import { userDataApi } from '../../api/userData'
import { workspaceApi } from '../../api/workspaces'
import { useAuthStore } from '../../stores/auth'
import { useWorkspaceStore } from '../../stores/workspace'
import { useThemeStore, type ThemeMode } from '../../stores/theme'
import { useNotificationStore } from '../../stores/notification'
import { useConfirm } from '../../composables/useConfirm'
import type { UserSettings } from '../../api/types'

const auth = useAuthStore()
const wsStore = useWorkspaceStore()
const theme = useThemeStore()
const notify = useNotificationStore()
const { confirm } = useConfirm()
const loading = ref(true)
const saving = ref(false)

// Data management
const exporting = ref(false)
const importing = ref(false)
const deletingContacts = ref(false)
const deletingEmails = ref(false)
const gdprEmail = ref('')
const gdprDays = ref(90)
const importFileRef = ref<HTMLInputElement | null>(null)

const form = ref<Partial<UserSettings>>({
  timezone: 'UTC',
  default_sender_name: '',
  default_sender_email: '',
  email_notifications: true,
  notification_email: '',
  webhook_retry_count: 3,
  api_key_expiry_days: 90,
  bounce_auto_suppress: true,
  daily_report: false,
})

const timezones = [
  'UTC', 'America/New_York', 'America/Chicago', 'America/Denver', 'America/Los_Angeles',
  'Europe/London', 'Europe/Paris', 'Europe/Berlin', 'Europe/Moscow',
  'Asia/Tokyo', 'Asia/Shanghai', 'Asia/Kolkata', 'Asia/Dubai',
  'Australia/Sydney', 'Pacific/Auckland','Africa/Kinshasa', 'Africa/Nairobi', 'Africa/Lagos','Africa/Lubumbashi',
]

// Domain Security
const requireVerifiedDomain = ref(false)
const domainSecurityLoading = ref(false)

// Theme
const themeModes: { value: ThemeMode; label: string; icon: string }[] = [
  { value: 'light', label: 'Light', icon: 'sun' },
  { value: 'dark', label: 'Dark', icon: 'moon' },
  { value: 'system', label: 'System', icon: 'monitor' },
]

onMounted(async () => {
  try {
    const [settingsRes, profileRes] = await Promise.all([
      settingsApi.getUserSettings(),
      authApi.me(),
    ])
    const s = settingsRes.data.data
    form.value = {
      timezone: s.timezone || 'UTC',
      default_sender_name: s.default_sender_name || '',
      default_sender_email: s.default_sender_email || '',
      email_notifications: s.email_notifications,
      notification_email: s.notification_email || '',
      webhook_retry_count: s.webhook_retry_count,
      api_key_expiry_days: s.api_key_expiry_days,
      bounce_auto_suppress: s.bounce_auto_suppress,
      daily_report: s.daily_report,
    }
    requireVerifiedDomain.value = profileRes.data.data.require_verified_domain
  } catch {
    notify.error('Failed to load settings')
  } finally {
    loading.value = false
  }
})

async function save() {
  saving.value = true
  try {
    const res = await settingsApi.updateUserSettings(form.value)
    const s = res.data.data
    form.value = {
      timezone: s.timezone,
      default_sender_name: s.default_sender_name,
      default_sender_email: s.default_sender_email,
      email_notifications: s.email_notifications,
      notification_email: s.notification_email,
      webhook_retry_count: s.webhook_retry_count,
      api_key_expiry_days: s.api_key_expiry_days,
      bounce_auto_suppress: s.bounce_auto_suppress,
      daily_report: s.daily_report,
    }
    notify.success('Settings saved')
  } catch {
    notify.error('Failed to save settings')
  } finally {
    saving.value = false
  }
}

async function exportData() {
  exporting.value = true
  try {
    const isWorkspace = wsStore.isWorkspaceContext
    const res = isWorkspace
      ? await workspaceApi.exportData()
      : await userDataApi.exportAll()
    const blob = new Blob([JSON.stringify(res.data.data, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    const prefix = isWorkspace
      ? `posta-workspace-export-${wsStore.currentWorkspace?.slug || 'ws'}`
      : 'posta-export'
    a.download = `${prefix}-${new Date().toISOString().slice(0, 10)}.json`
    a.click()
    URL.revokeObjectURL(url)
    notify.success('Data exported successfully')
  } catch {
    notify.error('Failed to export data')
  } finally {
    exporting.value = false
  }
}

function triggerImport() {
  importFileRef.value?.click()
}

async function handleImportFile(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0]
  if (!file) return

  const isWorkspace = wsStore.isWorkspaceContext
  const importMessage = isWorkspace
    ? 'This will import data into the current workspace. Duplicate items will be skipped. SMTP servers will be imported as disabled. Domains will require re-verification. Continue?'
    : 'This will import data from the selected file. Duplicate items will be skipped. Continue?'

  const confirmed = await confirm({
    title: isWorkspace ? 'Import Workspace Data' : 'Import Data',
    message: importMessage,
    confirmText: 'Import',
    variant: 'danger',
  })
  if (!confirmed) {
    if (importFileRef.value) importFileRef.value.value = ''
    return
  }

  importing.value = true
  try {
    const text = await file.text()
    const data = JSON.parse(text)
    const res = isWorkspace
      ? await workspaceApi.importData(data)
      : await userDataApi.importAll(data)
    notify.success(res.data.data.message || 'Data imported successfully')
  } catch (e: any) {
    if (e instanceof SyntaxError) {
      notify.error('Invalid JSON file')
    } else {
      notify.error(e.response?.data?.error?.message || 'Failed to import data')
    }
  } finally {
    importing.value = false
    if (importFileRef.value) importFileRef.value.value = ''
  }
}

async function deleteContacts() {
  const target = gdprEmail.value.trim()
  const msg = target
    ? `Delete contact "${target}" and remove from all lists and suppression?`
    : 'Delete ALL contacts? This cannot be undone.'
  const confirmed = await confirm({
    title: 'Delete Contact Data',
    message: msg,
    confirmText: 'Delete',
    variant: 'danger',
  })
  if (!confirmed) return

  deletingContacts.value = true
  try {
    const res = await userDataApi.deleteContacts(target || undefined)
    notify.success(res.data.data.message)
    gdprEmail.value = ''
  } catch (e: any) {
    notify.error(e.response?.data?.error?.message || 'Failed to delete contacts')
  } finally {
    deletingContacts.value = false
  }
}

async function deleteEmailLogs() {
  const days = gdprDays.value
  const msg = days > 0
    ? `Delete all email logs older than ${days} days and their associated bounces?`
    : 'Delete ALL email logs and associated bounces? This cannot be undone.'
  const confirmed = await confirm({
    title: 'Delete Email Logs',
    message: msg,
    confirmText: 'Delete',
    variant: 'danger',
  })
  if (!confirmed) return

  deletingEmails.value = true
  try {
    const res = await userDataApi.deleteEmailLogs(days)
    notify.success(res.data.data.message)
  } catch (e: any) {
    notify.error(e.response?.data?.error?.message || 'Failed to delete email logs')
  } finally {
    deletingEmails.value = false
  }
}

async function toggleDomainSecurity() {
  domainSecurityLoading.value = true
  try {
    const res = await authApi.updateProfile({
      name: auth.user?.name || '',
      require_verified_domain: !requireVerifiedDomain.value,
    })
    auth.user = res.data.data
    localStorage.setItem('posta_user', JSON.stringify(res.data.data))
    requireVerifiedDomain.value = res.data.data.require_verified_domain
    notify.success(requireVerifiedDomain.value ? 'Strict domain mode enabled' : 'Strict domain mode disabled')
  } catch (e: any) {
    const message = e?.response?.data?.error?.message || 'Failed to update setting'
    notify.error(message)
  } finally {
    domainSecurityLoading.value = false
  }
}
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Settings</h1>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <div v-else class="settings-grid">
      <!-- General -->
      <div class="card">
        <div class="card-header"><h2>General</h2></div>
        <div class="card-body">
          <form @submit.prevent="save" class="settings-form">
            <div class="form-group">
              <label class="form-label">Timezone</label>
              <select v-model="form.timezone" class="form-select">
                <option v-for="tz in timezones" :key="tz" :value="tz">{{ tz }}</option>
              </select>
              <span class="form-hint">Used for displaying timestamps and scheduling emails.</span>
            </div>
            <div class="form-group">
              <label class="form-label">Default Sender Name</label>
              <input v-model="form.default_sender_name" type="text" class="form-input" placeholder="e.g. My Company" />
              <span class="form-hint">Pre-filled sender name when sending emails.</span>
            </div>
            <div class="form-group">
              <label class="form-label">Default Sender Email</label>
              <input v-model="form.default_sender_email" type="email" class="form-input" placeholder="e.g. noreply@example.com" />
              <span class="form-hint">Pre-filled sender address when sending emails.</span>
            </div>
            <button type="submit" class="btn btn-primary" :disabled="saving">
              {{ saving ? 'Saving...' : 'Save Changes' }}
            </button>
          </form>
        </div>
      </div>

      <!-- Notifications -->
      <div class="card">
        <div class="card-header"><h2>Notifications</h2></div>
        <div class="card-body">
          <div class="toggle-row">
            <div>
              <label class="toggle-label">Email Notifications</label>
              <span class="form-hint">Receive notifications on failures, bounces, etc.</span>
            </div>
            <button
              :class="['toggle-btn', { active: form.email_notifications }]"
              @click="form.email_notifications = !form.email_notifications"
            >
              <span class="toggle-slider"></span>
            </button>
          </div>
          <div class="form-group" style="margin-top: 16px">
            <label class="form-label">Notification Email</label>
            <input v-model="form.notification_email" type="email" class="form-input" placeholder="Defaults to your login email" />
            <span class="form-hint">Where to send notifications (can differ from your login email).</span>
          </div>
          <div class="toggle-row" style="margin-top: 16px">
            <div>
              <label class="toggle-label">Daily Report</label>
              <span class="form-hint">Receive a daily email summary of send statistics.</span>
            </div>
            <button
              :class="['toggle-btn', { active: form.daily_report }]"
              @click="form.daily_report = !form.daily_report"
            >
              <span class="toggle-slider"></span>
            </button>
          </div>
        </div>
      </div>

      <!-- Email Delivery -->
      <div class="card">
        <div class="card-header"><h2>Email Delivery</h2></div>
        <div class="card-body">
          <div class="form-group">
            <label class="form-label">Webhook Retry Count</label>
            <input v-model.number="form.webhook_retry_count" type="number" class="form-input" min="0" max="10" />
            <span class="form-hint">How many times to retry failed webhook deliveries.</span>
          </div>
          <div class="toggle-row" style="margin-top: 16px">
            <div>
              <label class="toggle-label">Auto-Suppress on Bounce</label>
              <span class="form-hint">Automatically add to suppression list on hard bounce.</span>
            </div>
            <button
              :class="['toggle-btn', { active: form.bounce_auto_suppress }]"
              @click="form.bounce_auto_suppress = !form.bounce_auto_suppress"
            >
              <span class="toggle-slider"></span>
            </button>
          </div>
        </div>
      </div>

      <!-- Domain Security -->
      <div class="card">
        <div class="card-header">
          <h2>Domain Security</h2>
          <span v-if="requireVerifiedDomain" class="badge badge-success">Strict</span>
          <span v-else class="badge badge-secondary">Permissive</span>
        </div>
        <div class="card-body">
          <p class="section-description">
            When strict domain mode is enabled, emails can only be sent from domains you have registered and verified via DNS TXT record.
            This prevents sending from unverified domains and protects your sender reputation.
          </p>
          <div class="toggle-row">
            <label class="toggle-label">Require verified domain</label>
            <button
              :class="['toggle-btn', { active: requireVerifiedDomain }]"
              :disabled="domainSecurityLoading"
              @click="toggleDomainSecurity"
            >
              <span class="toggle-slider"></span>
            </button>
          </div>
        </div>
      </div>

      <!-- API & Templates -->
      <div class="card">
        <div class="card-header"><h2>API & Templates</h2></div>
        <div class="card-body">
          <div class="form-group">
            <label class="form-label">Default API Key Expiry (days)</label>
            <input v-model.number="form.api_key_expiry_days" type="number" class="form-input" min="1" max="365" />
            <span class="form-hint">Default expiration period for newly created API keys.</span>
          </div>
        </div>
      </div>

      <!-- Data Export/Import -->
      <div class="card">
        <div class="card-header">
          <h2>Data Export / Import</h2>
          <span v-if="wsStore.isWorkspaceContext" class="badge badge-primary">{{ wsStore.contextLabel }}</span>
        </div>
        <div class="card-body">
          <p v-if="wsStore.isWorkspaceContext" class="section-description">
            Export all workspace data (settings, templates, stylesheets, languages, contacts, contact lists, webhooks, SMTP servers, domains, subscribers, subscriber lists, and suppressions) as a JSON file.
            You can import this file to restore data on this or another workspace.
          </p>
          <p v-else class="section-description">
            Export all your data (templates, stylesheets, languages, contacts, contact lists, webhooks, suppressions, and settings) as a JSON file.
            You can import this file later to restore your data on this or another Posta instance.
          </p>
          <p v-if="wsStore.isWorkspaceContext" class="section-hint">
            Note: SMTP server passwords are not included in exports. Imported SMTP servers will be disabled until passwords are reconfigured.
            Imported domains will require re-verification.
          </p>
          <div class="flex gap-2">
            <button class="btn btn-primary" :disabled="exporting" @click="exportData">
              {{ exporting ? 'Exporting...' : wsStore.isWorkspaceContext ? 'Export Workspace Data' : 'Export All Data' }}
            </button>
            <button class="btn btn-secondary" :disabled="importing" @click="triggerImport">
              {{ importing ? 'Importing...' : 'Import Data' }}
            </button>
            <input ref="importFileRef" type="file" accept=".json" style="display: none" @change="handleImportFile" />
          </div>
        </div>
      </div>

      <!-- GDPR Data Management -->
      <div class="card">
        <div class="card-header">
          <h2>Data Management (GDPR)</h2>
        </div>
        <div class="card-body">
          <p class="section-description">
            Manage personal data for GDPR compliance. Delete specific contacts or purge old email logs.
          </p>

          <div class="gdpr-section">
            <h3 class="gdpr-title">Delete Contact Data</h3>
            <p class="section-description">
              Remove a specific contact by email (including suppression list and list memberships), or leave empty to delete all contacts.
            </p>
            <div class="flex gap-2 align-center">
              <input v-model="gdprEmail" type="email" class="form-input" placeholder="email@example.com (or leave empty for all)" style="flex: 1" />
              <button class="btn btn-danger" :disabled="deletingContacts" @click="deleteContacts">
                {{ deletingContacts ? 'Deleting...' : 'Delete' }}
              </button>
            </div>
          </div>

          <div class="gdpr-section" style="margin-top: 24px">
            <h3 class="gdpr-title">Delete Email Logs</h3>
            <p class="section-description">
              Delete email logs and associated bounce records. Set days to 0 to delete all logs.
            </p>
            <div class="flex gap-2 align-center">
              <label class="form-label" style="margin: 0; white-space: nowrap">Older than</label>
              <input v-model.number="gdprDays" type="number" class="form-input" min="0" style="width: 100px" />
              <label class="form-label" style="margin: 0">days</label>
              <button class="btn btn-danger" :disabled="deletingEmails" @click="deleteEmailLogs">
                {{ deletingEmails ? 'Deleting...' : 'Delete Logs' }}
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- Theme -->
      <div class="card">
        <div class="card-header"><h2>Theme</h2></div>
        <div class="card-body">
          <p class="section-description">Choose how the application looks to you.</p>
          <div class="theme-options">
            <button
              v-for="m in themeModes"
              :key="m.value"
              :class="['theme-option', { active: theme.mode === m.value }]"
              @click="theme.setMode(m.value)"
            >
              <div class="theme-option-icon">
                <svg v-if="m.icon === 'sun'" width="20" height="20" viewBox="0 0 16 16" fill="none"><circle cx="8" cy="8" r="3" stroke="currentColor" stroke-width="1.5"/><path d="M8 1v2M8 13v2M1 8h2M13 8h2M3.05 3.05l1.41 1.41M11.54 11.54l1.41 1.41M3.05 12.95l1.41-1.41M11.54 4.46l1.41-1.41" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>
                <svg v-else-if="m.icon === 'moon'" width="20" height="20" viewBox="0 0 16 16" fill="none"><path d="M14 9.5A6.5 6.5 0 016.5 2 6.5 6.5 0 1014 9.5z" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>
                <svg v-else width="20" height="20" viewBox="0 0 16 16" fill="none"><rect x="2" y="3" width="12" height="10" rx="1.5" stroke="currentColor" stroke-width="1.5"/><path d="M2 5.5h12" stroke="currentColor" stroke-width="1.5"/></svg>
              </div>
              <span class="theme-option-label">{{ m.label }}</span>
            </button>
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
  max-width: 640px;
}

.settings-form {
  display: grid;
  gap: 1rem;
}

.form-hint {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 4px;
  display: block;
}

.section-description {
  font-size: 13px;
  color: var(--text-secondary);
  margin-bottom: 16px;
}

.toggle-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.toggle-label {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-primary);
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

.toggle-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
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

.theme-options {
  display: flex;
  gap: 12px;
}

.theme-option {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 16px 24px;
  border: 2px solid var(--border-primary);
  border-radius: var(--radius);
  background: var(--bg-primary);
  color: var(--text-secondary);
  cursor: pointer;
  transition: all var(--transition);
  font-family: inherit;
  min-width: 90px;
}

.theme-option:hover {
  border-color: var(--primary-400);
  color: var(--text-primary);
}

.theme-option.active {
  border-color: var(--primary-600);
  color: var(--primary-600);
  background: var(--primary-50, rgba(79, 70, 229, 0.05));
}

.theme-option-icon {
  display: flex;
  align-items: center;
  justify-content: center;
}

.theme-option-label {
  font-size: 13px;
  font-weight: 500;
}

.align-center {
  align-items: center;
}

.gdpr-section h3.gdpr-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 4px;
}

.section-hint {
  font-size: 12px;
  color: var(--text-muted);
  margin-bottom: 16px;
}
</style>
