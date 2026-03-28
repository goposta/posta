<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { emailsApi } from '../../api/emails'
import { useNotificationStore } from '../../stores/notification'
import type { Email } from '../../api/types'
import Pagination from '../../components/Pagination.vue'
import { usePagination } from '../../composables/usePagination'

const router = useRouter()
const notify = useNotificationStore()
const loading = ref(true)
const emails = ref<Email[]>([])
const retryingId = ref<string | null>(null)

const { pageable, goToPage } = usePagination(async (page) => {
  loading.value = true
  try {
    const res = await emailsApi.list(page)
    emails.value = res.data.data
    pageable.value = res.data.pageable
  } catch (e) {
    console.error('Failed to load emails', e)
  } finally {
    loading.value = false
  }
})

async function retryEmail(e: Event, em: Email) {
  e.stopPropagation()
  if (retryingId.value) return
  retryingId.value = em.uuid
  try {
    const res = await emailsApi.retry(em.uuid)
    em.status = res.data.data.status as Email['status']
    em.error_message = ''
    notify.success('Email re-queued for delivery')
  } catch (err: any) {
    const msg = err.response?.data?.error?.message || 'Failed to retry email'
    notify.error(msg)
  } finally {
    retryingId.value = null
  }
}

function statusBadgeClass(status: string) {
  switch (status) {
    case 'sent': return 'badge badge-success'
    case 'failed': return 'badge badge-danger'
    case 'pending': return 'badge badge-warning'
    case 'queued': return 'badge badge-info'
    case 'processing': return 'badge badge-warning'
    case 'suppressed': return 'badge badge-secondary'
    case 'scheduled': return 'badge badge-info'
    default: return 'badge'
  }
}

function formatDate(date: string | null) {
  if (!date) return '-'
  return new Date(date).toLocaleString()
}
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Emails</h1>
      <button class="btn btn-secondary" @click="router.push('/emails/preview')">Preview Template</button>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <div v-else class="card">
      <div v-if="emails.length === 0" class="empty-state">
        <h3>No emails found</h3>
        <p>Emails sent through the API will appear here.</p>
      </div>
      <template v-else>
        <div class="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>Subject</th>
                <th>From</th>
                <th>Recipients</th>
                <th>Status</th>
                <th>Sent At</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="email in emails"
                :key="email.uuid"
                style="cursor: pointer"
                @click="router.push(`/emails/${email.uuid}`)"
              >
                <td>{{ email.subject }}</td>
                <td>{{ email.sender }}</td>
                <td>{{ email.recipients.join(', ') }}</td>
                <td><span :class="statusBadgeClass(email.status)">{{ email.status }}</span></td>
                <td>{{ formatDate(email.sent_at) }}</td>
                <td>
                  <button
                    v-if="email.status === 'failed'"
                    class="btn btn-secondary btn-sm"
                    :disabled="retryingId === email.uuid"
                    @click="retryEmail($event, email)"
                  >
                    {{ retryingId === email.uuid ? 'Retrying...' : 'Retry' }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <Pagination :pageable="pageable" @page="goToPage" />
      </template>
    </div>
  </div>
</template>
