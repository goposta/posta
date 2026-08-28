<script setup lang="ts">
import { computed } from "vue";

const props = withDefaults(
  defineProps<{
    values: number[];
    width?: number;
    height?: number;
    stroke?: string;
  }>(),
  {
    width: 120,
    height: 32,
    stroke: "var(--primary-500)",
  }
);

const pad = 2;

const line = computed(() => {
  const v = props.values;
  if (v.length < 2) return "";
  const max = Math.max(...v);
  const min = Math.min(...v);
  const range = max - min || 1;
  const step = (props.width - pad * 2) / (v.length - 1);
  return v
    .map((val, i) => {
      const x = pad + i * step;
      const y = pad + (props.height - pad * 2) * (1 - (val - min) / range);
      return `${i === 0 ? "M" : "L"}${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(" ");
});

const area = computed(() => {
  if (!line.value) return "";
  return `${line.value} L${props.width - pad},${props.height - pad} L${pad},${props.height - pad} Z`;
});
</script>

<template>
  <svg
    :width="width"
    :height="height"
    :viewBox="`0 0 ${width} ${height}`"
    class="sparkline"
    preserveAspectRatio="none"
    aria-hidden="true"
  >
    <path v-if="area" :d="area" class="spark-area" :style="{ fill: stroke }" />
    <path v-if="line" :d="line" class="spark-line" :style="{ stroke }" fill="none" />
  </svg>
</template>

<style scoped>
.sparkline {
  display: block;
  overflow: visible;
}
.spark-line {
  stroke-width: 1.5;
  vector-effect: non-scaling-stroke;
  stroke-linejoin: round;
  stroke-linecap: round;
}
.spark-area {
  opacity: 0.12;
}
</style>
