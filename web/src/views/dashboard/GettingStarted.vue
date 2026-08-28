<script setup lang="ts">
import { computed } from "vue";
import { useRouter } from "vue-router";
import type { DashboardStats } from "../../api/types";

const props = defineProps<{ stats: DashboardStats }>();
const emit = defineEmits<{ dismiss: [] }>();
const router = useRouter();

const steps = computed(() => {
  const s = props.stats;
  const verifiedDomains = s.total_domains - s.unverified_domains;
  return [
    {
      key: "domain",
      label: "Verify a sending domain",
      hint: "Prove ownership, then add SPF, DKIM, and DMARC records.",
      done: verifiedDomains > 0,
      to: "/domains",
    },
    {
      key: "smtp",
      label: "Connect an SMTP server",
      hint: "Posta hands your mail to this server for delivery.",
      done: s.total_smtp_servers > 0,
      to: "/smtp-servers",
    },
    {
      key: "key",
      label: "Create an API key",
      hint: "Scoped credentials your application sends with.",
      done: s.total_api_keys > 0,
      to: "/api-keys",
    },
    {
      key: "send",
      label: "Send your first email",
      hint: "Through the API, the SMTP relay, or a template.",
      done: s.total_emails > 0,
      to: "/emails",
    },
  ];
});

const doneCount = computed(() => steps.value.filter((s) => s.done).length);
const complete = computed(() => doneCount.value === steps.value.length);
const nextStep = computed(() => steps.value.find((s) => !s.done));
</script>

<template>
  <div v-if="!complete" class="card getting-started">
    <div class="card-header">
      <h2>Get Posta sending</h2>
      <div class="gs-head-right">
        <span class="gs-count">{{ doneCount }} of {{ steps.length }}</span>
        <button class="gs-dismiss" type="button" title="Hide this checklist" @click="emit('dismiss')">
          Dismiss
        </button>
      </div>
    </div>
    <div class="card-body">
      <div class="gs-progress" role="progressbar" :aria-valuenow="doneCount" aria-valuemin="0" :aria-valuemax="steps.length">
        <div class="gs-progress-fill" :style="{ width: `${(doneCount / steps.length) * 100}%` }"></div>
      </div>

      <ol class="gs-steps">
        <li
          v-for="step in steps"
          :key="step.key"
          class="gs-step"
          :class="{ 'is-done': step.done, 'is-next': step.key === nextStep?.key }"
        >
          <span class="gs-mark" aria-hidden="true">
            <svg v-if="step.done" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M3.5 8.5l3 3 6-7" />
            </svg>
          </span>
          <span class="gs-body">
            <span class="gs-label">{{ step.label }}</span>
            <span class="gs-hint">{{ step.hint }}</span>
          </span>
          <button v-if="!step.done" class="btn btn-secondary btn-sm" @click="router.push(step.to)">
            {{ step.key === nextStep?.key ? "Start" : "Open" }}
          </button>
        </li>
      </ol>
    </div>
  </div>
</template>

<style scoped>
.gs-head-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.gs-count {
  font-size: 12px;
  color: var(--text-tertiary);
  font-variant-numeric: tabular-nums;
}

.gs-dismiss {
  border: 0;
  background: none;
  padding: 0;
  font: inherit;
  font-size: 12px;
  color: var(--text-tertiary);
  cursor: pointer;
}

.gs-dismiss:hover {
  color: var(--text-secondary);
  text-decoration: underline;
}

.gs-progress {
  height: 4px;
  border-radius: 999px;
  background: var(--bg-tertiary);
  overflow: hidden;
  margin-bottom: 18px;
}

.gs-progress-fill {
  height: 100%;
  background: var(--primary-500);
  transition: width 0.3s ease;
}

.gs-steps {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.gs-step {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 0;
  border-bottom: 1px solid var(--border-secondary);
}

.gs-step:last-child {
  border-bottom: 0;
}

.gs-mark {
  flex: none;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  border: 1.5px solid var(--border-input);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--text-inverse);
}

.gs-mark svg {
  width: 12px;
  height: 12px;
}

.is-done .gs-mark {
  background: var(--success-600);
  border-color: var(--success-600);
}

.is-next .gs-mark {
  border-color: var(--primary-500);
  border-style: dashed;
}

.gs-body {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.gs-label {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-primary);
}

.is-done .gs-label {
  color: var(--text-tertiary);
  text-decoration: line-through;
}

.gs-hint {
  font-size: 12px;
  color: var(--text-tertiary);
}

.is-done .gs-hint {
  display: none;
}

.btn-sm {
  padding: 5px 12px;
  font-size: 12px;
}
</style>
