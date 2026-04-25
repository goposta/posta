<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { dashboardApi } from '../../api/dashboard'
import { emailsApi } from '../../api/emails'
import type { DashboardStats, Email } from '../../api/types'

const router = useRouter()
const loading = ref(true)
const stats = ref<DashboardStats | null>(null)
const recentEmails = ref<Email[]>([])

onMounted(async () => {
  try {
    const [statsRes, emailsRes] = await Promise.all([
      dashboardApi.getStats(),
      emailsApi.list(0, 10),
    ])
    stats.value = statsRes.data.data
    recentEmails.value = emailsRes.data.data
  } catch (e) {
    console.error('Failed to load dashboard data', e)
  } finally {
    loading.value = false
  }
})

const chartMax = computed(() => {
  if (!stats.value?.daily_volume) return 1
  const max = Math.max(...stats.value.daily_volume.map(d => d.sent + d.failed))
  return max || 1
})

function chartBarHeight(value: number): string {
  const pct = (value / chartMax.value) * 100
  return `${Math.max(pct, value > 0 ? 2 : 0)}%`
}

function formatChartDate(dateStr: string): string {
  const d = new Date(dateStr + 'T00:00:00')
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}

function formatChartWeekday(dateStr: string): string {
  const d = new Date(dateStr + 'T00:00:00')
  return d.toLocaleDateString(undefined, { weekday: 'short' })
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

function formatNumber(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 10_000) return (n / 1_000).toFixed(1) + 'K'
  return n.toLocaleString()
}

const deliveryRate = computed(() => {
  if (!stats.value || stats.value.total_emails === 0) return 0
  return ((stats.value.sent_emails / stats.value.total_emails) * 100)
})

