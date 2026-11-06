<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { storeToRefs } from "pinia";
import { analyticsApi } from "../../api/analytics";
import { useWorkspaceStore } from "../../stores/workspace";
import { useNotificationStore } from "../../stores/notification";
import { apiMessage } from "../../composables/apiError";
import type {
  DashboardAnalyticsResponse,
  ProviderBreakdownPoint,
  StatusBreakdown,
} from "../../api/types";
import { delta, isoDay, precedingWindow } from "./chart";
import AnalyticsHeader from "./AnalyticsHeader.vue";
import KpiTiles, { type Kpi } from "./KpiTiles.vue";
import DeliveryTrend from "./DeliveryTrend.vue";
import StatusMix from "./StatusMix.vue";
import ProviderTable from "./ProviderTable.vue";
import BounceTrend from "./BounceTrend.vue";
import LatencyPanel from "./LatencyPanel.vue";

const route = useRoute();
const router = useRouter();
const notify = useNotificationStore();
const ws = useWorkspaceStore();
const { currentWorkspaceId } = storeToRefs(ws);

const PRESET_DAYS: Record<string, number> = { "7d": 7, "30d": 30, "90d": 90 };

const preset = ref("30d");
const from = ref("");
const to = ref("");

const loading = ref(true);
const refreshing = ref(false);
const dash = ref<DashboardAnalyticsResponse | null>(null);
const statusRows = ref<StatusBreakdown[]>([]);
const providers = ref<ProviderBreakdownPoint[]>([]);
const previous = ref<{ sent: number; failed: number; bounces: number; latency: number } | null>(null);

const updatedAt = ref<Date | null>(null);
const nowTs = ref(Date.now());
let clockTimer = 0;

function applyPreset(key: string) {
  preset.value = key;
  if (key === "custom") return;
  const days = PRESET_DAYS[key] ?? 30;
  const end = new Date();
  const start = new Date();
  start.setDate(start.getDate() - (days - 1));
  from.value = isoDay(start);
  to.value = isoDay(end);
}

applyPreset("30d");

const priorRange = computed(() => precedingWindow(from.value, to.value));

const totals = computed(() => {
  const trends = dash.value?.delivery_rate_trends ?? [];
  let sent = 0;
  let failed = 0;
  for (const p of trends) {
    sent += p.sent;
    failed += p.failed;
  }
  const bounces = (dash.value?.bounce_rate_trends ?? []).reduce(
    (sum, p) => sum + p.hard + p.soft + p.complaint,
    0
  );
  const attempts = sent + failed;
  return {
    sent,
    failed,
    attempts,
    bounces,
    deliveryRate: attempts > 0 ? (sent / attempts) * 100 : 0,
    // Bounces over everything attempted, as the dashboard and health alerts define it, so this page agrees with the dashboard.
    bounceRate: attempts > 0 ? (bounces / attempts) * 100 : 0,
    latency: dash.value?.latency_percentiles?.p90 ?? 0,
  };
});

const comparable = computed(() => previous.value !== null);

const kpis = computed<Kpi[]>(() => {
  const t = totals.value;
  const p = previous.value;
  const priorAttempts = p ? p.sent + p.failed : 0;
  const priorDelivery = priorAttempts > 0 ? (p!.sent / priorAttempts) * 100 : null;
  const priorBounce = priorAttempts > 0 ? (p!.bounces / priorAttempts) * 100 : null;

  return [
    {
      key: "delivered",
      label: "Delivered",
      value: t.sent,
      kind: "count",
      hint: `of ${t.attempts.toLocaleString()} attempted`,
      change: p ? delta(t.sent, p.sent) : null,
      higherIsBetter: true,
    },
    {
      key: "delivery-rate",
      label: "Delivery rate",
      value: t.deliveryRate,
      kind: "percent",
      hint: t.attempts > 0 ? `${t.failed.toLocaleString()} failed` : "no sends yet",
      change: priorDelivery === null ? null : t.deliveryRate - priorDelivery,
      higherIsBetter: true,
    },
    {
      key: "bounce-rate",
      label: "Bounce rate",
      value: t.bounceRate,
      kind: "percent",
      hint: `${t.bounces.toLocaleString()} bounced of ${t.attempts.toLocaleString()} attempted`,
      change: priorBounce === null ? null : t.bounceRate - priorBounce,
      higherIsBetter: false,
    },
    {
      key: "latency",
      label: "Time to deliver (p90)",
      value: t.latency,
      kind: "latency",
      hint: "9 in 10 arrive faster",
      change: p && p.latency > 0 ? delta(t.latency, p.latency) : null,
      higherIsBetter: false,
    },
  ];
});

const updatedLabel = computed(() => {
  if (!updatedAt.value) return "";
  const secs = Math.max(0, Math.floor((nowTs.value - updatedAt.value.getTime()) / 1000));
  if (secs < 10) return "just now";
  if (secs < 60) return `${secs}s ago`;
  const mins = Math.floor(secs / 60);
  if (mins < 60) return `${mins}m ago`;
  return `${Math.floor(mins / 60)}h ago`;
});

const hasData = computed(
  () => (dash.value?.delivery_rate_trends.length ?? 0) > 0 || statusRows.value.length > 0
);

