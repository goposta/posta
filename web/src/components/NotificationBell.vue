<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from "vue";
import { storeToRefs } from "pinia";
import { useRouter } from "vue-router";
import { useInboxStore } from "../stores/inbox";
import type { InboxNotification } from "../api/inbox";

const store = useInboxStore();
const { unread, badge, items, loading } = storeToRefs(store);
const router = useRouter();

const open = ref(false);

function toggle() {
  open.value = !open.value;
  if (open.value) void store.loadRecent();
}

async function activate(n: InboxNotification) {
  if (!n.read_at) await store.markRead([n.id]);
  open.value = false;
  if (n.link) router.push(n.link);
}

function onDocClick(e: MouseEvent) {
  if (open.value && !(e.target as HTMLElement).closest?.(".bell")) open.value = false;
}

function timeAgo(iso: string): string {
  const s = Math.max(0, (Date.now() - new Date(iso).getTime()) / 1000);
  if (s < 60) return "just now";
  if (s < 3600) return `${Math.floor(s / 60)}m ago`;
  if (s < 86400) return `${Math.floor(s / 3600)}h ago`;
  if (s < 2592000) return `${Math.floor(s / 86400)}d ago`;
  return new Date(iso).toLocaleDateString();
}

onMounted(() => {
  store.start();
  document.addEventListener("click", onDocClick);
});

onBeforeUnmount(() => {
  document.removeEventListener("click", onDocClick);
  // Clear the inbox, not just the poll: another account signing in without a reload must not see the previous user's notifications.
  store.reset();
});
</script>

<template>
  <div class="bell">
    <button
      class="bell-btn"
      type="button"
      :aria-label="unread > 0 ? `Notifications, ${unread} unread` : 'Notifications'"
      :aria-expanded="open"
      @click.stop="toggle"
    >
      <svg
        width="18"
        height="18"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="1.8"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
      >
        <path d="M18 8A6 6 0 006 8c0 7-3 9-3 9h18s-3-2-3-9" />
        <path d="M13.73 21a2 2 0 01-3.46 0" />
      </svg>
      <span v-if="unread > 0" class="bell-badge">{{ badge }}</span>
    </button>

    <div v-if="open" class="bell-panel" @click.stop>
      <div class="bell-head">
        <span>Notifications</span>
        <button v-if="unread > 0" class="bell-link" type="button" @click="store.markAllRead()">
          Mark all read
        </button>
      </div>

      <div class="bell-list">
        <div v-if="loading && !items.length" class="bell-empty">
          <div class="spinner"></div>
        </div>
        <div v-else-if="!items.length" class="bell-empty">
          <p>You're all caught up.</p>
        </div>
        <button
          v-for="n in items"
          :key="n.id"
          type="button"
          class="bell-item"
          :class="{ unread: !n.read_at }"
          @click="activate(n)"
        >
          <span class="bell-sev" :class="`sev-${n.severity}`" aria-hidden="true"></span>
          <span class="bell-body">
            <span class="bell-title">{{ n.title }}</span>
            <span class="bell-sub">{{ n.body }}</span>
            <span class="bell-meta">
              {{ timeAgo(n.created_at) }}
              <template v-if="n.resolved_at"> · resolved</template>
            </span>
          </span>
        </button>
      </div>

      <div class="bell-foot">
        <RouterLink to="/notifications" class="bell-link" @click="open = false">
          View all notifications
        </RouterLink>
      </div>
    </div>
  </div>
</template>

<style scoped>
.bell {
  position: relative;
}

.bell-btn {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  border-radius: 8px;
  border: 1px solid transparent;
  background: none;
  color: var(--text-secondary);
  cursor: pointer;
}

.bell-btn:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.bell-badge {
  position: absolute;
  top: 2px;
  right: 1px;
  min-width: 16px;
  height: 16px;
  padding: 0 4px;
  border-radius: 8px;
  background: var(--danger-600);
  color: #fff;
  font-size: 10px;
  font-weight: 700;
  line-height: 16px;
  text-align: center;
}

.bell-panel {
  position: absolute;
  top: calc(100% + 8px);
  right: 0;
  width: 360px;
  max-width: calc(100vw - 24px);
  background: var(--bg-primary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-lg);
  z-index: 60;
  overflow: hidden;
}

.bell-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 14px;
  border-bottom: 1px solid var(--border-secondary);
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
}

.bell-link {
  border: 0;
  background: none;
  padding: 0;
  font: inherit;
  font-size: 12px;
  font-weight: 500;
  color: var(--primary-600);
  cursor: pointer;
  text-decoration: none;
}

.bell-link:hover {
  text-decoration: underline;
}

.bell-list {
  max-height: 380px;
  overflow-y: auto;
}

.bell-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 32px 14px;
  font-size: 13px;
  color: var(--text-tertiary);
}

.bell-item {
  display: flex;
  gap: 10px;
  width: 100%;
  padding: 12px 14px;
  border: 0;
  border-bottom: 1px solid var(--border-secondary);
  background: none;
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.bell-item:last-child {
  border-bottom: 0;
}

.bell-item:hover {
  background: var(--bg-hover);
}

.bell-item.unread {
  background: var(--primary-50);
}

.bell-item.unread:hover {
  background: var(--bg-hover);
}

.bell-sev {
  flex: none;
  width: 8px;
  height: 8px;
  margin-top: 5px;
  border-radius: 50%;
  background: var(--text-tertiary);
}

.sev-warning {
  background: var(--warning-500);
}

.sev-critical {
  background: var(--danger-600);
}

.sev-info {
  background: var(--primary-500);
}

.bell-body {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.bell-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
}

.bell-sub {
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.5;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

.bell-meta {
  font-size: 11px;
  color: var(--text-tertiary);
  margin-top: 2px;
}

.bell-foot {
  padding: 10px 14px;
  border-top: 1px solid var(--border-secondary);
  text-align: center;
}
</style>
