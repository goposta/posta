<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { adminApi } from "../../api/admin";
import { plansApi } from "../../api/plans";
import { useNotificationStore } from "../../stores/notification";
import { apiMessage } from "../../composables/apiError";
import { useConfirm } from "../../composables/useConfirm";
import type { AdminWorkspaceDetail, Plan } from "../../api/types";

const route = useRoute();
const router = useRouter();
const notify = useNotificationStore();

const id = Number(route.params.id);
const workspace = ref<AdminWorkspaceDetail | null>(null);
const plans = ref<Plan[]>([]);
const loading = ref(true);
const loadFailed = ref(false);
const changingPlan = ref(false);
const { confirm } = useConfirm();
const changingHosted = ref(false);

// Taking hosted pages offline is the abuse switch for one workspace: its share
// links stop at once, while forms embedded on its own sites keep working.
async function toggleHostedForms() {
  const ws = workspace.value;
  if (!ws) return;
  const enable = ws.hosted_forms_disabled;
  const ok = await confirm({
    title: enable ? "Turn hosted form pages back on?" : "Take hosted form pages offline?",
    message: enable
      ? `The share links of "${ws.name}" forms work again.`
      : `Every share link of "${ws.name}" forms answers "not found" at once, and forms can no longer be loaded or submitted from this Posta's domain. Forms embedded on the workspace's own websites, its submissions and its data are not affected. Use this when hosted forms are abused, for example for phishing.`,
    confirmText: enable ? "Turn back on" : "Take offline",
    variant: enable ? "info" : "danger",
  });
  if (!ok) return;
  changingHosted.value = true;
  try {
    const res = await adminApi.setWorkspaceHostedForms(ws.id, enable);
    ws.hosted_forms_disabled = res.data.data.hosted_forms_disabled;
    notify.success(enable ? "Hosted form pages are back on." : "Hosted form pages are offline.");
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to update hosted form pages"));
  } finally {
    changingHosted.value = false;
  }
}

async function load() {
  loading.value = true;
  try {
    const [wsRes, plansRes] = await Promise.all([
      adminApi.getWorkspace(id),
      plansApi.list(0, 100).catch(() => null),
    ]);
    workspace.value = wsRes.data.data;
    if (plansRes) plans.value = plansRes.data.data;
  } catch (e: any) {
    loadFailed.value = true;
    notify.error(apiMessage(e, "Failed to load the workspace"));
  } finally {
    loading.value = false;
  }
}

// The API only assigns plans; it cannot return a workspace to the default.
// Inactive plans are refused server-side, so they are only listed when one is
// already assigned and has to show as the current value.
const planOptions = computed(() =>
  plans.value.filter((p) => p.is_active || p.id === workspace.value?.plan_id)
);

async function changePlan(event: Event) {
  const ws = workspace.value;
  const planId = Number((event.target as HTMLSelectElement).value);
  if (!ws || !planId || planId === ws.plan_id) return;
  changingPlan.value = true;
  try {
    await plansApi.assignToWorkspace(ws.id, planId);
    ws.plan_id = planId;
    ws.plan_name = plans.value.find((p) => p.id === planId)?.name || "";
    notify.success(`Plan updated for workspace "${ws.name}"`);
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to update the workspace plan"));
    (event.target as HTMLSelectElement).value = String(ws.plan_id ?? "");
  } finally {
    changingPlan.value = false;
  }
}

const usageCards = computed(() => {
  const u = workspace.value?.usage;
  if (!u) return [];
  return [
    { label: "Emails", value: u.emails },
    { label: "Templates", value: u.templates },
    { label: "Contacts", value: u.contacts },
    { label: "Subscribers", value: u.subscribers },
    { label: "Campaigns", value: u.campaigns },
    { label: "Domains", value: u.domains },
    { label: "SMTP Servers", value: u.smtp_servers },
    { label: "API Keys", value: u.api_keys },
    { label: "Webhooks", value: u.webhooks },
  ];
});

function roleBadge(role: string) {
  switch (role) {
    case "owner":
      return "badge badge-info";
    case "admin":
      return "badge badge-success";
    default:
      return "badge badge-neutral";
  }
}

function formatDate(value?: string) {
  return value ? new Date(value).toLocaleString() : "—";
}

onMounted(load);
</script>

