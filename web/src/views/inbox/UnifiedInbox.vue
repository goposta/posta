<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { inboxItemRoute, unifiedInboxApi } from "../../api/unifiedInbox";
import type { InboxItem, InboxListParams, InboxSource } from "../../api/unifiedInbox";
import { dashboardApi } from "../../api/dashboard";
import Pagination from "../../components/Pagination.vue";
import { usePagination } from "../../composables/usePagination";
import { apiMessage } from "../../composables/apiError";
import { useAuthStore } from "../../stores/auth";
import { useChatStore } from "../../stores/chat";
import { useNotificationStore } from "../../stores/notification";
import { useWorkspaceStore } from "../../stores/workspace";

type Tab = "open" | "mine" | "unassigned" | "closed" | "all";

const route = useRoute();
const router = useRouter();
const notify = useNotificationStore();
const auth = useAuthStore();
const chat = useChatStore();
const wsStore = useWorkspaceStore();

const tabs: { key: Tab; label: string }[] = [
  { key: "open", label: "Open" },
  { key: "mine", label: "Mine" },
  { key: "unassigned", label: "Unassigned" },
  { key: "closed", label: "Closed" },
  { key: "all", label: "All" },
];
const sourceLabels: Record<InboxSource, string> = { form: "Forms", inbound: "Email", chat: "Chat" };
const sourceIcons: Record<InboxSource, string> = { form: "F", inbound: "@", chat: "C" };

const tab = ref<Tab>((tabs.find((t) => t.key === route.query.tab)?.key as Tab) || "open");
const source = ref<InboxSource | "">((route.query.source as InboxSource) || "");
const q = ref(String(route.query.q || ""));
const items = ref<InboxItem[]>([]);
const loading = ref(false);
const unavailable = ref(false);
const enabledSources = ref<InboxSource[]>([]);
const myId = computed(() => auth.user?.id);

function params(page: number): InboxListParams {
  const p: InboxListParams = { page, size: 30, q: q.value.trim() || undefined, source: source.value || undefined };
  switch (tab.value) {
    case "open":
      p.state = "open";
      break;
    case "closed":
      p.state = "closed";
      break;
    case "mine":
      p.state = "open";
      p.assigned = "me";
      break;
    case "unassigned":
      p.state = "open";
      p.assigned = "unassigned";
      break;
  }
  return p;
}

let currentPage = 0;
const { pageable, goToPage } = usePagination(async (page) => {
  currentPage = page;
  loading.value = true;
  try {
    const res = await unifiedInboxApi.list(params(page));
    items.value = res.data.data;
    pageable.value = res.data.pageable;
    unavailable.value = false;
  } catch (e: any) {
    if (e?.response?.status === 404) unavailable.value = true;
    else notify.error(apiMessage(e, "Failed to load the inbox"));
  } finally {
    loading.value = false;
  }
});

function reload() {
  goToPage(0);
}

function setTab(t: Tab) {
  tab.value = t;
  router.replace({ query: { ...route.query, tab: t === "open" ? undefined : t, page: undefined } });
  reload();
}

function setSource(s: InboxSource | "") {
  source.value = source.value === s ? "" : s;
  router.replace({ query: { ...route.query, source: source.value || undefined, page: undefined } });
  reload();
}

let searchTimer: ReturnType<typeof setTimeout> | null = null;
watch(q, () => {
  if (searchTimer) clearTimeout(searchTimer);
  searchTimer = setTimeout(() => {
    router.replace({ query: { ...route.query, q: q.value.trim() || undefined, page: undefined } });
    reload();
  }, 300);
});

watch(() => wsStore.currentWorkspaceId, () => reload());

// Quiet refreshes: chat events now, everything else every minute.
let refreshTimer: ReturnType<typeof setTimeout> | null = null;
function refreshSoon() {
  if (refreshTimer) clearTimeout(refreshTimer);
  refreshTimer = setTimeout(() => goToPage(currentPage), 800);
}
let interval: ReturnType<typeof setInterval> | null = null;
let unsubscribe: (() => void) | null = null;

onMounted(async () => {
  unsubscribe = chat.subscribe((ev) => {
    if (ev.type === "chat.message" || ev.type === "chat.conversation" || ev.type === "reconnected") refreshSoon();
  });
  interval = setInterval(() => {
    if (document.visibilityState === "visible" && !loading.value) goToPage(currentPage);
  }, 60_000);
  try {
    const features = (await dashboardApi.getStats()).data.data.features;
    const out: InboxSource[] = [];
    if (features?.messages) out.push("form");
    if (features?.inbound) out.push("inbound");
    if (features?.chat) out.push("chat");
    enabledSources.value = out;
  } catch {
    enabledSources.value = [];
  }
});

onUnmounted(() => {
  unsubscribe?.();
  if (interval) clearInterval(interval);
  if (refreshTimer) clearTimeout(refreshTimer);
  if (searchTimer) clearTimeout(searchTimer);
});

function sender(item: InboxItem): string {
  if (item.from_name && item.from_email) return `${item.from_name} <${item.from_email}>`;
  return item.from_name || item.from_email || (item.source === "chat" ? "Visitor" : "Unknown sender");
}

function assignee(item: InboxItem): string {
  if (!item.assigned_to_id) return "";
  return item.assigned_to_id === myId.value ? "you" : item.assigned_to_name || "assigned";
}

