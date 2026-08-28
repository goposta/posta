<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { storeToRefs } from "pinia";
import { dashboardApi } from "../../api/dashboard";
import { emailsApi } from "../../api/emails";
import { useAuthStore } from "../../stores/auth";
import { useWorkspaceStore } from "../../stores/workspace";
import { useNotificationStore } from "../../stores/notification";
import { apiMessage } from "../../composables/apiError";
import type { DashboardStats, Email } from "../../api/types";
import GettingStarted from "./GettingStarted.vue";
import HealthAlerts from "./HealthAlerts.vue";
import OverviewStats from "./OverviewStats.vue";
import QuickActions from "./QuickActions.vue";
import RecentEmails from "./RecentEmails.vue";
import ResourceCards from "./ResourceCards.vue";
import VolumeChart from "./VolumeChart.vue";

const CHECKLIST_KEY = "posta_dashboard_checklist_dismissed";
const REFRESH_MS = 60_000;
const RECENT_EMAIL_COUNT = 15;

const router = useRouter();
const auth = useAuthStore();
const ws = useWorkspaceStore();
const notify = useNotificationStore();
const { currentWorkspaceId } = storeToRefs(ws);

const loading = ref(true);
const refreshing = ref(false);
const stats = ref<DashboardStats | null>(null);
const recentEmails = ref<Email[]>([]);
const updatedAt = ref<Date | null>(null);
const now = ref(Date.now());
const checklistDismissed = ref(localStorage.getItem(CHECKLIST_KEY) === "true");

let refreshTimer: number | undefined;
let clockTimer: number | undefined;

const greeting = computed(() => {
  const h = new Date().getHours();
  const part = h < 12 ? "Good morning" : h < 18 ? "Good afternoon" : "Good evening";
  const name = auth.user?.name?.trim().split(" ")[0];
  return name ? `${part}, ${name}` : part;
});

const health = computed(() => {
  const s = stats.value;
  if (!s) return { tone: "neutral", text: "Loading…" };
  if (s.total_emails === 0) return { tone: "neutral", text: "Nothing sent yet" };
  if (s.failure_rate >= 10 || s.bounce_rate >= 5) {
    return { tone: "danger", text: "Delivery needs attention" };
  }
  if (s.unverified_domains > 0 || s.expiring_api_keys > 0) {
    return { tone: "warning", text: "Configuration needs attention" };
  }
  return { tone: "success", text: "Delivery healthy" };
});

const showChecklist = computed(() => !checklistDismissed.value && !!stats.value);

const updatedLabel = computed(() => {
  if (!updatedAt.value) return "";
  const secs = Math.max(0, Math.floor((now.value - updatedAt.value.getTime()) / 1000));
  if (secs < 10) return "just now";
  if (secs < 60) return `${secs}s ago`;
  const mins = Math.floor(secs / 60);
  if (mins < 60) return `${mins}m ago`;
  return `${Math.floor(mins / 60)}h ago`;
});

async function load(showSpinner: boolean) {
  if (showSpinner) loading.value = true;
  else refreshing.value = true;
  try {
    const [statsRes, emailsRes] = await Promise.all([
      dashboardApi.getStats(),
      emailsApi.list(0, RECENT_EMAIL_COUNT),
    ]);
    stats.value = statsRes.data.data;
    recentEmails.value = emailsRes.data.data;
    updatedAt.value = new Date();
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to load the dashboard"));
  } finally {
    loading.value = false;
    refreshing.value = false;
  }
}

function dismissChecklist() {
  checklistDismissed.value = true;
  localStorage.setItem(CHECKLIST_KEY, "true");
}

onMounted(() => {
  load(true);
  refreshTimer = window.setInterval(() => {
    if (document.visibilityState === "visible") load(false);
  }, REFRESH_MS);
  clockTimer = window.setInterval(() => {
    now.value = Date.now();
  }, 10_000);
});

onBeforeUnmount(() => {
  window.clearInterval(refreshTimer);
  window.clearInterval(clockTimer);
});

watch(currentWorkspaceId, () => load(true));
</script>

