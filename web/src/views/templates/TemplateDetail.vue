<script setup lang="ts">
import { ref, onMounted, computed } from "vue";
import { useRoute, useRouter } from "vue-router";
import { templatesApi } from "../../api/templates";
import { stylesheetsApi } from "../../api/stylesheets";
import { languagesApi } from "../../api/languages";
import type {
  Template,
  TemplateVersion,
  TemplateLocalization,
  TemplateLocalizationInput,
  TemplatePreview,
  StyleSheet,
  Language,
} from "../../api/types";
import { useNotificationStore } from "../../stores/notification";
import { useConfirm } from "../../composables/useConfirm";
import { useModalSafeClose } from "../../composables/useModalSafeClose";

const route = useRoute();
const router = useRouter();
const notify = useNotificationStore();
const { confirm } = useConfirm();

const templateId = Number(route.params.id);

const template = ref<Template | null>(null);
const versions = ref<TemplateVersion[]>([]);
const localizations = ref<TemplateLocalization[]>([]);
const stylesheets = ref<StyleSheet[]>([]);
const languages = ref<Language[]>([]);
const loading = ref(true);
const selectedVersion = ref<TemplateVersion | null>(null);

// Localization modal
const showLocModal = ref(false);
const editingLoc = ref<TemplateLocalization | null>(null);
const savingLoc = ref(false);
const locForm = ref<TemplateLocalizationInput>({
  language: "",
  subject_template: "",
  html_template: "",
  text_template: "",
});

// Preview
const showPreview = ref(false);
const previewLang = ref("");
const previewData = ref('{\n  "name": "John",\n  "company": "Acme"\n}');
const preview = ref<TemplatePreview | null>(null);
const previewLoading = ref(false);
const previewError = ref("");

// Send test
const showSendTest = ref(false);
const sendingTest = ref(false);
const sendTestForm = ref({
  to: "",
  from: "",
  language: "",
  data: '{\n  "name": "John",\n  "company": "Acme"\n}',
});

// Version creation
const creatingVersion = ref(false);
const newVersionStylesheetId = ref<number | null>(null);

// Version stylesheet edit
const showVersionStylesheetModal = ref(false);
const editingVersion = ref<TemplateVersion | null>(null);
const editVersionStylesheetId = ref<number | null>(null);
const savingVersionStylesheet = ref(false);

const isActive = computed(() => (v: TemplateVersion) =>
  template.value?.active_version_id === v.id
);

async function loadAll() {
  loading.value = true;
  try {
    const [tmplRes, versionsRes, ssRes, langRes] = await Promise.all([
      templatesApi.list(0, 100),
      templatesApi.listVersions(templateId),
      stylesheetsApi.list(0, 100),
      languagesApi.list(0, 100),
    ]);
    template.value = tmplRes.data.data.find((t: Template) => t.id === templateId) || null;
    versions.value = versionsRes.data.data || [];
    stylesheets.value = ssRes.data.data || [];
    languages.value = langRes.data.data || [];

    if (!template.value) {
      notify.error("Template not found");
      return;
    }

    // Auto-select the active version or the first one
    if (versions.value.length > 0) {
      const active = versions.value.find(
        (v) => v.id === template.value?.active_version_id
      );
      selectVersion(active || versions.value[0]);
    }
  } catch {
    notify.error("Failed to load template");
  } finally {
    loading.value = false;
  }
}

function selectVersion(v: TemplateVersion) {
  selectedVersion.value = v;
  localizations.value = v.localizations || [];
}

function syncLocalizationsToVersion() {
  if (!selectedVersion.value) return;
  selectedVersion.value.localizations = [...localizations.value];
  const idx = versions.value.findIndex((v) => v.id === selectedVersion.value!.id);
  if (idx >= 0) versions.value[idx].localizations = [...localizations.value];
}

