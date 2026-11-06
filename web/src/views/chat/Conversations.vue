<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { chatSnippetsApi, chatWidgetsApi, conversationsApi, type ConversationListParams } from "../../api/chat";
import type { ChatConversation, ChatMessage, ChatSnippet, ChatState, ChatVisitorContext } from "../../api/types";
import ChatAttachmentView from "./ChatAttachmentView.vue";
import { useAuthStore } from "../../stores/auth";
import { useChatStore, type ChatStreamEvent } from "../../stores/chat";
import { useNotificationStore } from "../../stores/notification";
import { useWorkspaceStore } from "../../stores/workspace";
import { apiMessage } from "../../composables/apiError";
import { useConfirm } from "../../composables/useConfirm";

const route = useRoute();
const router = useRouter();
const auth = useAuthStore();
const chat = useChatStore();
const notify = useNotificationStore();
const wsStore = useWorkspaceStore();
const { confirm } = useConfirm();

// A message the agent sent that the server has not confirmed yet. It has no
// sequence number until then, and is matched to the stored copy by its
// client_msg_id, whichever of the POST answer and the stream arrives first.
type ThreadMessage = ChatMessage & { pending?: boolean; failed?: boolean };

const TABS = [
  { key: "open", label: "Open" },
  { key: "mine", label: "Mine" },
  { key: "unassigned", label: "Unassigned" },
  { key: "closed", label: "Closed" },
] as const;
type Tab = (typeof TABS)[number]["key"];

const tab = ref<Tab>(TABS.find((t) => t.key === route.query.tab)?.key ?? "open");
const search = ref(typeof route.query.q === "string" ? route.query.q : "");
const widgetId = computed(() => Number(route.query.widget_id) || undefined);

const featureDisabled = ref(false);
const listLoading = ref(true);
const conversations = ref<ChatConversation[]>([]);
const page = ref(0);
const hasMore = ref(false);

const selectedId = computed(() => (typeof route.params.id === "string" ? route.params.id : ""));
const conversation = ref<ChatConversation | null>(null);
const threadLoading = ref(false);
const threadMissing = ref(false);
const messages = ref<ThreadMessage[]>([]);
const loadingOlder = ref(false);
const composer = ref("");
const threadEl = ref<HTMLElement | null>(null);

const canEdit = computed(() => wsStore.canEdit);
const canErase = computed(() => wsStore.isWorkspaceAdmin);
const myId = computed(() => auth.user?.id ?? 0);

// Live details of the open thread: the visitor's typing indicator, and the
// newest agent message the visitor has seen.
const visitorTyping = ref(false);
let typingTimer: ReturnType<typeof setTimeout> | null = null;
const seenSeq = computed(() => {
  const read = conversation.value?.visitor_read_seq ?? 0;
  let seen = 0;
  for (const m of messages.value) {
    if (m.author_kind === "agent" && m.seq > 0 && m.seq <= read) seen = m.seq;
  }
  return seen;
});

/** What happens to a reply now, when the visitor's widget is not connected. */
const visitorLeftNote = computed(() => {
  const c = conversation.value;
  if (!c || c.visitor_live) return "";
  if (c.reply_by_email && c.visitor?.email) return `Visitor left — your reply will be emailed to ${c.visitor.email}.`;
  if (!c.visitor?.email) return "Visitor left and gave no email; they'll see this if they return.";
  return "Visitor left.";
});
const hasOlder = computed(() => {
  const first = messages.value.find((m) => m.seq > 0);
  return !!first && first.seq > 1;
});

function listParams(p: number): ConversationListParams {
  const params: ConversationListParams = { page: p, size: 30, widget_id: widgetId.value };
  if (search.value.trim()) params.q = search.value.trim();
  switch (tab.value) {
    case "open":
      params.state = "open";
      break;
    case "mine":
      params.state = "open";
      params.assigned = "me";
      break;
    case "unassigned":
      params.state = "open";
      params.assigned = "unassigned";
      break;
    case "closed":
      params.state = "closed";
      break;
  }
  return params;
}

async function loadList(append = false) {
  if (!append) listLoading.value = true;
  const p = append ? page.value + 1 : 0;
  try {
    const res = await conversationsApi.list(listParams(p));
    conversations.value = append ? [...conversations.value, ...res.data.data] : res.data.data;
    page.value = p;
    const pg = res.data.pageable;
    hasMore.value = pg ? pg.current_page + 1 < pg.total_pages : false;
    featureDisabled.value = false;
  } catch (e: any) {
    if (e?.response?.status === 404) featureDisabled.value = true;
    else notify.error(apiMessage(e, "Failed to load conversations"));
  } finally {
    listLoading.value = false;
  }
}

// Several events in a row (a burst of new chats) reload the list once.
let listTimer: ReturnType<typeof setTimeout> | null = null;
function reloadListSoon() {
  if (listTimer) clearTimeout(listTimer);
  listTimer = setTimeout(() => {
    listTimer = null;
    loadList();
  }, 400);
}

function selectTab(t: Tab) {
  tab.value = t;
  router.replace({ query: { ...route.query, tab: t } });
  loadList();
}

let searchTimer: ReturnType<typeof setTimeout> | null = null;
watch(search, () => {
  if (searchTimer) clearTimeout(searchTimer);
  searchTimer = setTimeout(() => loadList(), 300);
});
watch(widgetId, () => loadList());

function open(c: ChatConversation) {
  router.push({ path: `/chat/${c.uuid}`, query: route.query });
}

// --- The thread -----------------------------------------------------------

function sortMessages() {
  messages.value.sort((a, b) => {
    if (a.seq === 0 && b.seq === 0) return 0;
    if (a.seq === 0) return 1;
    if (b.seq === 0) return -1;
    return a.seq - b.seq;
  });
}

