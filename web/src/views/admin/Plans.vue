<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { plansApi } from '../../api/plans'
import type { Plan, PlanInput, Pageable } from '../../api/types'
import { useNotificationStore } from '../../stores/notification'
import { useConfirm } from '../../composables/useConfirm'
import { useModalSafeClose } from '../../composables/useModalSafeClose'

const router = useRouter()
const notify = useNotificationStore()
const { confirm } = useConfirm()

const plans = ref<Plan[]>([])
const pageable = ref<Pageable | null>(null)
const loading = ref(true)
const currentPage = ref(0)

const showModal = ref(false)
const editing = ref<Plan | null>(null)
const saving = ref(false)

const defaultForm = (): PlanInput => ({
  name: '',
  description: '',
  is_default: false,
  daily_rate_limit: 0,
  hourly_rate_limit: 0,
  max_attachment_size_mb: 0,
  max_batch_size: 0,
  max_api_keys: 0,
  max_domains: 0,
  max_smtp_servers: 0,
  max_workspaces: 0,
  email_log_retention_days: 0,
})

const form = ref<PlanInput>(defaultForm())

async function fetchPlans() {
  loading.value = true
  try {
    const res = await plansApi.list(currentPage.value)
    plans.value = res.data.data
    pageable.value = res.data.pageable
  } catch {
    notify.error('Failed to load plans')
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editing.value = null
  form.value = defaultForm()
  showModal.value = true
}

function openEdit(plan: Plan) {
  editing.value = plan
  form.value = {
    name: plan.name,
    description: plan.description,
    is_default: plan.is_default,
    daily_rate_limit: plan.daily_rate_limit,
    hourly_rate_limit: plan.hourly_rate_limit,
    max_attachment_size_mb: plan.max_attachment_size_mb,
    max_batch_size: plan.max_batch_size,
    max_api_keys: plan.max_api_keys,
    max_domains: plan.max_domains,
    max_smtp_servers: plan.max_smtp_servers,
    max_workspaces: plan.max_workspaces,
    email_log_retention_days: plan.email_log_retention_days,
  }
  showModal.value = true
}

async function save() {
  saving.value = true
  try {
    if (editing.value) {
      await plansApi.update(editing.value.id, form.value)
      notify.success('Plan updated')
    } else {
      await plansApi.create(form.value)
      notify.success('Plan created')
    }
    showModal.value = false
    await fetchPlans()
  } catch (e: any) {
    const message = e?.response?.data?.error?.message || 'Failed to save plan'
    notify.error(message)
  } finally {
    saving.value = false
  }
}

async function deletePlan(plan: Plan) {
  const confirmed = await confirm({
    title: 'Delete Plan',
    message: `Are you sure you want to delete "${plan.name}"? Workspaces using this plan will fall back to the default plan or global settings.`,
    confirmText: 'Delete',
    variant: 'danger',
  })
  if (!confirmed) return
  try {
    await plansApi.delete(plan.id, true)
    notify.success('Plan deleted')
    await fetchPlans()
  } catch (e: any) {
    const message = e?.response?.data?.error?.message || 'Failed to delete plan'
    notify.error(message)
  }
}

async function setDefault(plan: Plan) {
  try {
    await plansApi.setDefault(plan.id)
    notify.success(`"${plan.name}" set as default`)
    await fetchPlans()
  } catch {
    notify.error('Failed to set default plan')
  }
}

function formatLimit(value: number): string {
  return value === 0 ? 'Unlimited' : value.toLocaleString()
}

function prevPage() {
  if (currentPage.value > 0) {
    currentPage.value--
    fetchPlans()
  }
}

function nextPage() {
  if (pageable.value && currentPage.value < pageable.value.total_pages - 1) {
    currentPage.value++
    fetchPlans()
  }
}

const { watchClickStart, confirmClickEnd } = useModalSafeClose(() => {
  showModal.value = false
})

onMounted(fetchPlans)
</script>

<template>
  <div>
    <div class="page-header">
      <div>
        <h1>Plans</h1>
        <p class="page-description">Manage usage plans and packages. Plans control rate limits, resource quotas, and retention policies for workspaces.</p>
      </div>
      <button class="btn btn-primary" @click="openCreate">Create Plan</button>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <template v-else>
      <div class="card">
        <div class="table-wrapper" v-if="plans.length > 0">
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Status</th>
                <th>Rate Limits</th>
                <th>Resources</th>
                <th>Workspaces</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="plan in plans" :key="plan.id">
                <td>
                  <strong>{{ plan.name }}</strong>
                  <span v-if="plan.is_default" class="badge badge-info" style="margin-left: 8px">Default</span>
                  <div v-if="plan.description" class="text-muted" style="font-size: 13px; margin-top: 2px">{{ plan.description }}</div>
                </td>
                <td>
                  <span class="badge badge-success" v-if="plan.is_active">Active</span>
                  <span class="badge badge-neutral" v-else>Inactive</span>
                </td>
                <td>
                  <div style="font-size: 13px">
                    <div>{{ formatLimit(plan.hourly_rate_limit) }}/hr</div>
                    <div>{{ formatLimit(plan.daily_rate_limit) }}/day</div>
                  </div>
                </td>
                <td>
                  <div style="font-size: 13px">
                    <div>Keys: {{ formatLimit(plan.max_api_keys) }}</div>
                    <div>Domains: {{ formatLimit(plan.max_domains) }}</div>
                    <div>SMTP: {{ formatLimit(plan.max_smtp_servers) }}</div>
                  </div>
                </td>
                <td>{{ formatLimit(plan.max_workspaces) }}</td>
                <td>
                  <div class="flex gap-2">
                    <button class="btn btn-secondary btn-sm" @click="router.push(`/admin/plans/${plan.id}`)">View</button>
                    <button class="btn btn-secondary btn-sm" @click="openEdit(plan)">Edit</button>
                    <button v-if="!plan.is_default" class="btn btn-secondary btn-sm" @click="setDefault(plan)">Set Default</button>
                    <button class="btn btn-danger btn-sm" @click="deletePlan(plan)">Delete</button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div v-else class="empty-state">
          <h3>No plans</h3>
          <p>Create a plan to define usage limits and resource quotas for workspaces.</p>
        </div>

        <div v-if="pageable && !pageable.empty" class="pagination">
          <span class="pagination-info">
            Page {{ pageable.current_page + 1 }} of {{ pageable.total_pages }}
            ({{ pageable.total_elements }} total)
          </span>
          <div class="pagination-buttons">
            <button class="btn btn-secondary btn-sm" :disabled="currentPage === 0" @click="prevPage">Previous</button>
            <button class="btn btn-secondary btn-sm" :disabled="currentPage >= pageable.total_pages - 1" @click="nextPage">Next</button>
          </div>
        </div>
      </div>
    </template>

    <!-- Create/Edit Modal -->
    <div v-if="showModal" class="modal-overlay" @mousedown="watchClickStart" @mouseup="confirmClickEnd">
      <div class="modal" @mousedown.stop @mouseup.stop style="max-width: 640px">
        <div class="modal-header">
          <h3>{{ editing ? 'Edit Plan' : 'Create Plan' }}</h3>
        </div>
        <form @submit.prevent="save">
          <div class="modal-body">
            <div class="form-group">
              <label class="form-label">Name</label>
              <input v-model="form.name" type="text" class="form-input" placeholder="e.g. Starter, Pro, Enterprise" required />
            </div>
            <div class="form-group">
              <label class="form-label">Description</label>
              <input v-model="form.description" type="text" class="form-input" placeholder="Brief description of this plan" />
            </div>
            <div class="form-group">
              <label class="form-label">
                <input type="checkbox" v-model="form.is_default" style="margin-right: 6px" />
                Set as default plan
              </label>
              <span class="form-hint">New workspaces without a plan will use the default plan's limits.</span>
            </div>

            <div style="margin: 16px 0 8px; font-weight: 600; font-size: 14px">Rate Limits</div>
            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 12px">
              <div class="form-group">
                <label class="form-label">Hourly Rate Limit</label>
                <input v-model.number="form.hourly_rate_limit" type="number" class="form-input" min="0" />
                <span class="form-hint">0 = unlimited</span>
              </div>
              <div class="form-group">
                <label class="form-label">Daily Rate Limit</label>
                <input v-model.number="form.daily_rate_limit" type="number" class="form-input" min="0" />
                <span class="form-hint">0 = unlimited</span>
              </div>
            </div>

            <div style="margin: 16px 0 8px; font-weight: 600; font-size: 14px">Email Constraints</div>
            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 12px">
              <div class="form-group">
                <label class="form-label">Max Attachment Size (MB)</label>
                <input v-model.number="form.max_attachment_size_mb" type="number" class="form-input" min="0" />
                <span class="form-hint">0 = unlimited</span>
              </div>
              <div class="form-group">
                <label class="form-label">Max Batch Size</label>
                <input v-model.number="form.max_batch_size" type="number" class="form-input" min="0" />
                <span class="form-hint">0 = unlimited</span>
              </div>
            </div>

            <div style="margin: 16px 0 8px; font-weight: 600; font-size: 14px">Resource Limits</div>
            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 12px">
              <div class="form-group">
                <label class="form-label">Max API Keys</label>
                <input v-model.number="form.max_api_keys" type="number" class="form-input" min="0" />
                <span class="form-hint">0 = unlimited</span>
              </div>
              <div class="form-group">
                <label class="form-label">Max Domains</label>
                <input v-model.number="form.max_domains" type="number" class="form-input" min="0" />
                <span class="form-hint">0 = unlimited</span>
              </div>
              <div class="form-group">
                <label class="form-label">Max SMTP Servers</label>
                <input v-model.number="form.max_smtp_servers" type="number" class="form-input" min="0" />
                <span class="form-hint">0 = unlimited</span>
              </div>
              <div class="form-group">
                <label class="form-label">Max Workspaces</label>
                <input v-model.number="form.max_workspaces" type="number" class="form-input" min="0" />
                <span class="form-hint">0 = unlimited</span>
              </div>
            </div>

            <div style="margin: 16px 0 8px; font-weight: 600; font-size: 14px">Data Retention</div>
            <div class="form-group">
              <label class="form-label">Email Log Retention (days)</label>
              <input v-model.number="form.email_log_retention_days" type="number" class="form-input" min="0" />
              <span class="form-hint">0 = use global retention setting</span>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" @click="showModal = false">Cancel</button>
            <button type="submit" class="btn btn-primary" :disabled="saving">
              {{ saving ? 'Saving...' : (editing ? 'Update' : 'Create') }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>
