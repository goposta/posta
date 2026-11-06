<script setup lang="ts">
import { computed, ref } from "vue";
import { axisLabels, formatNumber, heightPct, niceMax, shortDate } from "./chart";

// Series are given bottom-up: the first entry sits at the base of the stack.
export interface Series {
  key: string;
  label: string;
  colorVar: string;
}

export interface BarPoint {
  date: string;
  values: Record<string, number>;
}

const props = defineProps<{
  points: BarPoint[];
  series: Series[];
  chartLabel: string;
}>();

const hovered = ref<number | null>(null);

const totals = computed(() =>
  props.points.map((p) => props.series.reduce((sum, s) => sum + (p.values[s.key] ?? 0), 0))
);

const peak = computed(() => niceMax(totals.value));
const showLabel = computed(() => axisLabels(props.points.length));

// Rendered top-down so the first series ends up at the bottom of the stack.
const stackOrder = computed(() => [...props.series].reverse());
</script>

<template>
  <div class="chart" :aria-label="chartLabel" role="img">
    <div class="axis" aria-hidden="true">
      <span>{{ formatNumber(peak) }}</span>
      <span>{{ formatNumber(Math.round(peak / 2)) }}</span>
      <span>0</span>
    </div>

    <div class="plot">
      <div class="grid" aria-hidden="true"><span></span><span></span><span></span></div>

      <div class="bars">
        <div
          v-for="(p, idx) in points"
          :key="p.date"
          class="col"
          :class="{ hovered: hovered === idx }"
          @mouseenter="hovered = idx"
          @mouseleave="hovered = null"
        >
          <div class="stack">
            <div
              v-for="s in stackOrder"
              :key="s.key"
              class="bar"
              :style="{ height: heightPct(p.values[s.key] ?? 0, peak), background: `var(${s.colorVar})` }"
            ></div>
          </div>

          <div v-if="hovered === idx" class="tip" role="tooltip">
            <slot name="tip" :point="p" :index="idx" :total="totals[idx]" />
          </div>

          <div class="label">{{ showLabel(idx) ? shortDate(p.date) : "" }}</div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.chart {
  display: flex;
  gap: 10px;
}

.axis {
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  height: 200px;
  padding-bottom: 20px;
  font-size: 11px;
  color: var(--text-tertiary);
  font-variant-numeric: tabular-nums;
  text-align: right;
  min-width: 34px;
}

.plot {
  position: relative;
  flex: 1;
  min-width: 0;
}

.grid {
  position: absolute;
  inset: 0 0 20px 0;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  pointer-events: none;
}

.grid span {
  height: 1px;
  background: var(--border-secondary);
  opacity: 0.7;
}

.bars {
  position: relative;
  display: flex;
  align-items: flex-end;
  gap: 3px;
  height: 200px;
}

.col {
  position: relative;
  flex: 1;
  min-width: 0;
  height: 100%;
  display: flex;
  flex-direction: column;
  justify-content: flex-end;
}

.stack {
  display: flex;
  flex-direction: column;
  justify-content: flex-end;
  height: calc(100% - 20px);
  border-radius: 3px;
  overflow: hidden;
}

.col.hovered .stack {
  filter: brightness(1.08);
}

.bar {
  width: 100%;
  transition: height 0.2s ease;
}

.label {
  height: 20px;
  line-height: 20px;
  font-size: 10px;
  color: var(--text-tertiary);
  text-align: center;
  overflow: hidden;
  white-space: nowrap;
}

.tip {
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
  border: 1px solid var(--border-primary);
  background: var(--bg-primary);
  box-shadow: var(--shadow-lg);
  font-size: 12px;
  color: var(--text-secondary);
  white-space: nowrap;
  pointer-events: none;
}

.tip :deep(strong) {
  color: var(--text-primary);
  font-size: 12px;
}

.tip :deep(.key) {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 2px;
  margin-right: 6px;
}
</style>
