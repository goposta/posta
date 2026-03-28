<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { campaignsApi } from '../../api/campaigns'
import type { Campaign, CampaignMessage, CampaignMessageStatus, CampaignAnalyticsData, Pageable } from '../../api/types'
import { useNotificationStore } from '../../stores/notification'
import { useConfirm } from '../../composables/useConfirm'

const route = useRoute()
const router = useRouter()
const notify = useNotificationStore()
const { confirm } = useConfirm()

const campaignId = Number(route.params.id)
const campaign = ref<Campaign | null>(null)
const messages = ref<CampaignMessage[]>([])
const pageable = ref<Pageable>({ current_page: 0, size: 20, total_pages: 0, total_elements: 0, empty: true })
const loading = ref(true)
const messagesLoading = ref(false)
const messageStatusFilter = ref('')
const actionLoading = ref(false)

// Analytics
const activeTab = ref<'messages' | 'analytics'>('messages')
const analyticsData = ref<CampaignAnalyticsData | null>(null)
const analyticsLoading = ref(false)

const canSend = computed(() => campaign.value?.status === 'draft')
const canPause = computed(() => campaign.value?.status === 'sending')
const canResume = computed(() => campaign.value?.status === 'paused')
const canCancel = computed(() => ['sending', 'paused', 'scheduled'].includes(campaign.value?.status ?? ''))
const canDelete = computed(() => ['draft', 'cancelled'].includes(campaign.value?.status ?? ''))
const showAnalyticsTab = computed(() => ['sending', 'sent', 'paused', 'cancelled'].includes(campaign.value?.status ?? ''))

async function loadCampaign() {
  loading.value = true
  try {
    const res = await campaignsApi.get(campaignId)
    campaign.value = res.data.data
  } catch {
    notify.error('Failed to load campaign')
    router.push('/campaigns')
  } finally {
    loading.value = false
  }
}

async function loadMessages(page = 0) {
  messagesLoading.value = true
  try {
    const res = await campaignsApi.listMessages(campaignId, page, pageable.value.size, messageStatusFilter.value || undefined)
    messages.value = res.data.data ?? []
    pageable.value = res.data.pageable
  } catch {
    notify.error('Failed to load messages')
  } finally {
    messagesLoading.value = false
  }
}

async function loadAnalytics() {
  analyticsLoading.value = true
  try {
    const res = await campaignsApi.analytics(campaignId)
    analyticsData.value = res.data.data
  } catch {
    notify.error('Failed to load analytics')
  } finally {
    analyticsLoading.value = false
  }
}

function switchTab(tab: 'messages' | 'analytics') {
  activeTab.value = tab
  if (tab === 'analytics' && !analyticsData.value) {
    loadAnalytics()
  }
}

function switchMessageStatus(status: string) {
  messageStatusFilter.value = status
  loadMessages(0)
}

async function sendCampaign() {
  const confirmed = await confirm({
    title: 'Send Campaign',
    message: `Are you sure you want to send "${campaign.value?.name}"? This will start delivering emails to all subscribers in the list.`,
    confirmText: 'Send',
    variant: 'primary',
  })
  if (!confirmed) return
  actionLoading.value = true
  try {
    await campaignsApi.send(campaignId)
    notify.success('Campaign is being sent')
    await loadCampaign()
    await loadMessages(0)
  } catch (e: any) {
    notify.error(e?.response?.data?.error?.message || 'Failed to send campaign')
  } finally {
    actionLoading.value = false
  }
}

async function pauseCampaign() {
  actionLoading.value = true
  try {
    await campaignsApi.pause(campaignId)
    notify.success('Campaign paused')
    await loadCampaign()
  } catch (e: any) {
    notify.error(e?.response?.data?.error?.message || 'Failed to pause campaign')
  } finally {
    actionLoading.value = false
  }
}

async function resumeCampaign() {
  actionLoading.value = true
  try {
    await campaignsApi.resume(campaignId)
    notify.success('Campaign resumed')
    await loadCampaign()
  } catch (e: any) {
    notify.error(e?.response?.data?.error?.message || 'Failed to resume campaign')
  } finally {
    actionLoading.value = false
  }
}

