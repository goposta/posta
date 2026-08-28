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
                <td>
                  <span
                    v-for="reason in scanReasons"
                    :key="reason"
                    class="badge badge-warning"
                    style="margin-right: 6px"
                  >{{ reason }}</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div class="card" style="margin-bottom: 24px">
        <div class="card-header"><h2>Submission</h2></div>
        <div class="card-body">
          <table v-if="message.fields && message.fields.length">
            <tbody>
              <tr v-for="field in message.fields" :key="field.key">
                <td style="font-weight: 600; width: 200px">{{ field.key }}</td>
                <td style="white-space: pre-wrap">{{ field.value }}</td>
              </tr>
            </tbody>
          </table>
          <p v-else style="white-space: pre-wrap">{{ message.body }}</p>
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
