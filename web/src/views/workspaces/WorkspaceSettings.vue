<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { workspaceApi } from '../../api/workspaces'
import { oauthApi } from '../../api/oauth'
import { settingsApi } from '../../api/settings'
import type { Workspace, Plan, OAuthProviderInfo, WorkspaceSSOConfig, WorkspaceSettings } from '../../api/types'
import { useNotificationStore } from '../../stores/notification'
import { useWorkspaceStore } from '../../stores/workspace'
import { useConfirm } from '../../composables/useConfirm'

const route = useRoute()
const router = useRouter()
const notify = useNotificationStore()
const wsStore = useWorkspaceStore()
const { confirm } = useConfirm()

const wsId = Number(route.params.id)
const ws = ref<Workspace | null>(null)
const loading = ref(true)

// Members and invitations now live on a dedicated page; redirect legacy deep-links.
if (route.query.tab === 'members' || route.query.tab === 'invitations') {
  router.replace(`/workspaces/${wsId}/members?tab=${route.query.tab}`)
}

// Tabs
type SettingsTab = 'settings' | 'data' | 'sso' | 'plan'
const validTabs: SettingsTab[] = ['settings', 'data', 'sso', 'plan']
function tabFromQuery(value: unknown): SettingsTab {
  if (value === 'transfer') return 'settings'
  return validTabs.includes(value as SettingsTab) ? (value as SettingsTab) : 'settings'
}
const activeTab = ref<SettingsTab>(tabFromQuery(route.query.tab))
watch(() => route.query.tab, (value) => {
  if (value === 'members' || value === 'invitations') {
    router.replace(`/workspaces/${wsId}/members?tab=${value}`)
    return
  }
  if (value) activeTab.value = tabFromQuery(value)
})

// Plan
const currentPlan = ref<Plan | null>(null)
const planLoading = ref(false)
const planSource = ref<string | null>(null)

// SSO
const ssoConfig = ref<WorkspaceSSOConfig | null>(null)
const ssoProviders = ref<OAuthProviderInfo[]>([])
const ssoLoading = ref(false)
const ssoSaving = ref(false)
const ssoForm = ref({ provider_id: 0, enforce_sso: false, auto_provision: true, allowed_domains: '' })

// Data Export/Import
const exporting = ref(false)
const importing = ref(false)
const importFileRef = ref<HTMLInputElement | null>(null)

// Settings (workspace name/description)
const editName = ref('')
const editDescription = ref('')
const saving = ref(false)

// Operational workspace settings (timezone, sender defaults, webhook retries,
// API-key expiry, bounce auto-suppress) — backed by /workspaces/current/settings.
const timezones = [
  'UTC', 'America/New_York', 'America/Chicago', 'America/Denver', 'America/Los_Angeles',
  'Europe/London', 'Europe/Paris', 'Europe/Berlin', 'Europe/Moscow',
  'Asia/Tokyo', 'Asia/Shanghai', 'Asia/Kolkata', 'Asia/Dubai',
  'Australia/Sydney', 'Pacific/Auckland', 'Africa/Kinshasa', 'Africa/Nairobi', 'Africa/Lagos', 'Africa/Lubumbashi',
]
const wsSettings = ref<Partial<WorkspaceSettings>>({
  timezone: 'UTC',
  default_sender_name: '',
  default_sender_email: '',
  webhook_retry_count: 3,
  api_key_expiry_days: 90,
  bounce_auto_suppress: true,
  require_verified_domain: false,
})
const wsSettingsLoading = ref(false)
const wsSettingsSaving = ref(false)

// Data Management (GDPR) — scoped to this workspace.
const gdprEmail = ref('')
const gdprDays = ref(90)
const deletingContacts = ref(false)
const deletingEmails = ref(false)

const myRole = computed(() => {
  const membership = wsStore.workspaces.find(w => w.id === wsId)
  return membership?.role ?? 'viewer'
})
const isAdminOrOwner = computed(() => myRole.value === 'owner' || myRole.value === 'admin')
const isOwner = computed(() => myRole.value === 'owner')
// Personal workspaces are auto-provisioned and cannot be deleted (enforced server-side).
const isPersonal = computed(() => ws.value?.is_personal ?? false)

