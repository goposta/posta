<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { campaignsApi } from '../../api/campaigns'
import { subscriberListsApi } from '../../api/subscriberLists'
import type { Campaign, CampaignStatus, Pageable, SubscriberListItem } from '../../api/types'
import { useNotificationStore } from '../../stores/notification'
import { useConfirm } from '../../composables/useConfirm'
import { useModalSafeClose } from '../../composables/useModalSafeClose'

const router = useRouter()
const notify = useNotificationStore()
const { confirm } = useConfirm()

const campaigns = ref<Campaign[]>([])
const pageable = ref<Pageable>({ current_page: 0, size: 20, total_pages: 0, total_elements: 0, empty: true })
const loading = ref(true)
const statusFilter = ref<string>('')

const statusTabs: { label: string; value: string }[] = [
  { label: 'All', value: '' },
  { label: 'Draft', value: 'draft' },
  { label: 'Scheduled', value: 'scheduled' },
  { label: 'Sending', value: 'sending' },
  { label: 'Sent', value: 'sent' },
  { label: 'Paused', value: 'paused' },
  { label: 'Cancelled', value: 'cancelled' },
]

// Create modal
const showModal = ref(false)
const formName = ref('')
const formSubject = ref('')
const formFromEmail = ref('')
const formFromName = ref('')
const formTemplateId = ref<number | null>(null)
const formListId = ref<number | null>(null)
const formSendRate = ref(0)
const saving = ref(false)

// Lists for the create form
const lists = ref<SubscriberListItem[]>([])
const templates = ref<{ id: number; name: string }[]>([])

async function loadCampaigns(page = 0) {
  loading.value = true
  try {
    const res = await campaignsApi.list(page, pageable.value.size, statusFilter.value || undefined)
    campaigns.value = res.data.data ?? []
    pageable.value = res.data.pageable
  } catch {
    notify.error('Failed to load campaigns')
  } finally {
    loading.value = false
  }
}

function switchStatus(status: string) {
  statusFilter.value = status
  loadCampaigns(0)
}

async function loadFormData() {
  try {
    const [listsRes] = await Promise.all([
      subscriberListsApi.list(0, 100),
    ])
    lists.value = listsRes.data.data ?? []
  } catch {
    // non-critical
  }
  try {
    const { default: api } = await import('../../api/client')
    const res = await api.get<any>('/users/me/templates', { params: { page: 0, size: 100 } })
    templates.value = (res.data.data ?? []).map((t: any) => ({ id: t.id, name: t.name }))
  } catch {
    // non-critical
  }
}

function openCreate() {
  formName.value = ''
  formSubject.value = ''
  formFromEmail.value = ''
  formFromName.value = ''
  formTemplateId.value = null
  formListId.value = null
  formSendRate.value = 0
  showModal.value = true
  loadFormData()
}

async function saveCampaign() {
  if (!formName.value.trim() || !formSubject.value.trim() || !formFromEmail.value.trim() || !formTemplateId.value || !formListId.value) {
    notify.error('Please fill in all required fields')
    return
  }
  saving.value = true
  try {
    await campaignsApi.create({
      name: formName.value.trim(),
      subject: formSubject.value.trim(),
      from_email: formFromEmail.value.trim(),
      from_name: formFromName.value.trim(),
      template_id: formTemplateId.value,
      list_id: formListId.value,
      send_rate: formSendRate.value,
    })
    notify.success('Campaign created')
    showModal.value = false
    await loadCampaigns(pageable.value.current_page)
  } catch (e: any) {
    notify.error(e?.response?.data?.error?.message || 'Failed to create campaign')
  } finally {
    saving.value = false
  }
}

async function deleteCampaign(campaign: Campaign) {
  const confirmed = await confirm({
    title: 'Delete Campaign',
    message: `Are you sure you want to delete "${campaign.name}"? This action cannot be undone.`,
    confirmText: 'Delete',
    variant: 'danger',
  })
  if (!confirmed) return
  try {
    await campaignsApi.delete(campaign.id)
    notify.success('Campaign deleted')
    await loadCampaigns(pageable.value.current_page)
  } catch (e: any) {
    notify.error(e?.response?.data?.error?.message || 'Failed to delete campaign')
  }
}

function statusBadgeClass(status: CampaignStatus): string {
  switch (status) {
    case 'draft': return 'badge badge-neutral'
    case 'scheduled': return 'badge badge-info'
    case 'sending': return 'badge badge-primary'
    case 'sent': return 'badge badge-success'
    case 'paused': return 'badge badge-warning'
    case 'cancelled': return 'badge badge-danger'
    default: return 'badge'
  }
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
}

