<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { serversApi } from '../../api/servers'
import type { SharedServer, SharedServerInput, Pageable } from '../../api/types'
import { useNotificationStore } from '../../stores/notification'
import { useConfirm } from '../../composables/useConfirm'
import { useModalSafeClose } from '../../composables/useModalSafeClose';
import { usePagination } from '@/composables/usePagination'
import Pagination from '@/components/Pagination.vue'



const router = useRouter()
const notify = useNotificationStore()
const { confirm } = useConfirm()

const servers = ref<SharedServer[]>([])
const loading = ref(true)
const currentPage = ref(0)

const showModal = ref(false)
const editing = ref<SharedServer | null>(null)
  const form = ref<SharedServerInput>({
  name: '',
  host: '',
  port: 587,
  username: '',
  password: '',
  encryption: 'starttls',
  max_retries: 0,
  allowed_domains: [],
  security_mode: 'permissive',
})
const allowedDomainsText = ref('')
const saving = ref(false)
const { pageable, goToPage } = usePagination(async (page) => {
  loading.value = true
  try {
    const res = await serversApi.list(currentPage.value)
    servers.value = res.data.data
    pageable.value = res.data.pageable
  } catch (e) {
    console.error('Failed to load shared servers', e)
  } finally {
    loading.value = false
  }
})

function openCreate() {
  editing.value = null
  form.value = { name: '', host: '', port: 587, username: '', password: '', encryption: 'starttls', max_retries: 0, allowed_domains: [], security_mode: 'permissive' }
  allowedDomainsText.value = ''
  showModal.value = true
}

function openEdit(server: SharedServer) {
  editing.value = server
  form.value = {
    name: server.name,
    host: server.host,
    port: server.port,
    username: server.username,
    password: '',
    encryption: server.encryption,
    max_retries: server.max_retries ?? 0,
    allowed_domains: server.allowed_domains ?? [],
    security_mode: server.security_mode ?? 'permissive',
  }
  allowedDomainsText.value = (server.allowed_domains ?? []).join(', ')
  showModal.value = true
}

async function save() {
  saving.value = true
  const data: SharedServerInput = {
    ...form.value,
    allowed_domains: allowedDomainsText.value
      .split(',')
      .map(d => d.trim().toLowerCase())
      .filter(d => d.length > 0),
  }
  try {
    if (editing.value) {
      await serversApi.update(editing.value.id, data)
      notify.success('Server updated')
    } else {
      await serversApi.create(data)
      notify.success('Server created')
    }
    showModal.value = false
    await goToPage(pageable.value.current_page)
  } catch {
    notify.error('Failed to save server')
  } finally {
    saving.value = false
  }
}


async function deleteServer(server: SharedServer) {
  const confirmed = await confirm({
    title: 'Delete Shared Server',
    message: `Are you sure you want to delete "${server.name}"? Any accounts relying on this server will stop receiving email delivery.`,
    confirmText: 'Delete',
    variant: 'danger',
  })
  if (!confirmed) return
  try {
    await serversApi.delete(server.id)
    notify.success('Server deleted')
    await goToPage(pageable.value.current_page)
  } catch {
    notify.error('Failed to delete server')
  }
}

const { watchClickStart, confirmClickEnd } = useModalSafeClose(() => {
  showModal.value = false;
});

</script>

<template>
  <div>
    <div class="page-header">
      <div>
        <h1>Shared SMTP Servers</h1>
        <p class="page-description">Manage the shared SMTP pool.</p>
        <p class="page-description">Enabled servers are available to all accounts whose sender domain matches the allowed domains list.</p>
      </div>
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
                <th>Name</th>
                <th>Host</th>
                <th>Security</th>
                <th>Status</th>
                <th>Allowed Domains</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="server in servers" :key="server.id">
                <td><strong>{{ server.name }}</strong></td>
                <td>{{ server.host }}</td>
                <td>
                  <span class="badge badge-warning" v-if="server.security_mode === 'strict'">Strict</span>
                  <span class="badge badge-neutral" v-else>Permissive</span>
                </td>
                <td>
                  <span class="badge badge-success" v-if="server.status === 'enabled'">Enabled</span>
                  <span class="badge badge-danger" v-else-if="server.status === 'invalid'" :title="server.validation_error">Invalid</span>
                  <span class="badge badge-neutral" v-else>Disabled</span>
                  <div v-if="server.status === 'invalid' && server.validation_error" class="text-muted" style="font-size: 12px; margin-top: 2px;">
                    {{ server.validation_error }}
                  </div>
                </td>
                <td>
                  <span v-if="server.allowed_domains && server.allowed_domains.length > 0">
                    {{ server.allowed_domains.join(', ') }}
                  </span>
                  <span v-else class="text-muted">All</span>
                </td>
                <td>
                  <div class="flex gap-2">
                    <button class="btn btn-secondary btn-sm" @click="router.push(`/admin/servers/${server.id}`)">View</button>
                    <button class="btn btn-secondary btn-sm" @click="openEdit(server)">Edit</button>
                    <button class="btn btn-danger btn-sm" @click="deleteServer(server)">Delete</button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div v-else class="empty-state">
          <h3>No shared servers</h3>
          <p>Add a shared SMTP server to give accounts without personal SMTP configuration a delivery path.</p>
        </div>

<Pagination :pageable="pageable" @page="goToPage" />
       
      </div>
    </template>

    <!-- Create/Edit Modal -->
    <div v-if="showModal" class="modal-overlay" @mousedown="watchClickStart" @mouseup="confirmClickEnd">
      <div class="modal" @mousedown.stop @mouseup.stop>
        <div class="modal-header">
          <h3>{{ editing ? 'Edit Shared Server' : 'Add Shared Server' }}</h3>
        </div>
        <form @submit.prevent="save">
          <div class="modal-body">
            <div class="form-group">
              <label class="form-label">Name</label>
              <input v-model="form.name" type="text" class="form-input" placeholder="Primary Relay" required />
              <span class="form-hint">A human-readable label for this server.</span>
            </div>
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
              <input v-model="form.username" type="text" class="form-input" />
            </div>
            <div class="form-group">
              <label class="form-label">Password</label>
              <input v-model="form.password" type="password" class="form-input" :placeholder="editing ? 'Leave blank to keep current' : ''" :required="!editing" />
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
              <span class="form-hint">Number of times to retry a failed send. Set to 0 to disable retries.</span>
            </div>
            <div class="form-group">
              <label class="form-label">Allowed Domains</label>
              <input v-model="allowedDomainsText" type="text" class="form-input" placeholder="example.com, acme.org" />
              <span class="form-hint">Comma-separated list of sender domains this server accepts. Leave empty to allow all domains.</span>
            </div>
            <div class="form-group">
              <label class="form-label">Security Mode</label>
              <select v-model="form.security_mode" class="form-select">
                <option value="permissive">Permissive — allow any user whose sender domain matches</option>
                <option value="strict">Strict — require verified domain ownership</option>
              </select>
              <span class="form-hint">In strict mode, users must verify ownership of their sender domain before this server is available to them.</span>
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
