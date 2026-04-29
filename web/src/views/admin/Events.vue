<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { adminApi } from '../../api/admin'
import { useAuthStore } from '../../stores/auth'
import type { Event, Pageable } from '../../api/types'
import { usePagination } from '@/composables/usePagination'
import Pagination from '@/components/Pagination.vue'
const auth = useAuthStore()
const loading = ref(true)
const events = ref<Event[]>([])
const page = ref(0)
const category = ref('')
const liveEvents = ref<Event[]>([])
const streaming = ref(false)
let eventSource: EventSource | null = null

onMounted(() => {
  startStream()
})

onBeforeUnmount(() => {
  stopStream()
})


const { pageable, goToPage } = usePagination(async (page) => {
  loading.value = true
  try {
    const res = await adminApi.listEvents(page, 20, category.value || undefined)
    events.value = res.data.data
    pageable.value = res.data.pageable
  } catch (e) {
    console.error('Failed to load events', e)
  } finally {
    loading.value = false
  }
})



function filterByCategory(cat: string) {
  category.value = cat
  page.value = 0
  goToPage(pageable.value.current_page
  )
}

function startStream() {
  const baseUrl = import.meta.env.VITE_API_URL || '/api/v1'
  const token = auth.token
  if (!token) return
  const url = `${baseUrl}/admin/events/stream?token=${encodeURIComponent(token)}`
  eventSource = new EventSource(url)
  
  eventSource.onopen = () => {
    streaming.value = true
  }

  eventSource.addEventListener('message', (e) => {
    try {
      const evt: Event = JSON.parse(e.data)
      liveEvents.value.unshift(evt)
      if (liveEvents.value.length > 50) {
        liveEvents.value.pop()
      }
    } catch {
      // ignore parse errors
    }
  })

  // Listen for all named event types
  const eventTypes = [
    'user.login', 'user.created', 'user.updated', 'user.deleted',
    'email.queued', 'email.batch_queued',
    'apikey.revoked',
    'worker.connected', 'worker.disconnected',
  ]
  for (const type of eventTypes) {
    eventSource.addEventListener(type, (e) => {
      try {
        const evt: Event = JSON.parse((e as MessageEvent).data)
        liveEvents.value.unshift(evt)
        if (liveEvents.value.length > 50) {
          liveEvents.value.pop()
        }
      } catch {
        // ignore parse errors
      }
    })
  }

  eventSource.onerror = () => {
    streaming.value = false
  }
}

function stopStream() {
  if (eventSource) {
    eventSource.close()
    eventSource = null
    streaming.value = false
  }
}

function categoryBadgeClass(cat: string) {
  switch (cat) {
    case 'user': return 'badge badge-info'
    case 'email': return 'badge badge-success'
    case 'system': return 'badge badge-warning'
    default: return 'badge badge-neutral'
  }
}

function formatDate(date: string) {
  return new Date(date).toLocaleString()
}

function timeAgo(date: string) {
  const seconds = Math.floor((Date.now() - new Date(date).getTime()) / 1000)
  if (seconds < 60) return 'just now'
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`
  return `${Math.floor(seconds / 86400)}d ago`
}
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Events</h1>
      <div style="display: flex; align-items: center; gap: 0.75rem;">
        <span v-if="streaming" class="stream-indicator stream-active">Live</span>
        <span v-else class="stream-indicator stream-inactive">Disconnected</span>
      </div>
    </div>

    <!-- Live Events -->
    <div v-if="liveEvents.length > 0" class="card" style="margin-bottom: 1.5rem;">
      <div class="card-header">
        <h2>Live Feed</h2>
        <button class="btn btn-sm btn-secondary" @click="liveEvents = []">Clear</button>
      </div>
      <div class="card-body">
        <div class="live-events">
          <div v-for="(evt, i) in liveEvents" :key="'live-' + i" class="live-event-item">
            <span :class="categoryBadgeClass(evt.category)">{{ evt.category }}</span>
            <span class="event-type">{{ evt.type }}</span>
            <span v-if="evt.client_ip" class="event-ip">{{ evt.client_ip }}</span>
            <span class="event-message">{{ evt.message }}</span>
            <span class="event-time">{{ timeAgo(evt.created_at) }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Category Filter -->
    <div style="display: flex; gap: 0.5rem; margin-bottom: 1rem;">
      <button class="btn btn-sm" :class="category === '' ? 'btn-primary' : 'btn-secondary'" @click="filterByCategory('')">All</button>
      <button class="btn btn-sm" :class="category === 'user' ? 'btn-primary' : 'btn-secondary'" @click="filterByCategory('user')">User</button>
      <button class="btn btn-sm" :class="category === 'email' ? 'btn-primary' : 'btn-secondary'" @click="filterByCategory('email')">Email</button>
      <button class="btn btn-sm" :class="category === 'system' ? 'btn-primary' : 'btn-secondary'" @click="filterByCategory('system')">System</button>
    </div>

    <!-- Historical Events -->
    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <template v-else>
      <div class="card">
        <div class="card-header">
          <h2>Event History</h2>
        </div>
        <div v-if="events.length === 0" class="empty-state">
          <h3>No events found</h3>
          <p>No activity has been recorded yet.</p>
        </div>
        <div v-else class="card-body">
          <table class="table">
            <thead>
              <tr>
                <th>Category</th>
                <th>Type</th>
                <th>Actor</th>
                <th>IP Address</th>
                <th>Message</th>
                <th>Time</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="evt in events" :key="evt.id">
                <td><span :class="categoryBadgeClass(evt.category)">{{ evt.category }}</span></td>
                <td><code>{{ evt.type }}</code></td>
                <td>{{ evt.actor_name || '-' }}</td>
                <td><code v-if="evt.client_ip">{{ evt.client_ip }}</code><span v-else>-</span></td>
                <td>{{ evt.message }}</td>
                <td>{{ formatDate(evt.created_at) }}</td>
              </tr>
            </tbody>
          </table>
            <Pagination :pageable="pageable" @page="goToPage" />
          
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.stream-indicator {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  font-weight: 600;
  padding: 4px 10px;
  border-radius: 999px;
}
.stream-indicator::before {
  content: '';
  width: 8px;
  height: 8px;
  border-radius: 50%;
}
.stream-active {
  color: var(--success-700, #15803d);
  background: var(--success-50, #f0fdf4);
}
.stream-active::before {
  background: var(--success-500, #22c55e);
  animation: pulse 2s infinite;
}
.stream-inactive {
  color: var(--text-muted);
  background: var(--bg-secondary);
}
.stream-inactive::before {
  background: var(--text-muted);
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}

.live-events {
  display: flex;
  flex-direction: column;
  gap: 6px;
  max-height: 300px;
  overflow-y: auto;
}

.live-event-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px;
  border-radius: var(--radius-sm, 4px);
  background: var(--bg-secondary);
  font-size: 13px;
  animation: fadeIn 0.3s ease;
}

.event-type {
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 12px;
  color: var(--text-secondary);
  min-width: 120px;
}

.event-ip {
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 11px;
  color: var(--text-muted);
  white-space: nowrap;
}

.event-message {
  flex: 1;
  color: var(--text-primary);
}

.event-time {
  font-size: 11px;
  color: var(--text-muted);
  white-space: nowrap;
}

@keyframes fadeIn {
  from { opacity: 0; transform: translateY(-4px); }
  to { opacity: 1; transform: translateY(0); }
}
</style>
