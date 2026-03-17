<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { smtpApi } from '../../api/smtp'
import { useNotificationStore } from '../../stores/notification'
import { useConfirm } from '../../composables/useConfirm'
import type { SmtpServer, SmtpServerInput } from '../../api/types'
import { useModalSafeClose } from '../../composables/useModalSafeClose';

const route = useRoute()
const router = useRouter()
const notify = useNotificationStore()
const { confirm } = useConfirm()

const server = ref<SmtpServer | null>(null)
const loading = ref(true)

const showModal = ref(false)
const form = ref<SmtpServerInput>({
  host: '',
  port: 587,
  username: '',
  password: '',
  encryption: 'starttls',
  max_retries: 0,
  allowed_emails: [],
})
const allowedEmailsText = ref('')
const saving = ref(false)

async function fetchServer() {
  loading.value = true
  try {
    const id = Number(route.params.id)
    const res = await smtpApi.get(id)
    server.value = res.data.data
  } catch {
    notify.error('Failed to load SMTP server')
  } finally {
    loading.value = false
  }
}

function openEdit() {
  if (!server.value) return
  form.value = {
    host: server.value.host,
    port: server.value.port,
    username: server.value.username,
    password: '',
    encryption: server.value.encryption,
    max_retries: server.value.max_retries ?? 0,
    allowed_emails: server.value.allowed_emails ?? [],
  }
  allowedEmailsText.value = (server.value.allowed_emails ?? []).join(', ')
  showModal.value = true
}

async function save() {
  if (!server.value) return
  saving.value = true
  const data: SmtpServerInput = {
    ...form.value,
    allowed_emails: allowedEmailsText.value
      .split(',')
      .map(e => e.trim())
      .filter(e => e.length > 0),
  }
  try {
    await smtpApi.update(server.value.id, data)
    notify.success('SMTP server updated')
    showModal.value = false
    await fetchServer()
  } catch {
    notify.error('Failed to update SMTP server')
  } finally {
    saving.value = false
  }
}

async function toggleStatus() {
  if (!server.value) return
  try {
    const newStatus = server.value.status === 'disabled' ? 'enabled' : 'disabled'
    await smtpApi.update(server.value.id, { status: newStatus } as Partial<SmtpServerInput>)
    await fetchServer()
    notify.success(newStatus === 'enabled' ? 'SMTP server enabled' : 'SMTP server disabled')
  } catch {
    notify.error('Failed to update SMTP server')
  }
}

async function testServer() {
  if (!server.value) return
  try {
    const res = await smtpApi.test(server.value.id)
    if (res.data.data.success) {
      notify.success('Connection test successful')
    } else {
      notify.error('Connection test failed: ' + res.data.data.message)
    }
    await fetchServer()
  } catch {
    notify.error('Connection test failed')
  }
}

async function deleteServer() {
  if (!server.value) return
  const confirmed = await confirm({
    title: 'Delete SMTP Server',
    message: `Are you sure you want to delete "${server.value.host}"? Emails using this server will no longer be delivered.`,
    confirmText: 'Delete',
    variant: 'danger',
  })
  if (!confirmed) return
  try {
    await smtpApi.delete(server.value.id)
    notify.success('SMTP server deleted')
    router.push('/smtp-servers')
  } catch {
    notify.error('Failed to delete SMTP server')
  }
}

function formatDate(date: string | null) {
  if (!date) return '-'
  return new Date(date).toLocaleString()
}
const { watchClickStart, confirmClickEnd } = useModalSafeClose(() => {
  showModal.value = false;
});
onMounted(fetchServer)
</script>

