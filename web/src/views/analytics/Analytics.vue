<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { analyticsApi } from '../../api/analytics'
import type { DailyCount, StatusBreakdown, DashboardAnalyticsResponse, ProviderBreakdownPoint } from '../../api/types'

const loading = ref(true)
const dailyCounts = ref<DailyCount[]>([])
const statusBreakdown = ref<StatusBreakdown[]>([])
const dashAnalytics = ref<DashboardAnalyticsResponse | null>(null)
const providerBreakdown = ref<ProviderBreakdownPoint[]>([])

const fromDate = ref('')
const toDate = ref('')
const statusFilter = ref('')

// Set default range: last 30 days
const now = new Date()
const thirtyDaysAgo = new Date(now)
thirtyDaysAgo.setDate(thirtyDaysAgo.getDate() - 30)
fromDate.value = thirtyDaysAgo.toISOString().slice(0, 10)
toDate.value = now.toISOString().slice(0, 10)

async function loadAnalytics() {
  loading.value = true
  try {
    const [res, dashRes, provRes] = await Promise.all([
      analyticsApi.user(fromDate.value, toDate.value, statusFilter.value || undefined),
      analyticsApi.dashboardAnalytics(fromDate.value, toDate.value),
      analyticsApi.providerBreakdown(fromDate.value, toDate.value),
    ])
    dailyCounts.value = res.data.data.daily_counts || []
    statusBreakdown.value = res.data.data.status_breakdown || []
    dashAnalytics.value = dashRes.data.data
    providerBreakdown.value = provRes.data.data.providers || []
  } catch (e) {
    console.error('Failed to load analytics', e)
  } finally {
    loading.value = false
  }
}

onMounted(loadAnalytics)

const totalEmails = computed(() => dailyCounts.value.reduce((sum, d) => sum + d.count, 0))
const maxCount = computed(() => Math.max(...dailyCounts.value.map(d => d.count), 1))

function barHeight(count: number): string {
  return `${Math.max((count / maxCount.value) * 100, 2)}%`
}

function statusColor(status: string): string {
  switch (status) {
    case 'sent': return 'var(--success-500, #22c55e)'
    case 'failed': return 'var(--danger-500, #ef4444)'
    case 'pending': return 'var(--warning-500, #f59e0b)'
    case 'queued': return 'var(--info-500, #3b82f6)'
    case 'processing': return 'var(--warning-400, #fbbf24)'
    case 'suppressed': return 'var(--text-muted, #9ca3af)'
    case 'scheduled': return 'var(--primary-500, #6366f1)'
    default: return 'var(--text-muted)'
  }
}

function formatDate(dateStr: string): string {
  const d = new Date(dateStr)
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}

const totalBreakdown = computed(() => statusBreakdown.value.reduce((sum, s) => sum + s.count, 0))

function breakdownPercent(count: number): string {
  if (totalBreakdown.value === 0) return '0'
  return ((count / totalBreakdown.value) * 100).toFixed(1)
}

// Provider-specific colors for the mailbox provider breakdown.
const providerColors: Record<string, string> = {
  Gmail: '#ea4335',
  'Google Workspace': '#4285f4',
  Outlook: '#0078d4',
  Yahoo: '#6001d2',
  'Apple iCloud': '#8e8e93',
  Proton: '#6d4aff',
  AOL: '#00bfff',
  GMX: '#1c4587',
  Zoho: '#f04e23',
  Fastmail: '#2968a6',
  Yandex: '#ffcc00',
  China: '#c20000',
  Other: '#9ca3af',
}

function providerColor(name: string): string {
  return providerColors[name] ?? '#6366f1'
}

const providerTotal = computed(() => providerBreakdown.value.reduce((sum, p) => sum + p.total, 0))

function providerPercent(total: number): string {
  if (providerTotal.value === 0) return '0'
  return ((total / providerTotal.value) * 100).toFixed(1)
}

function providerRateClass(rate: number, total: number): string {
  if (total === 0) return ''
  if (rate < 75) return 'rate-danger'
  if (rate < 90) return 'rate-warning'
  return ''
}

// Delivery rate chart helpers
const deliveryRateMax = computed(() => {
  if (!dashAnalytics.value) return 1
  return Math.max(...dashAnalytics.value.delivery_rate_trends.map(d => d.total), 1)
})

function deliveryBarHeight(value: number): string {
  const pct = (value / deliveryRateMax.value) * 100
  return `${Math.max(pct, value > 0 ? 2 : 0)}%`
}

