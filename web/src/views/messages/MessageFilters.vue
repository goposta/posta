<script setup lang="ts">
import { ref } from "vue";
import { formsApi, messageFiltersApi, type FilterTestResult } from "../../api/messages";
import { useNotificationStore } from "../../stores/notification";
import { apiMessage } from "../../composables/apiError";
import type { Form, MessageFilterRule } from "../../api/types";

const notify = useNotificationStore();
const loading = ref(true);
const featureDisabled = ref(false);
const filters = ref<MessageFilterRule[]>([]);
const forms = ref<Form[]>([]);
const saving = ref(false);

const showModal = ref(false);
const draft = ref({
  form_id: 0,
  kind: "keyword",
  pattern: "",
  action: "score",
  score: 3,
  case_sensitive: false,
  note: "",
});

const testing = ref(false);
const testResult = ref<FilterTestResult | null>(null);

async function load() {
  loading.value = true;
  try {
    const [filtersRes, formsRes] = await Promise.all([
      messageFiltersApi.list(),
      formsApi.list({ size: 100 }),
    ]);
    filters.value = filtersRes.data.data;
    forms.value = formsRes.data.data;
    featureDisabled.value = false;
  } catch (e: any) {
    if (e?.response?.status === 404) {
      featureDisabled.value = true;
    } else {
      notify.error(apiMessage(e, "Failed to load filters"));
    }
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  draft.value = {
    form_id: 0,
    kind: "keyword",
    pattern: "",
    action: "score",
    score: 3,
    case_sensitive: false,
    note: "",
  };
  testResult.value = null;
  showModal.value = true;
}

async function create() {
  if (!draft.value.pattern.trim()) return;
  saving.value = true;
  try {
    await messageFiltersApi.create({
      form_id: draft.value.form_id || null,
      kind: draft.value.kind,
      pattern: draft.value.pattern.trim(),
      action: draft.value.action,
      score: draft.value.score,
      case_sensitive: draft.value.case_sensitive,
      note: draft.value.note,
    });
    showModal.value = false;
    notify.success("Filter created.");
    await load();
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to create filter"));
  } finally {
    saving.value = false;
  }
}

async function runTest() {
  if (!draft.value.pattern.trim()) return;
  testing.value = true;
  testResult.value = null;
  try {
    const res = await messageFiltersApi.test({
      kind: draft.value.kind,
      pattern: draft.value.pattern.trim(),
      case_sensitive: draft.value.case_sensitive,
    });
    testResult.value = res.data.data;
  } catch (e: any) {
    notify.error(apiMessage(e, "Test failed"));
  } finally {
    testing.value = false;
  }
}

async function toggle(filter: MessageFilterRule) {
  try {
    const res = await messageFiltersApi.update(filter.id, { enabled: !filter.enabled });
    Object.assign(filter, res.data.data);
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to update filter"));
  }
}

async function remove(filter: MessageFilterRule) {
  if (!confirm(`Delete the filter "${filter.pattern}"?`)) return;
  try {
    await messageFiltersApi.delete(filter.id);
    filters.value = filters.value.filter((f) => f.id !== filter.id);
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to delete filter"));
  }
}

function formName(formId: number | null | undefined) {
  if (!formId) return "All forms";
  return forms.value.find((f) => f.id === formId)?.name || `Form ${formId}`;
}

function actionBadgeClass(action: string) {
  switch (action) {
    case "reject":
      return "badge badge-danger";
    case "quarantine":
      return "badge badge-danger";
    case "flag":
      return "badge badge-warning";
    case "allowlist":
      return "badge badge-success";
    default:
      return "badge badge-secondary";
  }
}

function formatDate(date: string | null | undefined) {
  if (!date) return "never";
  return new Date(date).toLocaleString();
}

load();
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Spam Filters</h1>
      <div style="display: flex; gap: 8px">
        <router-link to="/messages" class="btn btn-secondary">Messages</router-link>
        <button class="btn btn-primary" @click="openCreate">New filter</button>
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
      <div v-if="filters.length === 0" class="empty-state">
        <h3>No filters</h3>
        <p>
          Built-in signals still run. Add a filter to score, quarantine, or reject
          submissions containing specific words, senders, or domains.
        </p>
        <button class="btn btn-primary" @click="openCreate">New filter</button>
      </div>

      <div v-else class="table-wrapper">
        <table>
          <thead>
            <tr>
              <th>Pattern</th>
              <th>Kind</th>
              <th>Action</th>
              <th>Score</th>
              <th>Scope</th>
              <th>Hits</th>
              <th>Last hit</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="filter in filters" :key="filter.id">
              <td>
                <code style="font-size: 12px">{{ filter.pattern }}</code>
                <div v-if="filter.note" class="muted" style="font-size: 12px">{{ filter.note }}</div>
              </td>
              <td>{{ filter.kind }}</td>
              <td><span :class="actionBadgeClass(filter.action)">{{ filter.action }}</span></td>
              <td>{{ filter.action === "score" ? filter.score : "-" }}</td>
              <td>{{ formName(filter.form_id) }}</td>
              <td>{{ filter.hit_count }}</td>
              <td>{{ formatDate(filter.last_hit_at) }}</td>
              <td style="text-align: right; white-space: nowrap">
                <button class="btn btn-secondary" @click="toggle(filter)">
                  {{ filter.enabled ? "Disable" : "Enable" }}
                </button>
                <button class="btn btn-danger" style="margin-left: 6px" @click="remove(filter)">
                  Delete
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div v-if="showModal" class="modal-overlay" @click.self="showModal = false">
      <div class="modal">
        <div class="modal-header">
          <h3>New filter</h3>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label class="form-label">Applies to</label>
            <select v-model.number="draft.form_id" class="form-select">
              <option :value="0">All forms</option>
              <option v-for="f in forms" :key="f.id" :value="f.id">{{ f.name }}</option>
            </select>
          </div>
          <div class="form-group">
            <label class="form-label">Kind</label>
            <select v-model="draft.kind" class="form-select">
              <option value="keyword">Keyword (whole word)</option>
              <option value="phrase">Phrase (substring)</option>
              <option value="regex">Regular expression</option>
              <option value="email">Sender email</option>
              <option value="domain">Sender domain</option>
              <option value="ip">Client IP</option>
            </select>
          </div>
          <div class="form-group">
            <label class="form-label">Pattern</label>
            <input v-model="draft.pattern" class="form-input" placeholder="casino" />
          </div>
          <div class="form-group">
            <label class="form-label">Action</label>
            <select v-model="draft.action" class="form-select">
              <option value="score">Add to spam score</option>
              <option value="flag">Flag</option>
              <option value="quarantine">Quarantine</option>
              <option value="reject">Reject</option>
              <option value="allowlist">Allowlist (skip scanning)</option>
            </select>
          </div>
          <div v-if="draft.action === 'score'" class="form-group">
            <label class="form-label">Score</label>
            <input v-model.number="draft.score" type="number" step="0.5" class="form-input" />
          </div>
          <div class="form-group">
            <label class="checkbox-label">
              <input v-model="draft.case_sensitive" type="checkbox" />
              Case sensitive
            </label>
          </div>
          <div class="form-group">
            <label class="form-label">Note</label>
            <input v-model="draft.note" class="form-input" placeholder="Why this rule exists" />
          </div>

          <div class="form-group">
            <button class="btn btn-secondary" :disabled="testing || !draft.pattern.trim()" @click="runTest">
              {{ testing ? "Testing..." : "Test against recent messages" }}
            </button>
          </div>
          <div v-if="testResult" class="alert alert-info">
            Matched {{ testResult.matched }} of {{ testResult.scanned }} recent messages.
            <ul v-if="testResult.samples.length" style="margin: 8px 0 0; padding-left: 18px">
              <li v-for="sample in testResult.samples" :key="sample.message_uuid">
                {{ sample.sender_email || "unknown" }} — {{ sample.subject || "(no subject)" }}
              </li>
            </ul>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="showModal = false">Cancel</button>
          <button class="btn btn-primary" :disabled="saving || !draft.pattern.trim()" @click="create">
            {{ saving ? "Creating..." : "Create filter" }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
