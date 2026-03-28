<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { plansApi } from '../../api/plans'
import type { Plan, PlanInput } from '../../api/types'
import { useNotificationStore } from '../../stores/notification'
import { useConfirm } from '../../composables/useConfirm'
import { useModalSafeClose } from '../../composables/useModalSafeClose'

const route = useRoute()
const router = useRouter()
const notify = useNotificationStore()
const { confirm } = useConfirm()

const plan = ref<Plan | null>(null)
const loading = ref(true)

const showModal = ref(false)
const saving = ref(false)
const form = ref<PlanInput>({
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

async function fetchPlan() {
  loading.value = true
  try {
    const id = Number(route.params.id)
    const res = await plansApi.get(id)
    plan.value = res.data.data
  } catch {
    notify.error('Failed to load plan')
  } finally {
    loading.value = false
  }
}

function openEdit() {
  if (!plan.value) return
  form.value = {
    name: plan.value.name,
    description: plan.value.description,
    is_default: plan.value.is_default,
    daily_rate_limit: plan.value.daily_rate_limit,
    hourly_rate_limit: plan.value.hourly_rate_limit,
    max_attachment_size_mb: plan.value.max_attachment_size_mb,
    max_batch_size: plan.value.max_batch_size,
    max_api_keys: plan.value.max_api_keys,
    max_domains: plan.value.max_domains,
    max_smtp_servers: plan.value.max_smtp_servers,
    max_workspaces: plan.value.max_workspaces,
    email_log_retention_days: plan.value.email_log_retention_days,
  }
  showModal.value = true
}

async function save() {
  if (!plan.value) return
  saving.value = true
  try {
    await plansApi.update(plan.value.id, form.value)
    notify.success('Plan updated')
    showModal.value = false
    await fetchPlan()
  } catch (e: any) {
    const message = e?.response?.data?.error?.message || 'Failed to update plan'
    notify.error(message)
  } finally {
    saving.value = false
  }
}

async function toggleActive() {
  if (!plan.value) return
  try {
    await plansApi.update(plan.value.id, { is_active: !plan.value.is_active })
    notify.success(plan.value.is_active ? 'Plan deactivated' : 'Plan activated')
    await fetchPlan()
  } catch {
    notify.error('Failed to update plan status')
  }
}

async function setDefault() {
  if (!plan.value) return
  try {
    await plansApi.setDefault(plan.value.id)
    notify.success(`"${plan.value.name}" set as default`)
    await fetchPlan()
  } catch {
    notify.error('Failed to set default plan')
  }
}

async function deletePlan() {
  if (!plan.value) return
  const confirmed = await confirm({
    title: 'Delete Plan',
    message: `Are you sure you want to delete "${plan.value.name}"? Workspaces using this plan will fall back to the default plan or global settings.`,
    confirmText: 'Delete',
    variant: 'danger',
  })
  if (!confirmed) return
  try {
    await plansApi.delete(plan.value.id, true)
    notify.success('Plan deleted')
    router.push('/admin/plans')
  } catch (e: any) {
    const message = e?.response?.data?.error?.message || 'Failed to delete plan'
    notify.error(message)
  }
}

function formatLimit(value: number): string {
  return value === 0 ? 'Unlimited' : value.toLocaleString()
}

function formatDate(date: string | null) {
  if (!date) return '-'
  return new Date(date).toLocaleString()
}

const { watchClickStart, confirmClickEnd } = useModalSafeClose(() => {
  showModal.value = false
})

onMounted(fetchPlan)
</script>

<template>
  <div>
    <div class="page-header">
      <div>
        <h1>{{ plan?.name ?? 'Plan Details' }}</h1>
      </div>
      <div class="flex gap-2">
        <button class="btn btn-secondary" @click="router.push('/admin/plans')">Back</button>
      </div>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <template v-else-if="plan">
      <!-- Info Card -->
      <div class="card" style="margin-bottom: 24px">
        <div class="card-header">
          <h2>{{ plan.name }}</h2>
          <div class="flex gap-2">
            <span class="badge badge-success" v-if="plan.is_active">Active</span>
            <span class="badge badge-neutral" v-else>Inactive</span>
            <span class="badge badge-info" v-if="plan.is_default">Default</span>
          </div>
        </div>
        <div class="card-body">
          <div v-if="plan.description" style="margin-bottom: 16px; color: var(--text-secondary)">{{ plan.description }}</div>
          <table>
            <tbody>
              <tr>
                <td style="font-weight: 600; width: 200px">Created</td>
                <td>{{ formatDate(plan.created_at) }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600">Updated</td>
                <td>{{ formatDate(plan.updated_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- Rate Limits -->
      <div class="card" style="margin-bottom: 24px">
        <div class="card-header"><h2>Rate Limits</h2></div>
        <div class="card-body">
          <table>
            <tbody>
              <tr>
                <td style="font-weight: 600; width: 200px">Hourly Rate Limit</td>
                <td>{{ formatLimit(plan.hourly_rate_limit) }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600">Daily Rate Limit</td>
                <td>{{ formatLimit(plan.daily_rate_limit) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- Email Constraints -->
      <div class="card" style="margin-bottom: 24px">
        <div class="card-header"><h2>Email Constraints</h2></div>
        <div class="card-body">
          <table>
            <tbody>
              <tr>
                <td style="font-weight: 600; width: 200px">Max Attachment Size</td>
                <td>{{ plan.max_attachment_size_mb === 0 ? 'Unlimited' : plan.max_attachment_size_mb + ' MB' }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600">Max Batch Size</td>
                <td>{{ formatLimit(plan.max_batch_size) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- Resource Limits -->
      <div class="card" style="margin-bottom: 24px">
        <div class="card-header"><h2>Resource Limits</h2></div>
        <div class="card-body">
          <table>
            <tbody>
              <tr>
                <td style="font-weight: 600; width: 200px">Max API Keys</td>
                <td>{{ formatLimit(plan.max_api_keys) }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600">Max Domains</td>
                <td>{{ formatLimit(plan.max_domains) }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600">Max SMTP Servers</td>
                <td>{{ formatLimit(plan.max_smtp_servers) }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600">Max Workspaces</td>
                <td>{{ formatLimit(plan.max_workspaces) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- Data Retention -->
      <div class="card" style="margin-bottom: 24px">
        <div class="card-header"><h2>Data Retention</h2></div>
        <div class="card-body">
          <table>
            <tbody>
              <tr>
                <td style="font-weight: 600; width: 200px">Email Log Retention</td>
                <td>{{ plan.email_log_retention_days === 0 ? 'Global setting' : plan.email_log_retention_days + ' days' }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- Actions -->
      <div class="card">
        <div class="card-header"><h2>Actions</h2></div>
        <div class="card-body">
          <div class="flex gap-2">
            <button class="btn btn-primary" @click="openEdit">Edit</button>
            <button class="btn btn-secondary" @click="toggleActive">
              {{ plan.is_active ? 'Deactivate' : 'Activate' }}
            </button>
            <button v-if="!plan.is_default" class="btn btn-secondary" @click="setDefault">Set as Default</button>
            <button class="btn btn-danger" @click="deletePlan">Delete</button>
          </div>
        </div>
      </div>
    </template>

    <div v-else class="empty-state">
      <h3>Plan not found</h3>
    </div>

    <!-- Edit Modal -->
    <div v-if="showModal" class="modal-overlay" @mousedown="watchClickStart" @mouseup="confirmClickEnd">
      <div class="modal" @mousedown.stop @mouseup.stop style="max-width: 640px">
        <div class="modal-header">
          <h3>Edit Plan</h3>
        </div>
        <form @submit.prevent="save">
          <div class="modal-body">
            <div class="form-group">
              <label class="form-label">Name</label>
              <input v-model="form.name" type="text" class="form-input" required />
            </div>
            <div class="form-group">
              <label class="form-label">Description</label>
              <input v-model="form.description" type="text" class="form-input" />
            </div>
            <div class="form-group">
              <label class="form-label">
                <input type="checkbox" v-model="form.is_default" style="margin-right: 6px" />
                Set as default plan
              </label>
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
              {{ saving ? 'Saving...' : 'Update' }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>
