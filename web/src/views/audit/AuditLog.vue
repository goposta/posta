<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { auditApi } from '../../api/audit'
import type { Event, Pageable } from '../../api/types'
import Pagination from '../../components/Pagination.vue'
import { usePagination } from '@/composables/usePagination'


const events = ref<Event[]>([])
const loading = ref(true)
const categoryFilter = ref('')



const { pageable, goToPage } = usePagination(async (page) => {
  loading.value = true
  try {
    const res = await auditApi.list(page, pageable.value.size, categoryFilter.value || undefined)
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

function categoryBadgeClass(category: string): string {
  switch (category) {
    case 'user': return 'badge badge-info'
    case 'email': return 'badge badge-success'
    case 'system': return 'badge badge-warning'
    default: return 'badge'
  }
}

</script>

<template>
  <div>
    <div class="page-header">
      <h1>Audit Log</h1>
      <div class="page-header-actions">
        <select v-model="categoryFilter" class="form-select" style="min-width: 140px" @change="goToPage(0)">
          <option value="">All categories</option>
          <option value="user">User</option>
          <option value="email">Email</option>
          <option value="system">System</option>
        </select>
      </div>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <div v-else class="card">
      <div v-if="events.length === 0" class="empty-state">
        <h3>No audit events</h3>
        <p>Your activity log will appear here as you use the platform.</p>
      </div>

      <template v-else>
        <div class="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>Date</th>
                <th>Category</th>
                <th>Type</th>
                <th>IP Address</th>
                <th>Message</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="event in events" :key="event.id">
                <td style="white-space: nowrap">{{ formatDate(event.created_at) }}</td>
                <td><span :class="categoryBadgeClass(event.category)">{{ event.category }}</span></td>
                <td><code>{{ event.type }}</code></td>
                <td><code v-if="event.client_ip">{{ event.client_ip }}</code><span v-else>-</span></td>
                <td>{{ event.message }}</td>
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
.page-header-actions {
  display: flex;
  gap: 8px;
  align-items: center;
}
</style>