async function withWorkspace<T>(fn: () => Promise<T>): Promise<T> {
  const prevWs = wsStore.currentWorkspaceId
  wsStore.setWorkspace(wsId)
  try {
    return await fn()
  } finally {
    wsStore.setWorkspace(prevWs)
  }
}

async function fetchWorkspace() {
  loading.value = true
  try {
    const res = await withWorkspace(() => workspaceApi.getCurrent())
    ws.value = res.data.data
    editName.value = ws.value.name
    editDescription.value = ws.value.description || ''
  } catch {
    notify.error('Failed to load workspace')
    router.push('/workspaces')
  } finally {
    loading.value = false
  }
}

async function fetchPlan() {
  planLoading.value = true
  try {
    const res = await withWorkspace(() => workspaceApi.getPlan())
    const data = res.data.data
    if (data && typeof data === 'object' && 'id' in data) {
      currentPlan.value = data as Plan
      planSource.value = null
    } else if (data && typeof data === 'object' && 'source' in data) {
      currentPlan.value = null
      planSource.value = (data as any).source ?? 'global_settings'
    } else {
      currentPlan.value = null
      planSource.value = 'global_settings'
    }
  } catch {
    currentPlan.value = null
    planSource.value = 'global_settings'
  } finally {
    planLoading.value = false
  }
}

async function fetchWorkspaceSettings() {
  wsSettingsLoading.value = true
  try {
    const res = await withWorkspace(() => settingsApi.getWorkspaceSettings())
    const s = res.data.data
    wsSettings.value = {
      timezone: s.timezone || 'UTC',
      default_sender_name: s.default_sender_name || '',
      default_sender_email: s.default_sender_email || '',
      webhook_retry_count: s.webhook_retry_count,
      api_key_expiry_days: s.api_key_expiry_days,
      bounce_auto_suppress: s.bounce_auto_suppress,
      require_verified_domain: s.require_verified_domain,
    }
  } catch {
    /* settings fall back to defaults */
  } finally {
    wsSettingsLoading.value = false
  }
}

async function saveWorkspaceSettings() {
  wsSettingsSaving.value = true
  try {
    await withWorkspace(() => settingsApi.updateWorkspaceSettings(wsSettings.value))
    notify.success('Workspace settings saved')
  } catch (err: any) {
    notify.error(err.response?.data?.error?.message || 'Failed to save workspace settings')
  } finally {
    wsSettingsSaving.value = false
  }
}

async function deleteWorkspaceContacts() {
  const target = gdprEmail.value.trim()
  const message = target
    ? `Delete contact "${target}" and remove it from all lists and suppression in this workspace?`
    : 'Delete ALL contacts in this workspace? This cannot be undone.'
  const confirmed = await confirm({ title: 'Delete Contact Data', message, confirmText: 'Delete', variant: 'danger' })
  if (!confirmed) return
  deletingContacts.value = true
  try {
    const res = await withWorkspace(() => workspaceApi.deleteContacts(target || undefined))
    notify.success(res.data.data.message)
    gdprEmail.value = ''
  } catch (e: any) {
    notify.error(e.response?.data?.error?.message || 'Failed to delete contacts')
  } finally {
    deletingContacts.value = false
  }
}

async function deleteWorkspaceEmailLogs() {
  const days = gdprDays.value
  const message = days > 0
    ? `Delete all email logs older than ${days} days and their associated bounces in this workspace?`
    : 'Delete ALL email logs and associated bounces in this workspace? This cannot be undone.'
  const confirmed = await confirm({ title: 'Delete Email Logs', message, confirmText: 'Delete', variant: 'danger' })
  if (!confirmed) return
  deletingEmails.value = true
  try {
    const res = await withWorkspace(() => workspaceApi.deleteEmailLogs(days))
    notify.success(res.data.data.message)
  } catch (e: any) {
    notify.error(e.response?.data?.error?.message || 'Failed to delete email logs')
  } finally {
    deletingEmails.value = false
  }
}

