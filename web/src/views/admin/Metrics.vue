<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { adminApi } from '../../api/admin'
import { analyticsApi } from '../../api/analytics'
import { useAuthStore } from '../../stores/auth'
import type { AdminMetrics, WorkerStatus, SystemStatus, AnalyticsResponse, DashboardAnalyticsResponse } from '../../api/types'

const auth = useAuthStore()
const loading = ref(true)
const metrics = ref<AdminMetrics | null>(null)
const workerStatus = ref<WorkerStatus | null>(null)
const analytics = ref<AnalyticsResponse | null>(null)
const dashAnalytics = ref<DashboardAnalyticsResponse | null>(null)
let workerSSE: EventSource | null = null

onMounted(async () => {
  try {
    const [metricsRes, analyticsRes, dashRes] = await Promise.all([
      adminApi.getMetrics(),
      analyticsApi.admin(),
      analyticsApi.adminDashboardAnalytics(),
    ])
    metrics.value = metricsRes.data.data
    analytics.value = analyticsRes.data.data
    dashAnalytics.value = dashRes.data.data
  } catch (e) {
    console.error('Failed to load metrics', e)
  } finally {
    loading.value = false
  }
  startWorkerStream()
})

onBeforeUnmount(() => {
  stopWorkerStream()
})

function startWorkerStream() {
  const baseUrl = import.meta.env.VITE_API_URL || '/api/v1'
  const token = auth.token
  if (!token) return
  const url = `${baseUrl}/admin/metrics/stream?token=${encodeURIComponent(token)}`
  workerSSE = new EventSource(url)

  workerSSE.addEventListener('worker.status', (e) => {
    try {
      const status: WorkerStatus = JSON.parse((e as MessageEvent).data)
      workerStatus.value = status
      if (metrics.value) {
        metrics.value.active_workers = status.active_workers
      }
    } catch {
      // ignore parse errors
    }
  })

  workerSSE.addEventListener('system.status', (e) => {
    try {
      const status: SystemStatus = JSON.parse((e as MessageEvent).data)
      if (metrics.value) {
        metrics.value.server_uptime_seconds = status.server_uptime_seconds
        metrics.value.current_goroutines = status.current_goroutines
        metrics.value.current_memory_usage = status.current_memory_usage
      }
    } catch {
      // ignore parse errors
    }
  })

  workerSSE.onerror = () => {
    // Connection lost; will auto-reconnect via EventSource
  }
}

function stopWorkerStream() {
  if (workerSSE) {
    workerSSE.close()
    workerSSE = null
  }
}

const activeWorkers = computed(() => workerStatus.value?.active_workers ?? metrics.value?.active_workers ?? 0)

const deliveryRate = computed(() => {
  if (!metrics.value || metrics.value.total_emails === 0) return 0
  return ((metrics.value.sent_emails / metrics.value.total_emails) * 100)
})

const revokedKeys = computed(() => {
  if (!metrics.value) return 0
  return metrics.value.total_api_keys - metrics.value.active_api_keys
})

function failureColor(rate: number): string {
  if (rate <= 1) return 'var(--success-600, #16a34a)'
  if (rate <= 5) return 'var(--warning-600, #ca8a04)'
  return 'var(--danger-600, #dc2626)'
}

function failedLoginColor(count: number): string {
  if (count === 0) return 'var(--success-600, #16a34a)'
  if (count < 10) return 'var(--warning-600, #ca8a04)'
  return 'var(--danger-600, #dc2626)'
}

// Analytics helpers
const maxDailyCount = computed(() => {
  if (!analytics.value?.daily_counts?.length) return 1
  return Math.max(...analytics.value.daily_counts.map(d => d.count), 1)
})

function barHeight(count: number): string {
  return `${Math.max((count / maxDailyCount.value) * 100, 2)}%`
}

function formatShortDate(dateStr: string): string {
  const d = new Date(dateStr)
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}

function statusColor(status: string): string {
  switch (status) {
    case 'sent': return 'var(--success-500, #22c55e)'
    case 'failed': return 'var(--danger-500, #ef4444)'
    case 'pending': return 'var(--warning-500, #f59e0b)'
    case 'queued': return 'var(--info-500, #3b82f6)'
    case 'suppressed': return 'var(--text-muted, #9ca3af)'
    default: return 'var(--text-muted)'
  }
}