function relative(date: string): string {
  const seconds = Math.round((Date.now() - new Date(date).getTime()) / 1000);
  if (seconds < 60) return "just now";
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.round(hours / 24);
  if (days < 7) return `${days}d ago`;
  return new Date(date).toLocaleDateString();
}
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Inbox</h1>
      <button class="btn btn-secondary" :disabled="loading" @click="goToPage(currentPage)">
        {{ loading ? "Refreshing…" : "Refresh" }}
      </button>
    </div>

    <div v-if="unavailable" class="card">
      <div class="card-body empty-state">
        <p>The inbox needs web form messages, inbound email or live chat to be enabled on this server.</p>
      </div>
    </div>

    <div v-else class="card">
      <div class="inbox-toolbar">
        <div class="tabs" role="tablist" aria-label="Inbox views">
          <button
            v-for="t in tabs"
            :key="t.key"
            role="tab"
            class="tab"
            :class="{ active: tab === t.key }"
            :aria-selected="tab === t.key"
            @click="setTab(t.key)"
          >
            {{ t.label }}
          </button>
        </div>
        <div class="filters">
          <div v-if="enabledSources.length > 1" class="chips" role="group" aria-label="Sources">
            <button
              v-for="s in enabledSources"
              :key="s"
              class="chip"
              :class="{ active: source === s }"
              :aria-pressed="source === s"
              @click="setSource(s)"
            >
              {{ sourceLabels[s] }}
            </button>
          </div>
          <input v-model="q" type="search" class="form-input search" placeholder="Search titles, senders and messages" />
        </div>
      </div>

      <div v-if="loading && !items.length" class="loading-page"><div class="spinner"></div></div>
      <div v-else-if="!items.length" class="empty-state">
        <p>Nothing here.</p>
      </div>
      <ul v-else class="inbox-list">
        <li v-for="item in items" :key="`${item.source}:${item.id}`">
          <router-link :to="inboxItemRoute(item)" class="inbox-row" :class="{ unread: item.unread }">
            <span class="source" :class="`source-${item.source}`" :title="sourceLabels[item.source]" aria-hidden="true">
              {{ sourceIcons[item.source] }}
            </span>
            <span class="main">
              <span class="top">
                <strong class="title">{{ item.title || "(no subject)" }}</strong>
                <span class="time" :title="new Date(item.last_activity_at).toLocaleString()">{{ relative(item.last_activity_at) }}</span>
              </span>
              <span class="sender">{{ sender(item) }}</span>
              <span v-if="item.preview" class="preview">{{ item.preview }}</span>
              <span class="meta">
                <span class="badge badge-secondary">{{ sourceLabels[item.source] }}</span>
                <span v-if="item.state !== 'open'" class="badge" :class="item.state === 'spam' ? 'badge-danger' : 'badge-secondary'">{{ item.state }}</span>
                <span v-if="assignee(item)" class="muted">· {{ assignee(item) }}</span>
                <span v-if="item.unread" class="unread-dot" title="Unread or awaiting a reply"></span>
              </span>
            </span>
          </router-link>
        </li>
      </ul>
      <Pagination :pageable="pageable" @page="goToPage" />
    </div>
  </div>
</template>

<style scoped>
.inbox-toolbar {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  border-bottom: 1px solid var(--border-color, #e5e7eb);
}
.tabs {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
}
.tab {
  background: none;
  border: 0;
  padding: 6px 12px;
  border-radius: 8px;
  cursor: pointer;
  color: var(--text-secondary);
  font: inherit;
}
.tab.active {
  background: var(--primary-light, rgba(124, 58, 237, 0.1));
  color: var(--primary, #7c3aed);
  font-weight: 600;
}
.filters {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}
.chips {
  display: flex;
  gap: 6px;
}
.chip {
  border: 1px solid var(--border-color, #e5e7eb);
  background: none;
  border-radius: 999px;
  padding: 4px 12px;
  font: inherit;
  font-size: 13px;
  cursor: pointer;
  color: var(--text-secondary);
}
.chip.active {
  border-color: var(--primary, #7c3aed);
  color: var(--primary, #7c3aed);
  font-weight: 600;
}
.search {
  min-width: 240px;
}
.inbox-list {
  list-style: none;
  margin: 0;
  padding: 0;
}
.inbox-row {
  display: flex;
  gap: 12px;
  padding: 12px 16px;
  border-bottom: 1px solid var(--border-color, #e5e7eb);
  color: inherit;
  text-decoration: none;
}
.inbox-row:hover {
  background: var(--bg-hover, rgba(0, 0, 0, 0.03));
}
.inbox-row.unread .title {
  font-weight: 700;
}
.source {
  flex: none;
  width: 32px;
  height: 32px;
  border-radius: 8px;
  display: grid;
  place-items: center;
  font-weight: 700;
  font-size: 13px;
}
.source-form {
  background: rgba(59, 130, 246, 0.12);
  color: #2563eb;
}
.source-inbound {
  background: rgba(16, 185, 129, 0.12);
  color: #059669;
}
.source-chat {
  background: rgba(124, 58, 237, 0.12);
  color: #7c3aed;
}
.main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.top {
  display: flex;
  justify-content: space-between;
  gap: 12px;
}
.title {
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.time {
  flex: none;
  font-size: 12px;
  color: var(--text-muted);
}
.sender {
  font-size: 13px;
  color: var(--text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.preview {
  font-size: 13px;
  color: var(--text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.meta {
  display: flex;
  gap: 6px;
  align-items: center;
  margin-top: 2px;
  font-size: 12px;
}
.unread-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--primary, #7c3aed);
}
.empty-state {
  padding: 32px 16px;
  text-align: center;
  color: var(--text-secondary);
}
@media (max-width: 640px) {
  .search {
    min-width: 0;
    width: 100%;
  }
  .filters {
    width: 100%;
  }
}
</style>
