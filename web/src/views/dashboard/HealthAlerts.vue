<script setup lang="ts">
import { computed } from "vue";
import { useRouter } from "vue-router";
import type { DashboardStats } from "../../api/types";

const props = defineProps<{ stats: DashboardStats }>();
const router = useRouter();

interface Alert {
  key: string;
  tone: "danger" | "warning";
  title: string;
  detail: string;
  action: string;
  to: string;
}

const alerts = computed<Alert[]>(() => {
  const s = props.stats;
  const out: Alert[] = [];

  if (s.unverified_domains > 0) {
    out.push({
      key: "domains",
      tone: "warning",
      title: `${s.unverified_domains} domain${s.unverified_domains === 1 ? "" : "s"} not verified`,
      detail:
        "Mail from an unverified domain is far more likely to be filtered, and some routes refuse to send from one at all.",
      action: "Verify domains",
      to: "/domains",
    });
  }

  if (s.total_emails >= 20 && s.bounce_rate >= 5) {
    out.push({
      key: "bounces",
      tone: "danger",
      title: `Bounce rate is ${s.bounce_rate.toFixed(1)}%`,
      detail:
        "Sustained bounces above 5% damage sender reputation. Clean the recipient list before sending more.",
      action: "Review bounces",
      to: "/bounces",
    });
  }

  if (s.total_emails >= 20 && s.failure_rate >= 10) {
    out.push({
      key: "failures",
      tone: "danger",
      title: `${s.failure_rate.toFixed(1)}% of sends are failing`,
      detail:
        "Check the SMTP server configuration and the most recent failures for a common cause.",
      action: "View failed emails",
      to: "/emails?status=failed",
    });
  }

  if (s.expiring_api_keys > 0) {
    out.push({
      key: "keys",
      tone: "warning",
      title: `${s.expiring_api_keys} API key${s.expiring_api_keys === 1 ? "" : "s"} expiring within 7 days`,
      detail: "Rotate them before they lapse, or the integrations using them will start failing.",
      action: "Manage API keys",
      to: "/api-keys",
    });
  }

  if (s.features?.messages && s.unread_messages > 0) {
    out.push({
      key: "messages",
      tone: "warning",
      title: `${s.unread_messages} unread message${s.unread_messages === 1 ? "" : "s"}`,
      detail: "Someone filled in one of your web forms and is waiting for a reply.",
      action: "Open inbox",
      to: "/messages",
    });
  }

  return out;
});

defineExpose({ alerts });
</script>

<template>
  <div v-if="alerts.length" class="alerts">
    <div v-for="a in alerts" :key="a.key" class="alert-banner" :class="`alert-${a.tone}`" role="status">
      <svg
        class="ab-icon"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <path d="M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z" />
        <line x1="12" y1="9" x2="12" y2="13" />
        <line x1="12" y1="17" x2="12.01" y2="17" />
      </svg>
      <div class="ab-text">
        <strong>{{ a.title }}</strong>
        <span>{{ a.detail }}</span>
      </div>
      <button class="btn btn-secondary btn-sm" @click="router.push(a.to)">{{ a.action }}</button>
    </div>
  </div>
</template>

<style scoped>
.alerts {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin-bottom: 20px;
}

.alert-banner {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  padding: 12px 16px;
  border-radius: 10px;
  border: 1px solid;
}

.alert-danger {
  background: var(--danger-50);
  border-color: var(--danger-500);
  color: var(--danger-600);
}

.alert-warning {
  background: var(--warning-50);
  border-color: var(--warning-500);
  color: var(--warning-600);
}

.ab-icon {
  width: 20px;
  height: 20px;
  flex: none;
}

.ab-text {
  flex: 1;
  min-width: 220px;
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-size: 13px;
  color: var(--text-secondary);
}

.ab-text strong {
  color: var(--text-primary);
  font-size: 13px;
}

.btn-sm {
  padding: 5px 12px;
  font-size: 12px;
}
</style>