async function createVersion() {
  creatingVersion.value = true;
  try {
    const res = await templatesApi.createVersion(templateId, {
      stylesheet_id: newVersionStylesheetId.value,
      sample_data: template.value?.sample_data || "",
    });
    versions.value.unshift(res.data.data);
    selectVersion(res.data.data);
    notify.success(`Version ${res.data.data.version} created`);
    newVersionStylesheetId.value = null;
  } catch {
    notify.error("Failed to create version");
  } finally {
    creatingVersion.value = false;
  }
}

function openEditVersionStylesheet(v: TemplateVersion) {
  editingVersion.value = v;
  editVersionStylesheetId.value = v.stylesheet_id ?? null;
  showVersionStylesheetModal.value = true;
}

async function saveVersionStylesheet() {
  if (!editingVersion.value) return;
  savingVersionStylesheet.value = true;
  try {
    const res = await templatesApi.updateVersion(templateId, editingVersion.value.id, {
      stylesheet_id: editVersionStylesheetId.value,
    });
    const idx = versions.value.findIndex((v) => v.id === editingVersion.value!.id);
    if (idx >= 0) versions.value[idx] = res.data.data;
    if (selectedVersion.value?.id === editingVersion.value.id) {
      selectedVersion.value = res.data.data;
    }
    notify.success("Version stylesheet updated");
    showVersionStylesheetModal.value = false;
  } catch {
    notify.error("Failed to update version stylesheet");
  } finally {
    savingVersionStylesheet.value = false;
  }
}

async function activateVersion(v: TemplateVersion) {
  try {
    const res = await templatesApi.activateVersion(templateId, v.id);
    template.value = res.data.data;
    notify.success(`Version ${v.version} activated`);
  } catch {
    notify.error("Failed to activate version");
  }
}

async function deleteVersion(v: TemplateVersion) {
  if (template.value?.active_version_id === v.id) {
    notify.error("Cannot delete the active version");
    return;
  }
  const confirmed = await confirm({
    title: "Delete Version",
    message: `Delete version ${v.version}? All its localizations will also be deleted.`,
    confirmText: "Delete",
    variant: "danger",
  });
  if (!confirmed) return;
  try {
    await templatesApi.deleteVersion(templateId, v.id);
    versions.value = versions.value.filter((x) => x.id !== v.id);
    if (selectedVersion.value?.id === v.id) {
      selectedVersion.value = versions.value[0] || null;
      if (selectedVersion.value) selectVersion(selectedVersion.value);
      else localizations.value = [];
    }
    notify.success("Version deleted");
  } catch {
    notify.error("Failed to delete version");
  }
}

function openCreateLoc() {
  editingLoc.value = null;
  locForm.value = {
    language: "",
    subject_template: "",
    html_template: "",
    text_template: "",
  };
  showLocModal.value = true;
}

function openEditLoc(l: TemplateLocalization) {
  editingLoc.value = l;
  locForm.value = {
    language: l.language,
    subject_template: l.subject_template,
    html_template: l.html_template,
    text_template: l.text_template,
  };
  showLocModal.value = true;
}

async function saveLoc() {
  if (!selectedVersion.value) return;
  savingLoc.value = true;
  try {
    if (editingLoc.value) {
      const res = await templatesApi.updateLocalization(editingLoc.value.id, {
        subject_template: locForm.value.subject_template,
        html_template: locForm.value.html_template,
        text_template: locForm.value.text_template,
      });
      const idx = localizations.value.findIndex((l) => l.id === editingLoc.value!.id);
      if (idx >= 0) localizations.value[idx] = res.data.data;
      notify.success("Localization updated");
    } else {
      const res = await templatesApi.createLocalization(
        templateId,
        selectedVersion.value.id,
        locForm.value
      );
      localizations.value.push(res.data.data);
      notify.success("Localization created");
    }
    syncLocalizationsToVersion();
    showLocModal.value = false;
  } catch {
    notify.error(
      editingLoc.value
        ? "Failed to update localization"
        : "Language already exists for this version"
    );
  } finally {
    savingLoc.value = false;
  }
}

