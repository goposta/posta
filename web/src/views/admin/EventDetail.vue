<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { adminApi } from '../../api/admin'
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
    const res = await adminApi.getEvent(id)
    event.value = res.data.data
  } catch {
    notify.error('Failed to load event')
  } finally {
    loading.value = false
  }
})

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleString()
}

function categoryBadgeClass(cat: string) {
  switch (cat) {
    case 'user': return 'badge badge-info'
    case 'email': return 'badge badge-success'
    case 'system': return 'badge badge-warning'
    default: return 'badge badge-neutral'
  }
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
      <h1>Event</h1>
      <div style="display: flex; gap: 8px">
        <button class="btn btn-secondary" @click="router.push('/admin/events')">Back to Events</button>
      </div>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <div v-else-if="!event" class="card">
      <div class="empty-state">
        <h3>Event not found</h3>
        <p>This event may have been removed by the retention policy.</p>
      </div>
    </div>

    <template v-else>
      <div class="card" style="margin-bottom: 24px">
        <div class="card-header">
          <h2><code>{{ event.type }}</code></h2>
          <span :class="categoryBadgeClass(event.category)">{{ event.category }}</span>
        </div>
        <div class="card-body">
          <dl class="detail-grid">
            <dt>Event ID</dt>
            <dd><code>{{ event.id }}</code></dd>

            <dt>Date</dt>
            <dd>{{ formatDate(event.created_at) }}</dd>

            <dt>Category</dt>
            <dd>{{ event.category }}</dd>

            <dt>Type</dt>
            <dd><code>{{ event.type }}</code></dd>

            <dt>Workspace</dt>
            <dd><code v-if="event.workspace_id">#{{ event.workspace_id }}</code><span v-else>— (platform)</span></dd>

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
