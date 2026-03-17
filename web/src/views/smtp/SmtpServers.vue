<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { smtpApi } from '../../api/smtp'
import type { SmtpServer, SmtpServerInput, Pageable } from '../../api/types'
import { useNotificationStore } from '../../stores/notification'
import { useConfirm } from '../../composables/useConfirm'
import { useModalSafeClose } from '../../composables/useModalSafeClose';

const router = useRouter()
const notify = useNotificationStore()
const { confirm } = useConfirm()

const servers = ref<SmtpServer[]>([])
const pageable = ref<Pageable | null>(null)
const loading = ref(true)
const currentPage = ref(0)

const showModal = ref(false)
const editing = ref<SmtpServer | null>(null)
const form = ref<SmtpServerInput>({
  host: '',
  port: 587,
  username: '',
  password: '',
  encryption: 'starttls',
  allowed_emails: [],
})
const allowedEmailsText = ref('')
const saving = ref(false)

async function fetchServers() {
  loading.value = true
  try {
    const res = await smtpApi.list(currentPage.value)
    servers.value = res.data.data
    pageable.value = res.data.pageable
  } catch {
    notify.error('Failed to load SMTP servers')
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editing.value = null
  form.value = { host: '', port: 587, username: '', password: '', encryption: 'starttls', max_retries: 0, allowed_emails: [] }
  allowedEmailsText.value = ''
  showModal.value = true
}

function openEdit(server: SmtpServer) {
  editing.value = server
  form.value = {
    host: server.host,
    port: server.port,
    username: server.username,
    password: '',
    encryption: server.encryption,
    max_retries: server.max_retries || 0,
    allowed_emails: server.allowed_emails || [],
  }
  allowedEmailsText.value = (server.allowed_emails || []).join(', ')
  showModal.value = true
}

async function save() {
  saving.value = true
  const data: SmtpServerInput = {
    ...form.value,
    allowed_emails: allowedEmailsText.value
      .split(',')
      .map(e => e.trim())
      .filter(e => e.length > 0),
  }
  try {
    if (editing.value) {
      await smtpApi.update(editing.value.id, data)
      notify.success('SMTP server updated')
    } else {
      await smtpApi.create(data)
      notify.success('SMTP server created')
    }
    showModal.value = false
    await fetchServers()
  } catch {
    notify.error('Failed to save SMTP server')
  } finally {
    saving.value = false
  }
}

async function deleteServer(server: SmtpServer) {
  const confirmed = await confirm({
    title: 'Delete SMTP Server',
    message: `Are you sure you want to delete "${server.host}"? Emails using this server will no longer be delivered.`,
    confirmText: 'Delete',
    variant: 'danger',
  })
  if (!confirmed) return
  try {
    await smtpApi.delete(server.id)
    notify.success('SMTP server deleted')
    await fetchServers()
  } catch {
    notify.error('Failed to delete SMTP server')
  }
}

function prevPage() {
  if (currentPage.value > 0) {
    currentPage.value--
    fetchServers()
  }
}

function nextPage() {
  if (pageable.value && currentPage.value < pageable.value.total_pages - 1) {
    currentPage.value++
    fetchServers()
  }
}
const { watchClickStart, confirmClickEnd } = useModalSafeClose(() => {
  showModal.value = false;
});
onMounted(fetchServers)
</script>

<template>
  <div>
    <div class="page-header">
      <h1>SMTP Servers</h1>
      <button class="btn btn-primary" @click="openCreate">Add Server</button>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <template v-else>
      <div class="card">
        <div class="table-wrapper" v-if="servers.length > 0">
          <table>
            <thead>
              <tr>
                <th>Host</th>
                <th>Port</th>
                <th>Username</th>
                <th>Encryption</th>
                <th>Status</th>
                <th>Max Retries</th>
                <th>Allowed Emails</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="server in servers" :key="server.id">
                <td>{{ server.host }}</td>
                <td>{{ server.port }}</td>
                <td>{{ server.username }}</td>
                <td>
                  <span class="badge badge-success" v-if="server.encryption === 'ssl'">SSL</span>
                  <span class="badge badge-info" v-else-if="server.encryption === 'starttls'">STARTTLS</span>
                  <span class="badge badge-neutral" v-else>None</span>
                </td>
                <td>
                  <span v-if="server.status === 'enabled'" class="badge badge-success">Enabled</span>
                  <span v-else-if="server.status === 'invalid'" class="badge badge-danger">Invalid</span>
                  <span v-else class="badge badge-neutral">Disabled</span>
                </td>
                <td>{{ server.max_retries }}</td>
                <td>
                  <span v-if="server.allowed_emails && server.allowed_emails.length > 0">
                    {{ server.allowed_emails.join(', ') }}
                  </span>
                  <span v-else class="text-muted">All</span>
                </td>
                <td>
                  <div class="flex gap-2">
                    <button class="btn btn-secondary btn-sm" @click="router.push(`/smtp-servers/${server.id}`)">View</button>
                    <button class="btn btn-secondary btn-sm" @click="openEdit(server)">Edit</button>
                    <button class="btn btn-danger btn-sm" @click="deleteServer(server)">Delete</button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div v-else class="empty-state">
          <h3>No SMTP servers</h3>
          <p>Add an SMTP server to start sending emails.</p>
        </div>

        <div v-if="pageable && !pageable.empty" class="pagination">
          <span class="pagination-info">
            Page {{ pageable.current_page + 1 }} of {{ pageable.total_pages }}
            ({{ pageable.total_elements }} total)
          </span>
          <div class="pagination-buttons">
            <button class="btn btn-secondary btn-sm" :disabled="currentPage === 0" @click="prevPage">Previous</button>
            <button class="btn btn-secondary btn-sm" :disabled="currentPage >= pageable.total_pages - 1" @click="nextPage">Next</button>
          </div>
        </div>
      </div>
    </template>

    <!-- Create/Edit Modal -->
    <div v-if="showModal" class="modal-overlay" @mousedown="watchClickStart" @mouseup="confirmClickEnd">
      <div class="modal" @mousedown.stop @mouseup.stop>
        <div class="modal-header">
          <h3>{{ editing ? 'Edit SMTP Server' : 'Add SMTP Server' }}</h3>
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
              <input v-model="form.password" type="password" class="form-input" :required="!editing" />
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
              <input v-model.number="form.max_retries" type="number" class="form-input" min="0" max="10" placeholder="0" />
              <span class="form-hint">Number of times to retry failed emails. Set to 0 to disable retries.</span>
            </div>
            <div class="form-group">
              <label class="form-label">Allowed Emails</label>
              <input v-model="allowedEmailsText" type="text" class="form-input" placeholder="user@example.com, admin@example.com" />
              <span class="form-hint">Comma-separated list of allowed sender emails. Leave empty to allow all.</span>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" @click="showModal = false">Cancel</button>
            <button type="submit" class="btn btn-primary" :disabled="saving">
              {{ saving ? 'Saving...' : (editing ? 'Update' : 'Create') }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>
