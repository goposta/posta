<script setup lang="ts">
import { computed } from "vue";
import { useRouter } from "vue-router";
import type { AdminMetrics } from "../../api/types";

const props = defineProps<{ metrics: AdminMetrics }>();
const router = useRouter();

interface Item {
  label: string;
  value: number;
  to?: string;
}

// Flat counts belong in one compact row, not six cards each carrying an icon
// and a chunk of vertical space. Runtime figures and ratios stay on the health
// card above; this is inventory only, and each number appears in exactly one
// of the two.
const items = computed<Item[]>(() => {
  const m = props.metrics;
  const out: Item[] = [
    { label: "Users", value: m.total_users, to: "/admin/users" },
    { label: "Workspaces", value: m.total_workspaces, to: "/admin/workspaces" },
    { label: "Domains", value: m.total_domains },
    { label: "Shared SMTP", value: m.shared_smtp_servers, to: "/admin/servers" },
    { label: "Emails", value: m.total_emails },
  ];
  if (m.total_inbound > 0) {
    out.push({ label: "Inbound", value: m.total_inbound });
  }
  return out;
});

function formatNumber(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + "M";
  if (n >= 10_000) return (n / 1_000).toFixed(1) + "K";
  return n.toLocaleString();
}
</script>

<template>
  <div class="inventory">
    <component
      v-for="item in items"
      :is="item.to ? 'button' : 'div'"
      :key="item.label"
      class="inv"
      :class="{ 'is-clickable': !!item.to }"
      :type="item.to ? 'button' : undefined"
      @click="item.to && router.push(item.to)"
    >
      <span class="inv-value">{{ formatNumber(item.value) }}</span>
      <span class="inv-label">{{ item.label }}</span>
    </component>
  </div>
</template>

<style scoped>
.inventory {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(110px, 1fr));
  gap: 1px;
  background: var(--border-primary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg, 10px);
  overflow: hidden;
  margin-bottom: 28px;
}

.inv {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 14px 16px;
  background: var(--bg-primary);
  border: 0;
  font: inherit;
  text-align: left;
  min-width: 0;
}

.is-clickable {
  cursor: pointer;
  transition: background 0.15s;
}

.is-clickable:hover {
  background: var(--bg-hover);
}

.is-clickable:focus-visible {
  outline: 2px solid var(--border-focus);
  outline-offset: -2px;
}

.inv-value {
  font-size: 20px;
  font-weight: 600;
  color: var(--text-primary);
  font-variant-numeric: tabular-nums;
  line-height: 1.2;
}

.inv-label {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  font-weight: 600;
  color: var(--text-tertiary);
}
</style>
