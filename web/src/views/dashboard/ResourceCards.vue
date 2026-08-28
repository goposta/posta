<script setup lang="ts">
import { computed } from "vue";
import { useRouter } from "vue-router";
import type { DashboardStats } from "../../api/types";

const props = defineProps<{ stats: DashboardStats }>();
const router = useRouter();

interface Row {
  label: string;
  value: string;
  secondary?: string;
  tone?: "danger" | "warning";
  to?: string;
}

function formatNumber(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + "M";
  if (n >= 10_000) return (n / 1_000).toFixed(1) + "K";
  return n.toLocaleString();
}

const infrastructure = computed<Row[]>(() => {
  const s = props.stats;
  const rows: Row[] = [
    {
      label: "Domains",
      value: String(s.total_domains - s.unverified_domains),
      secondary: `of ${s.total_domains} verified`,
      tone: s.unverified_domains > 0 ? "warning" : undefined,
      to: "/domains",
    },
    {
      label: "API keys",
      value: String(s.active_api_keys),
      secondary: `of ${s.total_api_keys} active`,
      tone: s.expiring_api_keys > 0 ? "warning" : undefined,
      to: "/api-keys",
    },
    { label: "SMTP servers", value: String(s.total_smtp_servers), to: "/smtp-servers" },
    { label: "Webhooks", value: String(s.total_webhooks), to: "/webhooks" },
    { label: "Templates", value: String(s.total_templates), to: "/templates" },
  ];
  if (s.features?.messages) {
    rows.push({ label: "Forms", value: String(s.total_forms), to: "/forms" });
  }
  return rows;
});

const deliverability = computed<Row[]>(() => {
  const s = props.stats;
  return [
    {
      label: "Bounces",
      value: formatNumber(s.total_bounces),
      secondary: s.total_emails > 0 ? `${s.bounce_rate.toFixed(1)}% of sends` : undefined,
      tone: s.bounce_rate >= 5 && s.total_emails >= 20 ? "danger" : undefined,
      to: "/bounces",
    },
    { label: "Suppressions", value: formatNumber(s.total_suppressions), to: "/bounces" },
    { label: "Suppressed sends", value: formatNumber(s.suppressed_emails) },
    { label: "Contacts", value: formatNumber(s.total_contacts), to: "/contacts" },
    { label: "Subscribers", value: formatNumber(s.total_subscribers), to: "/subscribers" },
    { label: "Campaigns", value: formatNumber(s.total_campaigns), to: "/campaigns" },
  ];
});

const webhookHealth = computed(() => props.stats.webhook_deliveries);
</script>

<template>
  <div class="card">
    <div class="card-header"><h2>Infrastructure</h2></div>
    <ul class="res-list">
      <li
        v-for="row in infrastructure"
        :key="row.label"
        class="res-row"
        :class="{ 'is-clickable': !!row.to }"
        @click="row.to && router.push(row.to)"
      >
        <span class="res-label">{{ row.label }}</span>
        <span class="res-value" :class="row.tone ? `tone-${row.tone}` : ''">
          {{ row.value }}
          <small v-if="row.secondary">{{ row.secondary }}</small>
        </span>
      </li>
    </ul>
  </div>

  <div class="card">
    <div class="card-header"><h2>Deliverability</h2></div>
    <ul class="res-list">
      <li
        v-for="row in deliverability"
        :key="row.label"
        class="res-row"
        :class="{ 'is-clickable': !!row.to }"
        @click="row.to && router.push(row.to)"
      >
        <span class="res-label">{{ row.label }}</span>
        <span class="res-value" :class="row.tone ? `tone-${row.tone}` : ''">
          {{ row.value }}
          <small v-if="row.secondary">{{ row.secondary }}</small>
        </span>
      </li>
    </ul>
  </div>

  <div v-if="webhookHealth && webhookHealth.total_deliveries > 0" class="card">
    <div class="card-header">
      <h2>Webhook deliveries</h2>
      <button class="btn btn-secondary btn-sm" @click="router.push('/webhook-deliveries')">Details</button>
    </div>
    <div class="card-body">
      <div class="wh-head">
        <span
          class="wh-rate"
          :class="webhookHealth.success_rate >= 95 ? 'tone-ok' : webhookHealth.success_rate >= 80 ? 'tone-warning' : 'tone-danger'"
        >{{ webhookHealth.success_rate.toFixed(1) }}%</span>
        <span class="wh-sub">
          {{ formatNumber(webhookHealth.success_deliveries) }} delivered ·
          {{ formatNumber(webhookHealth.failed_deliveries) }} failed
        </span>
      </div>
      <div class="wh-bar">
        <span class="wh-seg wh-ok" :style="{ width: `${webhookHealth.success_rate}%` }"></span>
        <span class="wh-seg wh-bad" :style="{ width: `${100 - webhookHealth.success_rate}%` }"></span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.res-list {
  list-style: none;
  margin: 0;
  padding: 0 24px 8px;
}

.res-row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  padding: 11px 0;
  border-bottom: 1px solid var(--border-secondary);
}

.res-row:last-child {
  border-bottom: 0;
}

.is-clickable {
  cursor: pointer;
  margin: 0 -12px;
  padding-left: 12px;
  padding-right: 12px;
  border-radius: 6px;
}

.is-clickable:hover {
  background: var(--bg-hover);
}

.res-label {
  font-size: 13px;
  color: var(--text-secondary);
}

.res-value {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
  font-variant-numeric: tabular-nums;
  text-align: right;
}

.res-value small {
  display: block;
  font-size: 11px;
  font-weight: 400;
  color: var(--text-tertiary);
}

.tone-warning { color: var(--warning-600); }
.tone-danger { color: var(--danger-600); }
.tone-ok { color: var(--success-600); }

.wh-head {
  display: flex;
  align-items: baseline;
  gap: 10px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}

.wh-rate {
  font-size: 24px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
}

.wh-sub {
  font-size: 12px;
  color: var(--text-tertiary);
}

.wh-bar {
  display: flex;
  height: 6px;
  border-radius: 999px;
  overflow: hidden;
  background: var(--bg-tertiary);
}

.wh-seg { display: block; }
.wh-ok { background: var(--success-500); }
.wh-bad { background: var(--danger-500); }

.btn-sm {
  padding: 5px 12px;
  font-size: 12px;
}
</style>