/** Adds or confirms a message, keeping one copy per client_msg_id and per seq. */
function merge(m: ChatMessage) {
  const list = messages.value;
  if (m.client_msg_id) {
    const i = list.findIndex((x) => x.client_msg_id === m.client_msg_id);
    if (i >= 0) {
      list[i] = { ...m };
      sortMessages();
      return;
    }
  }
  if (list.some((x) => x.seq > 0 && x.seq === m.seq)) return;
  list.push({ ...m });
  sortMessages();
}

/** The highest seq up to which the thread has every message. */
function contiguousLast(): number {
  const confirmed = messages.value.filter((m) => m.seq > 0);
  if (confirmed.length === 0) return 0;
  let last = confirmed[0]!.seq;
  for (const m of confirmed.slice(1)) {
    if (m.seq !== last + 1) break;
    last = m.seq;
  }
  return last;
}

function hasGap(): boolean {
  const confirmed = messages.value.filter((m) => m.seq > 0);
  const top = confirmed.length ? confirmed[confirmed.length - 1]!.seq : 0;
  return top > contiguousLast();
}

let catchingUp = false;
async function catchUp() {
  const uuid = selectedId.value;
  if (!uuid || catchingUp) return;
  catchingUp = true;
  try {
    for (;;) {
      const after = contiguousLast();
      const res = await conversationsApi.messages(uuid, { after, limit: 200 });
      if (uuid !== selectedId.value) return;
      res.data.data.forEach(merge);
      if (res.data.data.length < 200) break;
    }
  } catch {
    // The next event or reconnect tries again.
  } finally {
    catchingUp = false;
  }
  scrollToBottom();
}

function nearBottom(): boolean {
  const el = threadEl.value;
  return !el || el.scrollHeight - el.scrollTop - el.clientHeight < 120;
}

async function scrollToBottom(force = false) {
  if (!force && !nearBottom()) return;
  await nextTick();
  const el = threadEl.value;
  if (el) el.scrollTop = el.scrollHeight;
}

async function loadConversation(uuid: string) {
  conversation.value = null;
  messages.value = [];
  threadMissing.value = false;
  composer.value = "";
  if (!uuid) return;
  threadLoading.value = true;
  try {
    const [c, m] = await Promise.all([conversationsApi.get(uuid), conversationsApi.messages(uuid)]);
    if (uuid !== selectedId.value) return;
    conversation.value = c.data.data;
    messages.value = m.data.data.map((x) => ({ ...x }));
    sortMessages();
  } catch {
    threadMissing.value = true;
  } finally {
    threadLoading.value = false;
  }
  visitorTyping.value = false;
  scrollToBottom(true);
  markRead();
  loadVisitorContext(uuid);
  loadWidgetAttachments();
}

// --- Visitor context, ban -------------------------------------------------

const visitorCtx = ref<ChatVisitorContext | null>(null);
const ctxVisitor = computed(() => visitorCtx.value?.visitor ?? conversation.value?.visitor ?? null);
const attributeEntries = computed(() => Object.entries(ctxVisitor.value?.attributes ?? {}));

async function loadVisitorContext(uuid: string) {
  visitorCtx.value = null;
  if (!uuid) return;
  try {
    const res = await conversationsApi.visitor(uuid);
    if (uuid === selectedId.value) visitorCtx.value = res.data.data;
  } catch {
    // The sidebar falls back to the conversation's own visitor.
  }
}

async function ban() {
  const c = conversation.value;
  if (!c) return;
  const sure = await confirm({
    title: "Ban visitor",
    message:
      "The conversation is marked as spam, the visitor's further messages are silently dropped, and their IP cannot start a chat in this workspace for 24 hours.",
    confirmText: "Ban visitor",
    variant: "danger",
  });
  if (!sure) return;
  try {
    await conversationsApi.ban(c.uuid);
    notify.success("Visitor banned");
    await Promise.all([refreshConversation(), loadVisitorContext(c.uuid)]);
    reloadListSoon();
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to ban visitor"));
  }
}

async function eraseConversation() {
  const c = conversation.value;
  if (!c) return;
  const sure = await confirm({
    title: "Delete conversation",
    message: "Erases the conversation, its messages and files, and the visitor if it was their last. This cannot be undone.",
    confirmText: "Delete",
    variant: "danger",
  });
  if (!sure) return;
  try {
    await conversationsApi.remove(c.uuid);
    notify.success("Conversation deleted");
    conversations.value = conversations.value.filter((x) => x.uuid !== c.uuid);
    router.push({ name: "chat", query: route.query });
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to delete the conversation"));
  }
}

async function unban() {
  const c = conversation.value;
  if (!c) return;
  try {
    await conversationsApi.unban(c.uuid);
    notify.success("Ban lifted");
    await loadVisitorContext(c.uuid);
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to lift the ban"));
  }
}

// --- Attachments ----------------------------------------------------------

// Whether the conversation's widget takes files: true, false, or null when
// unknown (then the button shows and the server's answer decides).
const widgetAttachments = ref<boolean | null>(null);
const attachmentCache = new Map<number, boolean>();
const fileInput = ref<HTMLInputElement | null>(null);

async function loadWidgetAttachments() {
  const id = conversation.value?.widget_id;
  widgetAttachments.value = null;
  if (!id) return;
  if (attachmentCache.has(id)) {
    widgetAttachments.value = attachmentCache.get(id)!;
    return;
  }
  try {
    const res = await chatWidgetsApi.get(id);
    attachmentCache.set(id, !!res.data.data.allow_attachments);
    if (conversation.value?.widget_id === id) widgetAttachments.value = !!res.data.data.allow_attachments;
  } catch {
    widgetAttachments.value = null;
  }
}

function pickFile() {
  fileInput.value?.click();
}

