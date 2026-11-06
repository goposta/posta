<script setup lang="ts">
import { computed } from "vue";
import type { DeliveryRatePoint } from "../../api/types";
import { formatNumber, longDate, shortDate } from "./chart";
import StackedBars, { type BarPoint, type Series } from "./StackedBars.vue";

const props = defineProps<{ points: DeliveryRatePoint[] }>();

const series: Series[] = [
  { key: "sent", label: "Delivered", colorVar: "--success-500" },
  { key: "failed", label: "Failed", colorVar: "--danger-500" },
];

const bars = computed<BarPoint[]>(() =>
  props.points.map((p) => ({ date: p.date, values: { sent: p.sent, failed: p.failed } }))
);

const totals = computed(() => {
  let sent = 0;
  let failed = 0;
  for (const p of props.points) {
    sent += p.sent;
    failed += p.failed;
  }
  return { sent, failed, total: sent + failed };
});

const byDate = computed(() => {
  const map = new Map<string, DeliveryRatePoint>();
  for (const p of props.points) map.set(p.date, p);
  return map;
});

const worst = computed(() => {
  let found: DeliveryRatePoint | null = null;
  for (const p of props.points) {
    if (p.total === 0) continue;
    if (!found || p.delivery_rate < found.delivery_rate) found = p;
  }
  return found;
});
</script>

<template>
  <div class="card">
    <div class="card-header">
      <h2>Delivery</h2>
      <div class="summary">
        <strong>{{ formatNumber(totals.sent) }}</strong> delivered
        <span class="dot">·</span>
        <strong>{{ formatNumber(totals.failed) }}</strong> failed
        <template v-if="worst && worst.delivery_rate < 100">
          <span class="dot">·</span>
          <span>worst day {{ worst.delivery_rate.toFixed(0) }}% on {{ shortDate(worst.date) }}</span>
        </template>
      </div>
    </div>

    <div class="card-body">
      <div v-if="totals.total === 0" class="empty-state chart-empty">
        <h3>No sends in this window</h3>
        <p>Delivery appears here once emails start going out.</p>
      </div>

      <template v-else>
        <StackedBars :points="bars" :series="series" chart-label="Emails delivered and failed per day">
          <template #tip="{ point }">
            <strong>{{ longDate(point.date) }}</strong>
            <span><i class="key" style="background: var(--success-500)"></i>{{ point.values.sent.toLocaleString() }} delivered</span>
            <span><i class="key" style="background: var(--danger-500)"></i>{{ point.values.failed.toLocaleString() }} failed</span>
            <span v-if="byDate.get(point.date)?.total" class="tip-rate">
              {{ byDate.get(point.date)!.delivery_rate.toFixed(1) }}% delivered
            </span>
          </template>
        </StackedBars>

        <div class="legend">
          <span><i class="key" style="background: var(--success-500)"></i>Delivered</span>
          <span><i class="key" style="background: var(--danger-500)"></i>Failed</span>
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.summary {
  display: flex;
  align-items: baseline;
  gap: 6px;
  flex-wrap: wrap;
  font-size: 12px;
  color: var(--text-tertiary);
}

.summary strong {
  color: var(--text-secondary);
  font-size: 13px;
}

.dot {
  opacity: 0.6;
}

.chart-empty {
  padding: 36px 16px;
}

.legend {
  display: flex;
  gap: 16px;
  margin-top: 14px;
  font-size: 12px;
  color: var(--text-tertiary);
}

.key {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 2px;
  margin-right: 6px;
}

.tip-rate {
  padding-top: 4px;
  margin-top: 2px;
  border-top: 1px solid var(--border-secondary);
  color: var(--text-tertiary);
}
</style>