async function load(showSpinner: boolean) {
  if (showSpinner) loading.value = true;
  else refreshing.value = true;
  try {
    const prior = priorRange.value;
    const [dashRes, statusRes, provRes] = await Promise.all([
      analyticsApi.dashboardAnalytics(from.value, to.value),
      analyticsApi.user(from.value, to.value),
      analyticsApi.providerBreakdown(from.value, to.value),
    ]);

    dash.value = dashRes.data.data;
    statusRows.value = statusRes.data.data.status_breakdown || [];
    providers.value = provRes.data.data.providers || [];
    updatedAt.value = new Date();

    // Loaded separately so a slow or failing prior-window query never blocks the main numbers.
    try {
      const priorRes = await analyticsApi.dashboardAnalytics(prior.from, prior.to);
      const d = priorRes.data.data;
      previous.value = {
        sent: d.delivery_rate_trends.reduce((s, p) => s + p.sent, 0),
        failed: d.delivery_rate_trends.reduce((s, p) => s + p.failed, 0),
        bounces: d.bounce_rate_trends.reduce((s, p) => s + p.hard + p.soft + p.complaint, 0),
        latency: d.latency_percentiles?.p90 ?? 0,
      };
    } catch {
      previous.value = null;
    }
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to load analytics"));
  } finally {
    loading.value = false;
    refreshing.value = false;
  }
}

function exportCsv() {
  const lines: string[] = [];
  const esc = (v: string | number) => {
    const s = String(v);
    return /[",\n]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s;
  };

  lines.push(`# Posta analytics ${from.value} to ${to.value}`);
  lines.push("");
  lines.push("section,key,delivered,failed,other,rate");
  for (const p of dash.value?.delivery_rate_trends ?? []) {
    lines.push(["delivery", p.date, p.sent, p.failed, "", p.delivery_rate.toFixed(2)].map(esc).join(","));
  }
  for (const p of dash.value?.bounce_rate_trends ?? []) {
    lines.push(["bounces", p.date, "", "", p.hard + p.soft + p.complaint, ""].map(esc).join(","));
  }
  for (const r of statusRows.value) {
    lines.push(["status", r.status, r.count, "", "", ""].map(esc).join(","));
  }
  for (const r of providers.value) {
    lines.push(["provider", r.provider, r.sent, r.failed, r.suppressed, r.delivery_rate.toFixed(2)].map(esc).join(","));
  }

  const blob = new Blob([lines.join("\n")], { type: "text/csv;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = `posta-analytics-${from.value}-to-${to.value}.csv`;
  a.click();
  URL.revokeObjectURL(url);
}

// Mirror the range into the URL so a reload or a shared link opens the same
// report. Replace rather than push, so the back button still leaves the page.
function syncUrl() {
  const query: Record<string, string> = { range: preset.value };
  if (preset.value === "custom") {
    query.from = from.value;
    query.to = to.value;
  }
  if (query.range !== route.query.range || query.from !== route.query.from || query.to !== route.query.to) {
    router.replace({ query });
  }
}

function seedFromUrl() {
  const r = typeof route.query.range === "string" ? route.query.range : "";
  const qf = typeof route.query.from === "string" ? route.query.from : "";
  const qt = typeof route.query.to === "string" ? route.query.to : "";
  if (r === "custom" && qf && qt) {
    preset.value = "custom";
    from.value = qf;
    to.value = qt;
  } else if (r && PRESET_DAYS[r]) {
    applyPreset(r);
  }
}

function onPreset(key: string) {
  applyPreset(key);
  if (key !== "custom") {
    syncUrl();
    void load(true);
  }
}

function onCustomDate(which: "from" | "to", value: string) {
  if (!value) return;
  if (which === "from") from.value = value;
  else to.value = value;
  if (from.value > to.value) {
    notify.error("The start of the range has to come before its end");
    return;
  }
  syncUrl();
  void load(true);
}

onMounted(() => {
  seedFromUrl();
  syncUrl();
  void load(true);
  clockTimer = window.setInterval(() => {
    nowTs.value = Date.now();
  }, 10_000);
});

onBeforeUnmount(() => window.clearInterval(clockTimer));

watch(currentWorkspaceId, () => load(true));
</script>

<template>
  <div>
    <AnalyticsHeader
      :preset="preset"
      :from="from"
      :to="to"
      :loading="loading"
      :refreshing="refreshing"
      :updated-label="updatedLabel"
      :can-export="hasData"
      @update:preset="onPreset"
      @update:from="onCustomDate('from', $event)"
      @update:to="onCustomDate('to', $event)"
      @refresh="load(false)"
      @export="exportCsv"
    />

    <div v-if="loading" class="loading-page"><div class="spinner"></div></div>

    <template v-else>
      <KpiTiles :kpis="kpis" :comparable="comparable" />

      <div class="stack">
        <DeliveryTrend :points="dash?.delivery_rate_trends ?? []" />
        <StatusMix :rows="statusRows" />
        <ProviderTable :rows="providers" />
        <BounceTrend :points="dash?.bounce_rate_trends ?? []" />
        <LatencyPanel
          v-if="dash?.latency_percentiles"
          :latency="dash.latency_percentiles"
        />
      </div>
    </template>
  </div>
</template>

<style scoped>
.stack {
  display: flex;
  flex-direction: column;
  gap: 24px;
}
</style>
