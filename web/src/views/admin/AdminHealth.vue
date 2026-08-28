<script setup lang="ts">
import { computed } from "vue";
import Sparkline from "../../components/Sparkline.vue";
import type { AdminMetrics, WorkerStatus } from "../../api/types";

const props = defineProps<{
  metrics: AdminMetrics;
  workerStatus: WorkerStatus | null;
  activeWorkers: number;
  trends: { memory: number[]; goroutines: number[]; workers: number[] };
}>();

type Level = "ok" | "warn" | "crit";

const rank: Record<Level, number> = { ok: 0, warn: 1, crit: 2 };

interface Meter {
  key: string;
  label: string;
  value: string;
  detail: string;
  pct: number;
  level: Level;
  hint?: string;
}

function levelFor(pct: number, warn: number, crit: number): Level {
  if (pct <= crit) return "crit";
  if (pct <= warn) return "warn";
  return "ok";
}

const deliveryRate = computed(() => {
  const m = props.metrics;
  const attempted = m.sent_emails + m.failed_emails;
  return attempted === 0 ? 100 : (m.sent_emails / attempted) * 100;
});

const meters = computed<Meter[]>(() => {
  const m = props.metrics;
  const out: Meter[] = [];

  const attempted = m.sent_emails + m.failed_emails;
  if (attempted > 0) {
    const pct = deliveryRate.value;
    out.push({
      key: "delivery",
      label: "Delivery rate",
      value: `${pct.toFixed(1)}%`,
      detail: `${m.sent_emails.toLocaleString()} delivered of ${attempted.toLocaleString()} attempted`,
      pct,
      level: levelFor(pct, 95, 90),
      hint: pct <= 95 ? "Check SMTP server health and the most recent failures." : undefined,
    });
  }

  if (m.total_users > 0) {
    const pct = m.two_factor_adoption_rate;
    out.push({
      key: "twofa",
      label: "Two-factor adoption",
      value: `${pct.toFixed(0)}%`,
      detail: `${m.two_factor_users} of ${m.total_users} users enrolled`,
      pct,
      level: levelFor(pct, 50, 20),
      hint: pct <= 50 ? "Consider requiring 2FA in platform settings." : undefined,
    });
  }

  if (m.total_api_keys > 0) {
    const pct = (m.active_api_keys / m.total_api_keys) * 100;
    out.push({
      key: "keys",
      label: "API keys active",
      value: `${m.active_api_keys}/${m.total_api_keys}`,
      detail: `${m.total_api_keys - m.active_api_keys} revoked or expired`,
      pct,
      level: "ok",
    });
  }

  return out;
});

const health = computed(() => {
  const m = props.metrics;
  const reasons: string[] = [];
  let worst = 0;
  const raise = (l: Level) => {
    worst = Math.max(worst, rank[l]);
  };

  if (props.activeWorkers === 0) {
    raise("crit");
    reasons.push(
      m.queued_emails > 0
        ? `No workers connected — ${m.queued_emails.toLocaleString()} email(s) are stuck in the queue`
        : "No workers connected — queued email will not be delivered"
    );
  }

  if (m.users_without_workspace > 0) {
    raise("warn");
    reasons.push(
      `${m.users_without_workspace} user(s) belong to no workspace and land on an empty dashboard`
    );
  }

  for (const meter of meters.value) {
    if (meter.level === "ok") continue;
    raise(meter.level);
    reasons.push(`${meter.label} at ${meter.value}`);
  }

  if (props.workerStatus?.version_mismatch) {
    raise("warn");
    reasons.push(
      `A worker is running a different version than the server (${props.workerStatus.server_version || "unknown"})`
    );
  }

  if (m.failed_logins_last_24h >= 10) {
    raise("warn");
    reasons.push(`${m.failed_logins_last_24h} failed sign-ins in the last 24 hours`);
  }

  if (worst === 2) return { level: "crit" as Level, label: "Needs attention", reasons };
  if (worst === 1) return { level: "warn" as Level, label: "Degraded", reasons };
  return { level: "ok" as Level, label: "Operational", reasons };
});

function formatBytes(bytes: number): string {
  if (!bytes || bytes < 0) return "—";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let i = 0;
  let n = bytes;
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024;
    i++;
  }
  return `${n.toFixed(n >= 100 || i === 0 ? 0 : 1)} ${units[i]}`;
}

function formatUptime(seconds: number): string {
  if (!seconds || seconds < 0) return "—";
  const s = Math.floor(seconds);
  const d = Math.floor(s / 86400);
  const h = Math.floor((s % 86400) / 3600);
  const m = Math.floor((s % 3600) / 60);
  if (d > 0) return `${d}d ${h}h`;
  if (h > 0) return `${h}h ${m}m`;
  return `${m}m`;
}
</script>

