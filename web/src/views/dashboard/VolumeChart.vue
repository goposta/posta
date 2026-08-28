<script setup lang="ts">
import { computed, ref } from "vue";
import type { DailyVolume } from "../../api/types";

const props = defineProps<{ volume: DailyVolume[] }>();

const hovered = ref<number | null>(null);

const totals = computed(() => {
  let sent = 0;
  let failed = 0;
  for (const d of props.volume) {
    sent += d.sent;
    failed += d.failed;
  }
  return { sent, failed, total: sent + failed };
});

const NICE_STEPS = [1, 1.25, 1.5, 2, 2.5, 3, 4, 5, 6, 8, 10];

const peak = computed(() => {
  const max = Math.max(0, ...props.volume.map((d) => d.sent + d.failed));
  if (max === 0) return 0;
  const magnitude = Math.pow(10, Math.floor(Math.log10(max)));
  const ratio = max / magnitude;
  const step = NICE_STEPS.find((n) => n >= ratio) ?? 10;
  return Math.round(step * magnitude);
});

const busiest = computed(() => {
  let best: DailyVolume | null = null;
  for (const d of props.volume) {
    if (!best || d.sent + d.failed > best.sent + best.failed) best = d;
  }
  return best;
});

function heightPct(value: number) {
  if (peak.value === 0) return "0%";
  const pct = (value / peak.value) * 100;
  return `${value > 0 ? Math.max(pct, 1.5) : 0}%`;
}

function dayLabel(dateStr: string) {
  return new Date(dateStr + "T00:00:00").toLocaleDateString(undefined, {
    weekday: "short",
  });
}

function fullDate(dateStr: string) {
  return new Date(dateStr + "T00:00:00").toLocaleDateString(undefined, {
    month: "short",
    day: "numeric",
  });
}

function showAxisLabel(idx: number) {
  return idx === 0 || idx === props.volume.length - 1 || idx % 3 === 0;
}

function formatNumber(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + "M";
  if (n >= 10_000) return (n / 1_000).toFixed(1) + "K";
  return n.toLocaleString();
}
</script>

<template>
  <div class="card">
    <div class="card-header">
      <h2>Send volume</h2>
      <div class="vc-summary">
        <span>Last {{ volume.length }} days</span>
        <span class="vc-dot">·</span>
        <strong>{{ formatNumber(totals.total) }}</strong>
        <span v-if="busiest && totals.total > 0" class="vc-peak">
          peak {{ formatNumber(busiest.sent + busiest.failed) }} on {{ fullDate(busiest.date) }}
        </span>
      </div>
    </div>

    <div class="card-body">
      <div v-if="totals.total === 0" class="empty-state vc-empty">
        <h3>No sends in this window</h3>
        <p>Volume appears here once emails start going out.</p>
      </div>

      <div v-else class="vc">
        <div class="vc-axis" aria-hidden="true">
          <span>{{ formatNumber(peak) }}</span>
          <span>{{ formatNumber(Math.round(peak / 2)) }}</span>
          <span>0</span>
        </div>

        <div class="vc-plot">
          <div class="vc-grid" aria-hidden="true">
            <span></span><span></span><span></span>
          </div>

          <div class="vc-bars">
            <div
              v-for="(day, idx) in volume"
              :key="day.date"
              class="vc-col"
              :class="{ 'is-hovered': hovered === idx }"
              @mouseenter="hovered = idx"
              @mouseleave="hovered = null"
            >
              <div class="vc-stack">
                <div class="vc-bar vc-bar-failed" :style="{ height: heightPct(day.failed) }"></div>
                <div class="vc-bar vc-bar-sent" :style="{ height: heightPct(day.sent) }"></div>
              </div>

              <div v-if="hovered === idx" class="vc-tip" role="tooltip">
                <strong>{{ fullDate(day.date) }}</strong>
                <span><i class="vc-key vc-key-sent"></i>{{ day.sent.toLocaleString() }} sent</span>
                <span><i class="vc-key vc-key-failed"></i>{{ day.failed.toLocaleString() }} failed</span>
              </div>

              <div class="vc-label">{{ showAxisLabel(idx) ? dayLabel(day.date) : "" }}</div>
            </div>
          </div>
        </div>
      </div>

      <div v-if="totals.total > 0" class="vc-legend">
        <span><i class="vc-key vc-key-sent"></i>Sent {{ formatNumber(totals.sent) }}</span>
        <span><i class="vc-key vc-key-failed"></i>Failed {{ formatNumber(totals.failed) }}</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.vc-summary {
  display: flex;
  align-items: baseline;
  gap: 6px;
  flex-wrap: wrap;
  font-size: 12px;
  color: var(--text-tertiary);
}

.vc-summary strong {
  color: var(--text-secondary);
  font-size: 13px;
}

.vc-dot { opacity: 0.6; }
.vc-peak { font-size: 12px; }
.vc-empty { padding: 36px 16px; }

.vc {
  display: flex;
  gap: 10px;
}

.vc-axis {
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  height: 160px;
  font-size: 10px;
  color: var(--text-tertiary);
  font-variant-numeric: tabular-nums;
  padding-bottom: 18px;
  text-align: right;
  min-width: 28px;
}

.vc-plot {
  position: relative;
  flex: 1;
  min-width: 0;
}

.vc-grid {
  position: absolute;
  inset: 0 0 18px 0;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  pointer-events: none;
}

.vc-grid span {
  height: 1px;
  background: var(--border-secondary);
}

.vc-bars {
  position: relative;
  display: flex;
  align-items: flex-end;
  gap: 4px;
  height: 160px;
}

.vc-col {
  position: relative;
  flex: 1;
  min-width: 0;
  height: 100%;
  display: flex;
  flex-direction: column;
}

.vc-stack {
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: flex-end;
  border-radius: 3px;
  overflow: hidden;
}

.vc-col.is-hovered .vc-stack {
  outline: 1px solid var(--border-input);
  outline-offset: 1px;
}

.vc-bar {
  width: 100%;
  transition: height 0.25s ease;
}

.vc-bar-sent { background: var(--primary-500); }
.vc-bar-failed { background: var(--danger-500); }

.vc-label {
  height: 18px;
  line-height: 18px;
  text-align: center;
  font-size: 10px;
  color: var(--text-tertiary);
  white-space: nowrap;
  overflow: hidden;
}

.vc-tip {
  position: absolute;
  bottom: calc(100% - 12px);
  left: 50%;
  transform: translateX(-50%);
  z-index: 5;
  display: flex;
  flex-direction: column;
  gap: 3px;
  padding: 8px 10px;
  border-radius: 8px;
  background: var(--bg-primary);
  border: 1px solid var(--border-primary);
  box-shadow: var(--shadow-lg);
  font-size: 11px;
  color: var(--text-secondary);
  white-space: nowrap;
  pointer-events: none;
}

.vc-tip strong {
  color: var(--text-primary);
  font-size: 12px;
}

.vc-legend {
  display: flex;
  gap: 16px;
  margin-top: 14px;
  font-size: 12px;
  color: var(--text-tertiary);
}

.vc-legend span,
.vc-tip span {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.vc-key {
  width: 8px;
  height: 8px;
  border-radius: 2px;
  flex: none;
}

.vc-key-sent { background: var(--primary-500); }
.vc-key-failed { background: var(--danger-500); }
</style>