const totalBreakdown = computed(() => analytics.value?.status_breakdown?.reduce((s, b) => s + b.count, 0) || 0)

// Dashboard analytics helpers
const deliveryRateMax = computed(() => {
  if (!dashAnalytics.value) return 1
  return Math.max(...dashAnalytics.value.delivery_rate_trends.map(d => d.total), 1)
})

function deliveryBarHeight(value: number): string {
  const pct = (value / deliveryRateMax.value) * 100
  return `${Math.max(pct, value > 0 ? 2 : 0)}%`
}

const bounceMax = computed(() => {
  if (!dashAnalytics.value) return 1
  return Math.max(...dashAnalytics.value.bounce_rate_trends.map(d => d.total), 1)
})

function bounceBarHeight(value: number): string {
  const pct = (value / bounceMax.value) * 100
  return `${Math.max(pct, value > 0 ? 2 : 0)}%`
}

const totalBounces = computed(() => {
  if (!dashAnalytics.value) return 0
  return dashAnalytics.value.bounce_rate_trends.reduce((sum, d) => sum + d.total, 0)
})

function formatLatency(seconds: number): string {
  if (seconds < 1) return `${Math.round(seconds * 1000)}ms`
  if (seconds < 60) return `${seconds.toFixed(1)}s`
  return `${(seconds / 60).toFixed(1)}m`
}

function formatUptime(seconds: number): string {
  if (!seconds || seconds < 0) return '—'
  const s = Math.floor(seconds)
  const days = Math.floor(s / 86400)
  const hours = Math.floor((s % 86400) / 3600)
  const minutes = Math.floor((s % 3600) / 60)
  const secs = s % 60
  if (days > 0) return `${days}d ${hours}h ${minutes}m`
  if (hours > 0) return `${hours}h ${minutes}m`
  if (minutes > 0) return `${minutes}m ${secs}s`
  return `${secs}s`
}

function formatBytes(bytes: number): string {
  if (!bytes || bytes < 0) return '—'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let n = bytes
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024
    i++
  }
  return `${n.toFixed(n >= 100 || i === 0 ? 0 : 1)} ${units[i]}`
}