async function fetchSSO() {
  ssoLoading.value = true
  try {
    const [ssoRes, providersRes] = await withWorkspace(() => Promise.all([
      oauthApi.getSSO(),
      oauthApi.providers(),
    ]))
    ssoConfig.value = ssoRes.data.data
    ssoProviders.value = providersRes.data.data?.providers || []
    if (ssoConfig.value) {
      ssoForm.value = {
        provider_id: ssoConfig.value.provider_id,
        enforce_sso: ssoConfig.value.enforce_sso,
        auto_provision: ssoConfig.value.auto_provision,
        allowed_domains: ssoConfig.value.allowed_domains || '',
      }
    }
  } catch { /* ignore */ }
  finally {
    ssoLoading.value = false
  }
}

async function saveSSO() {
  if (!ssoForm.value.provider_id) {
    notify.error('Please select a provider')
    return
  }
  ssoSaving.value = true
  try {
    await withWorkspace(() => oauthApi.setSSO(ssoForm.value))
    notify.success('SSO configuration saved')
    await fetchSSO()
  } catch (err: any) {
    notify.error(err.response?.data?.error?.message || 'Failed to save SSO config')
  } finally {
    ssoSaving.value = false
  }
}

async function removeSSO() {
  const ok = await confirm({ title: 'Remove SSO', message: 'Remove SSO configuration from this workspace? Members will no longer be required to use SSO.', confirmText: 'Remove SSO', variant: 'danger' })
  if (!ok) return
  try {
    await withWorkspace(() => oauthApi.deleteSSO())
    ssoConfig.value = null
    ssoForm.value = { provider_id: 0, enforce_sso: false, auto_provision: true, allowed_domains: '' }
    notify.success('SSO configuration removed')
  } catch (err: any) {
    notify.error(err.response?.data?.error?.message || 'Failed to remove SSO config')
  }
}

function formatLimit(value: number): string {
  return value === 0 ? 'Unlimited' : value.toLocaleString()
}

async function saveSettings() {
  saving.value = true
  try {
    await withWorkspace(() => workspaceApi.update({ name: editName.value.trim(), description: editDescription.value.trim() }))
    notify.success('Workspace updated')
    await wsStore.fetchWorkspaces()
  } catch (err: any) {
    notify.error(err.response?.data?.error?.message || 'Failed to update')
  } finally {
    saving.value = false
  }
}

async function deleteWorkspace() {
  const ok = await confirm({ title: 'Delete Workspace', message: 'Are you sure you want to delete this workspace? This cannot be undone.', confirmText: 'Delete', variant: 'danger' })
  if (!ok) return
  try {
    await withWorkspace(() => workspaceApi.delete())
    notify.success('Workspace deleted')
    wsStore.setWorkspace(null)
    await wsStore.fetchWorkspaces()
    router.push('/workspaces')
  } catch (err: any) {
    notify.error(err.response?.data?.error?.message || 'Failed to delete')
  }
}

