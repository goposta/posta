<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { messagesApi } from "../../api/messages";
import { useNotificationStore } from "../../stores/notification";
import { apiMessage } from "../../composables/apiError";
import type { Message } from "../../api/types";

const route = useRoute();
const router = useRouter();
const notify = useNotificationStore();

const loading = ref(true);
const message = ref<Message | null>(null);
const loadFailed = ref(false);

const replySubject = ref("");
const replyText = ref("");
const sending = ref(false);
const busy = ref(false);

const uuid = String(route.params.id);

async function load() {
  loading.value = true;
  try {
    const res = await messagesApi.get(uuid);
    message.value = res.data.data;
    replySubject.value = defaultReplySubject(res.data.data.subject);
  } catch (e: any) {
    loadFailed.value = true;
    notify.error(apiMessage(e, "Failed to load message"));
  } finally {
    loading.value = false;
  }
}

function defaultReplySubject(subject: string) {
  const trimmed = (subject || "").trim();
  if (!trimmed) return "Re: your message";
  return /^re:/i.test(trimmed) ? trimmed : `Re: ${trimmed}`;
}

async function sendReply() {
  if (!message.value || !replyText.value.trim()) return;
  sending.value = true;
  try {
    await messagesApi.reply(uuid, {
      subject: replySubject.value,
      text: replyText.value,
      html: textToHtml(replyText.value),
    });
    replyText.value = "";
    notify.success("Reply sent.");
    await load();
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to send reply"));
  } finally {
    sending.value = false;
  }
}

function textToHtml(text: string) {
  const escaped = text
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
  return `<div style="white-space:pre-wrap">${escaped}</div>`;
}

async function setState(state: string) {
  busy.value = true;
  try {
    const res = await messagesApi.setState(uuid, { state });
    message.value = res.data.data;
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to update message"));
  } finally {
    busy.value = false;
  }
}

async function markSpam(createFilter: boolean, kind = "domain") {
  busy.value = true;
  try {
    const res = await messagesApi.markSpam(uuid, { create_filter: createFilter, kind });
    message.value = res.data.data;
    notify.success(createFilter ? "Marked as spam and filter created." : "Marked as spam.");
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to mark as spam"));
  } finally {
    busy.value = false;
  }
}

async function markNotSpam() {
  busy.value = true;
  try {
    const res = await messagesApi.markNotSpam(uuid);
    message.value = res.data.data;
    notify.success("Spam verdict cleared.");
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to clear the spam verdict"));
  } finally {
    busy.value = false;
  }
}

async function remove() {
  if (!confirm("Delete this message permanently?")) return;
  busy.value = true;
  try {
    await messagesApi.delete(uuid);
    router.push("/messages");
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to delete message"));
    busy.value = false;
  }
}

const BODY_KEYS = [
  "message",
  "body",
  "comments",
  "comment",
  "content",
  "description",
  "your-message",
];

const SCAN_REASON_LABELS: Record<string, string> = {
  honeypot: "Honeypot field was filled in",
  nonce_invalid: "Anti-bot token missing, expired, or reused",
  sender_suppressed: "Sender is on the suppression list",
  sender_unparseable: "Sender address does not parse",
  disposable_email: "Disposable email domain",
  url_shortener: "Contains a URL shortener",
  body_too_short: "Body is unusually short",
  body_too_long: "Body is unusually long",
  excessive_caps: "Mostly uppercase",
  embedded_markup: "Contains HTML or BBCode markup",
  embedded_script: "Contains a script tag or javascript: URL",
  bot_user_agent: "Submitted by a scripted client",
};

const showRaw = ref(false);

const bodyField = computed(() => {
  const fields = message.value?.fields ?? [];
  return fields.find((f) => BODY_KEYS.includes(f.key.trim().toLowerCase())) ?? null;
});

const detailFields = computed(() =>
  (message.value?.fields ?? []).filter((f) => f !== bodyField.value)
);

const bodyText = computed(() => bodyField.value?.value || message.value?.body || "");

const bodyLabel = computed(() =>
  bodyField.value ? humanizeKey(bodyField.value.key) : "Message"
);

const fieldCount = computed(() => message.value?.fields?.length ?? 0);

const rawFields = computed(() =>
  JSON.stringify(
    Object.fromEntries((message.value?.fields ?? []).map((f) => [f.key, f.value])),
    null,
    2
  )
);

