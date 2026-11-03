<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { auditApi } from '../../api/audit'
import type { Event } from '../../api/types'
import Pagination from '../../components/Pagination.vue'
import { usePagination } from '@/composables/usePagination'
import { ENTERPRISE_URL } from '../../api/license'
import { useAuthStore } from '../../stores/auth'
import { useLicenseStore } from '../../stores/license'

const router = useRouter()
const events = ref<Event[]>([])
const loading = ref(true)
const license = useLicenseStore()
const auth = useAuthStore()

// Posta Community shows the last week; the full history needs Enterprise.
const COMMUNITY_AUDIT_DAYS = 7

const { pageable, goToPage } = usePagination(async (page) => {
  loading.value = true
  try {
    await license.load()
    const res = await auditApi.list(page, pageable.value.size)
    events.value = res.data.data
    pageable.value = res.data.pageable
  } catch (e) {
    console.error('Failed to load audit log', e)
  } finally {
    loading.value = false
  }
})

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleString()
}

function truncate(text: string, max = 80): string {
  if (!text) return ''
  return text.length > max ? text.slice(0, max).replace(/\s+$/, '') + '…' : text
}

function openDetail(event: Event) {
  router.push({ name: 'audit-log-detail', params: { id: event.id } })
}
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Audit Log</h1>
    </div>

    <div v-if="license.ready && !license.licensed" class="app-banner app-banner--info audit-window" role="status">
      <div class="app-banner-content">
        <p class="app-banner-text">
          Posta Community shows the last {{ COMMUNITY_AUDIT_DAYS }} days of activity. The full history, kept for as
          long as your retention policy allows, comes with <span class="ee-badge">Enterprise</span>.
          <router-link v-if="auth.isAdmin" to="/admin/license">Install a license</router-link>
          <template v-if="auth.isAdmin"> · </template>
          <a :href="ENTERPRISE_URL" target="_blank" rel="noopener noreferrer">About Posta Enterprise</a>
        </p>
      </div>
    </div>


    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <div v-else class="card">
      <div v-if="events.length === 0" class="empty-state">
        <h3>No audit events</h3>
        <p>Workspace activity — member changes, server, webhook and API key updates — will appear here.</p>
      </div>

      <template v-else>
        <div class="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>Date</th>
                <th>Action</th>
                <th>Actor</th>
                <th>IP Address</th>
                <th>Message</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="event in events" :key="event.id" class="clickable-row" @click="openDetail(event)">
                <td style="white-space: nowrap">{{ formatDate(event.created_at) }}</td>
                <td><code>{{ event.type }}</code></td>
                <td>{{ event.actor_name || '—' }}</td>
                <td><code v-if="event.client_ip">{{ event.client_ip }}</code><span v-else>—</span></td>
                <td class="message-cell" :title="event.message">{{ truncate(event.message) }}</td>
              </tr>
            </tbody>
          </table>
        </div>

        <Pagination :pageable="pageable" @page="goToPage" />
      </template>
    </div>
  </div>
</template>

<style scoped>
.audit-window {
  margin-bottom: 16px;
}

.clickable-row {
  cursor: pointer;
}

.clickable-row:hover {
  background: var(--bg-hover);
}

.message-cell {
  max-width: 360px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
