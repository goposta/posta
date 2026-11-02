<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ENTERPRISE_URL, SALES_EMAIL, licenseApi, type LicenseView } from '../../api/license'
import { useNotificationStore } from '../../stores/notification'
import { useLicenseStore } from '../../stores/license'
import { useConfirm } from '../../composables/useConfirm'
import { apiMessage } from '../../composables/apiError'

// The Posta Enterprise license: install, inspect, remove. A license is
// verified offline and takes effect at once, with no restart.

const notify = useNotificationStore()
const licenseStore = useLicenseStore()
const { confirm } = useConfirm()

const loading = ref(true)
const view = ref<LicenseView | null>(null)
const token = ref('')
const saving = ref(false)

const STATE_LABEL: Record<string, { label: string; badge: string }> = {
  none: { label: 'Community', badge: 'badge-neutral' },
  valid: { label: 'Active', badge: 'badge-success' },
  grace: { label: 'Expired, in grace period', badge: 'badge-warning' },
  degraded: { label: 'Expired', badge: 'badge-danger' },
  binding_mismatch: { label: 'Bound to another installation', badge: 'badge-danger' },
  unknown_signing_key: { label: 'Signed by an unknown key', badge: 'badge-danger' },
}

const WARNING_TEXT: Record<string, string> = {
  nearing_expiry: 'The license expires soon. Renew it to keep Enterprise settings editable.',
  license_grace:
    'The license has expired and is in its grace period. Everything still works; renew before the grace period ends.',
  license_expired:
    'The license has expired. Enterprise features keep running, but their settings are read-only until you install a renewed license.',
  binding_mismatch:
    'This license is bound to a different installation, so Enterprise features are off. Install a license issued for this Install ID.',
  unknown_signing_key:
    'This license is signed by a key this version of Posta does not know. Upgrade Posta, or ask for a reissued license.',
}

const state = computed(() => STATE_LABEL[view.value?.state ?? 'none'] ?? STATE_LABEL.none)
const hasLicense = computed(() => !!view.value && view.value.state !== 'none')
const communityBuild = computed(() => view.value?.build === 'community')

function formatDate(iso?: string) {
  return iso ? new Date(iso).toLocaleDateString(undefined, { year: 'numeric', month: 'long', day: 'numeric' }) : '—'
}

function apply(next: LicenseView) {
  view.value = next
  licenseStore.set({
    edition: next.edition,
    state: next.state,
    licensed: ['valid', 'grace', 'degraded'].includes(next.state),
    mutable: ['valid', 'grace'].includes(next.state),
  })
}

async function load() {
  loading.value = true
  try {
    apply((await licenseApi.get()).data.data)
  } catch (e: any) {
    notify.error(apiMessage(e, 'Failed to load the license'))
  } finally {
    loading.value = false
  }
}

async function install() {
  const value = token.value.trim()
  if (!value) return
  saving.value = true
  try {
    apply((await licenseApi.install(value)).data.data)
    token.value = ''
    notify.success('License installed. Enterprise features are on.')
  } catch (e: any) {
    notify.error(apiMessage(e, 'Failed to install the license'))
  } finally {
    saving.value = false
  }
}

async function remove() {
  const ok = await confirm({
    title: 'Remove the license?',
    message:
      'Enterprise features stop at once: routing rules stop applying and teams can no longer be used. Their settings and data are kept, and come back when a license is installed.',
    confirmText: 'Remove license',
    variant: 'danger',
  })
  if (!ok) return
  saving.value = true
  try {
    apply((await licenseApi.remove()).data.data)
    notify.success('License removed.')
  } catch (e: any) {
    notify.error(apiMessage(e, 'Failed to remove the license'))
  } finally {
    saving.value = false
  }
}

async function copyInstallID() {
  if (!view.value?.instance_install_id) return
  try {
    await navigator.clipboard.writeText(view.value.instance_install_id)
    notify.success('Install ID copied.')
  } catch {
    notify.error('Could not copy; select the Install ID and copy it by hand.')
  }
}

onMounted(load)
</script>

