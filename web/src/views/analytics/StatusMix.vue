<script setup lang="ts">
import { computed } from "vue";
import type { StatusBreakdown } from "../../api/types";
import { formatNumber } from "./chart";

const props = defineProps<{ rows: StatusBreakdown[] }>();

const MEANING: Record<string, string> = {
  sent: "Handed off to the receiving server",
  failed: "The send did not succeed",
  queued: "Waiting for a worker",
  processing: "Being sent right now",
  scheduled: "Held until its send time",
  suppressed: "Blocked before sending by the suppression list",
  pending: "Accepted, not yet queued",
};

const COLOR: Record<string, string> = {
  sent: "var(--success-500)",
  failed: "var(--danger-500)",
  queued: "var(--primary-500)",
  processing: "var(--warning-500)",
  scheduled: "var(--primary-400)",
  suppressed: "var(--text-tertiary)",
  pending: "var(--warning-500)",
};

const total = computed(() => props.rows.reduce((sum, r) => sum + r.count, 0));

const sorted = computed(() => [...props.rows].sort((a, b) => b.count - a.count));

function percent(count: number): number {
  return total.value === 0 ? 0 : (count / total.value) * 100;
}

function color(status: string): string {
  return COLOR[status] ?? "var(--text-tertiary)";
}
</script>

<template>
  <div class="card">
    <div class="card-header">
      <h2>Status mix</h2>
      <div class="summary"><strong>{{ formatNumber(total) }}</strong> emails</div>
    </div>

    <div class="card-body">
      <div v-if="total === 0" class="empty-state chart-empty">
        <h3>No emails</h3>
        <p>Nothing was sent in this window.</p>
      </div>

      <template v-else>
        <div class="ribbon" role="img" aria-label="Share of emails by status">
          <div
            v-for="r in sorted"
            :key="r.status"
            class="ribbon-part"
            :style="{ width: percent(r.count) + '%', background: color(r.status) }"
            :title="`${r.status}: ${r.count}`"
          ></div>
        </div>

        <ul class="rows">
          <li v-for="r in sorted" :key="r.status" class="row">
            <span class="dot" :style="{ background: color(r.status) }" aria-hidden="true"></span>
            <span class="name">{{ r.status }}</span>
            <span class="meaning">{{ MEANING[r.status] || "" }}</span>
            <span class="count">{{ r.count.toLocaleString() }}</span>
            <span class="pct">{{ percent(r.count).toFixed(1) }}%</span>
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

.ribbon {
  display: flex;
  height: 10px;
  border-radius: 5px;
  overflow: hidden;
  background: var(--bg-hover);
}

.ribbon-part {
  height: 100%;
  min-width: 2px;
}

.rows {
  list-style: none;
  margin: 16px 0 0;
  padding: 0;
}

.row {
  display: grid;
  grid-template-columns: 10px 90px 1fr auto auto;
  align-items: center;
  gap: 10px;
  padding: 9px 0;
  border-bottom: 1px solid var(--border-secondary);
  font-size: 13px;
}

.row:last-child {
  border-bottom: 0;
}

.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.name {
  font-weight: 600;
  color: var(--text-primary);
  text-transform: capitalize;
}

.meaning {
  font-size: 12px;
  color: var(--text-tertiary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.count {
  font-variant-numeric: tabular-nums;
  color: var(--text-primary);
}

.pct {
  min-width: 52px;
  text-align: right;
  font-variant-numeric: tabular-nums;
  color: var(--text-tertiary);
}

@media (max-width: 640px) {
  .row {
    grid-template-columns: 10px 1fr auto;
  }

  .meaning {
    display: none;
  }
}
</style>