function statsLabel(campaign: Campaign): string {
  if (!campaign.stats) return '-'
  const s = campaign.stats
  if (s.total === 0) return '0'
  return `${s.sent}/${s.total} sent`
}

const { watchClickStart, confirmClickEnd } = useModalSafeClose(() => {
  showModal.value = false
})

onMounted(() => loadCampaigns())
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Campaigns</h1>
      <button class="btn btn-primary" @click="openCreate">Create Campaign</button>
    </div>

    <!-- Status filter tabs -->
    <div class="tabs" style="margin-bottom: 16px;">
      <button
        v-for="tab in statusTabs"
        :key="tab.value"
        class="btn btn-sm"
        :class="statusFilter === tab.value ? 'btn-primary' : 'btn-secondary'"
        @click="switchStatus(tab.value)"
        style="margin-right: 4px;"
      >
        {{ tab.label }}
      </button>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <div v-else class="card">
      <div v-if="campaigns.length === 0" class="empty-state">
        <h3>No Campaigns</h3>
        <p>Create a campaign to start sending emails to your subscribers.</p>
      </div>

      <template v-else>
        <div class="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Status</th>
                <th>Progress</th>
                <th>Created</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="campaign in campaigns" :key="campaign.id">
                <td>
                  <a class="link" @click="router.push(`/campaigns/${campaign.id}`)">{{ campaign.name }}</a>
                  <div style="font-size: 0.85em; color: var(--text-secondary)">{{ campaign.subject }}</div>
                </td>
                <td><span :class="statusBadgeClass(campaign.status)">{{ campaign.status }}</span></td>
                <td>{{ statsLabel(campaign) }}</td>
                <td>{{ formatDate(campaign.created_at) }}</td>
                <td>
                  <div style="display: flex; gap: 6px">
                    <button class="btn btn-secondary btn-sm" @click="router.push(`/campaigns/${campaign.id}`)">View</button>
                    <button
                      v-if="campaign.status === 'draft' || campaign.status === 'cancelled'"
                      class="btn btn-danger btn-sm"
                      @click="deleteCampaign(campaign)"
                    >Delete</button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="pagination">
          <span class="pagination-info">
            Page {{ pageable.current_page + 1 }} of {{ pageable.total_pages }} ({{ pageable.total_elements }} campaigns)
          </span>
          <div class="pagination-buttons">
            <button class="btn btn-secondary btn-sm" :disabled="pageable.current_page === 0" @click="loadCampaigns(pageable.current_page - 1)">Previous</button>
            <button class="btn btn-secondary btn-sm" :disabled="pageable.current_page >= pageable.total_pages - 1" @click="loadCampaigns(pageable.current_page + 1)">Next</button>
          </div>
        </div>
      </template>
    </div>

    <!-- Create Modal -->
    <div v-if="showModal" class="modal-overlay" @mousedown="watchClickStart" @mouseup="confirmClickEnd">
      <div class="modal" @mousedown.stop @mouseup.stop>
        <div class="modal-header">
          <h3>Create Campaign</h3>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label class="form-label">Name *</label>
            <input v-model="formName" class="form-input" placeholder="e.g. March Newsletter" />
          </div>
          <div class="form-group">
            <label class="form-label">Subject *</label>
            <input v-model="formSubject" class="form-input" placeholder="Email subject line" />
          </div>
          <div class="form-group">
            <label class="form-label">From Email *</label>
            <input v-model="formFromEmail" class="form-input" placeholder="sender@example.com" />
          </div>
          <div class="form-group">
            <label class="form-label">From Name</label>
            <input v-model="formFromName" class="form-input" placeholder="Sender Name" />
          </div>
          <div class="form-group">
            <label class="form-label">Template *</label>
            <select v-model="formTemplateId" class="form-select">
              <option :value="null" disabled>Select template</option>
              <option v-for="t in templates" :key="t.id" :value="t.id">{{ t.name }}</option>
            </select>
          </div>
          <div class="form-group">
            <label class="form-label">Subscriber List *</label>
            <select v-model="formListId" class="form-select">
              <option :value="null" disabled>Select list</option>
              <option v-for="l in lists" :key="l.id" :value="l.id">{{ l.name }} ({{ l.member_count }} members)</option>
            </select>
          </div>
          <div class="form-group">
            <label class="form-label">Send Rate (msgs/min, 0 = unlimited)</label>
            <input v-model.number="formSendRate" type="number" class="form-input" min="0" />
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="showModal = false">Cancel</button>
          <button class="btn btn-primary" :disabled="saving" @click="saveCampaign">
            {{ saving ? 'Creating...' : 'Create' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.link {
  color: var(--primary-600);
  cursor: pointer;
  font-weight: 500;
}
.link:hover {
  text-decoration: underline;
}
</style>