async function onFile(e: Event) {
  const input = e.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = "";
  const c = conversation.value;
  if (!file || !c) return;
  if (file.size > 5 * 1024 * 1024) {
    notify.error("Files can be at most 5 MB.");
    return;
  }
  const caption = composer.value.trim();
  const m: ThreadMessage = {
    id: `local-${newClientId()}`,
    seq: 0,
    author_kind: "agent",
    author_id: myId.value,
    author: { id: myId.value, name: auth.user?.name ?? "" },
    body: caption,
    client_msg_id: newClientId(),
    attachments: [{ filename: file.name, content_type: file.type, size: file.size }],
    created_at: new Date().toISOString(),
    pending: true,
  };
  messages.value.push(m);
  composer.value = "";
  scrollToBottom(true);
  try {
    const res = await conversationsApi.upload(c.uuid, file, { caption, client_msg_id: m.client_msg_id });
    if (c.uuid !== selectedId.value) return;
    merge(res.data.data);
    if (hasGap()) catchUp();
  } catch (err: any) {
    messages.value = messages.value.filter((x) => x.client_msg_id !== m.client_msg_id);
    if (caption && !composer.value) composer.value = caption;
    notify.error(apiMessage(err, "File not sent"));
  }
}

// --- Canned replies -------------------------------------------------------

const snippets = ref<ChatSnippet[]>([]);
const composerEl = ref<HTMLTextAreaElement | null>(null);
const snippetQuery = ref<string | null>(null);
const snippetIndex = ref(0);
const snippetMatches = computed(() => {
  const q = snippetQuery.value;
  if (q === null) return [];
  const needle = q.toLowerCase();
  return snippets.value
    .filter((s) => s.shortcut.includes(needle) || s.title.toLowerCase().includes(needle))
    .slice(0, 8);
});

async function loadSnippets() {
  try {
    snippets.value = (await chatSnippetsApi.list()).data.data;
  } catch {
    snippets.value = [];
  }
}

const SLASH = /(?:^|\s)\/([a-z0-9-]*)$/;

/** Opens the canned-reply popup while the word at the caret starts with "/". */
function updateSnippetQuery() {
  const el = composerEl.value;
  if (!el || !snippets.value.length) {
    snippetQuery.value = null;
    return;
  }
  const before = composer.value.slice(0, el.selectionStart ?? composer.value.length);
  const match = before.match(SLASH);
  snippetQuery.value = match ? match[1]! : null;
  snippetIndex.value = 0;
}

async function insertSnippet(s: ChatSnippet) {
  const el = composerEl.value;
  const caret = el?.selectionStart ?? composer.value.length;
  const before = composer.value.slice(0, caret);
  const after = composer.value.slice(caret);
  const start = before.lastIndexOf("/");
  composer.value = before.slice(0, start) + s.body + after;
  snippetQuery.value = null;
  await nextTick();
  if (el) {
    const pos = start + s.body.length;
    el.focus();
    el.setSelectionRange(pos, pos);
  }
}

/** Refetches the open conversation's details, keeping its messages. */
async function refreshConversation() {
  const uuid = selectedId.value;
  if (!uuid) return;
  try {
    const res = await conversationsApi.get(uuid);
    if (res.data.data.uuid === selectedId.value) conversation.value = res.data.data;
  } catch {
    // The next event or tick tries again.
  }
}

// --- Read receipts and typing ---------------------------------------------

let readSent = { uuid: "", seq: 0 };
/** Tells the visitor how far the agents have read, while the thread is in view. */
function markRead() {
  const c = conversation.value;
  if (!c || !canEdit.value || document.visibilityState !== "visible") return;
  const seq = contiguousLast();
  if (seq <= 0 || (readSent.uuid === c.uuid && readSent.seq >= seq)) return;
  readSent = { uuid: c.uuid, seq };
  conversationsApi.read(c.uuid, seq).catch(() => {
    readSent = { uuid: "", seq: 0 };
  });
}

function onVisibility() {
  if (document.visibilityState === "visible") markRead();
}

let typingSentAt = 0;
/** Shows the visitor that an agent is writing, at most every 3 seconds. */
function onComposerInput() {
  updateSnippetQuery();
  const c = conversation.value;
  if (!c || !composer.value.trim()) return;
  const now = Date.now();
  if (now - typingSentAt < 3000) return;
  typingSentAt = now;
  conversationsApi.typing(c.uuid).catch(() => {});
}

function stopVisitorTyping() {
  visitorTyping.value = false;
  if (typingTimer) clearTimeout(typingTimer);
  typingTimer = null;
}

// While the visitor is away, whether a reply goes by email changes with time
// (two minutes after they leave), so the banner is refreshed now and then.
let awayTimer: ReturnType<typeof setInterval> | null = null;
watch(
  () => conversation.value?.uuid && !conversation.value.visitor_live,
  (away) => {
    if (awayTimer) clearInterval(awayTimer);
    awayTimer = away ? setInterval(refreshConversation, 60000) : null;
  },
);

async function loadOlder() {
  const uuid = selectedId.value;
  const first = messages.value.find((m) => m.seq > 0);
  if (!uuid || !first) return;
  loadingOlder.value = true;
  const el = threadEl.value;
  const fromBottom = el ? el.scrollHeight - el.scrollTop : 0;
  try {
    const res = await conversationsApi.messages(uuid, { before: first.seq, limit: 50 });
    res.data.data.forEach(merge);
    await nextTick();
    if (el) el.scrollTop = el.scrollHeight - fromBottom;
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to load older messages"));
  } finally {
    loadingOlder.value = false;
  }
}

watch(selectedId, (uuid) => loadConversation(uuid));

// --- Sending --------------------------------------------------------------