async function deleteLoc(l: TemplateLocalization) {
  const confirmed = await confirm({
    title: "Delete Localization",
    message: `Delete the "${l.language}" localization? This cannot be undone.`,
    confirmText: "Delete",
    variant: "danger",
  });
  if (!confirmed) return;
  try {
    await templatesApi.deleteLocalization(l.id);
    localizations.value = localizations.value.filter((x) => x.id !== l.id);
    syncLocalizationsToVersion();
    notify.success("Localization deleted");
  } catch {
    notify.error("Failed to delete localization");
  }
}

async function renderPreview() {
  if (!selectedVersion.value || !previewLang.value) return;
  previewLoading.value = true;
  previewError.value = "";
  preview.value = null;

  let data: Record<string, any> = {};
  try {
    data = JSON.parse(previewData.value);
  } catch {
    previewError.value = "Invalid JSON in sample data";
    previewLoading.value = false;
    return;
  }

  try {
    const res = await templatesApi.previewLocalization(
      templateId,
      selectedVersion.value.id,
      {
        language: previewLang.value,
        template_data: data,
      }
    );
    preview.value = res.data.data;
  } catch (e: any) {
    previewError.value = e.response?.data?.error?.message || "Failed to render preview";
  } finally {
    previewLoading.value = false;
  }
}

function openSendTest() {
  sendTestForm.value = {
    to: "",
    from: "",
    language: template.value?.default_language || "en",
    data:
      selectedVersion.value?.sample_data ||
      '{\n  "name": "John",\n  "company": "Acme"\n}',
  };
  showSendTest.value = true;
}

async function sendTest() {
  if (!sendTestForm.value.to.trim()) return;
  sendingTest.value = true;

  let data: Record<string, any> = {};
  try {
    data = JSON.parse(sendTestForm.value.data);
  } catch {
    notify.error("Invalid JSON in sample data");
    sendingTest.value = false;
    return;
  }

  try {
    const to = sendTestForm.value.to
      .split(",")
      .map((e) => e.trim())
      .filter(Boolean);
    await templatesApi.sendTest(templateId, {
      to,
      from: sendTestForm.value.from || undefined,
      language: sendTestForm.value.language || undefined,
      template_data: data,
    });
    notify.success("Test email sent");
    showSendTest.value = false;
  } catch (e: any) {
    notify.error(e.response?.data?.error?.message || "Failed to send test email");
  } finally {
    sendingTest.value = false;
  }
}