function humanizeKey(key: string) {
  const cleaned = key
    .replace(/^_+/, "")
    .replace(/^your[-_\s]+/i, "")
    .replace(/[_-]+/g, " ")
    .replace(/([a-z0-9])([A-Z])/g, "$1 $2")
    .trim();
  if (!cleaned) return key;
  return cleaned.charAt(0).toUpperCase() + cleaned.slice(1);
}

function valueKind(value: string): "email" | "url" | "phone" | "text" {
  const v = value.trim();
  if (/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v)) return "email";
  if (/^https?:\/\/\S+$/i.test(v)) return "url";
  if (/^\+?\d[\d\s().\/-]{5,}$/.test(v)) return "phone";
  return "text";
}

function scanReasonLabel(reason: string) {
  if (SCAN_REASON_LABELS[reason]) return SCAN_REASON_LABELS[reason];
  const [head, ...rest] = reason.split(":");
  const tail = rest.join(":");
  switch (head) {
    case "links":
      return `${tail} links in the message`;
    case "repeat_ip":
      return `${tail} submissions from this IP in the last hour`;
    case "repeat_sender":
      return `${tail} submissions from this sender in the last hour`;
    case "filter":
      return rest.length > 1
        ? `Matched a ${rest[0]} filter: ${rest.slice(1).join(":")}`
        : `Matched a ${tail} filter`;
    default:
      return reason;
  }
}

async function copyValue(value: string) {
  try {
    await navigator.clipboard.writeText(value);
    notify.success("Copied to clipboard.");
  } catch {
    notify.error("Could not copy to clipboard.");
  }
}

const canReply = computed(
  () => !!message.value?.sender_email && message.value?.status !== "rejected"
);
const replyFromConfigured = computed(() => !!message.value?.form?.reply_from);
const scanReasons = computed(() => message.value?.scan_reasons || []);

function statusBadgeClass(value: string) {
  switch (value) {
    case "received":
      return "badge badge-success";
    case "flagged":
      return "badge badge-warning";
    default:
      return "badge badge-danger";
  }
}

function senderLabel(m: Message) {
  if (m.sender_name && m.sender_email) return `${m.sender_name} <${m.sender_email}>`;
  return m.sender_email || m.sender_name || "unknown sender";
}

function formatDate(date: string | null | undefined) {
  if (!date) return "-";
  return new Date(date).toLocaleString();
}

