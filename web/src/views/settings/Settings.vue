<script setup lang="ts">
import { ref, onMounted, watch, nextTick } from 'vue'
import { settingsApi } from '../../api/settings'
import { useThemeStore, type ThemeMode } from '../../stores/theme'
import { useNotificationStore } from '../../stores/notification'
import type { UserSettings } from '../../api/types'

const theme = useThemeStore()
const notify = useNotificationStore()
const loading = ref(true)

// Auto-save state. saveStatus drives the header indicator; ready gates the
// watcher so the initial load doesn't trigger a save.
type SaveStatus = 'idle' | 'unsaved' | 'saving' | 'saved' | 'error'
const saveStatus = ref<SaveStatus>('idle')
const ready = ref(false)
const dirty = ref(false)
const saving = ref(false)
const AUTOSAVE_DEBOUNCE_MS = 800
const SAVED_INDICATOR_MS = 2000
let debounceTimer: ReturnType<typeof setTimeout> | null = null
let savedClearTimer: ReturnType<typeof setTimeout> | null = null

// User settings are personal notification preferences only. Operational settings
// (sending defaults, webhooks, API-key expiry, domain security, data) live on the
// workspace — see Workspace Settings (workspace-only migration §2c/§5).
const form = ref<Partial<UserSettings>>({
  email_notifications: true,
  notification_email: '',
  daily_report: false,
})

// Theme
const themeModes: { value: ThemeMode; label: string; icon: string }[] = [
  { value: 'light', label: 'Light', icon: 'sun' },
  { value: 'dark', label: 'Dark', icon: 'moon' },
  { value: 'system', label: 'System', icon: 'monitor' },
]

onMounted(async () => {
  try {
    const res = await settingsApi.getUserSettings()
    const u = res.data.data
    form.value = {
      email_notifications: u.email_notifications,
      notification_email: u.notification_email || '',
      daily_report: u.daily_report,
    }
  } catch {
    notify.error('Failed to load settings')
  } finally {
    loading.value = false
    // Wait one tick so the watcher doesn't fire on the load-time assignment.
    await nextTick()
    ready.value = true
  }
})

watch(
  form,
  () => {
    if (!ready.value) return
    dirty.value = true
    saveStatus.value = 'unsaved'
    if (debounceTimer) clearTimeout(debounceTimer)
    debounceTimer = setTimeout(autoSave, AUTOSAVE_DEBOUNCE_MS)
  },
  { deep: true },
)

async function autoSave() {
  if (saving.value) return
  if (!dirty.value) return

  saving.value = true
  saveStatus.value = 'saving'
  dirty.value = false
  if (savedClearTimer) {
    clearTimeout(savedClearTimer)
    savedClearTimer = null
  }
  try {
    await settingsApi.updateUserSettings(form.value)
    saveStatus.value = 'saved'
    savedClearTimer = setTimeout(() => {
      if (saveStatus.value === 'saved') saveStatus.value = 'idle'
    }, SAVED_INDICATOR_MS)
  } catch {
    saveStatus.value = 'error'
    dirty.value = true
    notify.error('Failed to save settings')
  } finally {
    saving.value = false
    if (dirty.value) {
      if (debounceTimer) clearTimeout(debounceTimer)
      debounceTimer = setTimeout(autoSave, AUTOSAVE_DEBOUNCE_MS)
    }
  }
}
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Settings</h1>
      <span
        v-if="saveStatus !== 'idle'"
        class="save-status"
        :class="saveStatus"
        role="status"
        aria-live="polite"
      >
        <span class="save-status-dot" aria-hidden="true"></span>
        <template v-if="saveStatus === 'saving'">Saving…</template>
        <template v-else-if="saveStatus === 'saved'">Saved</template>
        <template v-else-if="saveStatus === 'unsaved'">Unsaved changes</template>
        <template v-else-if="saveStatus === 'error'">Save failed</template>
      </span>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <div v-else class="settings-grid">
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

.save-status {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  font-weight: 500;
  color: var(--text-muted);
  padding: 4px 10px;
  border-radius: 999px;
  background: var(--bg-secondary);
  transition: opacity 0.2s, background 0.2s, color 0.2s;
}

.save-status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}

.save-status.saving {
  color: var(--primary-600);
}

.save-status.saving .save-status-dot {
  animation: save-status-pulse 1s ease-in-out infinite;
}

.save-status.saved {
  color: var(--success-600, #16a34a);
}

.save-status.unsaved {
  color: var(--text-secondary);
}

.save-status.error {
  color: var(--danger-600);
  background: var(--danger-50, rgba(220, 38, 38, 0.08));
}

@keyframes save-status-pulse {
  0%, 100% { opacity: 0.4; }
  50% { opacity: 1; }
}
</style>