<template>
  <div>
    <div class="page-header">
      <div>
        <h1>{{ server?.host ?? 'SMTP Server Details' }}</h1>
      </div>
      <div class="flex gap-2">
        <button class="btn btn-secondary" @click="router.push('/smtp-servers')">Back</button>
      </div>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <template v-else-if="server">
      <!-- Info card -->
      <div class="card" style="margin-bottom: 24px;">
        <div class="card-header">
          <h2>{{ server.host }}:{{ server.port }}</h2>
          <div class="flex gap-2">
            <span class="badge badge-success" v-if="server.status === 'enabled'">Enabled</span>
            <span class="badge badge-danger" v-else-if="server.status === 'invalid'">Invalid</span>
            <span class="badge badge-neutral" v-else>Disabled</span>
          </div>
        </div>
        <div class="card-body">
          <table>
            <tbody>
              <tr>
                <td style="font-weight: 600; width: 160px;">Host</td>
                <td>{{ server.host }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600;">Port</td>
                <td>{{ server.port }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600;">Encryption</td>
                <td>
                  <span class="badge badge-success" v-if="server.encryption === 'ssl'">SSL</span>
                  <span class="badge badge-info" v-else-if="server.encryption === 'starttls'">STARTTLS</span>
                  <span class="badge badge-neutral" v-else>None</span>
                </td>
              </tr>
              <tr>
                <td style="font-weight: 600;">Username</td>
                <td>{{ server.username || '-' }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600;">Max Retries</td>
                <td>{{ server.max_retries }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600;">Allowed Emails</td>
                <td>
                  <span v-if="server.allowed_emails && server.allowed_emails.length > 0">
                    <span
                      v-for="e in server.allowed_emails"
                      :key="e"
                      class="badge badge-neutral"
                      style="margin-right: 6px;"
                    >{{ e }}</span>
                  </span>
                  <span v-else class="text-muted">All emails</span>
                </td>
              </tr>
              <tr v-if="server.status === 'invalid' && server.validation_error">
                <td style="font-weight: 600;">Validation Error</td>
                <td style="color: var(--color-danger, #e53e3e);">{{ server.validation_error }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600;">Last Validated</td>
                <td>{{ formatDate(server.validated_at) }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600;">Created</td>
                <td>{{ formatDate(server.created_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- Actions -->
      <div class="card">
        <div class="card-header"><h2>Actions</h2></div>
        <div class="card-body">
          <div class="flex gap-2">
            <button class="btn btn-primary" @click="openEdit">Edit</button>
            <button class="btn btn-secondary" @click="testServer">Test Connection</button>
            <button
              class="btn btn-secondary"
              @click="toggleStatus"
            >{{ server.status !== 'disabled' ? 'Disable' : 'Enable' }}</button>
            <button class="btn btn-danger" @click="deleteServer">Delete</button>
          </div>
        </div>
      </div>
    </template>

    <div v-else class="empty-state">
      <h3>SMTP server not found</h3>
    </div>

    <!-- Edit Modal -->
    <div v-if="showModal" class="modal-overlay" @mousedown="watchClickStart" @mouseup="confirmClickEnd">
      <div class="modal">
        <div class="modal-header" @mousedown.stop @mouseup.stop>
          <h3>Edit SMTP Server</h3>
        </div>
        <form @submit.prevent="save">
          <div class="modal-body">
            <div class="form-group">
              <label class="form-label">Host</label>
              <input v-model="form.host" type="text" class="form-input" placeholder="smtp.example.com" required />
            </div>
            <div class="form-group">
              <label class="form-label">Port</label>
              <input v-model.number="form.port" type="number" class="form-input" placeholder="587" required />
            </div>
            <div class="form-group">
              <label class="form-label">Username</label>
              <input v-model="form.username" type="text" class="form-input" required />
            </div>
            <div class="form-group">
              <label class="form-label">Password</label>
              <input v-model="form.password" type="password" class="form-input" placeholder="Leave blank to keep current" />
            </div>
            <div class="form-group">
              <label class="form-label">Encryption</label>
              <select v-model="form.encryption" class="form-select">
                <option value="none">None</option>
                <option value="starttls">STARTTLS (port 587)</option>
                <option value="ssl">SSL/TLS (port 465)</option>
              </select>
            </div>
            <div class="form-group">
              <label class="form-label">Max Retries</label>
              <input v-model.number="form.max_retries" type="number" class="form-input" min="0" max="10" />
              <span class="form-hint">Number of times to retry failed emails. Set to 0 to disable retries.</span>
            </div>
            <div class="form-group">
              <label class="form-label">Allowed Emails</label>
              <input v-model="allowedEmailsText" type="text" class="form-input" placeholder="user@example.com, admin@example.com" />
              <span class="form-hint">Comma-separated. Leave empty to allow all.</span>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" @click="showModal = false">Cancel</button>
            <button type="submit" class="btn btn-primary" :disabled="saving">
              {{ saving ? 'Saving...' : 'Update' }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>