<template>
  <div>
    <div class="page-header">
      <div>
        <h1>License</h1>
        <p class="page-description">
          Posta Enterprise adds routing rules and teams, branching forms, encryption of personal data, LDAP /
          Active Directory sign-in and several OAuth providers, on top of everything in Posta Community.
        </p>
      </div>
    </div>

    <div v-if="loading" class="loading-page"><div class="spinner"></div></div>

    <template v-else-if="view">
      <div
        v-for="w in view.warnings"
        :key="w"
        class="app-banner license-warning"
        :class="w === 'nearing_expiry' || w === 'license_grace' ? 'app-banner--warning' : 'app-banner--danger'"
        role="status"
      >
        <div class="app-banner-content">
          <p class="app-banner-text">{{ WARNING_TEXT[w] || w }}</p>
        </div>
      </div>

      <div class="license-grid">
        <div class="card">
          <div class="card-header"><h2>Edition</h2></div>
          <div class="card-body">
            <dl class="license-facts">
              <dt>Status</dt>
              <dd>
                <span class="badge" :class="state.badge">{{ state.label }}</span>
                <span v-if="view.trial" class="badge badge-info">Trial</span>
              </dd>
              <template v-if="hasLicense">
                <dt>Customer</dt>
                <dd>{{ view.customer || '—' }}</dd>
                <dt>License ID</dt>
                <dd><code>{{ view.license_id || '—' }}</code></dd>
                <dt>Expires</dt>
                <dd>{{ formatDate(view.not_after) }}</dd>
                <dt>Grace period ends</dt>
                <dd>{{ formatDate(view.grace_ends) }}</dd>
                <template v-if="view.install_id || view.url">
                  <dt>Bound to</dt>
                  <dd>
                    <span v-if="view.install_id"><code>{{ view.install_id }}</code></span>
                    <span v-if="view.install_id && view.url"> · </span>
                    <span v-if="view.url">{{ view.url }}</span>
                  </dd>
                </template>
              </template>
            </dl>
            <div v-if="hasLicense" class="license-actions">
              <button class="btn btn-secondary btn-sm" :disabled="saving" @click="remove">Remove license</button>
            </div>
          </div>
        </div>

        <div class="card">
          <div class="card-header"><h2>Install ID</h2></div>
          <div class="card-body">
            <p class="muted license-help">
              Give this ID to <a :href="`mailto:${SALES_EMAIL}`">{{ SALES_EMAIL }}</a> when buying a license, so it
              is issued for this instance.
            </p>
            <div class="install-id">
              <code>{{ view.instance_install_id || '—' }}</code>
              <button class="btn btn-ghost btn-sm" :disabled="!view.instance_install_id" @click="copyInstallID">
                Copy
              </button>
            </div>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="card-header"><h2>{{ hasLicense ? 'Replace the license' : 'Install a license' }}</h2></div>
        <div class="card-body">
          <p v-if="communityBuild" class="muted license-help">
            This Posta binary was built without Enterprise support, so it cannot install a license. Use the
            published image or binaries, which run as Posta Community until a license is installed.
          </p>
          <template v-else>
            <p class="muted license-help">
              Paste the license you received. It is checked here, without contacting any server, and takes
              effect at once. You can also set <code>POSTA_LICENSE</code> or <code>POSTA_LICENSE_FILE</code>.
            </p>
            <textarea
              v-model="token"
              class="form-input license-token"
              rows="4"
              spellcheck="false"
              autocomplete="off"
              placeholder="posta-v1.…"
              aria-label="License"
            ></textarea>
            <div class="license-actions">
              <button class="btn btn-primary" :disabled="saving || !token.trim()" @click="install">
                {{ saving ? 'Installing…' : 'Install license' }}
              </button>
              <a :href="ENTERPRISE_URL" target="_blank" rel="noopener noreferrer">About Posta Enterprise</a>
            </div>
          </template>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.license-warning {
  margin-bottom: 16px;
}

.license-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  gap: 16px;
  margin-bottom: 16px;
}

.license-facts {
  display: grid;
  grid-template-columns: max-content 1fr;
  gap: 8px 16px;
  margin: 0;
  font-size: 14px;
}

.license-facts dt {
  color: var(--text-tertiary);
}

.license-facts dd {
  margin: 0;
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
  min-width: 0;
  overflow-wrap: anywhere;
}

.license-help {
  margin: 0 0 12px;
  font-size: 13px;
}

.install-id {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.install-id code {
  overflow-wrap: anywhere;
  font-size: 13px;
}

.license-token {
  width: 100%;
  font-family: var(--font-mono, ui-monospace, monospace);
  font-size: 12px;
  resize: vertical;
}

.license-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 12px;
  flex-wrap: wrap;
}
</style>
