<script setup lang="ts">
import { computed } from "vue";
import { longDate } from "./chart";

const props = defineProps<{
  preset: string;
  from: string;
  to: string;
  loading: boolean;
  refreshing: boolean;
  updatedLabel: string;
  canExport: boolean;
}>();

const emit = defineEmits<{
  (e: "update:preset", value: string): void;
  (e: "update:from", value: string): void;
  (e: "update:to", value: string): void;
  (e: "refresh"): void;
  (e: "export"): void;
}>();

const PRESETS = [
  { key: "7d", label: "7 days" },
  { key: "30d", label: "30 days" },
  { key: "90d", label: "90 days" },
  { key: "custom", label: "Custom" },
];

// A relative range does not say where the charts start, and axis labels are in the reader's timezone, so spell the window out.
const tz = Intl.DateTimeFormat().resolvedOptions().timeZone || "local time";

const window = computed(() => `${longDate(props.from)} — ${longDate(props.to)}`);

const custom = computed(() => props.preset === "custom");
</script>

<template>
  <div>
    <div class="page-header">
      <div>
        <h1>Analytics</h1>
        <p class="page-subtitle">
          {{ window }} · {{ tz }}
          <span v-if="updatedLabel" class="dot">·</span>
          <span v-if="updatedLabel">updated {{ updatedLabel }}</span>
        </p>
      </div>
      <div class="header-actions">
        <button
          class="btn btn-secondary"
          :disabled="!canExport"
          :title="canExport ? 'Download this report as CSV' : 'Nothing to export yet'"
          @click="emit('export')"
        >
          Export CSV
        </button>
        <button class="btn btn-secondary" :disabled="loading || refreshing" @click="emit('refresh')">
          {{ refreshing ? "Refreshing…" : "Refresh" }}
        </button>
      </div>
    </div>

    <div class="range">
      <div class="chips" role="group" aria-label="Time range">
        <button
          v-for="p in PRESETS"
          :key="p.key"
          type="button"
          class="chip"
          :class="{ active: preset === p.key }"
          :aria-pressed="preset === p.key"
          @click="emit('update:preset', p.key)"
        >
          {{ p.label }}
        </button>
      </div>

      <div v-if="custom" class="custom">
        <label class="custom-field">
          <span>From</span>
          <input
            :value="from"
            type="date"
            class="form-input"
            :max="to"
            @change="emit('update:from', ($event.target as HTMLInputElement).value)"
          />
        </label>
        <label class="custom-field">
          <span>To</span>
          <input
            :value="to"
            type="date"
            class="form-input"
            :min="from"
            @change="emit('update:to', ($event.target as HTMLInputElement).value)"
          />
        </label>
      </div>
    </div>
  </div>
</template>

<style scoped>
.page-subtitle {
  font-size: 13px;
  color: var(--text-tertiary);
  margin-top: 4px;
}

.dot {
  opacity: 0.6;
  margin: 0 2px;
}

.header-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.range {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
  margin-bottom: 20px;
}

.chips {
  display: flex;
  gap: 6px;
}

.chip {
  padding: 6px 14px;
  border: 1px solid var(--border-primary);
  border-radius: 999px;
  background: var(--bg-primary);
  font-size: 12px;
  font-weight: 500;
  color: var(--text-secondary);
  cursor: pointer;
}

.chip:hover {
  color: var(--text-primary);
  background: var(--bg-hover);
}

.chip.active {
  background: var(--primary-600);
  border-color: var(--primary-600);
  color: #fff;
}

.custom {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.custom-field {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--text-tertiary);
}

.custom-field .form-input {
  width: auto;
}
</style>
