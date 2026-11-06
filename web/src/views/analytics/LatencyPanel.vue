<script setup lang="ts">
import { computed } from "vue";
import type { LatencyPercentiles } from "../../api/types";
import { formatLatency } from "./chart";

const props = defineProps<{ latency: LatencyPercentiles }>();

const rows = computed(() => {
  const l = props.latency;
  return [
    { key: "p50", label: "p50", value: l.p50, note: "Half of your mail is out faster than this" },
    { key: "p75", label: "p75", value: l.p75, note: "" },
    { key: "p90", label: "p90", value: l.p90, note: "" },
    { key: "p99", label: "p99", value: l.p99, note: "The slow tail — one send in a hundred waits this long", tail: true },
  ];
});

const scale = computed(() => Math.max(props.latency.p99, props.latency.avg, 0.001));

const empty = computed(() => props.latency.p50 === 0 && props.latency.avg === 0);

function width(value: number): string {
  return `${Math.max((value / scale.value) * 100, value > 0 ? 2 : 0)}%`;
}

// A long tail relative to the median means a queue that occasionally stalls, not uniform slowness.
const tailRatio = computed(() => (props.latency.p50 > 0 ? props.latency.p99 / props.latency.p50 : 0));
</script>

<template>
  <div class="card">
    <div class="card-header">
      <h2>Time to deliver</h2>
      <div class="summary">
        <template v-if="!empty">
          <strong>{{ formatLatency(latency.avg) }}</strong> average
        </template>
      </div>
    </div>

    <div class="card-body">
      <div v-if="empty" class="empty-state chart-empty">
        <h3>No delivered mail</h3>
        <p>Latency appears once emails complete delivery in this window.</p>
      </div>

      <template v-else>
        <p class="lead">
          How long each email took from accepted to delivered.
          <template v-if="tailRatio >= 10">
            The p99 is {{ tailRatio.toFixed(0) }}× the median, which points at a queue that
            occasionally stalls rather than sending that is uniformly slow.
          </template>
        </p>

        <ul class="rows">
          <li v-for="r in rows" :key="r.key" class="row">
            <span class="label">{{ r.label }}</span>
            <div class="track">
              <div class="fill" :class="{ tail: r.tail }" :style="{ width: width(r.value) }"></div>
            </div>
            <span class="value">{{ formatLatency(r.value) }}</span>
            <span class="note">{{ r.note }}</span>
          </li>
        </ul>
      </template>
    </div>
  </div>
</template>

<style scoped>
.summary {
  font-size: 12px;
  color: var(--text-tertiary);
}

.summary strong {
  color: var(--text-secondary);
  font-size: 13px;
}

.chart-empty {
  padding: 36px 16px;
}

.lead {
  margin: 0 0 16px;
  font-size: 12px;
  color: var(--text-tertiary);
  line-height: 1.6;
  max-width: 70ch;
}

.rows {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.row {
  display: grid;
  grid-template-columns: 34px minmax(80px, 1fr) 70px minmax(0, 1.3fr);
  align-items: center;
  gap: 12px;
}

.label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary);
}

.track {
  height: 8px;
  border-radius: 4px;
  background: var(--bg-hover);
  overflow: hidden;
}

.fill {
  height: 100%;
  border-radius: 4px;
  background: var(--primary-500);
}

.fill.tail {
  background: var(--warning-500);
}

.value {
  font-size: 13px;
  font-variant-numeric: tabular-nums;
  color: var(--text-primary);
  text-align: right;
}

.note {
  font-size: 12px;
  color: var(--text-tertiary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 720px) {
  .row {
    grid-template-columns: 34px 1fr 70px;
  }

  .note {
    display: none;
  }
}
</style>