<template>
  <div class="card health-card" :class="`health-${health.level}`">
    <div class="health-head">
      <span class="health-mark" aria-hidden="true">
        <svg v-if="health.level === 'ok'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <circle cx="12" cy="12" r="10" /><path d="m8 12 3 3 5-6" />
        </svg>
        <svg v-else-if="health.level === 'warn'" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />
          <line x1="12" y1="9" x2="12" y2="13" /><line x1="12" y1="17" x2="12.01" y2="17" />
        </svg>
        <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <polygon points="7.86 2 16.14 2 22 7.86 22 16.14 16.14 22 7.86 22 2 16.14 2 7.86 7.86 2" />
          <line x1="12" y1="8" x2="12" y2="12" /><line x1="12" y1="16" x2="12.01" y2="16" />
        </svg>
      </span>
      <div class="health-text">
        <span class="health-label">{{ health.label }}</span>
        <span v-if="!health.reasons.length" class="health-sub">
          Workers connected, delivery healthy, no migration backlog.
        </span>
        <ul v-else class="health-reasons">
          <li v-for="reason in health.reasons" :key="reason">{{ reason }}</li>
        </ul>
      </div>
    </div>

    <div class="runtime-row">
      <div class="rt">
        <span class="rt-label">Uptime</span>
        <span class="rt-value">{{ formatUptime(metrics.server_uptime_seconds) }}</span>
      </div>
      <div class="rt">
        <span class="rt-label">Workers</span>
        <span class="rt-value" :class="activeWorkers === 0 ? 'tone-danger' : ''">{{ activeWorkers }}</span>
        <Sparkline
          :values="trends.workers"
          :width="90"
          :height="24"
          :stroke="activeWorkers === 0 ? 'var(--danger-500)' : 'var(--success-500)'"
        />
      </div>
      <div class="rt">
        <span class="rt-label">Memory</span>
        <span class="rt-value">{{ formatBytes(metrics.current_memory_usage) }}</span>
        <Sparkline :values="trends.memory" :width="90" :height="24" />
      </div>
      <div class="rt">
        <span class="rt-label">Goroutines</span>
        <span class="rt-value">{{ metrics.current_goroutines.toLocaleString() }}</span>
        <Sparkline :values="trends.goroutines" :width="90" :height="24" />
      </div>
      <div class="rt">
        <span class="rt-label">Queue</span>
        <span class="rt-value">{{ (metrics.queued_emails + metrics.processing_emails).toLocaleString() }}</span>
        <span class="rt-sub">{{ metrics.queued_emails.toLocaleString() }} queued</span>
      </div>
    </div>

    <div v-if="meters.length" class="meters">
      <div v-for="meter in meters" :key="meter.key" class="meter" :class="`meter-${meter.level}`">
        <div class="meter-head">
          <span class="meter-label">{{ meter.label }}</span>
          <span class="meter-value">{{ meter.value }}</span>
        </div>
        <div class="meter-track">
          <div class="meter-fill" :style="{ width: `${Math.min(100, Math.max(0, meter.pct))}%` }"></div>
        </div>
        <div class="meter-detail">{{ meter.detail }}</div>
        <div v-if="meter.hint" class="meter-hint">{{ meter.hint }}</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.health-card {
  padding: 20px 24px;
  border-left: 3px solid var(--border-primary);
}

.health-ok { border-left-color: var(--success-500); }
.health-warn { border-left-color: var(--warning-500); }
.health-crit { border-left-color: var(--danger-500); }

.health-head {
  display: flex;
  gap: 12px;
  align-items: flex-start;
}

.health-mark {
  flex: none;
  width: 24px;
  height: 24px;
}

.health-mark svg {
  width: 24px;
  height: 24px;
}

.health-ok .health-mark { color: var(--success-600); }
.health-warn .health-mark { color: var(--warning-600); }
.health-crit .health-mark { color: var(--danger-600); }

.health-text {
  min-width: 0;
}

.health-label {
  display: block;
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
}

.health-sub {
  font-size: 13px;
  color: var(--text-tertiary);
}

.health-reasons {
  margin: 6px 0 0;
  padding-left: 18px;
  font-size: 13px;
  color: var(--text-secondary);
}

.health-reasons li {
  margin-bottom: 3px;
}

.runtime-row {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  gap: 16px;
  margin-top: 20px;
  padding-top: 18px;
  border-top: 1px solid var(--border-secondary);
}

.rt {
  display: flex;
  flex-direction: column;
  gap: 3px;
  min-width: 0;
}

.rt-label {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  font-weight: 600;
  color: var(--text-tertiary);
}

.rt-value {
  font-size: 19px;
  font-weight: 600;
  color: var(--text-primary);
  font-variant-numeric: tabular-nums;
}

.rt-sub {
  font-size: 11px;
  color: var(--text-tertiary);
}

.tone-danger { color: var(--danger-600); }

.meters {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 18px;
  margin-top: 20px;
  padding-top: 18px;
  border-top: 1px solid var(--border-secondary);
}

.meter-head {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 8px;
  margin-bottom: 6px;
}

.meter-label {
  font-size: 13px;
  color: var(--text-secondary);
}

.meter-value {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
  font-variant-numeric: tabular-nums;
}

.meter-track {
  height: 5px;
  border-radius: 999px;
  background: var(--bg-tertiary);
  overflow: hidden;
}

.meter-fill {
  height: 100%;
  border-radius: 999px;
  background: var(--success-500);
  transition: width 0.3s ease;
}

.meter-warn .meter-fill { background: var(--warning-500); }
.meter-crit .meter-fill { background: var(--danger-500); }

.meter-detail {
  margin-top: 6px;
  font-size: 11px;
  color: var(--text-tertiary);
}

.meter-hint {
  margin-top: 4px;
  font-size: 11px;
  color: var(--text-secondary);
}
</style>