async function cancelCampaign() {
  const confirmed = await confirm({
    title: 'Cancel Campaign',
    message: `Are you sure you want to cancel "${campaign.value?.name}"? Pending messages will not be sent.`,
    confirmText: 'Cancel Campaign',
    variant: 'danger',
  })
  if (!confirmed) return
  actionLoading.value = true
  try {
    await campaignsApi.cancel(campaignId)
    notify.success('Campaign cancelled')
    await loadCampaign()
  } catch (e: any) {
    notify.error(e?.response?.data?.error?.message || 'Failed to cancel campaign')
  } finally {
    actionLoading.value = false
  }
}

async function deleteCampaign() {
  const confirmed = await confirm({
    title: 'Delete Campaign',
    message: `Are you sure you want to delete "${campaign.value?.name}"? This action cannot be undone.`,
    confirmText: 'Delete',
    variant: 'danger',
  })
  if (!confirmed) return
  try {
    await campaignsApi.delete(campaignId)
    notify.success('Campaign deleted')
    router.push('/campaigns')
  } catch (e: any) {
    notify.error(e?.response?.data?.error?.message || 'Failed to delete campaign')
  }
}

function statusBadgeClass(status: string): string {
  switch (status) {
    case 'draft': return 'badge badge-neutral'
    case 'scheduled': return 'badge badge-info'
    case 'sending': return 'badge badge-primary'
    case 'sent': return 'badge badge-success'
    case 'paused': return 'badge badge-warning'
    case 'cancelled': case 'failed': return 'badge badge-danger'
    case 'pending': return 'badge badge-neutral'
    case 'queued': return 'badge badge-info'
    case 'skipped': return 'badge badge-warning'
    default: return 'badge'
  }
}

function formatDate(dateStr?: string): string {
  if (!dateStr) return '-'
  return new Date(dateStr).toLocaleString(undefined, {
    year: 'numeric', month: 'short', day: 'numeric',
    hour: '2-digit', minute: '2-digit',
  })
}

function formatRate(value: number): string {
  return value.toFixed(1) + '%'
}

function maxSeriesCount(series: Array<{ time: string; count: number }>): number {
  if (series.length === 0) return 1
  return Math.max(...series.map(p => p.count), 1)
}

const messageStatusTabs: { label: string; value: string }[] = [
  { label: 'All', value: '' },
  { label: 'Pending', value: 'pending' },
  { label: 'Queued', value: 'queued' },
  { label: 'Sent', value: 'sent' },
  { label: 'Failed', value: 'failed' },
  { label: 'Skipped', value: 'skipped' },
]

onMounted(async () => {
  await loadCampaign()
  await loadMessages()
})
</script>

