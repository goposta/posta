<script setup lang="ts">
import { useRouter } from "vue-router";

const router = useRouter();

// Destinations that have no number worth showing. Anything countable lives in
// the inventory row instead, where the count is the point and the tile is also
// the link — so no figure appears twice on this page.
const links = [
  { to: "/admin/plans", label: "Plans" },
  { to: "/admin/jobs", label: "Jobs" },
  { to: "/admin/events", label: "Events" },
  { to: "/admin/settings", label: "Settings" },
];
</script>

<template>
  <div class="ql-row">
    <button v-for="l in links" :key="l.to" class="ql" type="button" @click="router.push(l.to)">
      {{ l.label }}
    </button>
  </div>
</template>

<style scoped>
.ql-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 20px;
}

.ql {
  padding: 7px 14px;
  border-radius: 8px;
  background: var(--bg-primary);
  border: 1px solid var(--border-primary);
  color: var(--text-primary);
  font: inherit;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s, transform 0.15s;
}

.ql:hover {
  border-color: var(--primary-500);
  background: var(--bg-hover);
  transform: translateY(-1px);
}

.ql:focus-visible {
  outline: 2px solid var(--border-focus);
  outline-offset: 2px;
}

@media (max-width: 719px) {
  .ql-row {
    flex-wrap: nowrap;
    overflow-x: auto;
    scrollbar-width: none;
  }
  .ql-row::-webkit-scrollbar { display: none; }
  .ql { flex: 0 0 auto; }
}
</style>
