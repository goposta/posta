<script setup lang="ts">
import { ref } from "vue";
import { IDENTIFIER_HINT, toIdentifier } from "../../composables/identifier";
import { useRouter } from "vue-router";
import { formsApi } from "../../api/messages";
import { useNotificationStore } from "../../stores/notification";
import { apiMessage } from "../../composables/apiError";
import type { Form, FormMode } from "../../api/types";
import Pagination from "../../components/Pagination.vue";
import { usePagination } from "../../composables/usePagination";

const router = useRouter();
const notify = useNotificationStore();
const loading = ref(true);
const featureDisabled = ref(false);
const forms = ref<Form[]>([]);

const showCreate = ref(false);
const creating = ref(false);
const newDisplayName = ref("");
const newName = ref("");
// The name follows the display name until it is edited by hand.
const nameEdited = ref(false);
const newOrigins = ref("");
const newNotify = ref("");
const newMode = ref<FormMode>("inbox");

const { pageable, goToPage } = usePagination(async (page) => {
  loading.value = true;
  try {
    const res = await formsApi.list({ page });
    forms.value = res.data.data;
    pageable.value = res.data.pageable;
    featureDisabled.value = false;
  } catch (e: any) {
    if (e?.response?.status === 404) {
      featureDisabled.value = true;
    } else {
      notify.error(apiMessage(e, "Failed to load forms"));
    }
  } finally {
    loading.value = false;
  }
});

function syncName() {
  if (!nameEdited.value) newName.value = toIdentifier(newDisplayName.value);
}

function openCreate() {
  newDisplayName.value = "";
  newName.value = "";
  nameEdited.value = false;
  newOrigins.value = "";
  newNotify.value = "";
  newMode.value = "inbox";
  showCreate.value = true;
}

async function createForm() {
  if (!newDisplayName.value.trim()) return;
  creating.value = true;
  try {
    const res = await formsApi.create({
      display_name: newDisplayName.value.trim(),
      name: newName.value.trim() || undefined,
      mode: newMode.value,
      allowed_origins: splitList(newOrigins.value),
      notify_emails: splitList(newNotify.value),
    });
    showCreate.value = false;
    router.push(`/forms/${res.data.data.id}`);
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to create form"));
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
      <h1>Forms</h1>
      <div style="display: flex; gap: 8px">
        <router-link to="/messages" class="btn btn-secondary">Messages</router-link>
        <button class="btn btn-primary" @click="openCreate">New form</button>
      </div>
    </div>

    <div v-if="loading" class="loading-page">
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

    <div v-else class="card">
      <div v-if="forms.length === 0" class="empty-state">
        <h3>No forms yet</h3>
        <p>Create a form to get an endpoint you can post your website's contact form to.</p>
        <button class="btn btn-primary" @click="openCreate">New form</button>
      </div>

      <template v-else>
        <div class="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Public key</th>
                <th>Status</th>
                <th>Kind</th>
                <th>Submissions</th>
                <th>Spam</th>
                <th>Last submission</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="form in forms"
                :key="form.id"
                style="cursor: pointer"
                @click="router.push(`/forms/${form.id}`)"
              >
                <td>{{ form.display_name }}</td>
                <td><code style="font-size: 12px">{{ form.public_key }}</code></td>
                <td><span :class="statusBadgeClass(form.status)">{{ form.status }}</span></td>
                <td>{{ form.mode === "collect" ? "Responses" : "Messages" }}</td>
                <td>{{ form.message_count }}</td>
                <td>{{ form.spam_count }}</td>
                <td>{{ formatDate(form.last_message_at) }}</td>
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
          <h3>New form</h3>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label class="form-label">Display name</label>
            <input v-model="newDisplayName" class="form-input" placeholder="Contact form" @input="syncName" />
          </div>
          <div class="form-group">
            <label class="form-label">Name</label>
            <input v-model="newName" class="form-input mono" placeholder="contact-form" :title="IDENTIFIER_HINT"
              @input="nameEdited = true" />
            <span class="form-hint">Identifies the form in the API. {{ IDENTIFIER_HINT }}</span>
          </div>
          <div class="form-group">
            <label class="form-label">Submissions are</label>
            <select v-model="newMode" class="form-select">
              <option value="inbox">Messages to answer (contact, support)</option>
              <option value="collect">Responses to collect (surveys, signups, RSVPs)</option>
            </select>
            <small v-if="newMode === 'collect'" class="muted">
              Responses stay out of the inbox, and notifications default to a daily digest.
            </small>
          </div>
          <div class="form-group">
            <label class="form-label">Allowed origins</label>
            <input
              v-model="newOrigins"
              class="form-input"
              placeholder="https://example.com, https://www.example.com"
            />
            <small class="muted">
              Leave blank to accept any origin. Browsers only enforce this for scripted
              requests, so treat it as a spam speed bump rather than authentication.
            </small>
          </div>
          <div class="form-group">
            <label class="form-label">Notification recipients</label>
            <input
              v-model="newNotify"
              class="form-input"
              placeholder="team@example.com"
            />
            <small class="muted">Leave blank to notify workspace owners and admins.</small>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="showCreate = false">Cancel</button>
          <button class="btn btn-primary" :disabled="creating || !newDisplayName.trim()" @click="createForm">
            {{ creating ? "Creating..." : "Create form" }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
