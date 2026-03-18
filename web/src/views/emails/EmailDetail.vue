<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { emailsApi } from '../../api/emails'
import { useNotificationStore } from '../../stores/notification'
import type { Email } from '../../api/types'

const route = useRoute()
const router = useRouter()
const notification = useNotificationStore()
const loading = ref(true)
const retrying = ref(false)
const email = ref<Email | null>(null)
const activeTab = ref<'html' | 'text'>('html')
const contentRedacted = computed(() =>
  email.value?.html_body === '[redacted]' && email.value?.text_body === '[redacted]'
)

onMounted(async () => {
  try {
    const uuid = route.params.id as string
    const res = await emailsApi.get(uuid)
    email.value = res.data.data
  } catch (e) {
    console.error('Failed to load email', e)
  } finally {
    loading.value = false
  }
})

async function retryEmail() {
  if (!email.value || retrying.value) return
  retrying.value = true
  try {
    const res = await emailsApi.retry(email.value.uuid)
    email.value.status = res.data.data.status as Email['status']
    email.value.error_message = ''
    notification.success('Email re-queued for delivery')
  } catch (e: any) {
    const msg = e.response?.data?.error?.message || 'Failed to retry email'
    notification.error(msg)
  } finally {
    retrying.value = false
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

function parseHeaders(json: string): Record<string, string> | null {
  if (!json) return null
  try {
    const parsed = JSON.parse(json)
    if (Object.keys(parsed).length === 0) return null
    return parsed
  } catch { return null }
}
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Email Detail</h1>
      <div style="display: flex; gap: 8px">
        <button
          v-if="email && email.status === 'failed'"
          class="btn btn-primary"
          :disabled="retrying"
          @click="retryEmail"
        >
          {{ retrying ? 'Retrying...' : 'Retry' }}
        </button>
        <button class="btn btn-secondary" @click="router.push('/emails')">Back to Emails</button>
      </div>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <template v-else-if="email">
      <div class="card" style="margin-bottom: 24px">
        <div class="card-header">
          <h2>{{ email.subject }}</h2>
          <span :class="statusBadgeClass(email.status)">{{ email.status }}</span>
        </div>
        <div class="card-body">
          <table>
            <tbody>
              <tr>
                <td style="font-weight: 600; width: 140px">From</td>
                <td>{{ email.sender }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600">To</td>
                <td>{{ email.recipients.join(', ') }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600">Status</td>
                <td><span :class="statusBadgeClass(email.status)">{{ email.status }}</span></td>
              </tr>
              <tr>
                <td style="font-weight: 600">Created At</td>
                <td>{{ formatDate(email.created_at) }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600">Sent At</td>
                <td>{{ formatDate(email.sent_at) }}</td>
              </tr>
              <tr v-if="email.scheduled_at">
                <td style="font-weight: 600">Scheduled At</td>
                <td>{{ formatDate(email.scheduled_at) }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600">SMTP Server</td>
                <td>{{ email.smtp_hostname || 'N/A' }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600">Retry Count</td>
                <td>{{ email.retry_count }}</td>
              </tr>
              <tr v-if="email.list_unsubscribe_url">
                <td style="font-weight: 600">List-Unsubscribe</td>
                <td>
                  <code style="font-size: 12px">{{ email.list_unsubscribe_url }}</code>
                  <span v-if="email.list_unsubscribe_post" class="badge badge-info" style="margin-left: 8px">One-Click</span>
                </td>
              </tr>
              <tr v-if="parseHeaders(email.headers_json)">
                <td style="font-weight: 600">Custom Headers</td>
                <td>
                  <div v-for="(value, key) in parseHeaders(email.headers_json)" :key="key" style="font-size: 12px; margin-bottom: 2px">
                    <code>{{ key }}: {{ value }}</code>
                  </div>
                </td>
              </tr>
              <tr v-if="(email.status === 'failed' || email.status === 'suppressed') && email.error_message">
                <td style="font-weight: 600">Error</td>
                <td style="color: var(--danger-600)">{{ email.error_message }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div class="card">
        <template v-if="contentRedacted">
          <div class="card-body" style="text-align: center; padding: 48px 24px;">
            <span class="mdi mdi-eye-off" style="font-size: 48px; color: var(--text-muted); display: block; margin-bottom: 12px;"></span>
            <h3 style="margin: 0 0 8px; color: var(--text-secondary);">Content Hidden</h3>
            <p style="color: var(--text-muted); margin: 0;">Email body content is not available, email content disabled for privacy.</p>
          </div>
        </template>
        <template v-else>
          <div class="card-header">
            <div class="tabs" style="margin-bottom: 0;">
              <button class="tab" :class="{ active: activeTab === 'html' }" @click="activeTab = 'html'">HTML Preview</button>
              <button class="tab" :class="{ active: activeTab === 'text' }" @click="activeTab = 'text'">Text Content</button>
            </div>
          </div>
          <div class="card-body">
            <iframe
              v-if="activeTab === 'html'"
              :srcdoc="email.html_body"
              style="width: 100%; min-height: 400px; border: 1px solid var(--border-primary); border-radius: var(--radius)"
            ></iframe>
            <pre
              v-else
              style="white-space: pre-wrap; word-wrap: break-word; font-size: 14px; color: var(--text-secondary); line-height: 1.6"
            >{{ email.text_body }}</pre>
          </div>
        </template>
      </div>
    </template>

    <div v-else class="empty-state">
      <h3>Email not found</h3>
      <p>The email you are looking for does not exist.</p>
    </div>
  </div>
</template>
