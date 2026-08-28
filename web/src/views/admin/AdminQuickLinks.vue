<script setup lang="ts">
import { useRouter } from "vue-router";
import type { AdminMetrics } from "../../api/types";

const props = defineProps<{ metrics: AdminMetrics }>();
const router = useRouter();

const links = [
  { to: "/admin/users", label: "Users", count: () => props.metrics.total_users },
  { to: "/admin/workspaces", label: "Workspaces", count: () => props.metrics.total_workspaces },
  { to: "/admin/servers", label: "Shared SMTP", count: () => props.metrics.shared_smtp_servers },
  { to: "/admin/plans", label: "Plans", count: () => undefined },
  { to: "/admin/jobs", label: "Jobs", count: () => undefined },
  { to: "/admin/events", label: "Events", count: () => undefined },
  { to: "/admin/settings", label: "Settings", count: () => undefined },
];
</script>

<template>
  <div class="ql-row">
    <button v-for="l in links" :key="l.to" class="ql" @click="router.push(l.to)">
      <span class="ql-label">{{ l.label }}</span>
      <span v-if="l.count() !== undefined" class="ql-count">{{ l.count()!.toLocaleString() }}</span>
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
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 7px 13px;
  border-radius: 8px;
  background: var(--bg-primary);
  border: 1px solid var(--border-primary);
  color: var(--text-primary);
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

.ql-count {
  font-size: 11px;
  font-weight: 600;
  color: var(--text-tertiary);
  background: var(--bg-tertiary);
  padding: 1px 7px;
  border-radius: 999px;
  font-variant-numeric: tabular-nums;
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