<template>
  <div>
    <div class="page-header">
      <div>
        <h1>{{ greeting }}</h1>
        <p class="subtitle">
          Overview of <strong>{{ ws.contextLabel }}</strong>
          <span v-if="stats" class="health-pill" :class="`health-${health.tone}`">{{ health.text }}</span>
        </p>
      </div>
      <div class="header-actions">
        <span v-if="updatedAt" class="updated" :title="updatedAt.toLocaleString()">
          Updated {{ updatedLabel }}
        </span>
        <button class="btn btn-secondary" :disabled="refreshing || loading" @click="load(false)">
          {{ refreshing ? "Refreshing…" : "Refresh" }}
        </button>
        <button class="btn btn-primary" @click="router.push('/emails')">View emails</button>
      </div>
    </div>

    <div v-if="loading" class="stats-grid">
      <div v-for="i in 4" :key="i" class="stat-card">
        <span class="skeleton skeleton-line" style="width: 45%"></span>
        <span class="skeleton skeleton-line skeleton-value"></span>
        <span class="skeleton skeleton-line" style="width: 70%"></span>
      </div>
    </div>

    <template v-else-if="stats">
      <HealthAlerts :stats="stats" />

      <GettingStarted v-if="showChecklist" :stats="stats" @dismiss="dismissChecklist" />

      <QuickActions :features="stats.features" />

      <OverviewStats :stats="stats" />

      <div class="dash-section">
        <VolumeChart :volume="stats.daily_volume" />
      </div>

      <div class="dash-columns">
        <div class="dash-main">
          <RecentEmails :emails="recentEmails" />
        </div>
        <div class="dash-side">
          <ResourceCards :stats="stats" />
        </div>
      </div>
    </template>

    <div v-else-if="!ws.hasWorkspace" class="card">
      <div class="empty-state">
        <h3>No workspace yet</h3>
        <p>Everything in Posta belongs to a workspace. Create one to start sending.</p>
        <button class="btn btn-primary" @click="router.push('/workspaces')">Create workspace</button>
      </div>
    </div>

    <div v-else class="card">
      <div class="empty-state">
        <h3>Dashboard unavailable</h3>
        <p>The workspace statistics could not be loaded.</p>
        <button class="btn btn-primary" @click="load(true)">Try again</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.subtitle {
  font-size: 13px;
  color: var(--text-tertiary);
  margin-top: 4px;
  display: inline-flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px 10px;
  min-width: 0;
}

.subtitle strong {
  color: var(--text-secondary);
  word-break: break-word;
}

.health-pill {
  display: inline-flex;
  align-items: center;
  padding: 3px 10px;
  border-radius: 9999px;
  font-size: 12px;
  font-weight: 600;
}

.health-success { background: var(--success-50); color: var(--success-600); }
.health-warning { background: var(--warning-50); color: var(--warning-600); }
.health-danger { background: var(--danger-50); color: var(--danger-600); }
.health-neutral { background: var(--bg-tertiary); color: var(--text-tertiary); }

.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.updated {
  font-size: 12px;
  color: var(--text-tertiary);
  font-variant-numeric: tabular-nums;
}

.dash-section {
  margin-top: 20px;
}

.dash-columns {
  display: grid;
  grid-template-columns: 1fr;
  gap: 20px;
  align-items: start;
  margin-top: 20px;
}

.dash-main,
.dash-side {
  display: flex;
  flex-direction: column;
  gap: 20px;
  min-width: 0;
}

@media (min-width: 1180px) {
  .dash-columns {
    grid-template-columns: minmax(0, 1.7fr) minmax(300px, 1fr);
  }
}

.skeleton {
  display: block;
  border-radius: 6px;
  background: linear-gradient(90deg, var(--bg-tertiary) 25%, var(--bg-hover) 37%, var(--bg-tertiary) 63%);
  background-size: 400% 100%;
  animation: skeleton-shimmer 1.4s ease infinite;
}

.skeleton-line {
  height: 12px;
  margin-bottom: 10px;
}

.skeleton-value {
  height: 28px;
  width: 55%;
}

@keyframes skeleton-shimmer {
  0% { background-position: 100% 50%; }
  100% { background-position: 0 50%; }
}

@media (prefers-reduced-motion: reduce) {
  .skeleton { animation: none; }
}
</style>