function openPreview(lang: string) {
  previewLang.value = lang;
  preview.value = null;
  previewError.value = "";
  if (template.value?.sample_data) {
    previewData.value = template.value.sample_data;
  } else if (selectedVersion.value?.sample_data) {
    previewData.value = selectedVersion.value.sample_data;
  }
  showPreview.value = true;
  renderPreview();
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

const closeActiveModal = () => {
  if (showVersionStylesheetModal.value) showVersionStylesheetModal.value = false;
  if (showPreview.value) showPreview.value = false;
  if (showSendTest.value) showSendTest.value = false;
  if (showLocModal.value) showLocModal.value = false;
};

const { watchClickStart, confirmClickEnd } = useModalSafeClose(() => {
  closeActiveModal();
});
onMounted(loadAll);
</script>

<template>
  <div>
    <div class="page-header">
      <h1>{{ template?.name || "Template" }}</h1>
      <div class="flex gap-2">
        <button
          class="btn btn-primary"
          @click="openSendTest"
          :disabled="!template?.active_version_id"
        >
          Send Test
        </button>
        <button class="btn btn-secondary" @click="router.push('/templates')">
          Back to Templates
        </button>
      </div>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <template v-else-if="template">
      <!-- Template Info -->
      <div class="card" style="margin-bottom: 24px">
        <div class="card-body">
          <table>
            <tbody>
              <tr>
                <td class="info-label">Name</td>
                <td>{{ template.name }}</td>
              </tr>
              <tr v-if="template.description">
                <td class="info-label">Description</td>
                <td>{{ template.description }}</td>
              </tr>
              <tr>
                <td class="info-label">Default Language</td>
                <td>
                  <span class="badge badge-info">{{ template.default_language }}</span>
                </td>
              </tr>
              <tr>
                <td class="info-label">Active Version</td>
                <td>
                  <span v-if="template.active_version_id" class="badge badge-success">
                    v{{ versions.find(v => v.id === template!.active_version_id)?.version || '?' }}
                  </span>
                  <span v-else class="text-muted">None</span>
                </td>
              </tr>
              <tr>
                <td class="info-label">Created</td>
                <td>{{ formatDate(template.created_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- Versions -->
      <div class="card" style="margin-bottom: 24px">
        <div class="card-header">
          <h2>Versions</h2>
          <div class="flex gap-2 align-center">
            <select v-model="newVersionStylesheetId" class="form-select form-select-sm">
              <option :value="null">No stylesheet</option>
              <option v-for="ss in stylesheets" :key="ss.id" :value="ss.id">
                {{ ss.name }}
              </option>
            </select>
            <button
              class="btn btn-primary btn-sm"
              @click="createVersion"
              :disabled="creatingVersion"
            >
              {{ creatingVersion ? "Creating..." : "New Version" }}
            </button>
          </div>
        </div>

        <div v-if="versions.length === 0" class="empty-state">
          <p>No versions yet. Create one to start adding localizations.</p>
        </div>

        <div v-else class="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>Version</th>
                <th>Stylesheet</th>
                <th>Created</th>
                <th>Status</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="v in versions"
                :key="v.id"
                :class="{ 'row-selected': selectedVersion?.id === v.id }"
                style="cursor: pointer"
                @click="selectVersion(v)"
              >
                <td>
                  <strong>v{{ v.version }}</strong>
                </td>
                <td>
                  <span v-if="v.stylesheet" class="badge badge-neutral">{{
                    v.stylesheet.name
                  }}</span>
                  <span v-else class="text-muted">&mdash;</span>
                </td>
                <td>{{ formatDate(v.created_at) }}</td>
                <td>
                  <span v-if="isActive(v)" class="badge badge-success">Active</span>
                  <span v-else class="badge badge-neutral">Draft</span>
                </td>
                <td>
                  <div class="flex gap-2" @click.stop>
                    <button
                      class="btn btn-secondary btn-sm"
                      @click="openEditVersionStylesheet(v)"
                    >
                      Edit
                    </button>
                    <button
                      v-if="!isActive(v)"
                      class="btn btn-primary btn-sm"
                      @click="activateVersion(v)"
                    >
                      Activate
                    </button>
                    <button
                      v-if="!isActive(v)"
                      class="btn btn-danger btn-sm"
                      @click="deleteVersion(v)"
                    >
                      Delete
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- Localizations for selected version -->
      <div v-if="selectedVersion" class="card">
        <div class="card-header">
          <h2>Localizations &mdash; v{{ selectedVersion.version }}</h2>
          <button class="btn btn-primary btn-sm" @click="openCreateLoc">
            Add Language
          </button>
        </div>

        <div v-if="localizations.length === 0" class="empty-state">
          <p>No localizations yet. Add a language to start defining content.</p>
        </div>

        <div v-else class="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>Language</th>
                <th>Subject</th>
                <th>Updated</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="l in localizations" :key="l.id">
                <td>
                  <strong>{{ l.language }}</strong>
                  <span
                    v-if="l.language === template.default_language"
                    class="badge badge-info"
                    style="margin-left: 6px"
                    >default</span
                  >
                </td>
                <td class="truncate" style="max-width: 300px">
                  {{ l.subject_template }}
                </td>
                <td>
                  <span v-if="l.updated_at">{{ formatDate(l.updated_at) }}</span>
                  <span v-else>{{ formatDate(l.created_at) }}</span>
                </td>
                <td>
                  <div class="flex gap-2">
                    <button
                      class="btn btn-secondary btn-sm"
                      @click="openPreview(l.language)"
                    >
                      Preview
                    </button>
                    <button
                      class="btn btn-secondary btn-sm"
                      @click="router.push(`/templates/${templateId}/versions/${selectedVersion?.id}/localizations/${l.id}/edit`)"
                    >
                      Editor
                    </button>
                    <button class="btn btn-secondary btn-sm" @click="openEditLoc(l)">
                      Quick Edit
                    </button>
                    <button class="btn btn-danger btn-sm" @click="deleteLoc(l)">
                      Delete
                    </button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </template>

    <div v-else class="empty-state">
      <h3>Template not found</h3>
    </div>

    <!-- Localization Create/Edit Modal -->
    <div
      v-if="showLocModal"
      class="modal-overlay"
      @mousedown="watchClickStart"
      @mouseup="confirmClickEnd"
    >
      <div class="modal" style="max-width: 720px" @mousedown.stop @mouseup.stop>
        <div class="modal-header">
          <h3>
            {{
              editingLoc
                ? `Edit Localization (${editingLoc.language})`
                : "Add Localization"
            }}
          </h3>
        </div>
        <div class="modal-body">
          <div v-if="!editingLoc" class="form-group">
            <label class="form-label">Language</label>
            <select v-model="locForm.language" class="form-select">
              <option value="" disabled>Select a language</option>
              <option v-for="lang in languages" :key="lang.id" :value="lang.code">
                {{ lang.name }} ({{ lang.code }})
              </option>
            </select>
            <span class="form-hint">Choose from your configured languages</span>
          </div>
          <div class="form-group">
            <label class="form-label">Subject Template</label>
            <input
              v-model="locForm.subject_template"
              class="form-input"
              placeholder="e.g. Welcome {{name}}!"
            />
          </div>
          <div class="form-group">
            <label class="form-label">HTML Template</label>
            <textarea
              v-model="locForm.html_template"
              class="form-textarea"
              rows="8"
              placeholder="<html>...</html>"
            ></textarea>
          </div>
          <div class="form-group">
            <label class="form-label">Text Template</label>
            <textarea
              v-model="locForm.text_template"
              class="form-textarea"
              rows="4"
              placeholder="Plain text version..."
            ></textarea>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="showLocModal = false">Cancel</button>
          <button
            class="btn btn-primary"
            :disabled="
              savingLoc ||
              (!editingLoc && !locForm.language.trim()) ||
              !locForm.subject_template.trim()
            "
            @click="saveLoc"
          >
            {{ savingLoc ? "Saving..." : editingLoc ? "Update" : "Create" }}
          </button>
        </div>
      </div>
    </div>

    <!-- Send Test Modal -->
    <div
      v-if="showSendTest"
      class="modal-overlay"
      @mousedown="watchClickStart"
      @mouseup="confirmClickEnd"
    >
      <div class="modal" style="max-width: 560px" @mousedown.stop @mouseup.stop>
        <div class="modal-header">
          <h3>Send Test Email</h3>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label class="form-label">To</label>
            <input
              v-model="sendTestForm.to"
              class="form-input"
              placeholder="test@example.com"
            />
            <span class="form-hint">Comma-separated for multiple recipients</span>
          </div>
          <div class="form-group">
            <label class="form-label">From (optional)</label>
            <input
              v-model="sendTestForm.from"
              class="form-input"
              placeholder="noreply@localhost"
            />
          </div>
          <div class="form-group">
            <label class="form-label">Language</label>
            <select v-model="sendTestForm.language" class="form-select">
              <option v-for="l in localizations" :key="l.language" :value="l.language">
                {{ l.language }}
              </option>
            </select>
          </div>
          <div class="form-group">
            <label class="form-label">Sample Data (JSON)</label>
            <textarea
              v-model="sendTestForm.data"
              class="form-textarea"
              rows="4"
              placeholder='{"name": "John"}'
            ></textarea>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="showSendTest = false">Cancel</button>
          <button
            class="btn btn-primary"
            :disabled="sendingTest || !sendTestForm.to.trim()"
            @click="sendTest"
          >
            {{ sendingTest ? "Sending..." : "Send Test" }}
          </button>
        </div>
      </div>
    </div>

    <!-- Preview Modal -->
    <div
      v-if="showPreview"
      class="modal-overlay"
      @mousedown="watchClickStart"
      @mouseup="confirmClickEnd"
    >
      <div class="modal" style="max-width: 800px" @mousedown.stop @mouseup.stop>
        <div class="modal-header">
          <h3>Preview &mdash; {{ previewLang }}</h3>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label class="form-label">Sample Data (JSON)</label>
            <textarea
              v-model="previewData"
              class="form-textarea"
              rows="3"
              placeholder='{"name": "John"}'
            ></textarea>
          </div>
          <button
            class="btn btn-secondary btn-sm"
            style="margin-bottom: 16px"
            @click="renderPreview"
            :disabled="previewLoading"
          >
            {{ previewLoading ? "Rendering..." : "Refresh Preview" }}
          </button>

          <div v-if="previewError" class="preview-error">{{ previewError }}</div>

          <template v-if="preview">
            <div class="preview-section">
              <div class="preview-label">Subject</div>
              <div class="preview-content">{{ preview.subject }}</div>
            </div>
            <div v-if="preview.html" class="preview-section">
              <div class="preview-label">HTML</div>
              <div class="preview-html">
                <iframe :srcdoc="preview.html" sandbox="" class="preview-iframe"></iframe>
              </div>
            </div>
            <div v-if="preview.text" class="preview-section">
              <div class="preview-label">Plain Text</div>
              <div class="preview-text">{{ preview.text }}</div>
            </div>
          </template>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="showPreview = false">Close</button>
        </div>
      </div>
    </div>
    <!-- Edit Version Stylesheet Modal -->
    <div
      v-if="showVersionStylesheetModal"
      class="modal-overlay"
      @mousedown="watchClickStart"
      @mouseup="confirmClickEnd"
    >
      <div class="modal" style="max-width: 420px" @mousedown.stop @mouseup.stop>
        <div class="modal-header">
          <h3>Edit Version v{{ editingVersion?.version }} Stylesheet</h3>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label class="form-label">Stylesheet</label>
            <select v-model="editVersionStylesheetId" class="form-select">
              <option :value="null">No stylesheet</option>
              <option v-for="ss in stylesheets" :key="ss.id" :value="ss.id">
                {{ ss.name }}
              </option>
            </select>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="showVersionStylesheetModal = false">
            Cancel
          </button>
          <button
            class="btn btn-primary"
            :disabled="savingVersionStylesheet"
            @click="saveVersionStylesheet"
          >
            {{ savingVersionStylesheet ? "Saving..." : "Save" }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.info-label {
  font-weight: 600;
  width: 140px;
  color: var(--text-secondary);
}

.row-selected {
  background: var(--bg-tertiary);
}

.align-center {
  align-items: center;
}

.form-select-sm {
  padding: 4px 8px;
  font-size: 13px;
  max-width: 160px;
}

.preview-section {
  margin-bottom: 16px;
}

.preview-label {
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--text-muted);
  margin-bottom: 6px;
}

.preview-content {
  padding: 10px 14px;
  background: var(--bg-tertiary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius);
  font-size: 14px;
  color: var(--text-primary);
}

.preview-html {
  border: 1px solid var(--border-primary);
  border-radius: var(--radius);
  overflow: hidden;
}

.preview-iframe {
  width: 100%;
  min-height: 300px;
  border: none;
  background: #fff;
}

.preview-text {
  padding: 10px 14px;
  background: var(--bg-tertiary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius);
  font-family: "JetBrains Mono", "Fira Code", monospace;
  font-size: 13px;
  color: var(--text-secondary);
  white-space: pre-wrap;
}

.preview-error {
  padding: 10px 14px;
  background: var(--danger-50);
  color: var(--danger-600);
  border-radius: var(--radius);
  font-size: 13px;
  margin-bottom: 16px;
}
</style>
