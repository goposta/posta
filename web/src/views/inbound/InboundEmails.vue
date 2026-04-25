<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { inboundApi } from '../../api/inbound'
import type { InboundEmail } from '../../api/types'
import Pagination from '../../components/Pagination.vue'
import { usePagination } from '../../composables/usePagination'

const router = useRouter()
const loading = ref(true)
const emails = ref<InboundEmail[]>([])
const featureDisabled = ref(false)

const status = ref('')
const source = ref('')
const sender = ref('')
const q = ref('')

const { pageable, goToPage } = usePagination(async (page) => {
  loading.value = true
  try {
    const res = await inboundApi.list({
      page,
      status: status.value || undefined,
      source: source.value || undefined,
      sender: sender.value || undefined,
      q: q.value || undefined,
    })
    emails.value = res.data.data
    pageable.value = res.data.pageable
    featureDisabled.value = false
  } catch (e: any) {
    if (e?.response?.status === 404) {
      featureDisabled.value = true
    } else {
      console.error('Failed to load inbound emails', e)
    }
  } finally {
    loading.value = false
  }
})

function applyFilters() {
  goToPage(0)
}

function resetFilters() {
  status.value = ''
  source.value = ''
  sender.value = ''
  q.value = ''
  goToPage(0)
}

// Live updates via SSE — the backend filters to the authenticated user.
let sse: EventSource | null = null
onMounted(() => {
  try {
    sse = new EventSource(inboundApi.streamUrl(), { withCredentials: true })
    sse.addEventListener('email.inbound.received', () => {
      goToPage(pageable.value?.current_page ?? 0)
    })
    sse.onerror = () => { /* reconnects are handled by the browser */ }
  } catch { /* SSE not supported */ }
})
onBeforeUnmount(() => {
  if (sse) sse.close()
})

function statusBadgeClass(status: string) {
  switch (status) {
    case 'forwarded': return 'badge badge-success'
    case 'failed': return 'badge badge-danger'
    case 'received': return 'badge badge-info'
    case 'rejected': return 'badge badge-warning'
    default: return 'badge'
  }
}

function sourceBadgeClass(src: string) {
  return src === 'smtp' ? 'badge badge-secondary' : 'badge badge-info'
}

function formatDate(date: string | null | undefined) {
  if (!date) return '-'
  return new Date(date).toLocaleString()
}

function formatBytes(n: number) {
  if (!n) return '0 B'
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / (1024 * 1024)).toFixed(2)} MB`
}
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Inbound Emails</h1>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <div v-else-if="featureDisabled" class="card">
      <div class="empty-state">
        <h3>Inbound email is disabled</h3>
        <p>
          Ask your administrator to enable inbound processing by setting
          <code>POSTA_INBOUND_ENABLED=true</code> and configuring the SMTP receiver.
        </p>
      </div>
    </div>

    <template v-else>
      <div class="card" style="margin-bottom: 16px">
        <div class="card-body" style="display: flex; gap: 8px; flex-wrap: wrap; align-items: flex-end">
          <div>
            <label style="display: block; font-size: 12px; font-weight: 600">Status</label>
            <select v-model="status" @change="applyFilters">
              <option value="">Any</option>
              <option value="received">Received</option>
              <option value="forwarded">Forwarded</option>
              <option value="failed">Failed</option>
              <option value="rejected">Rejected</option>
            </select>
          </div>
          <div>
            <label style="display: block; font-size: 12px; font-weight: 600">Source</label>
            <select v-model="source" @change="applyFilters">
              <option value="">Any</option>
              <option value="smtp">SMTP</option>
              <option value="webhook">Webhook</option>
            </select>
          </div>
          <div>
            <label style="display: block; font-size: 12px; font-weight: 600">Sender</label>
            <input v-model="sender" placeholder="alice@example.com" @keyup.enter="applyFilters" />
          </div>
          <div style="flex: 1; min-width: 200px">
            <label style="display: block; font-size: 12px; font-weight: 600">Subject</label>
            <input v-model="q" placeholder="Search subject..." @keyup.enter="applyFilters" />
          </div>
          <button class="btn btn-primary" @click="applyFilters">Apply</button>
          <button class="btn btn-secondary" @click="resetFilters">Reset</button>
        </div>
      </div>

      <div class="card">
        <div v-if="emails.length === 0" class="empty-state">
          <h3>No inbound emails</h3>
          <p>Emails received via the built-in SMTP server or provider webhook will appear here.</p>
        </div>
        <template v-else>
          <div class="table-wrapper">
            <table>
              <thead>
                <tr>
                  <th>Subject</th>
                  <th>From</th>
                  <th>Recipients</th>
                  <th>Source</th>
                  <th>Status</th>
                  <th>Size</th>
                  <th>Received At</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="email in emails"
                  :key="email.uuid"
                  style="cursor: pointer"
                  @click="router.push(`/inbound-emails/${email.uuid}`)"
                >
                  <td>{{ email.subject || '(no subject)' }}</td>
                  <td>{{ email.sender }}</td>
                  <td>{{ email.recipients.join(', ') }}</td>
                  <td><span :class="sourceBadgeClass(email.source)">{{ email.source }}</span></td>
                  <td><span :class="statusBadgeClass(email.status)">{{ email.status }}</span></td>
                  <td>{{ formatBytes(email.size) }}</td>
                  <td>{{ formatDate(email.received_at) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <Pagination :pageable="pageable" @page="goToPage" />
        </template>
      </div>
    </template>
  </div>
</template>
