<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { adminApi } from '../../api/admin'
import { plansApi } from '../../api/plans'
import { useNotificationStore } from '../../stores/notification'
import type { UserDetailMetrics, AdminWorkspace, Plan } from '../../api/types'

const route = useRoute()
const router = useRouter()
const notification = useNotificationStore()
const loading = ref(true)
const metrics = ref<UserDetailMetrics | null>(null)
const disabling2FA = ref(false)
const workspaces = ref<AdminWorkspace[]>([])
const plans = ref<Plan[]>([])
const changingPlan = ref<number | null>(null)

onMounted(async () => {
  try {
    const id = Number(route.params.id)
    const [metricsRes, workspacesRes, plansRes] = await Promise.all([
      adminApi.getUserMetrics(id),
      adminApi.getUserWorkspaces(id),
      plansApi.list(0, 100),
    ])
    metrics.value = metricsRes.data.data
    workspaces.value = workspacesRes.data.data
    plans.value = plansRes.data.data
  } catch (e) {
    console.error('Failed to load user details', e)
  } finally {
    loading.value = false
  }
})

async function handleDisable2FA() {
  if (!metrics.value || !confirm('Are you sure you want to disable 2FA for this user?')) return
  disabling2FA.value = true
  try {
    await adminApi.disable2FA(metrics.value.user.id)
    metrics.value.user.two_factor_enabled = false
    notification.success('Two-factor authentication disabled.')
  } catch {
    notification.error('Failed to disable 2FA.')
  } finally {
    disabling2FA.value = false
  }
}

async function handleChangePlan(workspace: AdminWorkspace, planId: number | null) {
  changingPlan.value = workspace.id
  try {
    if (planId) {
      await plansApi.assignToWorkspace(workspace.id, planId)
      const plan = plans.value.find(p => p.id === planId)
      workspace.plan_id = planId
      workspace.plan_name = plan?.name || ''
      notification.success(`Plan updated for workspace "${workspace.name}".`)
    }
  } catch {
    notification.error('Failed to update workspace plan.')
  } finally {
    changingPlan.value = null
  }
}

function roleBadgeClass(role: string) {
  switch (role) {
    case 'admin': return 'badge badge-info'
    case 'user': return 'badge badge-neutral'
    default: return 'badge'
  }
}

function formatDate(date: string) {
  return new Date(date).toLocaleString()
}
</script>

<template>
  <div>
    <div class="page-header">
      <h1>User Details</h1>
      <button class="btn btn-secondary" @click="router.push('/admin/users')">Back to Users</button>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <template v-else-if="metrics">
      <div class="card" style="margin-bottom: 24px;">
        <div class="card-header">
          <h2>{{ metrics.user.name || metrics.user.email }}</h2>
          <span :class="roleBadgeClass(metrics.user.role)">{{ metrics.user.role }}</span>
        </div>
        <div class="card-body">
          <table>
            <tbody>
              <tr>
                <td style="font-weight: 600; width: 140px;">Email</td>
                <td>{{ metrics.user.email }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600;">Name</td>
                <td>{{ metrics.user.name || '-' }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600;">Role</td>
                <td><span :class="roleBadgeClass(metrics.user.role)">{{ metrics.user.role }}</span></td>
              </tr>
              <tr>
                <td style="font-weight: 600;">Status</td>
                <td>
                  <span :class="metrics.user.active ? 'badge badge-success' : 'badge badge-danger'">
                    {{ metrics.user.active ? 'Active' : 'Disabled' }}
                  </span>
                </td>
              </tr>
              <tr>
                <td style="font-weight: 600;">2FA</td>
                <td>
                  <span v-if="metrics.user.two_factor_enabled" class="badge badge-success">Enabled</span>
                  <span v-else class="badge badge-neutral">Disabled</span>
                  <button
                    v-if="metrics.user.two_factor_enabled"
                    class="btn btn-danger btn-sm"
                    style="margin-left: 12px;"
                    :disabled="disabling2FA"
                    @click="handleDisable2FA"
                  >
                    {{ disabling2FA ? 'Disabling...' : 'Disable 2FA' }}
                  </button>
                </td>
              </tr>
              <tr>
                <td style="font-weight: 600;">Created At</td>
                <td>{{ formatDate(metrics.user.created_at) }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600;">Last Login</td>
                <td>{{ metrics.user.last_login_at ? formatDate(metrics.user.last_login_at) : 'Never' }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div class="stats-grid">
        <div class="stat-card">
          <div class="stat-label">Total Emails</div>
          <div class="stat-value">{{ metrics.total_emails }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Sent Emails</div>
          <div class="stat-value">{{ metrics.sent_emails }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Failed Emails</div>
          <div class="stat-value">{{ metrics.failed_emails }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Suppressed Emails</div>
          <div class="stat-value">{{ metrics.suppressed_emails }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Failure Rate (%)</div>
          <div class="stat-value">{{ metrics.failure_rate.toFixed(1) }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Total API Keys</div>
          <div class="stat-value">{{ metrics.total_api_keys }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Active API Keys</div>
          <div class="stat-value">{{ metrics.active_api_keys }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Total Contacts</div>
          <div class="stat-value">{{ metrics.total_contacts }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Total Bounces</div>
          <div class="stat-value">{{ metrics.total_bounces }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Total Suppressions</div>
          <div class="stat-value">{{ metrics.total_suppressions }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Domains</div>
          <div class="stat-value">{{ metrics.total_domains }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">SMTP Servers</div>
          <div class="stat-value">{{ metrics.total_smtp_servers }}</div>
        </div>
      </div>

      <!-- Workspaces Section -->
      <div class="card" style="margin-top: 24px;">
        <div class="card-header">
          <h2>Workspaces</h2>
        </div>
        <div class="card-body">
          <div v-if="workspaces.length === 0" class="empty-state" style="padding: 24px 0;">
            <p>This user has no workspaces.</p>
          </div>
          <table v-else class="table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Slug</th>
                <th>Plan</th>
                <th>Created</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="ws in workspaces" :key="ws.id">
                <td>{{ ws.name }}</td>
                <td><code>{{ ws.slug }}</code></td>
                <td>
                  <select
                    class="form-select form-select-sm"
                    :value="ws.plan_id || ''"
                    :disabled="changingPlan === ws.id"
                    @change="handleChangePlan(ws, Number($event.target.value) || null)"
                  >
                    <option value="">No plan (use default)</option>
                    <option v-for="plan in plans" :key="plan.id" :value="plan.id">
                      {{ plan.name }}{{ !plan.is_active ? ' (inactive)' : '' }}{{ plan.is_default ? ' (default)' : '' }}
                    </option>
                  </select>
                </td>
                <td>{{ formatDate(ws.created_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </template>

    <div v-else class="empty-state">
      <h3>User not found</h3>
      <p>The user you are looking for does not exist.</p>
    </div>
  </div>
</template>
