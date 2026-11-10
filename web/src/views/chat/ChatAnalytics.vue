<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { chatAnalyticsApi, chatWidgetsApi } from "../../api/chat";
import type { ChatAnalytics, ChatWidget } from "../../api/types";
import { useNotificationStore } from "../../stores/notification";
import { apiMessage } from "../../composables/apiError";
import EnterpriseLock from "../../components/EnterpriseLock.vue";
import { useLicenseStore } from "../../stores/license";

const license = useLicenseStore();

const notify = useNotificationStore();

const PRESETS = [7, 30, 90] as const;
const days = ref<number>(30);
const widgetId = ref<number | 0>(0);
const widgets = ref<ChatWidget[]>([]);
const loading = ref(true);
const featureDisabled = ref(false);
const data = ref<ChatAnalytics | null>(null);

function widgetLabel(id: number): string {
  return widgets.value.find((w) => w.id === id)?.display_name ?? `#${id}`;
}

function isoDay(d: Date): string {
  return d.toISOString().slice(0, 10);
}

/** Every day of the range, so empty days show as empty bars. */
const series = computed(() => {
  const counts = new Map((data.value?.by_day ?? []).map((d) => [d.day, d.conversations]));
  const out: { day: string; conversations: number }[] = [];
  const end = new Date();
  for (let i = days.value - 1; i >= 0; i--) {
    const d = new Date(end);
    d.setUTCDate(end.getUTCDate() - i);
    const key = isoDay(d);
    out.push({ day: key, conversations: counts.get(key) ?? 0 });
  }
  return out;
});
const peak = computed(() => Math.max(1, ...series.value.map((d) => d.conversations)));

async function load() {
  loading.value = true;
  const to = new Date();
  to.setUTCDate(to.getUTCDate() + 1);
  const from = new Date();
  from.setUTCDate(from.getUTCDate() - (days.value - 1));
  try {
    const res = await chatAnalyticsApi.get({
      from: isoDay(from),
      to: isoDay(to),
      widget_id: widgetId.value || undefined,
    });
    data.value = res.data.data;
    featureDisabled.value = false;
  } catch (e: any) {
    if (e?.response?.status === 404) featureDisabled.value = true;
    else notify.error(apiMessage(e, "Failed to load chat analytics"));
  } finally {
    loading.value = false;
  }
}

async function loadWidgets() {
  try {
    widgets.value = (await chatWidgetsApi.list({ size: 100 })).data.data;
  } catch {
    widgets.value = [];
  }
}

/** Seconds as "2m 30s", "1h 5m", "3d 4h"; "—" when there is nothing to measure. */
function duration(seconds: number | null | undefined): string {
  if (seconds === null || seconds === undefined) return "—";
  const s = Math.round(seconds);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return s % 60 ? `${m}m ${s % 60}s` : `${m}m`;
  const h = Math.floor(m / 60);
  if (h < 24) return m % 60 ? `${h}h ${m % 60}m` : `${h}h`;
  const d = Math.floor(h / 24);
  return h % 24 ? `${d}d ${h % 24}h` : `${d}d`;
}

const tiles = computed(() => {
  const a = data.value;
  if (!a) return [];
  return [
    { label: "Conversations", value: a.conversations.toLocaleString(), hint: `${a.open} open · ${a.closed} closed` },
    { label: "Median first response", value: duration(a.first_response_median), hint: `average ${duration(a.first_response_avg)}` },
    { label: "Median resolution", value: duration(a.resolution_median), hint: `average ${duration(a.resolution_avg)}` },
    { label: "Unanswered", value: a.unanswered.toLocaleString(), hint: "no agent reply yet" },
    { label: "Spam", value: a.spam.toLocaleString(), hint: "marked as spam" },
    { label: "Messages", value: a.messages.toLocaleString(), hint: "from both sides" },
  ];
});

function shortDay(day: string): string {
  return new Date(`${day}T00:00:00Z`).toLocaleDateString([], { month: "short", day: "numeric", timeZone: "UTC" });
}