function formatBytes(n: number) {
  if (!n) return "0 B";
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(2)} MB`;
}

onMounted(load);
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Message</h1>
      <div style="display: flex; gap: 8px">
        <button v-if="message" class="btn btn-secondary" :disabled="busy" @click="setState('closed')">
          Close
        </button>
        <button
          v-if="message && message.status !== 'quarantined'"
          class="btn btn-secondary"
          :disabled="busy"
          @click="markSpam(false)"
        >
          Mark spam
        </button>
        <button
          v-if="message && message.status === 'quarantined'"
          class="btn btn-secondary"
          :disabled="busy"
          @click="markNotSpam"
        >
          Not spam
        </button>
        <button class="btn btn-danger" :disabled="busy" @click="remove">Delete</button>
        <button class="btn btn-secondary" @click="router.push('/messages')">Back</button>
      </div>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <div v-else-if="loadFailed" class="card">
      <div class="empty-state">
        <h3>Could not load this message</h3>
        <p>It may have been deleted, or it belongs to another workspace.</p>
        <button class="btn btn-secondary" @click="router.push('/messages')">Back to messages</button>
      </div>
    </div>

    <template v-else-if="message">
      <div class="card" style="margin-bottom: 24px">
        <div class="card-header">
          <h2>{{ message.subject || "(no subject)" }}</h2>
          <div style="display: flex; gap: 8px">
            <span :class="statusBadgeClass(message.status)">{{ message.status }}</span>
            <span class="badge badge-secondary">{{ message.state }}</span>
          </div>
        </div>
        <div class="card-body">
          <table>
            <tbody>
              <tr>
                <td style="font-weight: 600; width: 160px">From</td>
                <td>{{ senderLabel(message) }}</td>
              </tr>
              <tr v-if="message.sender_phone">
                <td style="font-weight: 600">Phone</td>
                <td><a :href="`tel:${message.sender_phone}`">{{ message.sender_phone }}</a></td>
              </tr>
              <tr>
                <td style="font-weight: 600">Form</td>
                <td>
                  <router-link v-if="message.form" :to="`/forms/${message.form.id}`">
                    {{ message.form.name }}
                  </router-link>
                  <span v-else>-</span>
                </td>
              </tr>
              <tr>
                <td style="font-weight: 600">Received</td>
                <td>{{ formatDate(message.created_at) }}</td>
              </tr>
              <tr v-if="message.origin">
                <td style="font-weight: 600">Origin</td>
                <td><code style="font-size: 12px">{{ message.origin }}</code></td>
              </tr>
              <tr v-if="message.client_ip">
                <td style="font-weight: 600">Client IP</td>
                <td><code style="font-size: 12px">{{ message.client_ip }}</code></td>
              </tr>
              <tr v-if="message.spam_score">
                <td style="font-weight: 600">Spam score</td>
                <td>{{ message.spam_score.toFixed(1) }}</td>
              </tr>
              <tr v-if="scanReasons.length">
                <td style="font-weight: 600">Scan reasons</td>
                <td class="scan-reasons">
                  <span
                    v-for="reason in scanReasons"
                    :key="reason"
                    class="badge badge-warning"
                    :title="reason"
                  >{{ scanReasonLabel(reason) }}</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div class="card" style="margin-bottom: 24px">
        <div class="card-header">
          <h2>Submission</h2>
          <div class="submission-actions">
            <span v-if="fieldCount" class="badge badge-neutral">
              {{ fieldCount }} field{{ fieldCount === 1 ? "" : "s" }}
            </span>
            <button
              v-if="fieldCount"
              class="btn btn-secondary btn-sm"
              @click="showRaw = !showRaw"
            >
              {{ showRaw ? "Formatted" : "Raw" }}
            </button>
          </div>
        </div>

        <div class="card-body">
          <pre v-if="showRaw" class="raw-fields">{{ rawFields }}</pre>

          <template v-else>
            <div v-if="bodyText" class="submission-body">
              <div class="submission-label">{{ bodyLabel }}</div>
              <div class="submission-body-text">{{ bodyText }}</div>
            </div>

            <dl v-if="detailFields.length" class="field-grid">
              <template v-for="(field, idx) in detailFields" :key="`${idx}-${field.key}`">
                <dt :title="field.key">{{ humanizeKey(field.key) }}</dt>
                <dd>
                  <span v-if="!field.value.trim()" class="field-empty">—</span>

                  <span v-else class="field-value">
                    <a
                      v-if="valueKind(field.value) === 'email'"
                      :href="`mailto:${field.value.trim()}`"
                    >{{ field.value }}</a>
                    <a
                      v-else-if="valueKind(field.value) === 'phone'"
                      :href="`tel:${field.value.trim()}`"
                    >{{ field.value }}</a>
                    <a
                      v-else-if="valueKind(field.value) === 'url'"
                      :href="field.value.trim()"
                      target="_blank"
                      rel="noopener noreferrer nofollow"
                    >{{ field.value }}</a>
                    <span v-else>{{ field.value }}</span>

                    <button
                      class="field-copy"
                      type="button"
                      :title="`Copy ${humanizeKey(field.key)}`"
                      @click="copyValue(field.value)"
                    >Copy</button>
                  </span>
                </dd>
              </template>
            </dl>

            <div v-if="!bodyText && !detailFields.length" class="empty-state">
              <h3>Nothing was submitted</h3>
              <p>This submission arrived with no readable fields.</p>
            </div>
          </template>
        </div>
      </div>

      <div
        v-if="message.attachments && message.attachments.length"
        class="card"
        style="margin-bottom: 24px"
      >
        <div class="card-header"><h2>Attachments</h2></div>
        <div class="card-body">
          <ul style="list-style: none; padding: 0; margin: 0">
            <li
              v-for="(attachment, idx) in message.attachments"
              :key="idx"
              style="padding: 6px 0"
            >
              <a
                :href="messagesApi.attachmentUrl(message.uuid, idx)"
                target="_blank"
                rel="noopener"
              >{{ attachment.filename }}</a>
              <span class="muted" style="margin-left: 8px">
                {{ attachment.content_type }} · {{ formatBytes(attachment.size) }}
              </span>
            </li>
          </ul>
        </div>
      </div>

      <div v-if="message.replies && message.replies.length" class="card" style="margin-bottom: 24px">
        <div class="card-header"><h2>Thread</h2></div>
        <div class="card-body">
          <div
            v-for="reply in message.replies"
            :key="reply.uuid"
            style="border-bottom: 1px solid var(--border); padding: 12px 0"
          >
            <div style="display: flex; justify-content: space-between; gap: 12px">
              <strong>{{ reply.author?.name || reply.from_addr }}</strong>
              <span class="muted">{{ formatDate(reply.created_at) }}</span>
            </div>
            <div class="muted" style="font-size: 13px">{{ reply.subject }}</div>
            <p style="white-space: pre-wrap; margin: 8px 0 0">{{ reply.text_body }}</p>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="card-header"><h2>Reply</h2></div>
        <div class="card-body">
          <div v-if="!canReply" class="empty-state">
            <p>This message has no reply address, so it cannot be answered from here.</p>
          </div>
          <div v-else-if="!replyFromConfigured" class="alert alert-warning">
            Set a reply sender address on
            <router-link v-if="message.form" :to="`/forms/${message.form.id}`">this form</router-link>
            <span v-else>this form</span>
            before replying. It must use a verified domain in this workspace.
          </div>
          <template v-else>
            <div class="form-group">
              <label class="form-label">To</label>
              <input class="form-input" :value="message.sender_email" disabled />
            </div>
            <div class="form-group">
              <label class="form-label">Subject</label>
              <input v-model="replySubject" class="form-input" />
            </div>
            <div class="form-group">
              <label class="form-label">Message</label>
              <textarea
                v-model="replyText"
                class="form-input"
                rows="8"
                placeholder="Write your reply..."
              ></textarea>
            </div>
            <button
              class="btn btn-primary"
              :disabled="sending || !replyText.trim()"
              @click="sendReply"
            >
              {{ sending ? "Sending..." : "Send reply" }}
            </button>
          </template>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.submission-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.btn-sm {
  padding: 4px 10px;
  font-size: 12px;
}

.submission-body {
  margin-bottom: 20px;
}

.submission-label,
.field-grid dt {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  font-weight: 600;
  color: var(--text-tertiary);
}

.submission-label {
  margin-bottom: 8px;
}

.submission-body-text {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  padding: 14px 16px;
  border-radius: 8px;
  background: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  line-height: 1.65;
  font-size: 14px;
  color: var(--text-primary);
}

.field-grid {
  display: grid;
  grid-template-columns: minmax(140px, 220px) 1fr;
  margin: 0;
  border: 1px solid var(--border-primary);
  border-radius: 8px;
  overflow: hidden;
}

.field-grid dt {
  padding: 11px 14px;
  background: var(--bg-secondary);
  border-bottom: 1px solid var(--border-secondary);
  overflow-wrap: anywhere;
}

.field-grid dd {
  margin: 0;
  padding: 11px 14px;
  font-size: 14px;
  color: var(--text-primary);
  border-bottom: 1px solid var(--border-secondary);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.field-grid dt:last-of-type,
.field-grid dd:last-of-type {
  border-bottom: 0;
}

.field-value {
  display: inline-flex;
  align-items: baseline;
  gap: 8px;
  max-width: 100%;
}

.field-empty {
  color: var(--text-muted);
}

.field-copy {
  flex: none;
  border: 0;
  background: none;
  padding: 0;
  font: inherit;
  font-size: 12px;
  color: var(--text-muted);
  cursor: pointer;
  opacity: 0;
  transition: opacity 0.12s ease, color 0.12s ease;
}

.field-grid dd:hover .field-copy,
.field-copy:focus-visible {
  opacity: 1;
}

.field-copy:hover {
  color: var(--primary-600);
  text-decoration: underline;
}

.raw-fields {
  margin: 0;
  padding: 16px;
  border-radius: 8px;
  background: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  font-size: 13px;
  line-height: 1.6;
  color: var(--text-primary);
  overflow-x: auto;
}

.scan-reasons {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

@media (max-width: 640px) {
  .field-grid {
    grid-template-columns: 1fr;
  }

  .field-grid dt {
    background: transparent;
    border-bottom: 0;
    padding-bottom: 0;
  }

  .field-copy {
    opacity: 1;
  }
}
</style>
