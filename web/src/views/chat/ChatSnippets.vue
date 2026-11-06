<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { chatSnippetsApi } from "../../api/chat";
import type { ChatSnippet } from "../../api/types";
import { useNotificationStore } from "../../stores/notification";
import { useWorkspaceStore } from "../../stores/workspace";
import { apiMessage } from "../../composables/apiError";
import { useConfirm } from "../../composables/useConfirm";

const notify = useNotificationStore();
const wsStore = useWorkspaceStore();
const { confirm } = useConfirm();

const loading = ref(true);
const featureDisabled = ref(false);
const snippets = ref<ChatSnippet[]>([]);
const search = ref("");

const showDialog = ref(false);
const saving = ref(false);
const editing = ref<ChatSnippet | null>(null);
const form = ref({ shortcut: "", title: "", body: "" });

const filtered = computed(() => {
  const q = search.value.trim().toLowerCase();
  if (!q) return snippets.value;
  return snippets.value.filter(
    (s) => s.shortcut.includes(q) || s.title.toLowerCase().includes(q) || s.body.toLowerCase().includes(q),
  );
});

async function load() {
  loading.value = true;
  try {
    snippets.value = (await chatSnippetsApi.list()).data.data;
    featureDisabled.value = false;
  } catch (e: any) {
    if (e?.response?.status === 404) featureDisabled.value = true;
    else notify.error(apiMessage(e, "Failed to load canned replies"));
  } finally {
    loading.value = false;
  }
}

function openCreate() {
  editing.value = null;
  form.value = { shortcut: "", title: "", body: "" };
  showDialog.value = true;
}

function openEdit(s: ChatSnippet) {
  editing.value = s;
  form.value = { shortcut: s.shortcut, title: s.title, body: s.body };
  showDialog.value = true;
}

/** Shortcuts are identifiers: lowercase letters, digits and hyphens. */
function normalizeShortcut() {
  form.value.shortcut = form.value.shortcut
    .replace(/^\/+/, "")
    .toLowerCase()
    .replace(/[^a-z0-9-]+/g, "-")
    .replace(/^-+/, "");
}

async function save() {
  normalizeShortcut();
  const payload = { ...form.value, shortcut: form.value.shortcut.replace(/-+$/, "") };
  if (!payload.shortcut || !payload.title.trim() || !payload.body.trim()) return;
  saving.value = true;
  try {
    if (editing.value) await chatSnippetsApi.update(editing.value.id, payload);
    else await chatSnippetsApi.create(payload);
    showDialog.value = false;
    notify.success(editing.value ? "Canned reply saved" : "Canned reply created");
    await load();
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to save canned reply"));
  } finally {
    saving.value = false;
  }
}

async function remove(s: ChatSnippet) {
  const sure = await confirm({
    title: "Delete canned reply",
    message: `Delete /${s.shortcut}? Agents can no longer insert it.`,
    confirmText: "Delete",
    variant: "danger",
  });
  if (!sure) return;
  try {
    await chatSnippetsApi.delete(s.id);
    snippets.value = snippets.value.filter((x) => x.id !== s.id);
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to delete canned reply"));
  }
}

onMounted(load);
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Canned replies</h1>
      <div style="display: flex; gap: 8px">
        <router-link to="/chat" class="btn btn-secondary">Conversations</router-link>
        <button v-if="wsStore.canEdit && !featureDisabled" class="btn btn-primary" @click="openCreate">New canned reply</button>
      </div>
    </div>

    <div v-if="loading" class="loading-page"><div class="spinner"></div></div>
    <div v-else-if="featureDisabled" class="card">
      <div class="empty-state">
        <h3>Live chat is disabled</h3>
        <p>Ask your administrator to enable it by setting <code>POSTA_CHAT_ENABLED=true</code>.</p>
      </div>
    </div>
    <div v-else class="card">
      <div v-if="snippets.length === 0" class="empty-state">
        <h3>No canned replies yet</h3>
        <p>Save answers you give often, then type <code>/</code> and the shortcut in a chat to insert them.</p>
        <button v-if="wsStore.canEdit" class="btn btn-primary" @click="openCreate">New canned reply</button>
      </div>
      <template v-else>
        <div class="card-body search-row">
          <input v-model="search" class="form-input" placeholder="Search canned replies" aria-label="Search canned replies" />
        </div>
        <div class="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>Shortcut</th>
                <th>Title</th>
                <th>Reply</th>
                <th v-if="wsStore.canEdit"><span class="sr-only">Actions</span></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="s in filtered" :key="s.id">
                <td><code>/{{ s.shortcut }}</code></td>
                <td>{{ s.title }}</td>
                <td class="body-cell">{{ s.body }}</td>
                <td v-if="wsStore.canEdit" class="actions">
                  <button class="btn btn-secondary btn-sm" @click="openEdit(s)">Edit</button>
                  <button class="btn btn-danger btn-sm" @click="remove(s)">Delete</button>
                </td>
              </tr>
              <tr v-if="filtered.length === 0">
                <td :colspan="wsStore.canEdit ? 4 : 3" class="muted">No canned reply matches.</td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>
    </div>

    <div v-if="showDialog" class="modal-overlay" @click.self="showDialog = false">
      <div class="modal">
        <div class="modal-header">
          <h3>{{ editing ? "Edit canned reply" : "New canned reply" }}</h3>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label class="form-label" for="snippet-shortcut">Shortcut</label>
            <div class="shortcut-input">
              <span aria-hidden="true">/</span>
              <input id="snippet-shortcut" v-model="form.shortcut" class="form-input" placeholder="shipping"
                maxlength="63" @blur="normalizeShortcut" />
            </div>
            <small class="muted">Type <code>/{{ form.shortcut || "shortcut" }}</code> in a chat to insert this reply.</small>
          </div>
          <div class="form-group">
            <label class="form-label" for="snippet-title">Title</label>
            <input id="snippet-title" v-model="form.title" class="form-input" maxlength="80" placeholder="Shipping times" />
          </div>
          <div class="form-group">
            <label class="form-label" for="snippet-body">Reply</label>
            <textarea id="snippet-body" v-model="form.body" class="form-input" rows="6"
              placeholder="We ship within 2 business days…"></textarea>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="showDialog = false">Cancel</button>
          <button class="btn btn-primary" :disabled="saving || !form.shortcut || !form.title.trim() || !form.body.trim()"
            @click="save">{{ saving ? "Saving…" : "Save" }}</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.search-row {
  padding-bottom: 0;
}
.body-cell {
  white-space: pre-wrap;
  max-width: 480px;
  color: var(--text-secondary);
}
.actions {
  white-space: nowrap;
  text-align: right;
}
.actions .btn + .btn {
  margin-left: 6px;
}
.shortcut-input {
  display: flex;
  align-items: center;
  gap: 6px;
}
.shortcut-input span {
  color: var(--text-muted);
  font-family: monospace;
}
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
}
</style>
