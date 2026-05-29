<script setup lang="ts">
import { ref } from 'vue'
import { webhookDeliveriesApi } from '../../api/webhooks'
import type { WebhookDelivery } from '../../api/types'
import Pagination from '../../components/Pagination.vue'
import { usePagination } from '../../composables/usePagination'

const loading = ref(true)
const deliveries = ref<WebhookDelivery[]>([])

const { pageable, goToPage } = usePagination(async (page) => {
  loading.value = true
  try {
    const res = await webhookDeliveriesApi.list(page)
    deliveries.value = res.data.data
    pageable.value = res.data.pageable
  } catch (e) {
    console.error('Failed to load webhook deliveries', e)
  } finally {
    loading.value = false
  }
})

function statusBadgeClass(status: string) {
  return status === 'success' ? 'badge badge-success' : 'badge badge-danger'
}

function formatDate(date: string) {
  return new Date(date).toLocaleString()
}
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Webhook Deliveries</h1>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <div v-else class="card">
      <div v-if="deliveries.length === 0" class="empty-state">
        <h3>No deliveries yet</h3>
        <p>Webhook delivery attempts will appear here.</p>
      </div>
      <template v-else>
        <div class="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>Event</th>
                <th>Status</th>
                <th>HTTP Status</th>
                <th>Attempt</th>
                <th>Error</th>
                <th>Delivered At</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="d in deliveries" :key="d.id">
                <td><span class="badge badge-neutral">{{ d.event }}</span></td>
                <td><span :class="statusBadgeClass(d.status)">{{ d.status }}</span></td>
                <td>{{ d.http_status_code || '-' }}</td>
                <td>{{ d.attempt }}</td>
                <td class="truncate" style="max-width: 280px;">{{ d.error_message || '-' }}</td>
                <td>{{ formatDate(d.created_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <Pagination :pageable="pageable" @page="goToPage" />
      </template>
    </div>
  </div>
</template>
