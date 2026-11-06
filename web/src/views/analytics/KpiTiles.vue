<script setup lang="ts">
import { computed } from "vue";
import { formatLatency, formatNumber, formatPercent } from "./chart";

export interface Kpi {
  key: string;
  label: string;
  value: number;
  kind: "count" | "percent" | "latency";
  hint: string;
  // Percentage points for rates, percent for counts; null when there is no prior figure.
  change: number | null;
  // Whether a rise is coloured as good; false for bounce and latency.
  higherIsBetter: boolean;
}

const props = defineProps<{ kpis: Kpi[]; comparable: boolean }>();

function display(k: Kpi): string {
  if (k.kind === "percent") return formatPercent(k.value);
  if (k.kind === "latency") return formatLatency(k.value);
  return formatNumber(k.value);
}

function changeLabel(k: Kpi): string {
  if (k.change === null) return "";
  const arrow = k.change > 0 ? "↑" : k.change < 0 ? "↓" : "→";
  const magnitude = Math.abs(k.change);
  // A rate's move is stated in points: "bounce rate rose 50%" is ambiguous between 2%→3% and 2%→52%.
  const unit = k.kind === "percent" ? " pts" : "%";
  return `${arrow} ${magnitude.toFixed(magnitude < 10 ? 1 : 0)}${unit}`;
}

function tone(k: Kpi): string {
  if (k.change === null || Math.abs(k.change) < 0.05) return "flat";
  const improving = k.change > 0 === k.higherIsBetter;
  return improving ? "good" : "bad";
}

const tiles = computed(() => props.kpis);
</script>

<template>
  <div class="kpis">
    <div v-for="k in tiles" :key="k.key" class="kpi">
      <div class="kpi-label">{{ k.label }}</div>
      <div class="kpi-value">{{ display(k) }}</div>
      <div class="kpi-foot">
        <span v-if="comparable && k.change !== null" class="kpi-change" :class="tone(k)">
          {{ changeLabel(k) }}
        </span>
        <span class="kpi-hint">{{ comparable && k.change !== null ? "vs previous period" : k.hint }}</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.kpis {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(190px, 1fr));
  gap: 16px;
  margin-bottom: 24px;
}

.kpi {
  padding: 16px 18px;
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  background: var(--bg-primary);
}

.kpi-label {
  font-size: 12px;
  font-weight: 500;
  color: var(--text-tertiary);
}

.kpi-value {
  margin-top: 6px;
  font-size: 26px;
  font-weight: 700;
  line-height: 1.1;
  color: var(--text-primary);
  font-variant-numeric: tabular-nums;
}

.kpi-foot {
  display: flex;
  align-items: baseline;
  gap: 6px;
  flex-wrap: wrap;
  margin-top: 8px;
  font-size: 12px;
}

.kpi-change {
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.kpi-change.good {
  color: var(--success-text);
}

.kpi-change.bad {
  color: var(--danger-text);
}

.kpi-change.flat {
  color: var(--text-tertiary);
}

.kpi-hint {
  color: var(--text-tertiary);
}
</style>