<template>
  <div>
    <div class="page-header">
      <div>
        <h1>
          {{ workspace?.name || "Workspace" }}
          <span v-if="workspace?.system" class="badge badge-info" style="margin-left: 8px; vertical-align: middle">
            system
          </span>
        </h1>
        <p v-if="workspace" class="page-subtitle">
          <code>{{ workspace.slug }}</code> · created {{ formatDate(workspace.created_at) }}
        </p>
      </div>
      <div class="header-actions">
        <button class="btn btn-secondary" @click="router.push('/admin/workspaces')">Back</button>
      </div>
    </div>

    <div v-if="loading" class="loading-page"><div class="spinner"></div></div>

    <div v-else-if="loadFailed || !workspace" class="card">
      <div class="empty-state">
        <h3>Workspace not found</h3>
        <button class="btn btn-secondary" @click="router.push('/admin/workspaces')">Back to workspaces</button>
      </div>
    </div>

    <template v-else>
      <div class="card" style="margin-bottom: 20px">
        <div class="card-header"><h2>Overview</h2></div>
        <div class="card-body">
          <table>
            <tbody>
              <tr>
                <td class="label-cell">Owner</td>
                <td>
                  <router-link :to="`/admin/users/${workspace.owner_id}`">
                    {{ workspace.owner_name || workspace.owner_email || `#${workspace.owner_id}` }}
                  </router-link>
                  <span v-if="workspace.owner_name && workspace.owner_email" class="muted">
                    {{ workspace.owner_email }}
                  </span>
                </td>
              </tr>
              <tr>
                <td class="label-cell">Description</td>
                <td>{{ workspace.description || "—" }}</td>
              </tr>
              <tr>
                <td class="label-cell">Default language</td>
                <td>{{ workspace.default_language || "—" }}</td>
              </tr>
              <tr>
                <td class="label-cell">Plan</td>
                <td>
                  <select
                    class="form-select form-select-sm plan-select"
                    :value="workspace.plan_id ?? ''"
                    :disabled="changingPlan"
                    @change="changePlan"
                  >
                    <option value="" disabled>No plan (uses default)</option>
                    <option v-for="plan in planOptions" :key="plan.id" :value="plan.id">
                      {{ plan.name }}{{ !plan.is_active ? " (inactive)" : "" }}{{ plan.is_default ? " (default)" : "" }}
                    </option>
                  </select>
                </td>
              </tr>
              <tr>
                <td class="label-cell">Hosted form pages</td>
                <td>
                  <span v-if="workspace.hosted_forms_disabled" class="badge badge-danger">Offline</span>
                  <span v-else class="badge badge-success">Online</span>
                  <button
                    class="btn btn-sm hosted-toggle"
                    :class="workspace.hosted_forms_disabled ? 'btn-secondary' : 'btn-danger'"
                    :disabled="changingHosted"
                    @click="toggleHostedForms"
                  >
                    {{ workspace.hosted_forms_disabled ? "Turn back on" : "Take offline" }}
                  </button>
                </td>
              </tr>
              <tr>
                <td class="label-cell">Pending invitations</td>
                <td>{{ workspace.pending_invitations }}</td>
              </tr>
              <tr>
                <td class="label-cell">Last updated</td>
                <td>{{ formatDate(workspace.updated_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div class="card" style="margin-bottom: 20px">
        <div class="card-header"><h2>Usage</h2></div>
        <div class="card-body">
          <div class="stats-grid">
            <div v-for="card in usageCards" :key="card.label" class="stat-card">
              <div class="stat-label">{{ card.label }}</div>
              <div class="stat-value">{{ card.value }}</div>
            </div>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="card-header">
          <h2>Members ({{ workspace.members.length }})</h2>
        </div>
        <div v-if="!workspace.members.length" class="empty-state">
          <p>This workspace has no members.</p>
        </div>
        <div v-else class="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Email</th>
                <th>Role</th>
                <th>Status</th>
                <th>Joined</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="m in workspace.members"
                :key="m.user_id"
                style="cursor: pointer"
                @click="router.push(`/admin/users/${m.user_id}`)"
              >
                <td>{{ m.name || "—" }}</td>
                <td class="muted-cell">{{ m.email }}</td>
                <td><span :class="roleBadge(m.role)">{{ m.role }}</span></td>
                <td>
                  <span :class="m.active ? 'badge badge-success' : 'badge badge-warning'">
                    {{ m.active ? "active" : "disabled" }}
                  </span>
                </td>
                <td class="muted-cell">{{ formatDate(m.joined_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.page-subtitle {
  font-size: 13px;
  color: var(--text-tertiary);
  margin-top: 4px;
}

.header-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.label-cell {
  font-weight: 600;
  width: 190px;
  color: var(--text-secondary);
}

.muted {
  margin-left: 8px;
  color: var(--text-tertiary);
  font-size: 13px;
}

.muted-cell {
  color: var(--text-tertiary);
  font-size: 13px;
}

.plan-select {
  max-width: 300px;
}

.hosted-toggle {
  margin-left: 10px;
}
</style>