// Chat analytics is Posta Enterprise: without a license there is nothing to load.
watch([days, widgetId], () => {
  if (license.licensed) load();
});
onMounted(async () => {
  await license.load();
  if (!license.licensed) return;
  load();
  loadWidgets();
});
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Chat analytics</h1>
      <div v-if="license.licensed" class="controls">
        <div class="presets" role="group" aria-label="Period">
          <button v-for="p in PRESETS" :key="p" class="btn btn-sm" :class="days === p ? 'btn-primary' : 'btn-secondary'"
            :aria-pressed="days === p" @click="days = p">{{ p }} days</button>
        </div>
        <select v-model.number="widgetId" class="form-select" aria-label="Widget">
          <option :value="0">All widgets</option>
          <option v-for="w in widgets" :key="w.id" :value="w.id">{{ w.display_name }}</option>
        </select>
      </div>
    </div>

    <EnterpriseLock
      feature="Chat analytics"
      description="See how many chats you get, how fast your team first answers, and how long chats take to resolve, per day and per widget."
    >
    <div v-if="featureDisabled" class="card">
      <div class="empty-state">
        <h3>Live chat is disabled</h3>
        <p>Ask your administrator to enable it by setting <code>POSTA_CHAT_ENABLED=true</code>.</p>
      </div>
    </div>
    <div v-else-if="loading && !data" class="loading-page"><div class="spinner"></div></div>
    <template v-else-if="data">
      <div class="tiles" :class="{ dim: loading }">
        <div v-for="t in tiles" :key="t.label" class="card tile">
          <div class="tile-label">{{ t.label }}</div>
          <div class="tile-value">{{ t.value }}</div>
          <div class="tile-hint">{{ t.hint }}</div>
        </div>
      </div>

      <div class="card">
        <div class="card-header"><h2>Conversations per day</h2></div>
        <div class="card-body">
          <div v-if="data.conversations === 0" class="empty-state"><p>No conversations in this period.</p></div>
          <template v-else>
            <div class="bars" role="img"
              :aria-label="`Conversations per day over the last ${days} days, peaking at ${peak}`">
              <div v-for="d in series" :key="d.day" class="bar-col" :title="`${shortDay(d.day)}: ${d.conversations}`">
                <div class="bar" :style="{ height: `${(d.conversations / peak) * 100}%` }"></div>
              </div>
            </div>
            <div class="axis">
              <span>{{ shortDay(series[0]!.day) }}</span>
              <span>peak {{ peak }}</span>
              <span>{{ shortDay(series[series.length - 1]!.day) }}</span>
            </div>
            <table class="sr-only">
              <caption>Conversations per day</caption>
              <tbody>
                <tr v-for="d in series" :key="d.day"><th scope="row">{{ d.day }}</th><td>{{ d.conversations }}</td></tr>
              </tbody>
            </table>
          </template>
        </div>
      </div>
      <div class="card" style="margin-top: 20px">
        <div class="card-header"><h2>Home clicks</h2></div>
        <div class="card-body">
          <p v-if="!data.home_clicks?.length" class="muted" style="margin: 0">
            No clicks on home screen items or help links in this period.
          </p>
          <div v-else class="table-wrapper"><table>
            <thead>
              <tr><th scope="col">Item</th><th v-if="!widgetId" scope="col">Widget</th><th scope="col" class="num">Clicks</th></tr>
            </thead>
            <tbody>
              <tr v-for="c in data.home_clicks" :key="`${c.widget_id}:${c.item_id}`">
                <td>{{ c.title || `Removed item (${c.item_id})` }}</td>
                <td v-if="!widgetId">{{ widgetLabel(c.widget_id) }}</td>
                <td class="num">{{ c.clicks }}</td>
              </tr>
            </tbody>
          </table></div>
          <small class="muted">Counted anonymously, per item and day: no visitor, IP or page is recorded.</small>
        </div>
      </div>
    </template>
    </EnterpriseLock>
  </div>
</template>

<style scoped>
.controls {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}
.presets {
  display: flex;
  gap: 4px;
}
.tiles {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(190px, 1fr));
  gap: 16px;
  margin-bottom: 16px;
  transition: opacity 0.15s;
}
.tiles.dim {
  opacity: 0.6;
}
.tile {
  margin: 0;
  padding: 16px 18px;
}
.tile-label {
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-muted);
}
.tile-value {
  font-size: 26px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 6px 0 2px;
  font-variant-numeric: tabular-nums;
}
.tile-hint {
  font-size: 12px;
  color: var(--text-secondary);
}
.bars {
  display: flex;
  align-items: flex-end;
  gap: 2px;
  height: 180px;
  border-bottom: 1px solid var(--border-primary);
}
.bar-col {
  flex: 1;
  height: 100%;
  display: flex;
  align-items: flex-end;
}
.bar {
  width: 100%;
  min-height: 1px;
  background: var(--primary-500, var(--primary, #7c3aed));
  border-radius: 3px 3px 0 0;
}
.axis {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 6px;
}
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
}
@media (prefers-reduced-motion: reduce) {
  .tiles {
    transition: none;
  }
}
.num {
  text-align: right;
}
</style>
