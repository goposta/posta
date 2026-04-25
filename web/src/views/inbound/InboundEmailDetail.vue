<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { inboundApi } from '../../api/inbound'
import { useNotificationStore } from '../../stores/notification'
import type { InboundEmail, InboundAttachmentMeta } from '../../api/types'

const route = useRoute()
const router = useRouter()
const notify = useNotificationStore()
const loading = ref(true)
const email = ref<InboundEmail | null>(null)
const activeTab = ref<'html' | 'text'>('html')
const retrying = ref(false)
const deleting = ref(false)

onMounted(async () => {
  await load()
})

async function load() {
  loading.value = true
  try {
    const uuid = route.params.id as string
    const res = await inboundApi.get(uuid)
    email.value = res.data.data
    if (!email.value?.html_body && email.value?.text_body) {
      activeTab.value = 'text'
    }
  } catch (e) {
    console.error('Failed to load inbound email', e)
  } finally {
    loading.value = false
  }
}

async function retryEmail() {
  if (!email.value || retrying.value) return
  retrying.value = true
  try {
    const res = await inboundApi.retry(email.value.uuid)
    email.value.status = res.data.data.status as InboundEmail['status']
    email.value.error_message = ''
    notify.success('Inbound email re-queued for webhook dispatch')
  } catch (e: any) {
    const msg = e.response?.data?.error?.message || 'Failed to retry inbound email'
    notify.error(msg)
  } finally {
    retrying.value = false
  }
}

async function deleteEmail() {
  if (!email.value || deleting.value) return
  if (!confirm('Delete this inbound email permanently? Attachments will also be removed.')) return
  deleting.value = true
  try {
    await inboundApi.delete(email.value.uuid)
    notify.success('Inbound email deleted')
    router.push('/inbound-emails')
  } catch (e: any) {
    const msg = e.response?.data?.error?.message || 'Failed to delete inbound email'
    notify.error(msg)
    deleting.value = false
  }
}

const headers = computed<Record<string, string> | null>(() => {
  if (!email.value?.headers_json) return null
  try {
    const parsed = JSON.parse(email.value.headers_json)
    return Object.keys(parsed).length > 0 ? parsed : null
  } catch { return null }
})

const attachments = computed<InboundAttachmentMeta[]>(() => {
  if (!email.value?.attachments_json) return []
  try {
    return JSON.parse(email.value.attachments_json)
  } catch { return [] }
})

const hasRawEml = computed(() => !!email.value?.raw_storage_key)

function statusBadgeClass(status: string) {
  switch (status) {
    case 'forwarded': return 'badge badge-success'
    case 'failed': return 'badge badge-danger'
    case 'received': return 'badge badge-info'
    case 'rejected': return 'badge badge-warning'
    default: return 'badge'
  }
}

function sourceBadgeClass(source: string) {
  return source === 'smtp' ? 'badge badge-secondary' : 'badge badge-info'
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
      <h1>Inbound Email</h1>
      <div style="display: flex; gap: 8px">
        <button
          v-if="email && (email.status === 'failed' || email.status === 'received')"
          class="btn btn-primary"
          :disabled="retrying"
          @click="retryEmail"
        >
          {{ retrying ? 'Retrying...' : 'Retry dispatch' }}
        </button>
        <a
          v-if="hasRawEml && email"
          class="btn btn-secondary"
          :href="inboundApi.rawUrl(email.uuid)"
          target="_blank"
          rel="noopener"
        >Download .eml</a>
        <button
          v-if="email"
          class="btn btn-danger"
          :disabled="deleting"
          @click="deleteEmail"
        >
          {{ deleting ? 'Deleting...' : 'Delete' }}
        </button>
        <button class="btn btn-secondary" @click="router.push('/inbound-emails')">Back to Inbound</button>
      </div>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <template v-else-if="email">
      <div class="card" style="margin-bottom: 24px">
        <div class="card-header">
          <h2>{{ email.subject || '(no subject)' }}</h2>
          <div style="display: flex; gap: 8px">
            <span :class="sourceBadgeClass(email.source)">{{ email.source }}</span>
            <span :class="statusBadgeClass(email.status)">{{ email.status }}</span>
          </div>
        </div>
        <div class="card-body">
          <table>
            <tbody>
              <tr>
                <td style="font-weight: 600; width: 160px">From</td>
                <td>{{ email.sender }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600">To</td>
                <td>{{ email.recipients.join(', ') }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600">Message-ID</td>
                <td><code style="font-size: 12px">{{ email.message_id || '-' }}</code></td>
              </tr>
              <tr>
                <td style="font-weight: 600">Size</td>
                <td>{{ formatBytes(email.size) }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600">Received At</td>
                <td>{{ formatDate(email.received_at) }}</td>
              </tr>
              <tr v-if="email.forwarded_at">
                <td style="font-weight: 600">Forwarded At</td>
                <td>{{ formatDate(email.forwarded_at) }}</td>
              </tr>
              <tr v-if="email.spam_score != null">
                <td style="font-weight: 600">Spam Score</td>
                <td>{{ email.spam_score }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600">Retry Count</td>
                <td>{{ email.retry_count }}</td>
              </tr>
              <tr v-if="email.error_message">
                <td style="font-weight: 600">Error</td>
                <td style="color: var(--danger-600)">{{ email.error_message }}</td>
              </tr>
              <tr v-if="headers">
                <td style="font-weight: 600">Headers</td>
                <td>
                  <div v-for="(value, key) in headers" :key="key" style="font-size: 12px; margin-bottom: 2px">
                    <code>{{ key }}: {{ value }}</code>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div v-if="attachments.length > 0" class="card" style="margin-bottom: 24px">
        <div class="card-header">
          <h3 style="margin: 0">Attachments ({{ attachments.length }})</h3>
        </div>
        <div class="card-body">
          <table>
            <thead>
              <tr>
                <th>Filename</th>
                <th>Content Type</th>
                <th>Size</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(att, idx) in attachments" :key="idx">
                <td>{{ att.filename }}</td>
                <td><code style="font-size: 12px">{{ att.content_type }}</code></td>
                <td>{{ formatBytes(att.size) }}</td>
                <td>
                  <a
                    class="btn btn-secondary btn-sm"
                    :href="inboundApi.attachmentUrl(email.uuid, idx)"
                    target="_blank"
                    rel="noopener"
                  >Download</a>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div class="card">
        <div class="card-header">
          <div class="tabs" style="margin-bottom: 0;">
            <button class="tab" :class="{ active: activeTab === 'html' }" @click="activeTab = 'html'" :disabled="!email.html_body">HTML</button>
            <button class="tab" :class="{ active: activeTab === 'text' }" @click="activeTab = 'text'" :disabled="!email.text_body">Text</button>
          </div>
        </div>
        <div class="card-body">
          <iframe
            v-if="activeTab === 'html' && email.html_body"
            :srcdoc="email.html_body"
            sandbox=""
            style="width: 100%; min-height: 400px; border: 1px solid var(--border-primary); border-radius: var(--radius)"
          ></iframe>
          <pre
            v-else-if="activeTab === 'text' && email.text_body"
            style="white-space: pre-wrap; word-wrap: break-word; font-size: 14px; color: var(--text-secondary); line-height: 1.6"
          >{{ email.text_body }}</pre>
          <div v-else class="empty-state" style="padding: 32px">
            <p>No body content.</p>
          </div>
        </div>
      </div>
    </template>

    <div v-else class="empty-state">
      <h3>Inbound email not found</h3>
      <p>The message you are looking for does not exist.</p>
    </div>
  </div>
</template>