function newClientId(): string {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) return crypto.randomUUID();
  return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`;
}

async function deliver(m: ThreadMessage) {
  const uuid = selectedId.value;
  m.failed = false;
  try {
    const res = await conversationsApi.send(uuid, { body: m.body, client_msg_id: m.client_msg_id });
    if (uuid !== selectedId.value) return;
    merge(res.data.data);
    if (hasGap()) catchUp();
    const c = conversation.value;
    if (c && !c.assigned_to_id) {
      // Replying claims an unassigned conversation.
      c.assigned_to_id = myId.value;
      c.assigned_to = { id: myId.value, name: auth.user?.name ?? "" };
    }
  } catch (e: any) {
    const local = messages.value.find((x) => x.client_msg_id === m.client_msg_id && x.pending);
    if (local) local.failed = true;
    notify.error(apiMessage(e, "Message not sent"));
  }
}

function send() {
  const body = composer.value.trim();
  if (!body || !conversation.value) return;
  const m: ThreadMessage = {
    id: `local-${newClientId()}`,
    seq: 0,
    author_kind: "agent",
    author_id: myId.value,
    author: { id: myId.value, name: auth.user?.name ?? "" },
    body,
    client_msg_id: newClientId(),
    created_at: new Date().toISOString(),
    pending: true,
  };
  messages.value.push(m);
  composer.value = "";
  scrollToBottom(true);
  deliver(m);
}

function onComposerKey(e: KeyboardEvent) {
  if (snippetQuery.value !== null && snippetMatches.value.length) {
    const n = snippetMatches.value.length;
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      snippetIndex.value = (snippetIndex.value + (e.key === "ArrowDown" ? 1 : n - 1)) % n;
      return;
    }
    if (e.key === "Enter" || e.key === "Tab") {
      e.preventDefault();
      insertSnippet(snippetMatches.value[snippetIndex.value]!);
      return;
    }
    if (e.key === "Escape") {
      e.preventDefault();
      snippetQuery.value = null;
      return;
    }
  }
  if (e.key === "Enter" && !e.shiftKey && !e.isComposing) {
    e.preventDefault();
    send();
  }
}

// --- Actions --------------------------------------------------------------

function applyConversation(updated: ChatConversation) {
  conversation.value = updated;
  const i = conversations.value.findIndex((c) => c.uuid === updated.uuid);
  if (i >= 0) conversations.value[i] = { ...conversations.value[i]!, ...updated };
}

async function assign(userId: number | null) {
  const c = conversation.value;
  if (!c) return;
  try {
    applyConversation((await conversationsApi.assign(c.uuid, userId)).data.data);
    reloadListSoon();
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to assign conversation"));
  }
}

async function setState(state: ChatState) {
  const c = conversation.value;
  if (!c) return;
  if (state === "spam") {
    const sure = await confirm({
      title: "Mark as spam",
      message: "The conversation leaves the inbox. The visitor is not told.",
      confirmText: "Mark as spam",
      variant: "warning",
    });
    if (!sure) return;
  }
  try {
    applyConversation((await conversationsApi.setState(c.uuid, state)).data.data);
    reloadListSoon();
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to update conversation"));
  }
}

// --- Live events ----------------------------------------------------------

function onEvent(ev: ChatStreamEvent) {
  if (ev.type === "reconnected") {
    loadList();
    if (selectedId.value) catchUp();
    return;
  }
  if (ev.type === "chat.message") {
    const { conversation_id, message } = ev.data;
    const row = conversations.value.find((c) => c.uuid === conversation_id);
    if (row) {
      row.last_seq = Math.max(row.last_seq, message.seq);
      row.last_message_at = message.created_at;
      row.last_author_kind = message.author_kind;
      row.last_message_preview = message.body.replace(/\s+/g, " ").slice(0, 140);
      conversations.value = [row, ...conversations.value.filter((c) => c !== row)];
    } else if (tab.value !== "closed") {
      reloadListSoon();
    }
    if (conversation_id === selectedId.value) {
      if (message.author_kind === "visitor") stopVisitorTyping();
      const before = contiguousLast();
      const known = message.client_msg_id && messages.value.some((m) => m.client_msg_id === message.client_msg_id);
      if (message.seq === before + 1 || known) merge(message);
      if (message.seq > before + 1 && !known) catchUp();
      else scrollToBottom();
      if (conversation.value) conversation.value.last_seq = Math.max(conversation.value.last_seq, message.seq);
      if (message.author_kind === "visitor") nextTick(markRead);
    }
    return;
  }
  if (ev.type === "chat.typing") {
    if (ev.data.conversation_id !== selectedId.value || ev.data.author !== "visitor") return;
    visitorTyping.value = true;
    if (typingTimer) clearTimeout(typingTimer);
    typingTimer = setTimeout(stopVisitorTyping, 4000);
    return;
  }
  if (ev.type === "chat.read") {
    const c = conversation.value;
    if (c && ev.data.conversation_id === c.uuid && ev.data.reader === "visitor") {
      c.visitor_read_seq = Math.max(c.visitor_read_seq ?? 0, ev.data.seq);
    }
    return;
  }
  if (ev.type === "chat.visitor") {
    const c = conversation.value;
    if (c && ev.data.conversation_id === c.uuid) {
      c.visitor_live = ev.data.live;
      if (!ev.data.live) stopVisitorTyping();
      refreshConversation();
    }
    return;
  }
  const d = ev.data;
  if (d.started) {
    reloadListSoon();
    return;
  }
  const row = conversations.value.find((c) => c.uuid === d.conversation_id);
  if (row) {
    const assignedChanged = row.assigned_to_id !== d.assigned_to_id;
    row.state = d.state;
    row.assigned_to_id = d.assigned_to_id;
    if (assignedChanged) reloadListSoon();
    else if ((tab.value === "closed") !== (d.state === "closed" || d.state === "spam")) reloadListSoon();
  } else {
    reloadListSoon();
  }
  if (d.conversation_id === selectedId.value && conversation.value) {
    const c = conversation.value;
    const assignedChanged = c.assigned_to_id !== d.assigned_to_id;
    c.state = d.state;
    c.assigned_to_id = d.assigned_to_id;
    // The event has no assignee name; refetch for it.
    if (assignedChanged) {
      conversationsApi.get(c.uuid).then((r) => {
        if (r.data.data.uuid === selectedId.value) conversation.value = r.data.data;
      }).catch(() => {});
    }
  }
}

let unsubscribe: (() => void) | null = null;
onMounted(() => {
  loadList();
  loadConversation(selectedId.value);
  loadSnippets();
  unsubscribe = chat.subscribe(onEvent);
  document.addEventListener("visibilitychange", onVisibility);
});
onBeforeUnmount(() => {
  unsubscribe?.();
  document.removeEventListener("visibilitychange", onVisibility);
  if (awayTimer) clearInterval(awayTimer);
  if (typingTimer) clearTimeout(typingTimer);
  if (listTimer) clearTimeout(listTimer);
  if (searchTimer) clearTimeout(searchTimer);
});

// --- Display --------------------------------------------------------------

type Segment = { text: string; href?: string };
const LINK = /\b(?:https?:\/\/|mailto:)[^\s<>"']+/gi;

/** Splits a plain-text body into text and http(s)/mailto links. Rendered as text nodes and anchors, never HTML. */
function segments(body: string): Segment[] {
  const out: Segment[] = [];
  let last = 0;
  for (const match of body.matchAll(LINK)) {
    let url = match[0];
    // Sentence punctuation after a link is not part of it.
    const trail = url.match(/[.,;:!?)\]]+$/);
    if (trail) url = url.slice(0, -trail[0].length);
    const start = match.index ?? 0;
    if (start > last) out.push({ text: body.slice(last, start) });
    out.push({ text: url, href: url });
    last = start + url.length;
  }
  if (last < body.length) out.push({ text: body.slice(last) });
  return out;
}

function isHttpURL(u: string | undefined): boolean {
  return !!u && /^https?:\/\//i.test(u);
}

function visitorLabel(c: ChatConversation | null): string {
  return c?.visitor?.name || c?.visitor?.email || "Visitor";
}

function authorLabel(m: ThreadMessage): string {
  if (m.author_kind === "visitor") return visitorLabel(conversation.value);
  if (m.author_kind === "system") return "System";
  if (m.author_id && m.author_id === myId.value) return "You";
  return m.author?.name || "Agent";
}

function formatTime(date: string | undefined): string {
  if (!date) return "";
  const d = new Date(date);
  const today = new Date().toDateString() === d.toDateString();
  return today ? d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }) : d.toLocaleString();
}

function stateBadge(state: string): string {
  switch (state) {
    case "open":
      return "badge badge-success";
    case "pending":
      return "badge badge-warning";
    case "spam":
      return "badge badge-danger";
    default:
      return "badge badge-secondary";
  }
}
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Chat</h1>
      <div style="display: flex; gap: 8px; align-items: center">
        <span v-if="chat.enabled" class="live-dot" :class="{ on: chat.connected }"
          :title="chat.connected ? 'Live' : 'Reconnecting…'"></span>
        <router-link to="/chat-widgets" class="btn btn-secondary">Chat widgets</router-link>
      </div>
    </div>

    <div v-if="featureDisabled" class="card">
      <div class="empty-state">
        <h3>Live chat is disabled</h3>
        <p>
          Ask your administrator to enable it by setting
          <code>POSTA_CHAT_ENABLED=true</code>.
        </p>
      </div>
    </div>

    <div v-else class="chat-layout" :class="{ 'has-selection': !!selectedId }">
      <!-- Conversation list -->
      <section class="card chat-list">
        <div class="chat-tabs">
          <button v-for="t in TABS" :key="t.key" class="chat-tab" :class="{ active: tab === t.key }"
            @click="selectTab(t.key)">{{ t.label }}</button>
        </div>
        <div class="chat-search">
          <input v-model="search" class="form-input" placeholder="Search name or email" />
          <div v-if="widgetId" class="muted widget-filter">
            One widget only ·
            <a href="#" @click.prevent="router.replace({ query: { ...route.query, widget_id: undefined } })">show all</a>
          </div>
        </div>
        <div v-if="listLoading" class="loading-page"><div class="spinner"></div></div>
        <div v-else-if="conversations.length === 0" class="empty-state">
          <p>No conversations here.</p>
        </div>
        <ul v-else class="conv-items">
          <li v-for="c in conversations" :key="c.uuid">
            <button class="conv-item" :class="{ active: c.uuid === selectedId }" @click="open(c)">
              <span class="conv-top">
                <strong class="conv-name">{{ visitorLabel(c) }}</strong>
                <span class="conv-time">{{ formatTime(c.last_message_at) }}</span>
              </span>
              <span v-if="c.last_message_preview" class="conv-preview">
                <template v-if="c.last_author_kind === 'agent'">You: </template>{{ c.last_message_preview }}
              </span>
              <span class="conv-bottom">
                <span class="muted">{{ c.widget?.display_name }}</span>
                <span v-if="c.status === 'flagged'" class="badge badge-warning">flagged</span>
                <span v-if="c.assigned_to" class="muted">· {{ c.assigned_to.id === myId ? "you" : c.assigned_to.name }}</span>
                <span v-if="c.state === 'open' && c.last_author_kind === 'visitor'" class="awaiting-dot"
                  title="Awaiting a reply"></span>
              </span>
            </button>
          </li>
        </ul>
        <button v-if="hasMore" class="btn btn-secondary load-more" @click="loadList(true)">Load more</button>
      </section>

      <!-- Thread -->
      <section class="card chat-thread">
        <div v-if="!selectedId" class="empty-state">
          <h3>Select a conversation</h3>
          <p>New chats appear here as visitors start them.</p>
        </div>
        <div v-else-if="threadLoading" class="loading-page"><div class="spinner"></div></div>
        <div v-else-if="threadMissing" class="empty-state">
          <h3>Conversation not found</h3>
          <p>It may have been deleted, or it belongs to another workspace.</p>
        </div>
        <template v-else-if="conversation">
          <header class="thread-header">
            <button class="btn btn-secondary btn-sm back-btn" @click="router.push({ path: '/chat', query: route.query })">Back</button>
            <div class="thread-title">
              <strong>{{ visitorLabel(conversation) }}</strong>
              <span :class="stateBadge(conversation.state)">{{ conversation.state }}</span>
              <span class="muted">
                {{ conversation.assigned_to ? `Assigned to ${conversation.assigned_to.id === myId ? "you" : conversation.assigned_to.name}` : "Unassigned" }}
              </span>
            </div>
            <div v-if="canEdit" class="thread-actions">
              <button v-if="conversation.assigned_to_id !== myId" class="btn btn-secondary btn-sm" @click="assign(myId)">Assign to me</button>
              <button v-if="conversation.assigned_to_id" class="btn btn-secondary btn-sm" @click="assign(null)">Unassign</button>
              <button v-if="conversation.state === 'open' || conversation.state === 'pending'" class="btn btn-primary btn-sm"
                @click="setState('closed')">Close</button>
              <button v-else class="btn btn-secondary btn-sm" @click="setState('open')">Reopen</button>
              <button v-if="conversation.state !== 'spam'" class="btn btn-secondary btn-sm" @click="setState('spam')">Spam</button>
              <button v-if="canErase" class="btn btn-danger btn-sm" @click="eraseConversation">Delete</button>
            </div>
          </header>

          <div ref="threadEl" class="thread-messages" aria-live="polite">
            <div v-if="hasOlder" class="older">
              <button class="btn btn-secondary btn-sm" :disabled="loadingOlder" @click="loadOlder">
                {{ loadingOlder ? "Loading…" : "Load earlier messages" }}
              </button>
            </div>
            <div v-for="m in messages" :key="m.client_msg_id || m.id" class="msg" :class="[`msg-${m.author_kind}`, { pending: m.pending, failed: m.failed }]">
              <div class="msg-meta">
                <span>{{ authorLabel(m) }}</span>
                <span>
                  <span v-if="m.channel === 'email'" class="badge badge-neutral msg-tag" title="The visitor answered by email">via email</span>
                  <span v-else-if="m.email_uuid" class="badge badge-neutral msg-tag" title="The visitor had left, so this was emailed to them">emailed</span>
                  {{ formatTime(m.created_at) }}
                </span>
              </div>
              <div v-if="m.body" class="msg-body"><template v-for="(s, i) in segments(m.body)" :key="i"><a v-if="s.href" :href="s.href"
                    target="_blank" rel="noopener noreferrer nofollow">{{ s.text }}</a><template v-else>{{ s.text }}</template></template></div>
              <ChatAttachmentView v-for="(a, i) in m.attachments || []" :key="i" :conversation="conversation.uuid"
                :message-id="m.id" :index="i" :attachment="a" />
              <div v-if="m.failed" class="msg-failed">
                Not sent · <a href="#" @click.prevent="deliver(m)">Retry</a>
              </div>
              <div v-if="m.seq > 0 && m.seq === seenSeq" class="msg-seen">Seen</div>
            </div>
            <div v-if="visitorTyping" class="typing muted">{{ visitorLabel(conversation) }} is typing…</div>
          </div>

          <div v-if="visitorLeftNote" class="visitor-left" role="status">{{ visitorLeftNote }}</div>

          <footer v-if="canEdit" class="composer">
            <ul v-if="snippetQuery !== null && snippetMatches.length" class="snippet-pop" role="listbox"
              aria-label="Canned replies">
              <li v-for="(s, i) in snippetMatches" :key="s.id" role="option" :aria-selected="i === snippetIndex"
                :class="{ active: i === snippetIndex }" @mousedown.prevent="insertSnippet(s)">
                <strong>/{{ s.shortcut }}</strong> <span class="muted">{{ s.title }}</span>
              </li>
            </ul>
            <button v-if="widgetAttachments !== false" class="btn btn-secondary attach-btn" type="button"
              title="Send an image or PDF (up to 5 MB)" aria-label="Attach a file" @click="pickFile">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
                stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
                <path d="M21.44 11.05l-9.19 9.19a6 6 0 0 1-8.49-8.49l9.19-9.19a4 4 0 0 1 5.66 5.66l-9.2 9.19a2 2 0 0 1-2.83-2.83l8.49-8.48" />
              </svg>
            </button>
            <input ref="fileInput" type="file" accept="image/png,image/jpeg,image/gif,image/webp,application/pdf"
              class="file-input" @change="onFile" />
            <textarea ref="composerEl" v-model="composer" class="form-input" rows="2"
              placeholder="Reply… (Enter to send, Shift+Enter for a new line, / for canned replies)"
              @keydown="onComposerKey" @input="onComposerInput" @blur="snippetQuery = null"></textarea>
            <button class="btn btn-primary" :disabled="!composer.trim()" @click="send">Send</button>
          </footer>
          <footer v-else class="composer muted">Viewers can read conversations but not reply.</footer>
        </template>
      </section>

      <!-- Visitor -->
      <aside v-if="conversation && selectedId" class="card chat-visitor">
        <div class="card-header"><h2>Visitor</h2></div>
        <div class="card-body">
          <dl class="facts">
            <dt>Status</dt>
            <dd>
              <span v-if="conversation.visitor_live" class="badge badge-success">On the page</span>
              <span v-else class="badge badge-secondary">Away</span>
            </dd>
            <template v-if="ctxVisitor?.banned_at">
              <dt>Ban</dt>
              <dd><span class="badge badge-danger">banned</span> {{ new Date(ctxVisitor.banned_at).toLocaleString() }}</dd>
            </template>
            <dt>Name</dt>
            <dd>{{ ctxVisitor?.name || "—" }}</dd>
            <dt>Email</dt>
            <dd>
              <template v-if="ctxVisitor?.email">
                {{ ctxVisitor.email }}
                <span v-if="ctxVisitor.email_verified" class="badge badge-success"
                  title="Signed by your website's server">✓ verified</span>
                <span v-else class="badge badge-neutral" title="Typed by the visitor, not verified">unverified</span>
              </template>
              <template v-else>—</template>
            </dd>
            <template v-if="ctxVisitor?.external_id">
              <dt>User ID</dt>
              <dd class="break">{{ ctxVisitor.external_id }}</dd>
            </template>
            <template v-if="visitorCtx?.contact">
              <dt>Contact</dt>
              <dd>
                <router-link :to="`/contacts/${visitorCtx.contact.id}`">
                  {{ visitorCtx.contact.name || visitorCtx.contact.email }}
                </router-link>
              </dd>
            </template>
            <dt>Current page</dt>
            <dd>
              <a v-if="isHttpURL(conversation.visitor?.last_page_url)" :href="conversation.visitor!.last_page_url"
                target="_blank" rel="noopener noreferrer nofollow" class="break">{{ conversation.visitor!.last_page_url }}</a>
              <template v-else>—</template>
            </dd>
            <dt>Started on</dt>
            <dd>
              <a v-if="isHttpURL(conversation.start_page_url)" :href="conversation.start_page_url" target="_blank"
                rel="noopener noreferrer nofollow" class="break">{{ conversation.start_page_url }}</a>
              <template v-else>—</template>
            </dd>
            <dt>Widget</dt>
            <dd>
              <router-link v-if="conversation.widget" :to="`/chat-widgets/${conversation.widget.id}`">
                {{ conversation.widget.display_name }}
              </router-link>
            </dd>
            <dt>Started</dt>
            <dd>{{ new Date(conversation.created_at).toLocaleString() }}</dd>
            <dt>Last seen</dt>
            <dd>{{ conversation.visitor?.last_seen_at ? new Date(conversation.visitor.last_seen_at).toLocaleString() : "—" }}</dd>
            <dt>IP</dt>
            <dd>
              {{ conversation.visitor?.last_seen_ip || "—" }}
              <span v-if="conversation.visitor && conversation.visitor.first_seen_ip !== conversation.visitor.last_seen_ip"
                class="muted">(first {{ conversation.visitor.first_seen_ip }})</span>
            </dd>
            <dt>Browser</dt>
            <dd class="break muted">{{ conversation.visitor?.user_agent || "—" }}</dd>
            <template v-if="conversation.status !== 'received'">
              <dt>Spam scan</dt>
              <dd>
                <span :class="conversation.status === 'flagged' ? 'badge badge-warning' : 'badge badge-danger'">
                  {{ conversation.status }}
                </span>
                score {{ conversation.spam_score }}
                <div v-if="conversation.scan_reasons?.length" class="muted">{{ conversation.scan_reasons.join(", ") }}</div>
              </dd>
            </template>
          </dl>

          <template v-if="attributeEntries.length">
            <h3 class="side-title">Attributes</h3>
            <table class="attrs">
              <tbody>
                <tr v-for="[k, v] in attributeEntries" :key="k">
                  <th scope="row">{{ k }}</th>
                  <td class="break">{{ String(v) }}</td>
                </tr>
              </tbody>
            </table>
          </template>

          <template v-if="visitorCtx?.history.length">
            <h3 class="side-title">Previous conversations</h3>
            <ul class="history">
              <li v-for="h in visitorCtx.history" :key="h.uuid">
                <router-link :to="{ path: `/chat/${h.uuid}`, query: route.query }">
                  {{ new Date(h.created_at).toLocaleDateString() }}
                </router-link>
                <span :class="stateBadge(h.state)">{{ h.state }}</span>
                <span v-if="h.widget" class="muted">{{ h.widget.display_name }}</span>
                <div v-if="h.last_message_preview" class="muted history-preview">{{ h.last_message_preview }}</div>
              </li>
            </ul>
          </template>

          <div v-if="canEdit" class="ban-actions">
            <button v-if="ctxVisitor?.banned_at" class="btn btn-secondary btn-sm" @click="unban">Lift ban</button>
            <button v-else class="btn btn-danger btn-sm" @click="ban">Ban visitor</button>
          </div>
        </div>
      </aside>
    </div>
  </div>
</template>

<style scoped>
.chat-layout {
  display: grid;
  grid-template-columns: 300px minmax(0, 1fr) 280px;
  gap: 16px;
  height: calc(100vh - 170px);
  min-height: 480px;
}

.chat-layout > .card {
  margin: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.chat-tabs {
  display: flex;
  border-bottom: 1px solid var(--border-primary);
}

.chat-tab {
  flex: 1;
  padding: 10px 4px;
  background: none;
  border: 0;
  border-bottom: 2px solid transparent;
  color: var(--text-secondary);
  font-size: 13px;
  cursor: pointer;
}

.chat-tab.active {
  color: var(--primary-600);
  border-bottom-color: var(--primary-600);
  font-weight: 600;
}

.chat-search {
  padding: 10px;
  border-bottom: 1px solid var(--border-primary);
}

.conv-preview {
  display: block;
  font-size: 13px;
  color: var(--text-secondary, #6b7280);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  margin: 2px 0;
}

.msg-tag {
  margin-right: 6px;
  font-size: 10px;
}

.msg-seen {
  font-size: 11px;
  opacity: 0.75;
  text-align: right;
  margin-top: 2px;
}

.typing {
  font-size: 12px;
  font-style: italic;
  padding: 2px 4px;
}

.visitor-left {
  margin: 0 16px;
  padding: 8px 12px;
  border-radius: 8px;
  font-size: 13px;
  background: var(--bg-tertiary, #f3f4f6);
  color: var(--text-secondary);
}

.widget-filter {
  font-size: 12px;
  margin-top: 6px;
}

.conv-items {
  list-style: none;
  margin: 0;
  padding: 0;
  overflow-y: auto;
  flex: 1;
}

.conv-item {
  width: 100%;
  text-align: left;
  background: none;
  border: 0;
  border-bottom: 1px solid var(--border-secondary);
  padding: 10px 12px;
  cursor: pointer;
  display: flex;
  flex-direction: column;
  gap: 4px;
  color: var(--text-primary);
}

.conv-item:hover {
  background: var(--bg-hover);
}

.conv-item.active {
  background: var(--bg-active);
}

.conv-top,
.conv-bottom {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
}

.conv-name {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.conv-time {
  color: var(--text-tertiary);
  font-size: 12px;
  white-space: nowrap;
}

.conv-bottom {
  font-size: 12px;
}

.awaiting-dot {
  margin-left: auto;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--primary-600);
}

.load-more {
  margin: 8px;
}

.thread-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  border-bottom: 1px solid var(--border-primary);
  flex-wrap: wrap;
}

.thread-title {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 0;
  font-size: 13px;
}

.thread-actions {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.back-btn {
  display: none;
}

.thread-messages {
  flex: 1;
  overflow-y: auto;
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.older {
  text-align: center;
}

.msg {
  max-width: 75%;
  align-self: flex-start;
}

.msg-agent {
  align-self: flex-end;
}

.msg-system {
  align-self: center;
  max-width: 90%;
}

.msg-meta {
  display: flex;
  gap: 8px;
  font-size: 11px;
  color: var(--text-tertiary);
  margin-bottom: 2px;
}

.msg-agent .msg-meta {
  justify-content: flex-end;
}

.msg-body {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  padding: 8px 12px;
  border-radius: 12px;
  background: var(--bg-tertiary);
  color: var(--text-primary);
  font-size: 14px;
  line-height: 1.45;
}

.msg-agent .msg-body {
  background: var(--primary-600);
  color: #fff;
}

.msg-agent .msg-body a {
  color: #fff;
}

.msg-system .msg-body {
  background: none;
  color: var(--text-tertiary);
  font-size: 12px;
}

.msg.pending .msg-body {
  opacity: 0.6;
}

.msg-failed {
  font-size: 12px;
  color: var(--danger-600, #dc2626);
  text-align: right;
}

.composer {
  position: relative;
}

.snippet-pop {
  position: absolute;
  bottom: calc(100% - 4px);
  left: 12px;
  right: 12px;
  max-height: 240px;
  overflow-y: auto;
  margin: 0;
  padding: 4px;
  list-style: none;
  background: var(--bg-primary, #fff);
  border: 1px solid var(--border-primary);
  border-radius: 8px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
  z-index: 5;
}

.snippet-pop li {
  padding: 6px 10px;
  border-radius: 6px;
  cursor: pointer;
  font-size: 13px;
}

.snippet-pop li.active {
  background: var(--bg-secondary);
}

.attach-btn {
  align-self: flex-end;
  padding: 8px 10px;
}

.file-input {
  display: none;
}

.side-title {
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-muted);
  margin: 18px 0 8px;
}

.attrs {
  width: 100%;
  font-size: 13px;
  border-collapse: collapse;
}

.attrs th {
  text-align: left;
  font-weight: 500;
  color: var(--text-secondary);
  padding: 3px 8px 3px 0;
  vertical-align: top;
}

.attrs td {
  padding: 3px 0;
}

.history {
  list-style: none;
  margin: 0;
  padding: 0;
  font-size: 13px;
}

.history li {
  padding: 6px 0;
  border-bottom: 1px solid var(--border-primary);
}

.history-preview {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ban-actions {
  margin-top: 18px;
}

.composer {
  display: flex;
  gap: 8px;
  padding: 12px;
  border-top: 1px solid var(--border-primary);
  align-items: flex-end;
}

.composer textarea {
  flex: 1;
  resize: vertical;
  min-height: 42px;
}

.facts {
  margin: 0;
  display: grid;
  grid-template-columns: 1fr;
  gap: 2px;
  font-size: 13px;
}

.facts dt {
  color: var(--text-tertiary);
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  margin-top: 10px;
}

.facts dd {
  margin: 0;
}

.break {
  overflow-wrap: anywhere;
}

.chat-visitor {
  overflow-y: auto;
}

.chat-visitor .card-body {
  overflow-y: auto;
}

.live-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--text-tertiary);
}

.live-dot.on {
  background: var(--success-500);
}

@media (max-width: 1200px) {
  .chat-layout {
    grid-template-columns: 280px minmax(0, 1fr);
  }

  .chat-visitor {
    display: none !important;
  }
}

@media (max-width: 768px) {
  .chat-layout {
    grid-template-columns: 1fr;
    height: calc(100vh - 140px);
  }

  .chat-layout.has-selection .chat-list {
    display: none;
  }

  .chat-layout:not(.has-selection) .chat-thread {
    display: none;
  }

  .back-btn {
    display: inline-flex;
  }
}
</style>
