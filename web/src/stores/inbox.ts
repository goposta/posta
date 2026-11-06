import { defineStore } from "pinia";
import { computed, ref } from "vue";
import { inboxApi, type InboxNotification } from "../api/inbox";

// Polled, not pushed: no inbox item is urgent enough for a socket, and a poll degrades gracefully in background tabs.
const POLL_MS = 60_000;

export const useInboxStore = defineStore("inbox", () => {
  const unread = ref(0);
  const items = ref<InboxNotification[]>([]);
  const banner = ref<InboxNotification[]>([]);
  const loading = ref(false);
  let timer: ReturnType<typeof setInterval> | null = null;

  const badge = computed(() => (unread.value > 99 ? "99+" : String(unread.value)));

  async function loadCounts() {
    try {
      unread.value = (await inboxApi.counts()).data.data?.unread ?? 0;
    } catch {
      /* transient; the badge is not worth an error toast */
    }
  }

  async function loadRecent(limit = 20) {
    loading.value = true;
    try {
      items.value = (await inboxApi.list({ limit })).data.data ?? [];
    } catch {
      items.value = [];
    } finally {
      loading.value = false;
    }
  }

  async function loadBanner() {
    try {
      banner.value = (await inboxApi.banner()).data.data ?? [];
    } catch {
      banner.value = [];
    }
  }

  function start() {
    if (timer) return;
    void loadCounts();
    timer = setInterval(() => void loadCounts(), POLL_MS);
  }

  function stop() {
    if (timer) clearInterval(timer);
    timer = null;
  }

  function stamp(list: InboxNotification[], ids: number[], field: "read_at" | "dismissed_at") {
    const now = new Date().toISOString();
    for (const n of list) {
      if (ids.includes(n.id) && !n[field]) n[field] = now;
    }
  }

  async function markRead(ids: number[]) {
    if (!ids.length) return;
    stamp(items.value, ids, "read_at");
    await inboxApi.markRead(ids);
    await loadCounts();
  }

  async function markAllRead() {
    stamp(
      items.value,
      items.value.map((n) => n.id),
      "read_at"
    );
    unread.value = 0;
    await inboxApi.markAllRead();
  }

  // The server binds a dismissal to the condition as it stands, so the item returns if the condition materially worsens.
  async function dismiss(id: number) {
    banner.value = banner.value.filter((n) => n.id !== id);
    stamp(items.value, [id], "dismissed_at");
    stamp(items.value, [id], "read_at");
    await inboxApi.dismiss([id]);
    await loadCounts();
  }

  async function dismissAll() {
    banner.value = [];
    await inboxApi.dismissAll();
    await loadCounts();
  }

  function reset() {
    stop();
    unread.value = 0;
    items.value = [];
    banner.value = [];
  }

  return {
    unread,
    badge,
    items,
    banner,
    loading,
    start,
    stop,
    reset,
    loadCounts,
    loadRecent,
    loadBanner,
    markRead,
    markAllRead,
    dismiss,
    dismissAll,
  };
});
