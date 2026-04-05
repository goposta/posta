<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { adminApi } from '../../api/admin'
import { plansApi } from '../../api/plans'
import { useNotificationStore } from '../../stores/notification'
import { useConfirm } from '../../composables/useConfirm'
import type { UserDetailMetrics, AdminWorkspace, Plan } from '../../api/types'

const route = useRoute()
const router = useRouter()
const notification = useNotificationStore()
const { confirm } = useConfirm()
const loading = ref(true)
const metrics = ref<UserDetailMetrics | null>(null)
const disabling2FA = ref(false)
const revokingSessions = ref(false)
const deleting = ref(false)
const cancellingDeletion = ref(false)
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
  if (!metrics.value) return
  const confirmed = await confirm({
    title: 'Disable 2FA',
    message: 'Are you sure you want to disable 2FA for this user?',
    confirmText: 'Disable 2FA',
    variant: 'danger',
  })
  if (!confirmed) return
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

async function handleRevokeSessions() {
  if (!metrics.value) return
  const confirmed = await confirm({
    title: 'Revoke All Sessions',
    message: 'Are you sure you want to revoke all active sessions for this user? They will be logged out immediately.',
    confirmText: 'Revoke All Sessions',
    variant: 'danger',
  })
  if (!confirmed) return
  revokingSessions.value = true
  try {
    const res = await adminApi.revokeUserSessions(metrics.value.user.id)
    notification.success(res.data.data.message)
  } catch {
    notification.error('Failed to revoke sessions.')
  } finally {
    revokingSessions.value = false
  }
}

async function handleDeleteUser() {
  if (!metrics.value) return
  const confirmed = await confirm({
    title: 'Delete User',
    message: `Are you sure you want to delete "${metrics.value.user.email}"? The account will be disabled immediately and permanently deleted after 7 days.`,
    confirmText: 'Delete User',
    variant: 'danger',
  })
  if (!confirmed) return
  deleting.value = true
  try {
    await adminApi.deleteUser(metrics.value.user.id)
    const res = await adminApi.getUserMetrics(metrics.value.user.id)
    metrics.value = res.data.data
    notification.success('Account disabled and scheduled for deletion.')
  } catch (e: any) {
    const message = e?.response?.data?.error?.message || 'Failed to delete user'
    notification.error(message)
  } finally {
    deleting.value = false
  }
}

async function handleCancelDeletion() {
  if (!metrics.value) return
  cancellingDeletion.value = true
  try {
    await adminApi.cancelUserDeletion(metrics.value.user.id)
    metrics.value.user.scheduled_deletion_at = null
    metrics.value.user.active = true
    notification.success('Account deletion cancelled.')
  } catch (e: any) {
    const message = e?.response?.data?.error?.message || 'Failed to cancel deletion'
    notification.error(message)
  } finally {
    cancellingDeletion.value = false
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
                <td style="font-weight: 600;">Auth Type</td>
                <td>
                  <span class="badge badge-neutral">{{ metrics.user.auth_method || 'password' }}</span>
                </td>
              </tr>
              <tr>
                <td style="font-weight: 600;">Status</td>
                <td>
                  <span :class="metrics.user.active ? 'badge badge-success' : 'badge badge-danger'">
                    {{ metrics.user.active ? 'Active' : 'Disabled' }}
                  </span>
                  <span v-if="metrics.user.scheduled_deletion_at" class="badge badge-danger" style="margin-left: 8px;">
                    Scheduled for deletion on {{ formatDate(metrics.user.scheduled_deletion_at) }}
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
                <td style="font-weight: 600;">Sessions</td>
                <td>
                  <button
                    class="btn btn-danger btn-sm"
                    :disabled="revokingSessions"
                    @click="handleRevokeSessions"
                  >
                    {{ revokingSessions ? 'Revoking...' : 'Revoke All Sessions' }}
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

          <div style="margin-top: 24px; padding-top: 16px; border-top: 1px solid var(--color-border);">
            <div v-if="metrics.user.scheduled_deletion_at" style="display: flex; align-items: center; gap: 12px;">
              <button
                class="btn btn-primary"
                :disabled="cancellingDeletion"
                @click="handleCancelDeletion"
              >
                {{ cancellingDeletion ? 'Cancelling...' : 'Cancel Scheduled Deletion' }}
              </button>
              <span style="color: var(--color-text-muted); font-size: 0.875rem;">
                This will re-enable the account and cancel the scheduled deletion.
              </span>
            </div>
            <div v-else style="display: flex; align-items: center; gap: 12px;">
              <button
                class="btn btn-danger"
                :disabled="deleting"
                @click="handleDeleteUser"
              >
                {{ deleting ? 'Deleting...' : 'Delete User' }}
              </button>
              <span style="color: var(--color-text-muted); font-size: 0.875rem;">
                The account will be disabled and permanently deleted after 7 days.
              </span>
            </div>
          </div>
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