async function exportWorkspaceData() {
  exporting.value = true
  try {
    const res = await withWorkspace(() => workspaceApi.exportData())
    const blob = new Blob([JSON.stringify(res.data.data, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `posta-workspace-export-${ws.value?.slug || wsId}-${new Date().toISOString().slice(0, 10)}.json`
    a.click()
    URL.revokeObjectURL(url)
    notify.success('Workspace data exported successfully')
  } catch {
    notify.error('Failed to export workspace data')
  } finally {
    exporting.value = false
  }
}

function triggerWorkspaceImport() {
  importFileRef.value?.click()
}

async function handleWorkspaceImportFile(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0]
  if (!file) return

  const confirmed = await confirm({
    title: 'Import Workspace Data',
    message: 'This will import data into this workspace from the selected file. Duplicate items will be skipped. SMTP servers will be imported as disabled (passwords are not exported). Domains will require re-verification. Continue?',
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
    const res = await withWorkspace(() => workspaceApi.importData(data))
    notify.success(res.data.data.message || 'Workspace data imported successfully')
  } catch (e: any) {
    if (e instanceof SyntaxError) {
      notify.error('Invalid JSON file')
    } else {
      notify.error(e.response?.data?.error?.message || 'Failed to import workspace data')
    }
  } finally {
    importing.value = false
    if (importFileRef.value) importFileRef.value.value = ''
  }
}

onMounted(async () => {
  await fetchWorkspace()
  await fetchPlan()
})

watch(activeTab, (value) => {
  if (value === 'settings' && isAdminOrOwner.value) fetchWorkspaceSettings()
  if (value === 'sso' && isOwner.value) fetchSSO()
}, { immediate: true })
</script>

<template>
  <div>
    <div class="page-header">
      <div>
        <h1>{{ ws?.name || 'Workspace' }} — Settings</h1>
        <p v-if="ws?.slug" style="color: var(--text-muted); font-size: 13px; margin-top: 2px;">
          <code>{{ ws.slug }}</code>
        </p>
      </div>
      <div style="display: flex; gap: 8px;">
        <button class="btn btn-secondary" @click="router.push(`/workspaces/${wsId}/members`)">Members</button>
        <button class="btn btn-secondary" @click="router.push('/workspaces')">Back</button>
      </div>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <template v-else>
      <!-- Tabs -->
      <div class="tabs">
        <button v-if="isAdminOrOwner" :class="['tab', { active: activeTab === 'settings' }]" @click="activeTab = 'settings'">
          Settings
        </button>
        <button v-if="isAdminOrOwner" :class="['tab', { active: activeTab === 'data' }]" @click="activeTab = 'data'">
          Data Management
        </button>
        <button v-if="isOwner" :class="['tab', { active: activeTab === 'sso' }]" @click="activeTab = 'sso'">
          SSO
        </button>
        <button :class="['tab', { active: activeTab === 'plan' }]" @click="activeTab = 'plan'">
          Plan
        </button>
      </div>

      <!-- Plan Tab -->
      <div v-if="activeTab === 'plan'">
        <div v-if="planLoading" class="loading-page"><div class="spinner"></div></div>
        <template v-else-if="currentPlan">
          <div class="card" style="margin-bottom: 16px">
            <div class="card-header">
              <div>
                <h2 style="margin: 0">{{ currentPlan.name }}</h2>
                <p v-if="currentPlan.description" style="margin: 4px 0 0; font-size: 13px; color: var(--text-secondary)">{{ currentPlan.description }}</p>
              </div>
              <div class="flex gap-2">
                <span class="badge badge-success" v-if="currentPlan.is_active">Active</span>
                <span class="badge badge-info" v-if="currentPlan.is_default">Default</span>
              </div>
            </div>
          </div>

          <div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 16px;">
            <!-- Rate Limits -->
            <div class="card">
              <div class="card-header"><h3 style="margin: 0; font-size: 14px">Rate Limits</h3></div>
              <div class="card-body">
                <table>
                  <tbody>
                    <tr>
                      <td style="font-weight: 600; width: 140px">Hourly</td>
                      <td>{{ formatLimit(currentPlan.hourly_rate_limit) }}</td>
                    </tr>
                    <tr>
                      <td style="font-weight: 600">Daily</td>
                      <td>{{ formatLimit(currentPlan.daily_rate_limit) }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>

            <!-- Email Constraints -->
            <div class="card">
              <div class="card-header"><h3 style="margin: 0; font-size: 14px">Email Constraints</h3></div>
              <div class="card-body">
                <table>
                  <tbody>
                    <tr>
                      <td style="font-weight: 600; width: 140px">Attachment</td>
                      <td>{{ currentPlan.max_attachment_size_mb === 0 ? 'Unlimited' : currentPlan.max_attachment_size_mb + ' MB' }}</td>
                    </tr>
                    <tr>
                      <td style="font-weight: 600">Batch Size</td>
                      <td>{{ formatLimit(currentPlan.max_batch_size) }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>

            <!-- Resource Limits -->
            <div class="card">
              <div class="card-header"><h3 style="margin: 0; font-size: 14px">Resource Limits</h3></div>
              <div class="card-body">
                <table>
                  <tbody>
                    <tr>
                      <td style="font-weight: 600; width: 140px">API Keys</td>
                      <td>{{ formatLimit(currentPlan.max_api_keys) }}</td>
                    </tr>
                    <tr>
                      <td style="font-weight: 600">Domains</td>
                      <td>{{ formatLimit(currentPlan.max_domains) }}</td>
                    </tr>
                    <tr>
                      <td style="font-weight: 600">SMTP Servers</td>
                      <td>{{ formatLimit(currentPlan.max_smtp_servers) }}</td>
                    </tr>
                    <tr>
                      <td style="font-weight: 600">Workspaces</td>
                      <td>{{ formatLimit(currentPlan.max_workspaces) }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>

            <!-- Retention -->
            <div class="card">
              <div class="card-header"><h3 style="margin: 0; font-size: 14px">Data Retention</h3></div>
              <div class="card-body">
                <table>
                  <tbody>
                    <tr>
                      <td style="font-weight: 600; width: 140px">Email Logs</td>
                      <td>{{ currentPlan.email_log_retention_days === 0 ? 'Default' : currentPlan.email_log_retention_days + ' days' }}</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        </template>
        <div v-else class="card">
          <div class="empty-state">
            <h3>No plan assigned</h3>
            <p>This workspace is using the platform's global default settings. Contact your administrator to assign a plan.</p>
          </div>
        </div>
      </div>

      <!-- Settings Tab -->
      <div v-if="activeTab === 'settings' && isAdminOrOwner" class="card">
        <div class="card-header"><span>Workspace Settings</span></div>
        <div class="card-body">
          <div class="form-group">
            <label class="form-label">Name</label>
            <input v-model="editName" class="form-input" />
          </div>
          <div class="form-group">
            <label class="form-label">Description</label>
            <input v-model="editDescription" class="form-input" placeholder="Optional description" />
          </div>
          <div class="form-group">
            <label class="form-label">Slug</label>
            <input :value="ws?.slug" class="form-input" disabled />
            <small style="font-size: 12px; color: var(--text-muted); margin-top: 4px; display: block;">Slug cannot be changed after creation.</small>
          </div>
          <button class="btn btn-primary" :disabled="saving" @click="saveSettings">
            {{ saving ? 'Saving...' : 'Save Changes' }}
          </button>
        </div>

        <!-- Sending & Operational Settings -->
        <div class="card-body" style="border-top: 1px solid var(--border-primary);">
          <h4 style="font-size: 14px; font-weight: 600; margin-bottom: 8px;">Sending &amp; Operational Settings</h4>
          <p style="font-size: 13px; color: var(--text-secondary); margin-bottom: 16px;">
            Defaults applied to this workspace's emails, webhooks, and API keys.
          </p>
          <div v-if="wsSettingsLoading" class="loading-page" style="padding: 24px 0;"><div class="spinner"></div></div>
          <template v-else>
            <div class="form-group">
              <label class="form-label">Timezone</label>
              <select v-model="wsSettings.timezone" class="form-select">
                <option v-for="tz in timezones" :key="tz" :value="tz">{{ tz }}</option>
              </select>
            </div>
            <div class="form-group">
              <label class="form-label">Default Sender Name</label>
              <input v-model="wsSettings.default_sender_name" class="form-input" placeholder="Acme Inc." />
            </div>
            <div class="form-group">
              <label class="form-label">Default Sender Email</label>
              <input v-model="wsSettings.default_sender_email" type="email" class="form-input" placeholder="no-reply@example.com" />
            </div>
            <div class="form-group">
              <label class="form-label">Webhook Retry Count</label>
              <input v-model.number="wsSettings.webhook_retry_count" type="number" min="0" max="10" class="form-input" />
            </div>
            <div class="form-group">
              <label class="form-label">API Key Expiry (days)</label>
              <input v-model.number="wsSettings.api_key_expiry_days" type="number" min="0" class="form-input" />
              <small style="font-size: 12px; color: var(--text-muted); margin-top: 4px; display: block;">0 means keys never expire by default.</small>
            </div>
            <div class="form-group">
              <label style="display: flex; align-items: center; gap: 8px; cursor: pointer;">
                <input type="checkbox" v-model="wsSettings.bounce_auto_suppress" />
                <span>Auto-suppress on hard bounce</span>
              </label>
              <small style="font-size: 12px; color: var(--text-muted); margin-top: 4px; display: block;">Automatically add hard-bounced addresses to the suppression list.</small>
            </div>
            <button class="btn btn-primary" :disabled="wsSettingsSaving" @click="saveWorkspaceSettings">
              {{ wsSettingsSaving ? 'Saving...' : 'Save Settings' }}
            </button>
          </template>
        </div>

        <!-- Domain Security -->
        <div class="card-body" style="border-top: 1px solid var(--border-primary);">
          <h4 style="font-size: 14px; font-weight: 600; margin-bottom: 8px;">Domain Security</h4>
          <p style="font-size: 13px; color: var(--text-secondary); margin-bottom: 16px;">
            When strict domain mode is enabled, this workspace can only send from domains it has
            added and ownership-verified. Disable it to send from any sender domain.
          </p>
          <div v-if="wsSettingsLoading" class="loading-page" style="padding: 24px 0;"><div class="spinner"></div></div>
          <template v-else>
            <div class="form-group">
              <label style="display: flex; align-items: center; gap: 8px; cursor: pointer;">
                <input type="checkbox" v-model="wsSettings.require_verified_domain" />
                <span>Require verified sender domain (strict mode)</span>
              </label>
            </div>
            <button class="btn btn-primary" :disabled="wsSettingsSaving" @click="saveWorkspaceSettings">
              {{ wsSettingsSaving ? 'Saving...' : 'Save' }}
            </button>
          </template>
        </div>

        <div class="card-body" style="border-top: 1px solid var(--border-primary);">
          <h4 style="font-size: 14px; font-weight: 600; margin-bottom: 8px;">Data Export / Import</h4>
          <p style="font-size: 13px; color: var(--text-secondary); margin-bottom: 12px;">
            Export all workspace data (settings, templates, stylesheets, languages, contacts, contact lists, webhooks,
            SMTP servers, domains, subscribers, subscriber lists, and suppressions) as a JSON file.
          </p>
          <p style="font-size: 12px; color: var(--text-muted); margin-bottom: 16px;">
            SMTP server passwords are not included in exports. Imported SMTP servers will be disabled until reconfigured.
            Imported domains will require re-verification.
          </p>
          <div style="display: flex; gap: 8px; flex-wrap: wrap;">
            <button class="btn btn-primary" :disabled="exporting" @click="exportWorkspaceData">
              {{ exporting ? 'Exporting...' : 'Export Workspace Data' }}
            </button>
            <button class="btn btn-secondary" :disabled="importing" @click="triggerWorkspaceImport">
              {{ importing ? 'Importing...' : 'Import Data' }}
            </button>
            <input ref="importFileRef" type="file" accept=".json" style="display: none" @change="handleWorkspaceImportFile" />
          </div>
        </div>

        <div v-if="isOwner && !isPersonal" class="card-body" style="border-top: 1px solid var(--border-primary);">
          <h4 style="color: var(--danger-600); margin-bottom: 8px;">Danger Zone</h4>
          <p style="font-size: 13px; color: var(--text-muted); margin-bottom: 12px;">
            Deleting this workspace will permanently remove all its resources. This cannot be undone.
          </p>
          <button class="btn btn-danger" @click="deleteWorkspace">Delete Workspace</button>
        </div>
        <div v-else-if="isOwner && isPersonal" class="card-body" style="border-top: 1px solid var(--border-primary);">
          <p style="font-size: 13px; color: var(--text-muted);">
            This is your personal workspace and cannot be deleted. It is removed only if you delete your account.
          </p>
        </div>
      </div>

      <!-- Data Management (GDPR) Tab — admin/owner only -->
      <div v-if="activeTab === 'data' && isAdminOrOwner" class="card">
        <div class="card-header"><span>Data Management (GDPR)</span></div>
        <div class="card-body">
          <p style="font-size: 13px; color: var(--text-secondary); margin-bottom: 20px;">
            Manage this workspace's data for GDPR compliance. Delete specific contacts or purge old email logs.
          </p>

          <div>
            <h4 style="font-size: 14px; font-weight: 600; margin-bottom: 4px;">Delete Contact Data</h4>
            <p style="font-size: 13px; color: var(--text-secondary); margin-bottom: 12px;">
              Remove a specific contact by email (including suppression list and list memberships), or leave empty to delete all contacts in this workspace.
            </p>
            <div style="display: flex; gap: 8px; align-items: center;">
              <input v-model="gdprEmail" type="email" class="form-input" placeholder="email@example.com (or leave empty for all)" style="flex: 1" />
              <button class="btn btn-danger" :disabled="deletingContacts" @click="deleteWorkspaceContacts">
                {{ deletingContacts ? 'Deleting...' : 'Delete' }}
              </button>
            </div>
          </div>

          <div style="margin-top: 24px;">
            <h4 style="font-size: 14px; font-weight: 600; margin-bottom: 4px;">Delete Email Logs</h4>
            <p style="font-size: 13px; color: var(--text-secondary); margin-bottom: 12px;">
              Delete email logs and associated bounce records. Set days to 0 to delete all logs.
            </p>
            <div style="display: flex; gap: 8px; align-items: center;">
              <label class="form-label" style="margin: 0; white-space: nowrap">Older than</label>
              <input v-model.number="gdprDays" type="number" class="form-input" min="0" style="width: 100px" />
              <label class="form-label" style="margin: 0">days</label>
              <button class="btn btn-danger" :disabled="deletingEmails" @click="deleteWorkspaceEmailLogs">
                {{ deletingEmails ? 'Deleting...' : 'Delete Logs' }}
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- SSO Tab — owners only -->
      <div v-if="activeTab === 'sso' && isOwner" class="card">
        <div class="card-header">
          <span>Single Sign-On (SSO)</span>
          <button v-if="ssoConfig" class="btn btn-danger btn-sm" @click="removeSSO">Remove SSO</button>
        </div>
        <div class="card-body">
          <p style="font-size: 13px; color: var(--text-secondary); margin-bottom: 16px;">
            Configure SSO to allow workspace members to authenticate using an OAuth provider.
          </p>

          <div v-if="ssoLoading" class="loading-page" style="padding: 24px 0;"><div class="spinner"></div></div>
          <template v-else>
            <div v-if="ssoProviders.length === 0" class="empty-state" style="padding: 24px 0">
              <h3>No OAuth providers available</h3>
              <p>An administrator must configure OAuth providers before SSO can be enabled.</p>
            </div>

            <form v-else @submit.prevent="saveSSO" style="display: grid; gap: 16px; max-width: 480px;">
              <div class="form-group">
                <label class="form-label">OAuth Provider</label>
                <select v-model.number="ssoForm.provider_id" class="form-select" required>
                  <option :value="0" disabled>Select a provider</option>
                  <option v-for="p in ssoProviders" :key="p.slug" :value="p.id">{{ p.name }} ({{ p.type }})</option>
                </select>
              </div>

              <div class="form-group">
                <label class="form-label">Allowed Domains</label>
                <input v-model="ssoForm.allowed_domains" type="text" class="form-input" placeholder="example.com, company.org" />
                <small style="font-size: 12px; color: var(--text-muted); margin-top: 4px; display: block;">Comma-separated email domains. Leave empty to allow all domains.</small>
              </div>

              <div class="form-group">
                <label style="display: flex; align-items: center; gap: 8px; cursor: pointer;">
                  <input type="checkbox" v-model="ssoForm.enforce_sso" />
                  <span>Enforce SSO</span>
                </label>
                <small style="font-size: 12px; color: var(--text-muted); margin-top: 4px; display: block;">Require all workspace members to authenticate via SSO.</small>
              </div>

              <div class="form-group">
                <label style="display: flex; align-items: center; gap: 8px; cursor: pointer;">
                  <input type="checkbox" v-model="ssoForm.auto_provision" />
                  <span>Auto-provision users</span>
                </label>
                <small style="font-size: 12px; color: var(--text-muted); margin-top: 4px; display: block;">Automatically create accounts for new users who authenticate via SSO.</small>
              </div>

              <button type="submit" class="btn btn-primary" :disabled="ssoSaving" style="justify-self: start;">
                {{ ssoSaving ? 'Saving...' : (ssoConfig ? 'Update SSO' : 'Enable SSO') }}
              </button>
            </form>
          </template>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.tabs {
  display: flex;
  gap: 0;
  border-bottom: 1px solid var(--border-primary);
  margin-bottom: 20px;
}
.tab {
  padding: 10px 20px;
  font-size: 14px;
  font-weight: 500;
  color: var(--text-muted);
  background: none;
  border: none;
  border-bottom: 2px solid transparent;
  cursor: pointer;
  transition: all var(--transition);
}
.tab:hover { color: var(--text-primary); }
.tab.active {
  color: var(--primary-600);
  border-bottom-color: var(--primary-600);
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 20px;
  border-bottom: 1px solid var(--border-primary);
  font-weight: 600;
  font-size: 14px;
}

.card-body { padding: 20px; }
</style>
