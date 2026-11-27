<script setup lang="ts">
import { useId } from 'vue'

// SMIL ignores the CSS media query, so reduced motion has to drop the <animate> elements instead.
const still = typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches
const gid = `auth-ribbons-${useId()}`
const count = 30

function fan(i: number, t: number) {
  const k = i / (count - 1)
  const c1x = 200 + k * 120 + t * 70
  const c1y = 820 - k * 200
  const c2x = 420 - k * 60
  const c2y = 300 + k * 260 - t * 90
  return `M -40 940 C ${c1x.toFixed(1)} ${c1y.toFixed(1)} ${c2x.toFixed(1)} ${c2y.toFixed(1)} 760 ${(-60 + k * 760).toFixed(1)}`
}

const lines = Array.from({ length: count }, (_, i) => {
  const d = fan(i, 0)
  return {
    d,
    values: `${d};${fan(i, 1)};${d}`,
    dur: `${11 + (i % 5) * 1.2}s`,
    width: i % 7 === 0 ? 2.2 : 1.1,
    opacity: 0.35 + 0.65 * (i / count),
  }
})

const streaks = [4, 10, 15, 21, 26].map((i, j) => ({ d: fan(i, 0), delay: `${j * -1.4}s` }))
</script>

<template>
  <svg class="ribbons" aria-hidden="true" viewBox="0 0 720 900" preserveAspectRatio="xMidYMid slice">
    <defs>
      <linearGradient :id="gid" x1="0" y1="900" x2="720" y2="0" gradientUnits="userSpaceOnUse">
        <stop offset="0" stop-color="#c084fc" stop-opacity=".15" />
        <stop offset=".45" stop-color="#e9d5ff" stop-opacity=".75" />
        <stop offset=".8" stop-color="#f9a8d4" stop-opacity=".6" />
        <stop offset="1" stop-color="#f9a8d4" stop-opacity="0" />
      </linearGradient>
    </defs>
    <g fill="none" :stroke="`url(#${gid})`">
      <path v-for="(l, i) in lines" :key="i" :d="l.d" :stroke-width="l.width" :stroke-opacity="l.opacity">
        <animate v-if="!still" attributeName="d" :values="l.values" :dur="l.dur" repeatCount="indefinite" calcMode="spline" keyTimes="0;0.5;1" keySplines="0.45 0 0.55 1;0.45 0 0.55 1" />
      </path>
    </g>
    <g v-if="!still" fill="none">
      <path v-for="(s, i) in streaks" :key="i" class="streak" :d="s.d" pathLength="1000" :style="{ animationDelay: s.delay }" />
    </g>
  </svg>
</template>

<style scoped>
.ribbons {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
}
.streak {
  stroke: #f5d0fe;
  stroke-opacity: 0.55;
  stroke-width: 2;
  stroke-linecap: round;
  stroke-dasharray: 90 1000;
  animation: streak 7s linear infinite;
}
@keyframes streak {
  from { stroke-dashoffset: 1090; }
  to { stroke-dashoffset: 0; }
}
@media (prefers-reduced-motion: reduce) {
  .streak { animation: none; display: none; }
}
</style>