<template>
  <div>
    <div class="page-header">
      <div>
        <button class="btn btn-secondary btn-sm" @click="router.push('/campaigns')" style="margin-bottom: 8px;">&larr; Back to Campaigns</button>
        <h1 v-if="campaign">{{ campaign.name }}</h1>
      </div>
      <div v-if="campaign" style="display: flex; gap: 8px; align-items: flex-start;">
        <button v-if="canSend" class="btn btn-primary" :disabled="actionLoading" @click="sendCampaign">Send</button>
        <button v-if="canPause" class="btn btn-warning" :disabled="actionLoading" @click="pauseCampaign">Pause</button>
        <button v-if="canResume" class="btn btn-primary" :disabled="actionLoading" @click="resumeCampaign">Resume</button>
        <button v-if="canCancel" class="btn btn-danger" :disabled="actionLoading" @click="cancelCampaign">Cancel</button>
        <button v-if="canDelete" class="btn btn-danger" @click="deleteCampaign">Delete</button>
      </div>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <template v-else-if="campaign">
      <!-- Campaign Info -->
      <div class="card" style="margin-bottom: 24px;">
        <div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 16px;">
          <div>
            <div style="font-size: 0.85em; color: var(--text-secondary);">Status</div>
            <span :class="statusBadgeClass(campaign.status)" style="margin-top: 4px;">{{ campaign.status }}</span>
          </div>
          <div>
            <div style="font-size: 0.85em; color: var(--text-secondary);">Subject</div>
            <div style="font-weight: 500;">{{ campaign.subject }}</div>
          </div>
          <div>
            <div style="font-size: 0.85em; color: var(--text-secondary);">From</div>
            <div>{{ campaign.from_name ? `${campaign.from_name} <${campaign.from_email}>` : campaign.from_email }}</div>
          </div>
          <div>
            <div style="font-size: 0.85em; color: var(--text-secondary);">Send Rate</div>
            <div>{{ campaign.send_rate > 0 ? `${campaign.send_rate} msgs/min` : 'Unlimited' }}</div>
          </div>
          <div>
            <div style="font-size: 0.85em; color: var(--text-secondary);">Created</div>
            <div>{{ formatDate(campaign.created_at) }}</div>
          </div>
          <div v-if="campaign.started_at">
            <div style="font-size: 0.85em; color: var(--text-secondary);">Started</div>
            <div>{{ formatDate(campaign.started_at) }}</div>
          </div>
          <div v-if="campaign.completed_at">
            <div style="font-size: 0.85em; color: var(--text-secondary);">Completed</div>
            <div>{{ formatDate(campaign.completed_at) }}</div>
          </div>
          <div v-if="campaign.scheduled_at">
            <div style="font-size: 0.85em; color: var(--text-secondary);">Scheduled</div>
            <div>{{ formatDate(campaign.scheduled_at) }}</div>
          </div>
        </div>
      </div>

      <!-- Stats -->
      <div v-if="campaign.stats && campaign.stats.total > 0" class="card" style="margin-bottom: 24px;">
        <h3 style="margin-bottom: 12px;">Delivery Stats</h3>
        <div style="display: flex; gap: 24px; flex-wrap: wrap;">
          <div style="text-align: center;">
            <div style="font-size: 1.5em; font-weight: 600;">{{ campaign.stats.total }}</div>
            <div style="font-size: 0.85em; color: var(--text-secondary);">Total</div>
          </div>
          <div style="text-align: center;">
            <div style="font-size: 1.5em; font-weight: 600; color: var(--success-600);">{{ campaign.stats.sent }}</div>
            <div style="font-size: 0.85em; color: var(--text-secondary);">Sent</div>
          </div>
          <div style="text-align: center;">
            <div style="font-size: 1.5em; font-weight: 600;">{{ campaign.stats.queued }}</div>
            <div style="font-size: 0.85em; color: var(--text-secondary);">Queued</div>
          </div>
          <div style="text-align: center;">
            <div style="font-size: 1.5em; font-weight: 600;">{{ campaign.stats.pending }}</div>
            <div style="font-size: 0.85em; color: var(--text-secondary);">Pending</div>
          </div>
          <div style="text-align: center;">
            <div style="font-size: 1.5em; font-weight: 600; color: var(--danger-600);">{{ campaign.stats.failed }}</div>
            <div style="font-size: 0.85em; color: var(--text-secondary);">Failed</div>
          </div>
          <div style="text-align: center;">
            <div style="font-size: 1.5em; font-weight: 600;">{{ campaign.stats.skipped }}</div>
            <div style="font-size: 0.85em; color: var(--text-secondary);">Skipped</div>
          </div>
        </div>

        <!-- Progress bar -->
        <div v-if="campaign.stats.total > 0" style="margin-top: 16px;">
          <div style="width: 100%; height: 8px; background: var(--bg-secondary); border-radius: 4px; overflow: hidden; display: flex;">
            <div :style="{ width: (campaign.stats.sent / campaign.stats.total * 100) + '%', background: 'var(--success-600)' }"></div>
            <div :style="{ width: (campaign.stats.failed / campaign.stats.total * 100) + '%', background: 'var(--danger-600)' }"></div>
            <div :style="{ width: (campaign.stats.queued / campaign.stats.total * 100) + '%', background: 'var(--primary-600)' }"></div>
          </div>
        </div>
      </div>

      <!-- Tab navigation -->
      <div class="tabs" style="margin-bottom: 16px;">
        <button
          class="btn btn-sm"
          :class="activeTab === 'messages' ? 'btn-primary' : 'btn-secondary'"
          @click="switchTab('messages')"
          style="margin-right: 4px;"
        >
          Messages
        </button>
        <button
          v-if="showAnalyticsTab"
          class="btn btn-sm"
          :class="activeTab === 'analytics' ? 'btn-primary' : 'btn-secondary'"
          @click="switchTab('analytics')"
          style="margin-right: 4px;"
        >
          Analytics
        </button>
      </div>

      <!-- Messages Tab -->
      <div v-if="activeTab === 'messages'" class="card">
        <h3 style="margin-bottom: 12px;">Messages</h3>

        <div class="tabs" style="margin-bottom: 16px;">
          <button
            v-for="tab in messageStatusTabs"
            :key="tab.value"
            class="btn btn-sm"
            :class="messageStatusFilter === tab.value ? 'btn-primary' : 'btn-secondary'"
            @click="switchMessageStatus(tab.value)"
            style="margin-right: 4px;"
          >
            {{ tab.label }}
          </button>
        </div>

        <div v-if="messagesLoading" class="loading-page">
          <div class="spinner"></div>
        </div>

        <template v-else>
          <div v-if="messages.length === 0" class="empty-state">
            <p>No messages yet.</p>
          </div>

          <template v-else>
            <div class="table-wrapper">
              <table>
                <thead>
                  <tr>
                    <th>Subscriber</th>
                    <th>Status</th>
                    <th>Error</th>
                    <th>Sent At</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="msg in messages" :key="msg.id">
                    <td>{{ msg.subscriber?.email ?? `#${msg.subscriber_id}` }}</td>
                    <td><span :class="statusBadgeClass(msg.status)">{{ msg.status }}</span></td>
                    <td style="max-width: 300px; overflow: hidden; text-overflow: ellipsis;">{{ msg.error_message || '-' }}</td>
                    <td>{{ formatDate(msg.sent_at) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>

            <div class="pagination">
              <span class="pagination-info">
                Page {{ pageable.current_page + 1 }} of {{ pageable.total_pages }} ({{ pageable.total_elements }} messages)
              </span>
              <div class="pagination-buttons">
                <button class="btn btn-secondary btn-sm" :disabled="pageable.current_page === 0" @click="loadMessages(pageable.current_page - 1)">Previous</button>
                <button class="btn btn-secondary btn-sm" :disabled="pageable.current_page >= pageable.total_pages - 1" @click="loadMessages(pageable.current_page + 1)">Next</button>
              </div>
            </div>
          </template>
        </template>
      </div>

      <!-- Analytics Tab -->
      <div v-if="activeTab === 'analytics'">
        <div v-if="analyticsLoading" class="loading-page">
          <div class="spinner"></div>
        </div>

        <template v-else-if="analyticsData">
          <!-- Rate Cards -->
          <div class="card" style="margin-bottom: 24px;">
            <h3 style="margin-bottom: 16px;">Engagement Rates</h3>
            <div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(160px, 1fr)); gap: 16px;">
              <div style="text-align: center; padding: 16px; background: var(--bg-secondary); border-radius: 8px;">
                <div style="font-size: 1.8em; font-weight: 700; color: var(--success-600);">{{ formatRate(analyticsData.analytics.delivery_rate) }}</div>
                <div style="font-size: 0.85em; color: var(--text-secondary); margin-top: 4px;">Delivery Rate</div>
                <div style="font-size: 0.75em; color: var(--text-secondary);">{{ analyticsData.analytics.sent_messages }} / {{ analyticsData.analytics.total_messages }}</div>
              </div>
              <div style="text-align: center; padding: 16px; background: var(--bg-secondary); border-radius: 8px;">
                <div style="font-size: 1.8em; font-weight: 700; color: var(--primary-600);">{{ formatRate(analyticsData.analytics.open_rate) }}</div>
                <div style="font-size: 0.85em; color: var(--text-secondary); margin-top: 4px;">Open Rate</div>
                <div style="font-size: 0.75em; color: var(--text-secondary);">{{ analyticsData.analytics.opened_messages }} opened</div>
              </div>
              <div style="text-align: center; padding: 16px; background: var(--bg-secondary); border-radius: 8px;">
                <div style="font-size: 1.8em; font-weight: 700; color: var(--primary-600);">{{ formatRate(analyticsData.analytics.click_rate) }}</div>
                <div style="font-size: 0.85em; color: var(--text-secondary); margin-top: 4px;">Click Rate</div>
                <div style="font-size: 0.75em; color: var(--text-secondary);">{{ analyticsData.analytics.clicked_messages }} clicked</div>
              </div>
              <div style="text-align: center; padding: 16px; background: var(--bg-secondary); border-radius: 8px;">
                <div style="font-size: 1.8em; font-weight: 700; color: var(--danger-600);">{{ formatRate(analyticsData.analytics.bounce_rate) }}</div>
                <div style="font-size: 0.85em; color: var(--text-secondary); margin-top: 4px;">Bounce Rate</div>
                <div style="font-size: 0.75em; color: var(--text-secondary);">{{ analyticsData.analytics.bounced_messages }} bounced</div>
              </div>
              <div style="text-align: center; padding: 16px; background: var(--bg-secondary); border-radius: 8px;">
                <div style="font-size: 1.8em; font-weight: 700; color: var(--warning-600);">{{ formatRate(analyticsData.analytics.unsubscribe_rate) }}</div>
                <div style="font-size: 0.85em; color: var(--text-secondary); margin-top: 4px;">Unsubscribe Rate</div>
                <div style="font-size: 0.75em; color: var(--text-secondary);">{{ analyticsData.analytics.unsubscribed }} unsubscribed</div>
              </div>
            </div>
          </div>

          <!-- Link Click Table -->
          <div v-if="analyticsData.links && analyticsData.links.length > 0" class="card" style="margin-bottom: 24px;">
            <h3 style="margin-bottom: 12px;">Link Clicks</h3>
            <div class="table-wrapper">
              <table>
                <thead>
                  <tr>
                    <th>URL</th>
                    <th style="text-align: right; width: 120px;">Clicks</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="link in analyticsData.links" :key="link.id">
                    <td style="max-width: 500px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">
                      <a :href="link.original_url" target="_blank" rel="noopener" style="color: var(--primary-600);">{{ link.original_url }}</a>
                    </td>
                    <td style="text-align: right; font-weight: 600;">{{ link.click_count }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <!-- Time Series: Opens -->
          <div v-if="analyticsData.open_series && analyticsData.open_series.length > 0" class="card" style="margin-bottom: 24px;">
            <h3 style="margin-bottom: 12px;">Opens Over Time</h3>
            <div style="display: flex; align-items: flex-end; gap: 2px; height: 120px; padding: 8px 0;">
              <div
                v-for="(point, idx) in analyticsData.open_series"
                :key="'open-' + idx"
                :title="point.time + ': ' + point.count + ' opens'"
                :style="{
                  flex: '1',
                  minWidth: '4px',
                  maxWidth: '32px',
                  height: Math.max((point.count / maxSeriesCount(analyticsData.open_series)) * 100, 2) + '%',
                  background: 'var(--primary-600)',
                  borderRadius: '2px 2px 0 0',
                }"
              ></div>
            </div>
            <div style="display: flex; justify-content: space-between; font-size: 0.7em; color: var(--text-secondary); margin-top: 4px;">
              <span>{{ analyticsData.open_series[0]?.time }}</span>
              <span>{{ analyticsData.open_series[analyticsData.open_series.length - 1]?.time }}</span>
            </div>
          </div>

          <!-- Time Series: Clicks -->
          <div v-if="analyticsData.click_series && analyticsData.click_series.length > 0" class="card" style="margin-bottom: 24px;">
            <h3 style="margin-bottom: 12px;">Clicks Over Time</h3>
            <div style="display: flex; align-items: flex-end; gap: 2px; height: 120px; padding: 8px 0;">
              <div
                v-for="(point, idx) in analyticsData.click_series"
                :key="'click-' + idx"
                :title="point.time + ': ' + point.count + ' clicks'"
                :style="{
                  flex: '1',
                  minWidth: '4px',
                  maxWidth: '32px',
                  height: Math.max((point.count / maxSeriesCount(analyticsData.click_series)) * 100, 2) + '%',
                  background: 'var(--success-600)',
                  borderRadius: '2px 2px 0 0',
                }"
              ></div>
            </div>
            <div style="display: flex; justify-content: space-between; font-size: 0.7em; color: var(--text-secondary); margin-top: 4px;">
              <span>{{ analyticsData.click_series[0]?.time }}</span>
              <span>{{ analyticsData.click_series[analyticsData.click_series.length - 1]?.time }}</span>
            </div>
          </div>
        </template>

        <div v-else class="card">
          <div class="empty-state">
            <p>No analytics data available yet.</p>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>
