<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { auditApi } from '../../api/audit'
import type { Event } from '../../api/types'
import { useNotificationStore } from '../../stores/notification'

const route = useRoute()
const router = useRouter()
const notify = useNotificationStore()

const loading = ref(true)
const event = ref<Event | null>(null)

onMounted(async () => {
  try {
    const id = Number(route.params.id)
    const res = await auditApi.get(id)
    event.value = res.data.data
  } catch {
    notify.error('Failed to load audit event')
  } finally {
    loading.value = false
  }
})

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleString()
}

// Pretty-print the JSON metadata blob; fall back to the raw string if it
// isn't valid JSON (or empty).
const prettyMetadata = computed(() => {
  const raw = event.value?.metadata
  if (!raw) return ''
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return raw
  }
})
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Audit Event</h1>
      <div style="display: flex; gap: 8px">
        <button class="btn btn-secondary" @click="router.push('/audit-log')">Back to Audit Log</button>
      </div>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <div v-else-if="!event" class="card">
      <div class="empty-state">
        <h3>Audit event not found</h3>
        <p>This event may have been removed by the workspace retention policy.</p>
      </div>
    </div>

    <template v-else>
      <div class="card" style="margin-bottom: 24px">
        <div class="card-header">
          <h2><code>{{ event.type }}</code></h2>
        </div>
        <div class="card-body">
          <dl class="detail-grid">
            <dt>Event ID</dt>
            <dd><code>{{ event.id }}</code></dd>

            <dt>Date</dt>
            <dd>{{ formatDate(event.created_at) }}</dd>

            <dt>Action</dt>
            <dd><code>{{ event.type }}</code></dd>

            <dt>Actor</dt>
            <dd>{{ event.actor_name || '—' }}<span v-if="event.actor_id"> (#{{ event.actor_id }})</span></dd>

            <dt>IP Address</dt>
            <dd><code v-if="event.client_ip">{{ event.client_ip }}</code><span v-else>—</span></dd>

            <dt>Message</dt>
            <dd class="detail-message">{{ event.message }}</dd>
          </dl>
        </div>
      </div>

      <div class="card">
        <div class="card-header">
          <h2>Metadata</h2>
        </div>
        <div class="card-body">
          <div v-if="!prettyMetadata" class="empty-state">
            <p>No additional metadata for this event.</p>
          </div>
          <pre v-else class="metadata-block">{{ prettyMetadata }}</pre>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.detail-grid {
  display: grid;
  grid-template-columns: 160px 1fr;
  gap: 12px 16px;
  margin: 0;
}

.detail-grid dt {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-muted);
}

.detail-grid dd {
  margin: 0;
  font-size: 14px;
  color: var(--text-primary);
  word-break: break-word;
}

.detail-message {
  white-space: pre-wrap;
}

.metadata-block {
  margin: 0;
  padding: 12px 14px;
  background: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-sm);
  font-size: 13px;
  overflow-x: auto;
  white-space: pre-wrap;
  word-break: break-word;
}
</style>
