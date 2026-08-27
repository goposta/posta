<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { formsApi, messagesApi } from "../../api/messages";
import type { Form, Message, MessageStats } from "../../api/types";
import Pagination from "../../components/Pagination.vue";
import { usePagination } from "../../composables/usePagination";

const router = useRouter();
const route = useRoute();
const loading = ref(true);
const featureDisabled = ref(false);
const messages = ref<Message[]>([]);
const forms = ref<Form[]>([]);
const stats = ref<MessageStats | null>(null);

const formId = ref<number>(Number(route.query.form_id) || 0);
const status = ref("");
const state = ref("");
const unread = ref(false);
const q = ref("");

const { pageable, goToPage } = usePagination(async (page) => {
  loading.value = true;
  try {
    const res = await messagesApi.list({
      page,
      form_id: formId.value || undefined,
      status: status.value || undefined,
      state: state.value || undefined,
      unread: unread.value || undefined,
      q: q.value || undefined,
    });
    messages.value = res.data.data;
    pageable.value = res.data.pageable;
    featureDisabled.value = false;
  } catch (e: any) {
    if (e?.response?.status === 404) {
      featureDisabled.value = true;
    } else {
      console.error("Failed to load messages", e);
    }
  } finally {
    loading.value = false;
  }
});

async function loadSidebarData() {
  try {
    const [formsRes, statsRes] = await Promise.all([
      formsApi.list({ size: 100 }),
      messagesApi.stats(),
    ]);
    forms.value = formsRes.data.data;
    stats.value = statsRes.data.data;
  } catch {
    /* the list view already reports a disabled feature */
  }
}

function applyFilters() {
  goToPage(0);
}

function resetFilters() {
  formId.value = 0;
  status.value = "";
  state.value = "";
  unread.value = false;
  q.value = "";
  goToPage(0);
}

let sse: EventSource | null = null;
onMounted(async () => {
  await loadSidebarData();
  try {
    sse = new EventSource(messagesApi.streamUrl(), { withCredentials: true });
    sse.addEventListener("message.received", () => {
      goToPage(pageable.value?.current_page ?? 0);
      loadSidebarData();
    });
    sse.onerror = () => {
      /* reconnects are handled by the browser */
    };
  } catch {
    /* SSE not supported */
  }
});
onBeforeUnmount(() => {
  if (sse) sse.close();
});

const hasForms = computed(() => forms.value.length > 0);

function statusBadgeClass(value: string) {
  switch (value) {
    case "received":
      return "badge badge-success";
    case "flagged":
      return "badge badge-warning";
    case "quarantined":
      return "badge badge-danger";
    case "rejected":
      return "badge badge-danger";
    default:
      return "badge";
  }
}

function stateBadgeClass(value: string) {
  switch (value) {
    case "new":
      return "badge badge-info";
    case "replied":
      return "badge badge-success";
    case "closed":
      return "badge badge-secondary";
    case "spam":
      return "badge badge-danger";
    default:
      return "badge";
  }
}

function senderLabel(message: Message) {
  if (message.sender_name && message.sender_email)
    return `${message.sender_name} <${message.sender_email}>`;
  return message.sender_email || message.sender_name || "unknown sender";
}

function formatDate(date: string | null | undefined) {
  if (!date) return "-";
  return new Date(date).toLocaleString();
}
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Messages</h1>
      <router-link to="/forms" class="btn btn-secondary">Manage forms</router-link>
    </div>

    <div v-if="loading && !messages.length" class="loading-page">
      <div class="spinner"></div>
    </div>

    <div v-else-if="featureDisabled" class="card">
      <div class="empty-state">
        <h3>Web form messages are disabled</h3>
        <p>
          Ask your administrator to enable them by setting
          <code>POSTA_MESSAGES_ENABLED=true</code>.
        </p>
      </div>
    </div>

    <template v-else>
      <div v-if="stats" class="stats-grid" style="margin-bottom: 20px">
        <div class="stat-card">
          <div class="stat-label">Messages</div>
          <div class="stat-value">{{ stats.total }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Unread</div>
          <div class="stat-value">{{ stats.unread }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Spam</div>
          <div class="stat-value">{{ stats.spam }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Forms</div>
          <div class="stat-value">{{ stats.forms }}</div>
        </div>
      </div>

      <div class="card">
        <div
          class="card-body"
          style="display: flex; gap: 12px; flex-wrap: wrap; align-items: flex-end"
        >
          <div style="flex: 1 1 160px">
            <label class="form-label">Form</label>
            <select v-model.number="formId" class="form-select" @change="applyFilters">
              <option :value="0">All forms</option>
              <option v-for="f in forms" :key="f.id" :value="f.id">{{ f.name }}</option>
            </select>
          </div>

          <div style="flex: 1 1 140px">
            <label class="form-label">Status</label>
            <select v-model="status" class="form-select" @change="applyFilters">
              <option value="">Any</option>
              <option value="received">Received</option>
              <option value="flagged">Flagged</option>
              <option value="quarantined">Quarantined</option>
            </select>
          </div>

          <div style="flex: 1 1 140px">
            <label class="form-label">State</label>
            <select v-model="state" class="form-select" @change="applyFilters">
              <option value="">Any</option>
              <option value="new">New</option>
              <option value="open">Open</option>
              <option value="replied">Replied</option>
              <option value="closed">Closed</option>
              <option value="spam">Spam</option>
            </select>
          </div>

          <div style="flex: 2 1 200px">
            <label class="form-label">Search</label>
            <input
              v-model="q"
              class="form-input"
              placeholder="Sender, subject, or body..."
              @keyup.enter="applyFilters"
            />
          </div>

          <div style="display: flex; gap: 8px; align-items: center">
            <label class="form-label" style="display: flex; gap: 6px; align-items: center; margin: 0">
              <input v-model="unread" type="checkbox" @change="applyFilters" />
              Unread only
            </label>
            <button class="btn btn-primary" @click="applyFilters">Apply</button>
            <button class="btn btn-secondary" @click="resetFilters">Reset</button>
          </div>
        </div>

        <div v-if="messages.length === 0" class="empty-state">
          <h3>No messages yet</h3>
          <p v-if="hasForms">
            Submissions to your forms will appear here as soon as they arrive.
          </p>
          <p v-else>
            Create a form first, then embed it on your website.
            <router-link to="/forms">Create a form</router-link>
          </p>
        </div>

        <template v-else>
          <div class="table-wrapper">
            <table>
              <thead>
                <tr>
                  <th>From</th>
                  <th>Subject</th>
                  <th>Form</th>
                  <th>Status</th>
                  <th>State</th>
                  <th>Received</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="message in messages"
                  :key="message.uuid"
                  style="cursor: pointer"
                  @click="router.push(`/messages/${message.uuid}`)"
                >
                  <td :style="{ fontWeight: message.read_at ? '400' : '600' }">
                    {{ senderLabel(message) }}
                  </td>
                  <td>{{ message.subject || "(no subject)" }}</td>
                  <td>{{ message.form?.name || "-" }}</td>
                  <td>
                    <span :class="statusBadgeClass(message.status)">{{ message.status }}</span>
                  </td>
                  <td>
                    <span :class="stateBadgeClass(message.state)">{{ message.state }}</span>
                  </td>
                  <td>{{ formatDate(message.created_at) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <Pagination :pageable="pageable" @page="goToPage" />
        </template>
      </div>
    </template>
  </div>
</template>
