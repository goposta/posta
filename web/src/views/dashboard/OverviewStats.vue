<script setup lang="ts">
import { computed } from "vue";
import { useRouter } from "vue-router";
import Sparkline from "../../components/Sparkline.vue";
import type { DashboardStats } from "../../api/types";

const props = defineProps<{ stats: DashboardStats }>();
const router = useRouter();

const sentSeries = computed(() => props.stats.daily_volume.map((d) => d.sent));
const failedSeries = computed(() => props.stats.daily_volume.map((d) => d.failed));

const deliveryRate = computed(() => {
  const s = props.stats;
  const attempted = s.sent_emails + s.failed_emails;
  return attempted === 0 ? 0 : (s.sent_emails / attempted) * 100;
});

const inFlight = computed(() => props.stats.queued_emails + props.stats.processing_emails);

function formatNumber(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + "M";
  if (n >= 10_000) return (n / 1_000).toFixed(1) + "K";
  return n.toLocaleString();
}

function deliveryTone(rate: number) {
  if (rate >= 98) return "success";
  if (rate >= 90) return "warning";
  return "danger";
}
</script>

<template>
  <div class="stats-grid">
    <button class="stat-card stat-clickable" @click="router.push('/emails')">
      <div class="stat-header">
        <div class="stat-label">Total emails</div>
        <div class="stat-icon stat-icon-primary">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 4h16c1.1 0 2 .9 2 2v12c0 1.1-.9 2-2 2H4c-1.1 0-2-.9-2-2V6c0-1.1.9-2 2-2z"/><polyline points="22,6 12,13 2,6"/></svg>
        </div>
      </div>
      <div class="stat-value">{{ formatNumber(stats.total_emails) }}</div>
      <Sparkline class="stat-spark" :values="sentSeries" :height="28" />
    </button>

    <button class="stat-card stat-clickable" @click="router.push('/analytics')">
      <div class="stat-header">
        <div class="stat-label">Delivery rate</div>
        <div class="stat-icon" :class="`stat-icon-${deliveryTone(deliveryRate)}`">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="23 6 13.5 15.5 8.5 10.5 1 18"/><polyline points="17 6 23 6 23 12"/></svg>
        </div>
      </div>
      <div class="stat-value">{{ deliveryRate.toFixed(1) }}%</div>
      <div class="stat-sub">{{ formatNumber(stats.sent_emails) }} delivered of {{ formatNumber(stats.sent_emails + stats.failed_emails) }} attempted</div>
    </button>

    <button
      class="stat-card stat-clickable"
      :class="{ 'stat-alert': stats.failed_emails > 0 && stats.failure_rate >= 10 }"
      @click="router.push('/emails?status=failed')"
    >
      <div class="stat-header">
        <div class="stat-label">Failed</div>
        <div class="stat-icon stat-icon-danger">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><line x1="15" y1="9" x2="9" y2="15"/><line x1="9" y1="9" x2="15" y2="15"/></svg>
        </div>
      </div>
      <div class="stat-value">{{ formatNumber(stats.failed_emails) }}</div>
      <Sparkline class="stat-spark" :values="failedSeries" :height="28" stroke="var(--danger-500)" />
    </button>

    <button v-if="inFlight > 0" class="stat-card stat-clickable" @click="router.push('/emails?status=queued')">
      <div class="stat-header">
        <div class="stat-label">In flight</div>
        <div class="stat-icon stat-icon-info">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
        </div>
      </div>
      <div class="stat-value">{{ formatNumber(inFlight) }}</div>
      <div class="stat-sub">{{ formatNumber(stats.queued_emails) }} queued · {{ formatNumber(stats.processing_emails) }} sending</div>
    </button>

    <button
      v-if="stats.features?.messages"
      class="stat-card stat-clickable"
      :class="{ 'stat-alert': stats.unread_messages > 0 }"
      @click="router.push('/messages')"
    >
      <div class="stat-header">
        <div class="stat-label">Messages</div>
        <div class="stat-icon stat-icon-primary">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>
        </div>
      </div>
      <div class="stat-value">{{ formatNumber(stats.total_messages) }}</div>
      <div class="stat-sub">
        <span v-if="stats.unread_messages > 0" class="stat-sub-strong">{{ stats.unread_messages }} unread</span>
        <span v-else>all read</span>
        <span v-if="stats.spam_messages > 0"> · {{ formatNumber(stats.spam_messages) }} spam</span>
      </div>
    </button>

    <button
      v-if="stats.features?.inbound && stats.total_inbound > 0"
      class="stat-card stat-clickable"
      @click="router.push('/inbound-emails')"
    >
      <div class="stat-header">
        <div class="stat-label">Inbound</div>
        <div class="stat-icon stat-icon-info">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="22 12 16 12 14 15 10 15 8 12 2 12"/><path d="M5.45 5.11L2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z"/></svg>
        </div>
      </div>
      <div class="stat-value">{{ formatNumber(stats.total_inbound) }}</div>
      <div class="stat-sub">{{ formatNumber(stats.forwarded_inbound) }} forwarded · {{ formatNumber(stats.failed_inbound) }} failed</div>
    </button>
  </div>
</template>

<style scoped>
.stat-clickable {
  display: block;
  width: 100%;
  text-align: left;
  font: inherit;
  cursor: pointer;
  transition: border-color 0.15s, transform 0.15s, box-shadow 0.15s;
}

.stat-clickable:hover {
  border-color: var(--border-input);
  transform: translateY(-1px);
  box-shadow: var(--shadow-md);
}

.stat-clickable:focus-visible {
  outline: 2px solid var(--border-focus);
  outline-offset: 2px;
}

.stat-alert {
  border-color: var(--warning-500);
}

.stat-spark {
  margin-top: 10px;
  width: 100%;
}

.stat-sub-strong {
  color: var(--warning-600);
  font-weight: 600;
}
</style>
