<script setup lang="ts">
import { ref } from "vue";
import { useRouter } from "vue-router";
import NameField from "../../components/NameField.vue";
import Pagination from "../../components/Pagination.vue";
import { chatWidgetsApi } from "../../api/chat";
import type { ChatWidget } from "../../api/types";
import { useNotificationStore } from "../../stores/notification";
import { useWorkspaceStore } from "../../stores/workspace";
import { apiMessage } from "../../composables/apiError";
import { usePagination } from "../../composables/usePagination";

const router = useRouter();
const notify = useNotificationStore();
const wsStore = useWorkspaceStore();
const loading = ref(true);
const featureDisabled = ref(false);
const widgets = ref<ChatWidget[]>([]);

const showCreate = ref(false);
const creating = ref(false);
const newName = ref("");
const newOrigins = ref("");

const { pageable, goToPage } = usePagination(async (page) => {
  loading.value = true;
  try {
    const res = await chatWidgetsApi.list({ page });
    widgets.value = res.data.data;
    pageable.value = res.data.pageable;
    featureDisabled.value = false;
  } catch (e: any) {
    if (e?.response?.status === 404) {
      featureDisabled.value = true;
    } else {
      notify.error(apiMessage(e, "Failed to load chat widgets"));
    }
  } finally {
    loading.value = false;
  }
});

function openCreate() {
  newName.value = "";
  newOrigins.value = "";
  showCreate.value = true;
}

async function createWidget() {
  if (!newName.value.trim()) return;
  creating.value = true;
  try {
    const res = await chatWidgetsApi.create({
      name: newName.value.trim(),
      allowed_origins: splitList(newOrigins.value),
    });
    showCreate.value = false;
    router.push(`/chat-widgets/${res.data.data.id}`);
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to create chat widget"));
  } finally {
    creating.value = false;
  }
}

function splitList(value: string) {
  return value
    .split(/[\s,]+/)
    .map((v) => v.trim())
    .filter(Boolean);
}

function statusBadgeClass(status: string) {
  switch (status) {
    case "active":
      return "badge badge-success";
    case "paused":
      return "badge badge-warning";
    default:
      return "badge badge-secondary";
  }
}

function formatDate(date: string | null | undefined) {
  if (!date) return "-";
  return new Date(date).toLocaleString();
}
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Chat widgets</h1>
      <div style="display: flex; gap: 8px">
        <router-link to="/chat" class="btn btn-secondary">Conversations</router-link>
        <button v-if="wsStore.canEdit" class="btn btn-primary" @click="openCreate">New widget</button>
      </div>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <div v-else-if="featureDisabled" class="card">
      <div class="empty-state">
        <h3>Live chat is disabled</h3>
        <p>
          Ask your administrator to enable it by setting
          <code>POSTA_CHAT_ENABLED=true</code>.
        </p>
      </div>
    </div>

    <div v-else class="card">
      <div v-if="widgets.length === 0" class="empty-state">
        <h3>No chat widgets yet</h3>
        <p>Create a widget to get a script tag that adds live chat to your website.</p>
        <button v-if="wsStore.canEdit" class="btn btn-primary" @click="openCreate">New widget</button>
      </div>

      <template v-else>
        <div class="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Public key</th>
                <th>Status</th>
                <th>Allowed origins</th>
                <th>Created</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="w in widgets"
                :key="w.id"
                style="cursor: pointer"
                @click="router.push(`/chat-widgets/${w.id}`)"
              >
                <td>{{ w.display_name }}</td>
                <td><code style="font-size: 12px">{{ w.public_key }}</code></td>
                <td><span :class="statusBadgeClass(w.status)">{{ w.status }}</span></td>
                <td>{{ w.allowed_origins?.length ? w.allowed_origins.join(", ") : "Any" }}</td>
                <td>{{ formatDate(w.created_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <Pagination :pageable="pageable" @page="goToPage" />
      </template>
    </div>

    <div v-if="showCreate" class="modal-overlay" @click.self="showCreate = false">
      <div class="modal">
        <div class="modal-header">
          <h3>New chat widget</h3>
        </div>
        <div class="modal-body">
          <NameField v-model="newName" resource="chat widget" usage="Identifies the widget in the API."
            placeholder="e.g. Website chat" autofocus />
          <div class="form-group">
            <label class="form-label">Allowed origins</label>
            <input v-model="newOrigins" class="form-input" placeholder="https://example.com, https://www.example.com" />
            <small class="muted">
              The websites that may load this widget. Leave blank to allow any; you can change this later.
            </small>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="showCreate = false">Cancel</button>
          <button class="btn btn-primary" :disabled="creating || !newName.trim()" @click="createWidget">
            {{ creating ? "Creating..." : "Create widget" }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