// Bounce rate chart helpers
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

// Latency formatting
function formatLatency(seconds: number): string {
  if (seconds < 1) return `${Math.round(seconds * 1000)}ms`
  if (seconds < 60) return `${seconds.toFixed(1)}s`
  return `${(seconds / 60).toFixed(1)}m`
}

// Average delivery rate
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
      <h1>Analytics</h1>
    </div>

    <!-- Filters -->
    <div class="card" style="margin-bottom: 24px">
      <div class="card-body">
        <div class="filters">
          <div class="form-group">
            <label class="form-label">From</label>
            <input v-model="fromDate" type="date" class="form-input" />
          </div>
          <div class="form-group">
            <label class="form-label">To</label>
            <input v-model="toDate" type="date" class="form-input" />
          </div>
          <div class="form-group">
            <label class="form-label">Status</label>
            <select v-model="statusFilter" class="form-input">
              <option value="">All</option>
              <option value="sent">Sent</option>
              <option value="failed">Failed</option>
              <option value="pending">Pending</option>
              <option value="queued">Queued</option>
              <option value="suppressed">Suppressed</option>
              <option value="scheduled">Scheduled</option>
            </select>
          </div>
          <div class="form-group" style="align-self: flex-end">
            <button class="btn btn-primary" @click="loadAnalytics" :disabled="loading">
              {{ loading ? 'Loading...' : 'Apply' }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <template v-else>
      <!-- Summary -->
      <div class="stats-grid" style="margin-bottom: 24px">
        <div class="stat-card">
          <div class="stat-header">
            <div class="stat-label">Total in Period</div>
          </div>
          <div class="stat-value">{{ totalEmails }}</div>
        </div>
        <div v-for="s in statusBreakdown" :key="s.status" class="stat-card">
          <div class="stat-header">
            <div class="stat-label" style="text-transform: capitalize">{{ s.status }}</div>
          </div>
          <div class="stat-value">{{ s.count }}</div>
          <div class="stat-sub">{{ breakdownPercent(s.count) }}%</div>
        </div>
      </div>

      <!-- Daily Chart -->
      <div class="card" style="margin-bottom: 24px">
        <div class="card-header"><h2>Daily Email Volume</h2></div>
        <div class="card-body">
          <div v-if="dailyCounts.length === 0" class="empty-state">
            <h3>No data</h3>
            <p>No emails found in the selected date range.</p>
          </div>
          <div v-else class="chart-container">
            <div class="chart">
              <div
                v-for="day in dailyCounts"
                :key="day.date"
                class="chart-bar-group"
                :title="`${formatDate(day.date)}: ${day.count} emails`"
              >
                <div class="chart-bar" :style="{ height: barHeight(day.count) }">
                  <span v-if="day.count > 0" class="chart-bar-label">{{ day.count }}</span>
                </div>
                <div class="chart-bar-date">{{ formatDate(day.date) }}</div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Status Breakdown -->
      <div class="card">
        <div class="card-header"><h2>Status Breakdown</h2></div>
        <div class="card-body">
          <div v-if="statusBreakdown.length === 0" class="empty-state">
            <h3>No data</h3>
            <p>No emails found in the selected date range.</p>
          </div>
          <div v-else class="breakdown">
            <div v-for="s in statusBreakdown" :key="s.status" class="breakdown-row">
              <div class="breakdown-label">
                <span class="breakdown-dot" :style="{ background: statusColor(s.status) }"></span>
                <span style="text-transform: capitalize">{{ s.status }}</span>
              </div>
              <div class="breakdown-bar-track">
                <div
                  class="breakdown-bar-fill"
                  :style="{ width: breakdownPercent(s.count) + '%', background: statusColor(s.status) }"
                ></div>
              </div>
              <div class="breakdown-value">{{ s.count }} <span class="breakdown-pct">({{ breakdownPercent(s.count) }}%)</span></div>
            </div>
          </div>
        </div>
      </div>

      <!-- Deliverability by Provider -->
      <div class="card" style="margin-top: 24px">
        <div class="card-header">
          <h2>Deliverability by Provider <span class="card-header-sub">{{ providerTotal }} recipients in period</span></h2>
        </div>
        <div class="card-body">
          <div v-if="providerBreakdown.length === 0" class="empty-state">
            <h3>No data</h3>
            <p>No recipient data in the selected range.</p>
          </div>
          <div v-else class="provider-table">
            <div class="provider-row provider-head">
              <div class="provider-cell provider-name">Provider</div>
              <div class="provider-cell provider-bar-cell">Volume</div>
              <div class="provider-cell provider-num">Sent</div>
              <div class="provider-cell provider-num">Failed</div>
              <div class="provider-cell provider-num">Rate</div>
            </div>
            <div v-for="p in providerBreakdown" :key="p.provider" class="provider-row">
              <div class="provider-cell provider-name">
                <span class="breakdown-dot" :style="{ background: providerColor(p.provider) }"></span>
                {{ p.provider }}
              </div>
              <div class="provider-cell provider-bar-cell">
                <div class="breakdown-bar-track">
                  <div
                    class="breakdown-bar-fill"
                    :style="{ width: providerPercent(p.total) + '%', background: providerColor(p.provider) }"
                  ></div>
                </div>
                <div class="provider-pct">{{ providerPercent(p.total) }}%</div>
              </div>
              <div class="provider-cell provider-num">{{ p.sent }}</div>
              <div class="provider-cell provider-num">{{ p.failed }}</div>
              <div
                class="provider-cell provider-num provider-rate"
                :class="providerRateClass(p.delivery_rate, p.sent + p.failed)"
              >
                {{ (p.sent + p.failed) > 0 ? p.delivery_rate.toFixed(1) + '%' : '—' }}
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Dashboard Analytics: Delivery Rate, Bounce Rate, Latency -->
      <template v-if="dashAnalytics">
        <!-- Delivery Rate Trends -->
        <div class="card" style="margin-top: 24px">
          <div class="card-header">
            <h2>Delivery Rate Trends <span class="card-header-sub">{{ avgDeliveryRate.toFixed(1) }}% avg</span></h2>
          </div>
          <div class="card-body">
            <div v-if="dashAnalytics.delivery_rate_trends.length === 0" class="empty-state">
              <h3>No data</h3>
              <p>No delivery data in the selected range.</p>
            </div>
            <div v-else>
              <div class="trend-chart">
                <div class="trend-chart-bars">
                  <div
                    v-for="day in dashAnalytics.delivery_rate_trends"
                    :key="day.date"
                    class="trend-bar-group"
                    :title="`${formatDate(day.date)}: ${day.sent} sent, ${day.failed} failed (${day.delivery_rate.toFixed(1)}%)`"
                  >
                    <div class="trend-bar-stack">
                      <div class="trend-bar trend-bar-failed" :style="{ height: deliveryBarHeight(day.failed) }"></div>
                      <div class="trend-bar trend-bar-sent" :style="{ height: deliveryBarHeight(day.sent) }"></div>
                    </div>
                    <div class="trend-rate-label" :class="{ 'rate-warning': day.delivery_rate < 90 && day.total > 0, 'rate-danger': day.delivery_rate < 75 && day.total > 0 }">
                      {{ day.total > 0 ? day.delivery_rate.toFixed(0) + '%' : '' }}
                    </div>
                    <div class="trend-bar-date">{{ formatDate(day.date) }}</div>
                  </div>
                </div>
                <div class="trend-chart-legend">
                  <span class="trend-legend-item"><span class="trend-legend-dot trend-legend-sent"></span> Sent</span>
                  <span class="trend-legend-item"><span class="trend-legend-dot trend-legend-failed"></span> Failed</span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Bounce Rate Graph -->
        <div class="card" style="margin-top: 24px">
          <div class="card-header">
            <h2>Bounce Rate <span class="card-header-sub">{{ totalBounces }} total bounces</span></h2>
          </div>
          <div class="card-body">
            <div v-if="totalBounces === 0" class="empty-state">
              <h3>No bounces</h3>
              <p>No bounces recorded in the selected range.</p>
            </div>
            <div v-else>
              <div class="trend-chart">
                <div class="trend-chart-bars">
                  <div
                    v-for="day in dashAnalytics.bounce_rate_trends"
                    :key="day.date"
                    class="trend-bar-group"
                    :title="`${formatDate(day.date)}: ${day.hard} hard, ${day.soft} soft, ${day.complaint} complaint`"
                  >
                    <div class="trend-bar-stack">
                      <div class="trend-bar bounce-bar-complaint" :style="{ height: bounceBarHeight(day.complaint) }"></div>
                      <div class="trend-bar bounce-bar-soft" :style="{ height: bounceBarHeight(day.soft) }"></div>
                      <div class="trend-bar bounce-bar-hard" :style="{ height: bounceBarHeight(day.hard) }"></div>
                    </div>
                    <div class="trend-bar-date">{{ formatDate(day.date) }}</div>
                  </div>
                </div>
                <div class="trend-chart-legend">
                  <span class="trend-legend-item"><span class="trend-legend-dot bounce-legend-hard"></span> Hard</span>
                  <span class="trend-legend-item"><span class="trend-legend-dot bounce-legend-soft"></span> Soft</span>
                  <span class="trend-legend-item"><span class="trend-legend-dot bounce-legend-complaint"></span> Complaint</span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Latency Percentiles -->
        <div class="card" style="margin-top: 24px">
          <div class="card-header">
            <h2>Delivery Latency</h2>
          </div>
          <div class="card-body">
            <div v-if="dashAnalytics.latency_percentiles.p50 === 0 && dashAnalytics.latency_percentiles.avg === 0" class="empty-state">
              <h3>No data</h3>
              <p>No delivered emails in the selected range.</p>
            </div>
            <div v-else class="latency-grid">
              <div class="latency-card">
                <div class="latency-value">{{ formatLatency(dashAnalytics.latency_percentiles.avg) }}</div>
                <div class="latency-label">Average</div>
              </div>
              <div class="latency-card">
                <div class="latency-value">{{ formatLatency(dashAnalytics.latency_percentiles.p50) }}</div>
                <div class="latency-label">p50 (Median)</div>
              </div>
              <div class="latency-card">
                <div class="latency-value">{{ formatLatency(dashAnalytics.latency_percentiles.p75) }}</div>
                <div class="latency-label">p75</div>
              </div>
              <div class="latency-card">
                <div class="latency-value">{{ formatLatency(dashAnalytics.latency_percentiles.p90) }}</div>
                <div class="latency-label">p90</div>
              </div>
              <div class="latency-card">
                <div class="latency-value latency-value-tail">{{ formatLatency(dashAnalytics.latency_percentiles.p99) }}</div>
                <div class="latency-label">p99</div>
              </div>
            </div>
          </div>
        </div>
      </template>
    </template>
  </div>
</template>

<style scoped>
.filters {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
  align-items: flex-end;
}

.filters .form-group {
  min-width: 140px;
}

.stat-sub {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 2px;
}

.chart-container {
  overflow-x: auto;
  padding-bottom: 8px;
}

.chart {
  display: flex;
  align-items: flex-end;
  gap: 4px;
  height: 220px;
  min-width: fit-content;
}

.chart-bar-group {
  display: flex;
  flex-direction: column;
  align-items: center;
  flex: 1;
  min-width: 28px;
  max-width: 48px;
  height: 100%;
  justify-content: flex-end;
}

.chart-bar {
  width: 100%;
  background: var(--primary-500, #6366f1);
  border-radius: 4px 4px 0 0;
  min-height: 2px;
  position: relative;
  transition: height 0.3s ease;
}

.chart-bar-label {
  position: absolute;
  top: -20px;
  left: 50%;
  transform: translateX(-50%);
  font-size: 10px;
  color: var(--text-muted);
  white-space: nowrap;
}

.chart-bar-date {
  font-size: 10px;
  color: var(--text-muted);
  margin-top: 6px;
  white-space: nowrap;
  transform: rotate(-45deg);
  transform-origin: top center;
}

.breakdown {
  display: grid;
  gap: 12px;
}

.breakdown-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.breakdown-label {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 110px;
  font-size: 13px;
  font-weight: 500;
  color: var(--text-primary);
}

.breakdown-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
}

.breakdown-bar-track {
  flex: 1;
  height: 8px;
  background: var(--bg-secondary);
  border-radius: 4px;
  overflow: hidden;
}

.breakdown-bar-fill {
  height: 100%;
  border-radius: 4px;
  transition: width 0.3s ease;
}

.breakdown-value {
  min-width: 100px;
  text-align: right;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
}

.breakdown-pct {
  font-weight: 400;
  color: var(--text-muted);
}

/* Card header subtitle */
.card-header-sub {
  font-weight: 400;
  font-size: 13px;
  color: var(--text-muted);
}

/* Delivery rate & bounce trend charts */
.trend-chart {
  padding: 8px 0;
}

.trend-chart-bars {
  display: flex;
  align-items: flex-end;
  gap: 3px;
  height: 160px;
  padding-bottom: 44px;
  position: relative;
  overflow-x: auto;
}

.trend-bar-group {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  height: 100%;
  min-width: 24px;
  max-width: 42px;
  position: relative;
}

.trend-bar-stack {
  flex: 1;
  width: 100%;
  max-width: 32px;
  display: flex;
  flex-direction: column;
  justify-content: flex-end;
  position: relative;
}

.trend-bar {
  width: 100%;
  min-width: 0;
  transition: height 0.3s ease;
}

.trend-bar-sent {
  background: var(--success-500, #22c55e);
  border-radius: 3px 3px 0 0;
}

.trend-bar-failed {
  background: var(--danger-400, #f87171);
}

.trend-bar-stack .trend-bar:first-child {
  border-radius: 3px 3px 0 0;
}

.trend-rate-label {
  font-size: 10px;
  color: var(--success-600, #16a34a);
  font-weight: 600;
  margin-top: 2px;
  white-space: nowrap;
}

.trend-rate-label.rate-warning {
  color: var(--warning-600, #ca8a04);
}

.trend-rate-label.rate-danger {
  color: var(--danger-600, #dc2626);
}

.trend-bar-date {
  position: absolute;
  bottom: -38px;
  font-size: 10px;
  color: var(--text-muted);
  white-space: nowrap;
  transform: rotate(-45deg);
  transform-origin: top center;
}

.trend-chart-legend {
  display: flex;
  gap: 16px;
  justify-content: center;
  margin-top: 12px;
}

.trend-legend-item {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--text-muted);
}

.trend-legend-dot {
  width: 10px;
  height: 10px;
  border-radius: 2px;
}

.trend-legend-sent { background: var(--success-500, #22c55e); }
.trend-legend-failed { background: var(--danger-400, #f87171); }

/* Bounce type colors */
.bounce-bar-hard { background: var(--danger-500, #ef4444); }
.bounce-bar-soft { background: var(--warning-500, #f59e0b); }
.bounce-bar-complaint { background: var(--purple-500, #a855f7); }
.bounce-legend-hard { background: var(--danger-500, #ef4444); }
.bounce-legend-soft { background: var(--warning-500, #f59e0b); }
.bounce-legend-complaint { background: var(--purple-500, #a855f7); }

/* Latency percentiles */
.latency-grid {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 12px;
}

@media (max-width: 768px) {
  .latency-grid {
    grid-template-columns: repeat(3, 1fr);
  }
}

@media (max-width: 480px) {
  .latency-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

.latency-card {
  text-align: center;
  padding: 16px 8px;
  background: var(--bg-secondary);
  border-radius: 8px;
}

.latency-value {
  font-size: 22px;
  font-weight: 700;
  color: var(--text-primary);
}

.latency-value-tail {
  color: var(--warning-600, #ca8a04);
}

.latency-label {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 4px;
}

/* Provider breakdown */
.provider-table {
  display: grid;
  gap: 6px;
}

.provider-row {
  display: grid;
  grid-template-columns: minmax(140px, 1.4fr) minmax(200px, 3fr) 80px 80px 80px;
  align-items: center;
  gap: 12px;
  padding: 8px 0;
  border-bottom: 1px solid var(--border-color, #e5e7eb);
}

.provider-row:last-child {
  border-bottom: none;
}

.provider-head {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--text-muted);
  font-weight: 600;
  padding-bottom: 6px;
}

.provider-cell {
  font-size: 13px;
  color: var(--text-primary);
}

.provider-name {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 500;
}

.provider-bar-cell {
  display: flex;
  align-items: center;
  gap: 10px;
}

.provider-bar-cell .breakdown-bar-track {
  flex: 1;
}

.provider-pct {
  font-size: 12px;
  color: var(--text-muted);
  min-width: 44px;
  text-align: right;
}

.provider-num {
  text-align: right;
  font-variant-numeric: tabular-nums;
}

.provider-rate {
  font-weight: 600;
}

.provider-rate.rate-warning {
  color: var(--warning-600, #ca8a04);
}

.provider-rate.rate-danger {
  color: var(--danger-600, #dc2626);
}

@media (max-width: 640px) {
  .provider-row {
    grid-template-columns: minmax(120px, 1fr) 60px 60px 60px;
  }
  .provider-bar-cell {
    grid-column: 1 / -1;
    order: 99;
    margin-top: 4px;
  }
}
</style>