const avgDeliveryRate = computed(() => {
  if (!dashAnalytics.value) return 0
  const points = dashAnalytics.value.delivery_rate_trends.filter(d => d.total > 0)
  if (points.length === 0) return 0
  const totalSent = points.reduce((sum, d) => sum + d.sent, 0)
  const totalAll = points.reduce((sum, d) => sum + d.total, 0)
  return totalAll > 0 ? (totalSent / totalAll) * 100 : 0
})
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Platform Metrics</h1>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <template v-else-if="metrics">
      <!-- Overview -->
      <div class="metrics-section-label">Overview</div>
      <div class="stats-grid">
        <div class="stat-card">
          <div class="stat-header">
            <div class="stat-label">Total Users</div>
            <div class="stat-icon stat-icon-primary">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>
            </div>
          </div>
          <div class="stat-value">{{ metrics.total_users }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-header">
            <div class="stat-label">Total Emails</div>
            <div class="stat-icon stat-icon-primary">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z"/><polyline points="22,6 12,13 2,6"/></svg>
            </div>
          </div>
          <div class="stat-value">{{ metrics.total_emails }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-header">
            <div class="stat-label">Active Workers</div>
            <div class="stat-icon" :class="activeWorkers > 0 ? 'stat-icon-success' : 'stat-icon-danger'">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="2" width="20" height="8" rx="2"/><rect x="2" y="14" width="20" height="8" rx="2"/><circle cx="6" cy="6" r="1" fill="currentColor"/><circle cx="6" cy="18" r="1" fill="currentColor"/></svg>
            </div>
          </div>
          <div class="stat-value">{{ activeWorkers }}</div>
          <div class="stat-sub">{{ activeWorkers > 0 ? 'Processing emails' : 'No workers running' }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-header">
            <div class="stat-label">Shared SMTP Servers</div>
            <div class="stat-icon stat-icon-primary">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="2" width="20" height="8" rx="2"/><rect x="2" y="14" width="20" height="8" rx="2"/><line x1="6" y1="6" x2="6.01" y2="6"/><line x1="6" y1="18" x2="6.01" y2="18"/></svg>
            </div>
          </div>
          <div class="stat-value">{{ metrics.shared_smtp_servers }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-header">
            <div class="stat-label">Total Domains</div>
            <div class="stat-icon stat-icon-primary">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/></svg>
            </div>
          </div>
          <div class="stat-value">{{ metrics.total_domains }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-header">
            <div class="stat-label">Total Workspaces</div>
            <div class="stat-icon stat-icon-primary">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="7" width="20" height="14" rx="2" ry="2"/><path d="M16 7V5a2 2 0 0 0-2-2h-4a2 2 0 0 0-2 2v2"/></svg>
            </div>
          </div>
          <div class="stat-value">{{ metrics.total_workspaces }}</div>
        </div>
      </div>

      <!-- System -->
      <div class="metrics-section-label">System Status</div>
      <div class="stats-grid">
        <div class="stat-card">
          <div class="stat-header">
            <div class="stat-label">Server Uptime</div>
            <div class="stat-icon stat-icon-success">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
            </div>
          </div>
          <div class="stat-value">{{ formatUptime(metrics.server_uptime_seconds) }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-header">
            <div class="stat-label">Current Goroutines</div>
            <div class="stat-icon stat-icon-info">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z"/></svg>
            </div>
          </div>
          <div class="stat-value">{{ metrics.current_goroutines.toLocaleString() }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-header">
            <div class="stat-label">Current Memory Usage</div>
            <div class="stat-icon stat-icon-primary">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="4" y="4" width="16" height="16" rx="2"/><rect x="9" y="9" width="6" height="6"/><line x1="9" y1="2" x2="9" y2="4"/><line x1="15" y1="2" x2="15" y2="4"/><line x1="9" y1="20" x2="9" y2="22"/><line x1="15" y1="20" x2="15" y2="22"/><line x1="20" y1="9" x2="22" y2="9"/><line x1="20" y1="14" x2="22" y2="14"/><line x1="2" y1="9" x2="4" y2="9"/><line x1="2" y1="14" x2="4" y2="14"/></svg>
            </div>
          </div>
          <div class="stat-value">{{ formatBytes(metrics.current_memory_usage) }}</div>
          <div class="stat-sub">heap allocated</div>
        </div>
      </div>
      <!-- Worker Details -->
      <template v-if="workerStatus && workerStatus.workers.length > 0">
        <div class="metrics-section-label">Workers</div>
        <div class="card" style="margin-bottom: 28px;">
          <div class="table-wrapper">
            <table>
              <thead>
                <tr>
                  <th>Host</th>
                  <th>PID</th>
                  <th>Type</th>
                  <th>Queues</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="w in workerStatus.workers" :key="`${w.host}:${w.pid}`">
                  <td>{{ w.host }}</td>
                  <td>{{ w.pid }}</td>
                  <td>
                    <span class="badge" :class="w.type === 'embedded' ? 'badge-info' : 'badge-success'">
                      {{ w.type }}
                    </span>
                  </td>
                  <td>
                    <span v-for="(concurrency, queue) in w.queues" :key="queue" class="badge badge-neutral" style="margin-right: 6px;">
                      {{ queue }}: {{ concurrency }}
                    </span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </template>

      <!-- Security -->
      <div class="metrics-section-label">Security</div>
      <div class="stats-grid">
        <div class="stat-card">
          <div class="stat-header">
            <div class="stat-label">Active Sessions</div>
            <div class="stat-icon stat-icon-primary">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/></svg>
            </div>
          </div>
          <div class="stat-value">{{ metrics.active_sessions.toLocaleString() }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-header">
            <div class="stat-label">Failed Logins (24h)</div>
            <div class="stat-icon" :class="metrics.failed_logins_last_24h > 0 ? 'stat-icon-warning' : 'stat-icon-success'">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0 1 9.9-1"/></svg>
            </div>
          </div>
          <div class="stat-value" :style="{ color: failedLoginColor(metrics.failed_logins_last_24h) }">{{ metrics.failed_logins_last_24h.toLocaleString() }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-header">
            <div class="stat-label">2FA Adoption</div>
            <div class="stat-icon" :class="metrics.two_factor_adoption_rate >= 50 ? 'stat-icon-success' : 'stat-icon-warning'">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/><path d="m9 12 2 2 4-4"/></svg>
            </div>
          </div>
          <div class="stat-value">{{ metrics.two_factor_adoption_rate.toFixed(1) }}%</div>
          <div class="stat-sub">{{ metrics.two_factor_users }} of {{ metrics.total_users }} users</div>
        </div>
      </div>

      <!-- Email Delivery -->
      <div class="metrics-section-label">Email Delivery</div>
      <div class="stats-grid">
        <div class="stat-card">
          <div class="stat-header">
            <div class="stat-label">Sent</div>
            <div class="stat-icon stat-icon-success">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>
            </div>
          </div>
          <div class="stat-value">{{ metrics.sent_emails }}</div>
          <div class="stat-sub">{{ deliveryRate.toFixed(1) }}% delivery rate</div>
        </div>
        <div v-if="metrics.queued_emails > 0 || metrics.processing_emails > 0" class="stat-card">
          <div class="stat-header">
            <div class="stat-label">In Queue</div>
            <div class="stat-icon stat-icon-info">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
            </div>
          </div>
          <div class="stat-value">{{ metrics.queued_emails + metrics.processing_emails }}</div>
          <div class="stat-sub">{{ metrics.queued_emails }} queued, {{ metrics.processing_emails }} processing</div>
        </div>
        <div class="stat-card">
          <div class="stat-header">
            <div class="stat-label">Failed</div>
            <div class="stat-icon stat-icon-danger">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>
            </div>
          </div>
          <div class="stat-value">{{ metrics.failed_emails }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-header">
            <div class="stat-label">Suppressed</div>
            <div class="stat-icon stat-icon-secondary">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="4.93" y1="4.93" x2="19.07" y2="19.07"/></svg>
            </div>
          </div>
          <div class="stat-value">{{ metrics.suppressed_emails }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-header">
            <div class="stat-label">Failure Rate</div>
            <div class="stat-icon stat-icon-warning">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>
            </div>
          </div>
          <div class="stat-value" :style="{ color: failureColor(metrics.failure_rate) }">{{ metrics.failure_rate.toFixed(1) }}%</div>
        </div>
      </div>

      <!-- Inbound Email Metrics -->
      <div class="metrics-section-label">Inbound Email</div>
      <div class="stats-grid" style="margin-bottom: 28px;">
        <div class="stat-card">
          <div class="stat-header">
            <div class="stat-label">Total Inbound</div>
            <div class="stat-icon stat-icon-info">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 12 16 12 14 15 10 15 8 12 2 12"/><path d="M5.45 5.11L2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z"/></svg>
            </div>
          </div>
          <div class="stat-value">{{ metrics.total_inbound ?? 0 }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-header">
            <div class="stat-label">Forwarded</div>
            <div class="stat-icon stat-icon-success">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>
            </div>
          </div>
          <div class="stat-value">{{ metrics.forwarded_inbound ?? 0 }}</div>
          <div class="stat-sub">Delivered to user webhooks</div>
        </div>
        <div class="stat-card">
          <div class="stat-header">
            <div class="stat-label">Failed</div>
            <div class="stat-icon stat-icon-danger">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>
            </div>
          </div>
          <div class="stat-value">{{ metrics.failed_inbound ?? 0 }}</div>
          <div class="stat-sub">Webhook dispatch exhausted retries</div>
        </div>
        <div class="stat-card">
          <div class="stat-header">
            <div class="stat-label">Received (pending)</div>
            <div class="stat-icon stat-icon-info">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
            </div>
          </div>
          <div class="stat-value">{{ metrics.received_inbound ?? 0 }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-header">
            <div class="stat-label">Rejected</div>
            <div class="stat-icon stat-icon-warning">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="4.93" y1="4.93" x2="19.07" y2="19.07"/></svg>
            </div>
          </div>
          <div class="stat-value">{{ metrics.rejected_inbound ?? 0 }}</div>
          <div class="stat-sub">Unverified domain / suppressed / too large</div>
        </div>
      </div>

      <!-- Email delivery progress bar -->
      <div class="card" style="margin-bottom: 28px;">
        <div class="card-body">
          <div class="delivery-bar-label">
            <span>Email Status Distribution</span>
            <span class="delivery-bar-total">{{ metrics.total_emails }} total</span>
          </div>
          <div class="delivery-bar" v-if="metrics.total_emails > 0">
            <div class="delivery-segment delivery-sent" :style="{ width: (metrics.sent_emails / metrics.total_emails * 100) + '%' }" :title="`Sent: ${metrics.sent_emails}`"></div>
            <div class="delivery-segment delivery-queued" :style="{ width: ((metrics.queued_emails + metrics.processing_emails) / metrics.total_emails * 100) + '%' }" :title="`In queue: ${metrics.queued_emails + metrics.processing_emails}`"></div>
            <div class="delivery-segment delivery-failed" :style="{ width: (metrics.failed_emails / metrics.total_emails * 100) + '%' }" :title="`Failed: ${metrics.failed_emails}`"></div>
            <div class="delivery-segment delivery-suppressed" :style="{ width: (metrics.suppressed_emails / metrics.total_emails * 100) + '%' }" :title="`Suppressed: ${metrics.suppressed_emails}`"></div>
          </div>
          <div v-else class="delivery-bar">
            <div class="delivery-segment delivery-empty" style="width: 100%"></div>
          </div>
          <div class="delivery-legend">
            <span class="legend-item"><span class="legend-dot delivery-sent"></span> Sent</span>
            <span class="legend-item"><span class="legend-dot delivery-queued"></span> Queued</span>
            <span class="legend-item"><span class="legend-dot delivery-failed"></span> Failed</span>
            <span class="legend-item"><span class="legend-dot delivery-suppressed"></span> Suppressed</span>
          </div>
        </div>
      </div>

      <!-- Analytics (last 30 days) -->
      <template v-if="analytics">
        <div class="metrics-section-label">Email Volume (Last 30 Days)</div>
        <div class="card" style="margin-bottom: 28px;">
          <div class="card-body">
            <div v-if="analytics.daily_counts && analytics.daily_counts.length > 0" class="admin-chart">
              <div
                v-for="day in analytics.daily_counts"
                :key="day.date"
                class="admin-chart-bar-group"
                :title="`${formatShortDate(day.date)}: ${day.count}`"
              >
                <div class="admin-chart-bar" :style="{ height: barHeight(day.count) }">
                  <span v-if="day.count > 0" class="admin-chart-bar-label">{{ day.count }}</span>
                </div>
                <div class="admin-chart-bar-date">{{ formatShortDate(day.date) }}</div>
              </div>
            </div>
            <div v-else style="text-align: center; color: var(--text-muted); padding: 24px;">No data</div>
          </div>
        </div>

        <div v-if="analytics.status_breakdown && analytics.status_breakdown.length > 0" class="card" style="margin-bottom: 28px;">
          <div class="card-header"><h2>Status Breakdown</h2></div>
          <div class="card-body">
            <div class="admin-breakdown">
              <div v-for="s in analytics.status_breakdown" :key="s.status" class="admin-breakdown-row">
                <div class="admin-breakdown-label">
                  <span class="admin-breakdown-dot" :style="{ background: statusColor(s.status) }"></span>
                  <span style="text-transform: capitalize">{{ s.status }}</span>
                </div>
                <div class="admin-breakdown-bar-track">
                  <div
                    class="admin-breakdown-bar-fill"
                    :style="{ width: (totalBreakdown > 0 ? (s.count / totalBreakdown * 100) : 0) + '%', background: statusColor(s.status) }"
                  ></div>
                </div>
                <div class="admin-breakdown-value">{{ s.count }}</div>
              </div>
            </div>
          </div>
        </div>
      </template>

      <!-- Webhook Deliveries -->
      <template v-if="metrics.webhook_deliveries && metrics.webhook_deliveries.total_deliveries > 0">
        <div class="metrics-section-label">Webhook Deliveries</div>
        <div class="stats-grid">
          <div class="stat-card">
            <div class="stat-header">
              <div class="stat-label">Total Deliveries</div>
              <div class="stat-icon stat-icon-primary">
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 16.98h-5.99c-1.1 0-1.95.68-2.95 1.76C8.07 19.82 6.22 20 5 20c-1.22 0-2.2-.38-3-1"/><path d="M18 16.98h-5.99c-1.66 0-2.61-1.22-3.15-2.59C8.23 12.64 8 10.66 8 9c0-3.87 3.13-7 7-7"/><circle cx="12" cy="12" r="2"/></svg>
              </div>
            </div>
            <div class="stat-value">{{ metrics.webhook_deliveries.total_deliveries }}</div>
          </div>
          <div class="stat-card">
            <div class="stat-header">
              <div class="stat-label">Successful</div>
              <div class="stat-icon stat-icon-success">
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>
              </div>
            </div>
            <div class="stat-value">{{ metrics.webhook_deliveries.success_deliveries }}</div>
          </div>
          <div class="stat-card">
            <div class="stat-header">
              <div class="stat-label">Failed</div>
              <div class="stat-icon stat-icon-danger">
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>
              </div>
            </div>
            <div class="stat-value">{{ metrics.webhook_deliveries.failed_deliveries }}</div>
          </div>
          <div class="stat-card">
            <div class="stat-header">
              <div class="stat-label">Success Rate</div>
              <div class="stat-icon stat-icon-info">
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M22 12h-4l-3 9L9 3l-3 9H2"/></svg>
              </div>
            </div>
            <div class="stat-value" :style="{ color: failureColor(100 - metrics.webhook_deliveries.success_rate) }">
              {{ metrics.webhook_deliveries.success_rate.toFixed(1) }}%
            </div>
          </div>
        </div>
        <div class="card" style="margin-bottom: 28px;">
          <div class="card-body">
            <div class="delivery-bar-label">
              <span>Webhook Delivery Distribution</span>
              <span class="delivery-bar-total">{{ metrics.webhook_deliveries.total_deliveries }} total</span>
            </div>
            <div class="delivery-bar">
              <div class="delivery-segment delivery-sent" :style="{ width: metrics.webhook_deliveries.success_rate + '%' }" :title="`Success: ${metrics.webhook_deliveries.success_deliveries}`"></div>
              <div class="delivery-segment delivery-failed" :style="{ width: (100 - metrics.webhook_deliveries.success_rate) + '%' }" :title="`Failed: ${metrics.webhook_deliveries.failed_deliveries}`"></div>
            </div>
            <div class="delivery-legend">
              <span class="legend-item"><span class="legend-dot delivery-sent"></span> Success</span>
              <span class="legend-item"><span class="legend-dot delivery-failed"></span> Failed</span>
            </div>
          </div>
        </div>
      </template>

      <!-- Dashboard Analytics: Delivery Rate, Bounce Rate, Latency -->
      <template v-if="dashAnalytics">
        <div class="metrics-section-label">Delivery Rate Trends <span class="metrics-section-sub">{{ avgDeliveryRate.toFixed(1) }}% avg</span></div>
        <div class="card" style="margin-bottom: 28px;">
          <div class="card-body">
            <div v-if="dashAnalytics.delivery_rate_trends.length === 0" style="text-align: center; color: var(--text-muted); padding: 24px;">No data</div>
            <div v-else>
              <div class="da-chart">
                <div
                  v-for="day in dashAnalytics.delivery_rate_trends"
                  :key="day.date"
                  class="da-bar-group"
                  :title="`${formatShortDate(day.date)}: ${day.sent} sent, ${day.failed} failed (${day.delivery_rate.toFixed(1)}%)`"
                >
                  <div class="da-bar-stack">
                    <div class="da-bar da-bar-failed" :style="{ height: deliveryBarHeight(day.failed) }"></div>
                    <div class="da-bar da-bar-sent" :style="{ height: deliveryBarHeight(day.sent) }"></div>
                  </div>
                  <div class="da-rate-label" :class="{ 'da-rate-warning': day.delivery_rate < 90 && day.total > 0, 'da-rate-danger': day.delivery_rate < 75 && day.total > 0 }">
                    {{ day.total > 0 ? day.delivery_rate.toFixed(0) + '%' : '' }}
                  </div>
                  <div class="da-bar-date">{{ formatShortDate(day.date) }}</div>
                </div>
              </div>
              <div class="da-legend">
                <span class="legend-item"><span class="legend-dot delivery-sent"></span> Sent</span>
                <span class="legend-item"><span class="legend-dot delivery-failed"></span> Failed</span>
              </div>
            </div>
          </div>
        </div>

        <div class="metrics-section-label">Bounce Rate <span class="metrics-section-sub">{{ totalBounces }} total</span></div>
        <div class="card" style="margin-bottom: 28px;">
          <div class="card-body">
            <div v-if="totalBounces === 0" style="text-align: center; color: var(--text-muted); padding: 24px;">No bounces</div>
            <div v-else>
              <div class="da-chart">
                <div
                  v-for="day in dashAnalytics.bounce_rate_trends"
                  :key="day.date"
                  class="da-bar-group"
                  :title="`${formatShortDate(day.date)}: ${day.hard} hard, ${day.soft} soft, ${day.complaint} complaint`"
                >
                  <div class="da-bar-stack">
                    <div class="da-bar da-bounce-complaint" :style="{ height: bounceBarHeight(day.complaint) }"></div>
                    <div class="da-bar da-bounce-soft" :style="{ height: bounceBarHeight(day.soft) }"></div>
                    <div class="da-bar da-bounce-hard" :style="{ height: bounceBarHeight(day.hard) }"></div>
                  </div>
                  <div class="da-bar-date">{{ formatShortDate(day.date) }}</div>
                </div>
              </div>
              <div class="da-legend">
                <span class="legend-item"><span class="legend-dot da-bounce-hard"></span> Hard</span>
                <span class="legend-item"><span class="legend-dot da-bounce-soft"></span> Soft</span>
                <span class="legend-item"><span class="legend-dot da-bounce-complaint"></span> Complaint</span>
              </div>
            </div>
          </div>
        </div>

        <div class="metrics-section-label">Delivery Latency</div>
        <div v-if="dashAnalytics.latency_percentiles.p50 === 0 && dashAnalytics.latency_percentiles.avg === 0" class="card" style="margin-bottom: 28px;">
          <div class="card-body" style="text-align: center; color: var(--text-muted); padding: 24px;">No latency data</div>
        </div>
        <div v-else class="stats-grid" style="margin-bottom: 28px;">
          <div class="stat-card">
            <div class="stat-header"><div class="stat-label">Average</div></div>
            <div class="stat-value">{{ formatLatency(dashAnalytics.latency_percentiles.avg) }}</div>
          </div>
          <div class="stat-card">
            <div class="stat-header"><div class="stat-label">p50 (Median)</div></div>
            <div class="stat-value">{{ formatLatency(dashAnalytics.latency_percentiles.p50) }}</div>
          </div>
          <div class="stat-card">
            <div class="stat-header"><div class="stat-label">p75</div></div>
            <div class="stat-value">{{ formatLatency(dashAnalytics.latency_percentiles.p75) }}</div>
          </div>
          <div class="stat-card">
            <div class="stat-header"><div class="stat-label">p90</div></div>
            <div class="stat-value">{{ formatLatency(dashAnalytics.latency_percentiles.p90) }}</div>
          </div>
          <div class="stat-card">
            <div class="stat-header"><div class="stat-label">p99</div></div>
            <div class="stat-value" style="color: var(--warning-600, #ca8a04)">{{ formatLatency(dashAnalytics.latency_percentiles.p99) }}</div>
          </div>
        </div>
      </template>

      <!-- API Keys & Reputation -->
      <div class="metrics-section-label">API Keys & Reputation</div>
      <div class="stats-grid">
        <div class="stat-card">
          <div class="stat-header">
            <div class="stat-label">Active API Keys</div>
            <div class="stat-icon stat-icon-success">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 2l-2 2m-7.61 7.61a5.5 5.5 0 1 1-7.78 7.78 5.5 5.5 0 0 1 7.78-7.78zm0 0L15.5 7.5m0 0l3 3L22 7l-3-3m-3.5 3.5L19 4"/></svg>
            </div>
          </div>
          <div class="stat-value">{{ metrics.active_api_keys }}</div>
          <div class="stat-sub">{{ revokedKeys }} revoked</div>
        </div>
        <div class="stat-card">
          <div class="stat-header">
            <div class="stat-label">Total Bounces</div>
            <div class="stat-icon stat-icon-warning">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>
            </div>
          </div>
          <div class="stat-value">{{ metrics.total_bounces }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-header">
            <div class="stat-label">Suppressions</div>
            <div class="stat-icon stat-icon-secondary">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="4.93" y1="4.93" x2="19.07" y2="19.07"/></svg>
            </div>
          </div>
          <div class="stat-value">{{ metrics.total_suppressions }}</div>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.metrics-section-label {
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  color: var(--text-muted);
  margin-bottom: 12px;
}

.delivery-bar-label {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
}

.delivery-bar-total {
  font-weight: 500;
  color: var(--text-muted);
}

.delivery-bar {
  display: flex;
  height: 12px;
  border-radius: 6px;
  overflow: hidden;
  background: var(--bg-tertiary);
  margin-bottom: 12px;
}

.delivery-segment {
  transition: width 0.6s ease;
  min-width: 0;
}

.delivery-sent { background: var(--success-500, #22c55e); }
.delivery-queued { background: var(--primary-400, #60a5fa); }
.delivery-failed { background: var(--danger-500, #ef4444); }
.delivery-suppressed { background: var(--text-muted); }
.delivery-empty { background: var(--bg-tertiary); }

.delivery-legend {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--text-secondary);
}

.legend-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
}

.admin-chart {
  display: flex;
  align-items: flex-end;
  gap: 4px;
  height: 180px;
  min-width: fit-content;
  overflow-x: auto;
}

.admin-chart-bar-group {
  display: flex;
  flex-direction: column;
  align-items: center;
  flex: 1;
  min-width: 24px;
  max-width: 40px;
  height: 100%;
  justify-content: flex-end;
}

.admin-chart-bar {
  width: 100%;
  background: var(--primary-500, #6366f1);
  border-radius: 3px 3px 0 0;
  min-height: 2px;
  position: relative;
  transition: height 0.3s ease;
}

.admin-chart-bar-label {
  position: absolute;
  top: -18px;
  left: 50%;
  transform: translateX(-50%);
  font-size: 10px;
  color: var(--text-muted);
  white-space: nowrap;
}

.admin-chart-bar-date {
  font-size: 9px;
  color: var(--text-muted);
  margin-top: 4px;
  white-space: nowrap;
  transform: rotate(-45deg);
  transform-origin: top center;
}

.admin-breakdown {
  display: grid;
  gap: 10px;
}

.admin-breakdown-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.admin-breakdown-label {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 100px;
  font-size: 13px;
  font-weight: 500;
}

.admin-breakdown-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
}

.admin-breakdown-bar-track {
  flex: 1;
  height: 8px;
  background: var(--bg-secondary);
  border-radius: 4px;
  overflow: hidden;
}

.admin-breakdown-bar-fill {
  height: 100%;
  border-radius: 4px;
  transition: width 0.3s ease;
}

.admin-breakdown-value {
  min-width: 60px;
  text-align: right;
  font-size: 13px;
  font-weight: 600;
}

.metrics-section-sub {
  font-weight: 400;
  text-transform: none;
  letter-spacing: 0;
  font-size: 12px;
  color: var(--text-muted);
}

/* Dashboard analytics charts */
.da-chart {
  display: flex;
  align-items: flex-end;
  gap: 3px;
  height: 160px;
  padding-bottom: 44px;
  position: relative;
  overflow-x: auto;
}

.da-bar-group {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  height: 100%;
  min-width: 24px;
  max-width: 42px;
  position: relative;
}

.da-bar-stack {
  flex: 1;
  width: 100%;
  max-width: 32px;
  display: flex;
  flex-direction: column;
  justify-content: flex-end;
}

.da-bar {
  width: 100%;
  min-width: 0;
  transition: height 0.3s ease;
}

.da-bar-sent { background: var(--success-500, #22c55e); border-radius: 3px 3px 0 0; }
.da-bar-failed { background: var(--danger-400, #f87171); }
.da-bar-stack .da-bar:first-child { border-radius: 3px 3px 0 0; }

.da-rate-label {
  font-size: 10px;
  color: var(--success-600, #16a34a);
  font-weight: 600;
  margin-top: 2px;
  white-space: nowrap;
}
.da-rate-warning { color: var(--warning-600, #ca8a04); }
.da-rate-danger { color: var(--danger-600, #dc2626); }

.da-bar-date {
  position: absolute;
  bottom: -38px;
  font-size: 9px;
  color: var(--text-muted);
  white-space: nowrap;
  transform: rotate(-45deg);
  transform-origin: top center;
}

.da-legend {
  display: flex;
  gap: 16px;
  justify-content: center;
  margin-top: 12px;
}

.da-bounce-hard { background: var(--danger-500, #ef4444); }
.da-bounce-soft { background: var(--warning-500, #f59e0b); }
.da-bounce-complaint { background: var(--purple-500, #a855f7); }
</style>