const totalVolume14d = computed(() => {
  if (!stats.value?.daily_volume) return 0
  return stats.value.daily_volume.reduce((sum, d) => sum + d.sent + d.failed, 0)
})
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Dashboard</h1>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <template v-else-if="stats">
      <!-- Primary stats row -->
      <div class="stats-grid">
        <div class="stat-card">
          <div class="stat-header">
            <div class="stat-label">Total Emails</div>
            <div class="stat-icon stat-icon-primary">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z"/><polyline points="22,6 12,13 2,6"/></svg>
            </div>
          </div>
          <div class="stat-value">{{ formatNumber(stats.total_emails) }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-header">
            <div class="stat-label">Sent</div>
            <div class="stat-icon stat-icon-success">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>
            </div>
          </div>
          <div class="stat-value">{{ formatNumber(stats.sent_emails) }}</div>
          <div class="stat-sub">{{ deliveryRate.toFixed(1) }}% delivery rate</div>
        </div>
        <div class="stat-card">
          <div class="stat-header">
            <div class="stat-label">Failed</div>
            <div class="stat-icon stat-icon-danger">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>
            </div>
          </div>
          <div class="stat-value">{{ formatNumber(stats.failed_emails) }}</div>
          <div class="stat-sub">{{ stats.failure_rate.toFixed(1) }}% failure rate</div>
        </div>
        <div v-if="stats.queued_emails > 0 || stats.processing_emails > 0" class="stat-card">
          <div class="stat-header">
            <div class="stat-label">In Queue</div>
            <div class="stat-icon stat-icon-info">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
            </div>
          </div>
          <div class="stat-value">{{ formatNumber(stats.queued_emails + stats.processing_emails) }}</div>
        </div>
        <div v-if="(stats.total_inbound ?? 0) > 0" class="stat-card" style="cursor: pointer" @click="$router.push('/inbound-emails')">
          <div class="stat-header">
            <div class="stat-label">Inbound</div>
            <div class="stat-icon stat-icon-info">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 12 16 12 14 15 10 15 8 12 2 12"/><path d="M5.45 5.11L2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z"/></svg>
            </div>
          </div>
          <div class="stat-value">{{ formatNumber(stats.total_inbound ?? 0) }}</div>
          <div class="stat-sub">{{ formatNumber(stats.forwarded_inbound ?? 0) }} forwarded · {{ formatNumber(stats.failed_inbound ?? 0) }} failed</div>
        </div>
      </div>

      <!-- Daily volume chart -->
      <div class="card" style="margin-top: 20px">
        <div class="card-header">
          <h2>Send Volume <span style="font-weight: 400; font-size: 13px; color: var(--text-muted)">Last 14 days &middot; {{ formatNumber(totalVolume14d) }} emails</span></h2>
        </div>
        <div class="volume-chart">
          <div class="volume-chart-bars">
            <div
              v-for="day in stats.daily_volume"
              :key="day.date"
              class="volume-bar-group"
              :title="`${formatChartDate(day.date)}: ${day.sent} sent, ${day.failed} failed`"
            >
              <div class="volume-bar-stack">
                <div class="volume-bar volume-bar-failed" :style="{ height: chartBarHeight(day.failed) }"></div>
                <div class="volume-bar volume-bar-sent" :style="{ height: chartBarHeight(day.sent) }"></div>
              </div>
              <div class="volume-bar-label">{{ formatChartWeekday(day.date) }}</div>
            </div>
          </div>
          <div class="volume-chart-legend">
            <span class="volume-legend-item"><span class="volume-legend-dot volume-legend-sent"></span> Sent</span>
            <span class="volume-legend-item"><span class="volume-legend-dot volume-legend-failed"></span> Failed</span>
          </div>
        </div>
      </div>

      <!-- Webhook Deliveries -->
      <div v-if="stats.webhook_deliveries && stats.webhook_deliveries.total_deliveries > 0" class="card" style="margin-top: 20px">
        <div class="card-header" style="display: flex; justify-content: space-between; align-items: center">
          <h2>Webhook Deliveries</h2>
          <button class="btn btn-secondary btn-sm" @click="router.push('/webhooks')">Manage Webhooks</button>
        </div>
        <div class="card-body">
          <div class="wh-stats-row">
            <div class="wh-stat">
              <div class="wh-stat-value">{{ formatNumber(stats.webhook_deliveries.total_deliveries) }}</div>
              <div class="wh-stat-label">Total</div>
            </div>
            <div class="wh-stat">
              <div class="wh-stat-value wh-stat-success">{{ formatNumber(stats.webhook_deliveries.success_deliveries) }}</div>
              <div class="wh-stat-label">Successful</div>
            </div>
            <div class="wh-stat">
              <div class="wh-stat-value wh-stat-failed">{{ formatNumber(stats.webhook_deliveries.failed_deliveries) }}</div>
              <div class="wh-stat-label">Failed</div>
            </div>
            <div class="wh-stat">
              <div class="wh-stat-value" :class="stats.webhook_deliveries.success_rate >= 95 ? 'wh-stat-success' : stats.webhook_deliveries.success_rate >= 80 ? 'wh-stat-warning' : 'wh-stat-failed'">
                {{ stats.webhook_deliveries.success_rate.toFixed(1) }}%
              </div>
              <div class="wh-stat-label">Success Rate</div>
            </div>
          </div>
          <div class="wh-delivery-bar" v-if="stats.webhook_deliveries.total_deliveries > 0">
            <div class="wh-delivery-segment wh-delivery-success" :style="{ width: stats.webhook_deliveries.success_rate + '%' }"></div>
            <div class="wh-delivery-segment wh-delivery-failed" :style="{ width: (100 - stats.webhook_deliveries.success_rate) + '%' }"></div>
          </div>
        </div>
      </div>

      <!-- Two-column layout: Infrastructure + Deliverability -->
      <div class="dashboard-grid" style="margin-top: 20px">
        <!-- Infrastructure -->
        <div class="card">
          <div class="card-header">
            <h2>Infrastructure</h2>
          </div>
          <div class="dashboard-metric-list">
            <div class="dashboard-metric" @click="router.push('/api-keys')" style="cursor: pointer">
              <div class="dashboard-metric-label">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m21 2-2 2m-7.61 7.61a5.5 5.5 0 1 1-7.778 7.778 5.5 5.5 0 0 1 7.777-7.777zm0 0L15.5 7.5m0 0 3 3L22 7l-3-3m-3.5 3.5L19 4"/></svg>
                API Keys
              </div>
              <div class="dashboard-metric-value">
                <span>{{ stats.active_api_keys }}</span>
                <span class="dashboard-metric-secondary">/ {{ stats.total_api_keys }}</span>
              </div>
            </div>
            <div class="dashboard-metric" @click="router.push('/domains')" style="cursor: pointer">
              <div class="dashboard-metric-label">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="2" y1="12" x2="22" y2="12"/><path d="M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/></svg>
                Domains
              </div>
              <div class="dashboard-metric-value">{{ stats.total_domains }}</div>
            </div>
            <div class="dashboard-metric" @click="router.push('/smtp-servers')" style="cursor: pointer">
              <div class="dashboard-metric-label">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="2" width="20" height="8" rx="2"/><rect x="2" y="14" width="20" height="8" rx="2"/><line x1="6" y1="6" x2="6.01" y2="6"/><line x1="6" y1="18" x2="6.01" y2="18"/></svg>
                SMTP Servers
              </div>
              <div class="dashboard-metric-value">{{ stats.total_smtp_servers }}</div>
            </div>
            <div class="dashboard-metric" @click="router.push('/webhooks')" style="cursor: pointer">
              <div class="dashboard-metric-label">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 16.98h-5.99c-1.1 0-1.95.68-2.95 1.76C8.07 19.82 6.22 20 5 20c-1.22 0-2.2-.38-3-1"/><path d="M18 16.98h-5.99c-1.66 0-2.61-1.22-3.15-2.59C8.23 12.64 8 10.66 8 9c0-3.87 3.13-7 7-7"/><circle cx="12" cy="12" r="2"/></svg>
                Webhooks
              </div>
              <div class="dashboard-metric-value">{{ stats.total_webhooks }}</div>
            </div>
          </div>
        </div>

        <!-- Deliverability -->
        <div class="card">
          <div class="card-header">
            <h2>Deliverability</h2>
          </div>
          <div class="dashboard-metric-list">
            <div class="dashboard-metric" @click="router.push('/contacts')" style="cursor: pointer">
              <div class="dashboard-metric-label">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>
                Contacts
              </div>
              <div class="dashboard-metric-value">{{ formatNumber(stats.total_contacts) }}</div>
            </div>
            <div class="dashboard-metric">
              <div class="dashboard-metric-label">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="4.93" y1="4.93" x2="19.07" y2="19.07"/></svg>
                Suppressed
              </div>
              <div class="dashboard-metric-value">{{ formatNumber(stats.suppressed_emails) }}</div>
            </div>
            <div class="dashboard-metric" @click="router.push('/bounces')" style="cursor: pointer">
              <div class="dashboard-metric-label">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="9 17 4 12 9 7"/><path d="M20 18v-2a4 4 0 0 0-4-4H4"/></svg>
                Bounces
              </div>
              <div class="dashboard-metric-value">{{ formatNumber(stats.total_bounces) }}</div>
            </div>
            <div class="dashboard-metric">
              <div class="dashboard-metric-label">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>
                Suppressions
              </div>
              <div class="dashboard-metric-value">{{ formatNumber(stats.total_suppressions) }}</div>
            </div>
          </div>
        </div>
      </div>

      <!-- Recent Emails -->
      <div class="card" style="margin-top: 20px">
        <div class="card-header" style="display: flex; justify-content: space-between; align-items: center">
          <h2>Recent Emails</h2>
          <button class="btn btn-secondary btn-sm" @click="router.push('/emails')">View All</button>
        </div>
        <div v-if="recentEmails.length === 0" class="empty-state">
          <h3>No emails yet</h3>
          <p>Emails sent through the API will appear here.</p>
        </div>
        <div v-else class="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>Subject</th>
                <th>Recipients</th>
                <th>Status</th>
                <th>Date</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="email in recentEmails"
                :key="email.uuid"
                style="cursor: pointer"
                @click="router.push(`/emails/${email.uuid}`)"
              >
                <td>{{ email.subject }}</td>
                <td>{{ email.recipients.join(', ') }}</td>
                <td><span :class="statusBadgeClass(email.status)">{{ email.status }}</span></td>
                <td>{{ formatDate(email.created_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.dashboard-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

@media (max-width: 768px) {
  .dashboard-grid {
    grid-template-columns: 1fr;
  }
}

.dashboard-metric-list {
  padding: 4px 0;
}

.dashboard-metric {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  border-bottom: 1px solid var(--border-light);
  transition: background 0.15s ease;
}

.dashboard-metric:last-child {
  border-bottom: none;
}

.dashboard-metric:hover {
  background: var(--bg-secondary);
}

.dashboard-metric-label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  color: var(--text-secondary);
}

.dashboard-metric-label svg {
  color: var(--text-muted);
  flex-shrink: 0;
}

.dashboard-metric-value {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
}

.dashboard-metric-secondary {
  font-weight: 400;
  color: var(--text-muted);
  font-size: 13px;
}

/* Webhook delivery stats */
.wh-stats-row {
  display: flex;
  gap: 24px;
  margin-bottom: 16px;
}

.wh-stat {
  flex: 1;
  text-align: center;
}

.wh-stat-value {
  font-size: 22px;
  font-weight: 700;
  color: var(--text-primary);
}

.wh-stat-label {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 2px;
}

.wh-stat-success { color: var(--success-600, #16a34a); }
.wh-stat-failed { color: var(--danger-600, #dc2626); }
.wh-stat-warning { color: var(--warning-600, #ca8a04); }

.wh-delivery-bar {
  display: flex;
  height: 8px;
  border-radius: 4px;
  overflow: hidden;
  background: var(--bg-tertiary);
}

.wh-delivery-segment {
  transition: width 0.4s ease;
  min-width: 0;
}

.wh-delivery-success { background: var(--success-500, #22c55e); }
.wh-delivery-failed { background: var(--danger-500, #ef4444); }

/* Volume chart */
.volume-chart {
  padding: 16px;
}

.volume-chart-bars {
  display: flex;
  align-items: flex-end;
  gap: 4px;
  height: 140px;
  padding-bottom: 24px;
  position: relative;
}

.volume-bar-group {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  height: 100%;
  position: relative;
}

.volume-bar-stack {
  flex: 1;
  width: 100%;
  max-width: 36px;
  display: flex;
  flex-direction: column;
  justify-content: flex-end;
  position: relative;
}

.volume-bar {
  width: 100%;
  border-radius: 3px 3px 0 0;
  min-width: 0;
  transition: height 0.3s ease;
}

.volume-bar-sent {
  background: var(--primary-500);
}

.volume-bar-failed {
  background: var(--danger-400);
  border-radius: 3px 3px 0 0;
}

.volume-bar-sent + .volume-bar-failed,
.volume-bar-failed + .volume-bar-sent {
  border-radius: 0;
}

.volume-bar-stack .volume-bar:first-child {
  border-radius: 3px 3px 0 0;
}

.volume-bar-label {
  position: absolute;
  bottom: -22px;
  font-size: 11px;
  color: var(--text-muted);
  white-space: nowrap;
}

.volume-chart-legend {
  display: flex;
  gap: 16px;
  justify-content: center;
  margin-top: 12px;
}

.volume-legend-item {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--text-muted);
}

.volume-legend-dot {
  width: 10px;
  height: 10px;
  border-radius: 2px;
}

.volume-legend-sent {
  background: var(--primary-500);
}

.volume-legend-failed {
  background: var(--danger-400);
}
</style>
