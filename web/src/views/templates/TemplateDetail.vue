<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { templatesApi } from "../../api/templates";
import { stylesheetsApi } from "../../api/stylesheets";
import { languagesApi } from "../../api/languages";
import type {
  Language,
  StyleSheet,
  Template,
  TemplateDraft,
  TemplateLocalization,
  TemplateLocalizationInput,
  TemplatePreview,
  TemplateVersion,
} from "../../api/types";
import { useNotificationStore } from "../../stores/notification";
import { useConfirm } from "../../composables/useConfirm";
import { useModalSafeClose } from "../../composables/useModalSafeClose";
import { useWorkspaceStore } from "../../stores/workspace";
import { apiMessage } from "../../composables/apiError";
import TemplateModal from "@/components/TemplateModal.vue";
import TemplateSummary from "./detail/TemplateSummary.vue";
import VersionList from "./detail/VersionList.vue";
import LocalizationList from "./detail/LocalizationList.vue";
import LocalizationModal from "./detail/LocalizationModal.vue";
import PreviewModal from "./detail/PreviewModal.vue";
import SendTestModal, { type SendTestForm } from "./detail/SendTestModal.vue";
import VersionStylesheetModal from "./detail/VersionStylesheetModal.vue";

const route = useRoute();
const router = useRouter();
const notify = useNotificationStore();
const wsStore = useWorkspaceStore();
const { confirm } = useConfirm();

const templateId = Number(route.params.id);

const template = ref<Template | null>(null);
const versions = ref<TemplateVersion[]>([]);
const stylesheets = ref<StyleSheet[]>([]);
const languages = ref<Language[]>([]);
const loading = ref(true);
const selectedVersion = ref<TemplateVersion | null>(null);

const localizations = computed(() => selectedVersion.value?.localizations ?? []);

// The languages the template actually sends: those on the active version, not
// on whichever draft happens to be selected.
const activeLanguages = computed(() => {
  const active = versions.value.find((v) => v.id === template.value?.active_version_id);
  return (active?.localizations ?? []).map((l) => l.language).sort();
});

const showLocModal = ref(false);
const editingLoc = ref<TemplateLocalization | null>(null);
const savingLoc = ref(false);
const locForm = ref<TemplateLocalizationInput>({
  language: "",
  subject_template: "",
  html_template: "",
  text_template: "",
});

const showPreview = ref(false);
const previewLang = ref("");
const previewData = ref("{}");
const preview = ref<TemplatePreview | null>(null);
const previewLoading = ref(false);
const previewError = ref("");

const showSendTest = ref(false);
const sendingTest = ref(false);
const sendTestForm = ref<SendTestForm>({ to: "", from: "", language: "", data: "{}" });

const creatingVersion = ref(false);
const newVersionStylesheetId = ref<number | null>(null);

const showVersionStylesheetModal = ref(false);
const editingVersion = ref<TemplateVersion | null>(null);
const editVersionStylesheetId = ref<number | null>(null);
const savingVersionStylesheet = ref(false);

const savingTemplate = ref(false);
const showTemplateEditModal = ref(false);
const templateForm = ref<TemplateDraft>({
  name: "",
  display_name: "",
  sample_data: "",
  default_language: "en",
  description: "",
});

async function loadAll() {
  loading.value = true;
  try {
    const [tmplRes, versionsRes, ssRes, langRes] = await Promise.all([
      templatesApi.get(templateId),
      templatesApi.listVersions(templateId),
      stylesheetsApi.list(0, 100),
      languagesApi.list(0, 100),
    ]);
    template.value = tmplRes.data.data || null;
    versions.value = versionsRes.data.data || [];
    stylesheets.value = ssRes.data.data || [];
    languages.value = langRes.data.data || [];

    if (versions.value.length) {
      const active = versions.value.find((v) => v.id === template.value?.active_version_id);
      selectedVersion.value = active || versions.value[0];
    } else {
      selectedVersion.value = null;
    }
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to load template"));
  } finally {
    loading.value = false;
  }
}

function selectVersion(v: TemplateVersion) {
  selectedVersion.value = v;
}

// Version objects are held in one place — the versions array — and the selection
// points into it, so a localization change is written once and both the table
// and the selected view reflect it.
function replaceVersion(v: TemplateVersion) {
  const idx = versions.value.findIndex((x) => x.id === v.id);
  if (idx >= 0) versions.value[idx] = v;
  if (selectedVersion.value?.id === v.id) selectedVersion.value = versions.value[idx] ?? v;
}

function updateLocalizations(mutate: (list: TemplateLocalization[]) => TemplateLocalization[]) {
  const v = selectedVersion.value;
  if (!v) return;
  replaceVersion({ ...v, localizations: mutate([...(v.localizations ?? [])]) });
}

async function createVersion(stylesheetId: number | null) {
  creatingVersion.value = true;
  try {
    const res = await templatesApi.createVersion(templateId, {
      stylesheet_id: stylesheetId,
      sample_data: template.value?.sample_data || "",
    });
    versions.value.unshift(res.data.data);
    selectedVersion.value = res.data.data;
    newVersionStylesheetId.value = null;
    notify.success(`Version ${res.data.data.version} created`);
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to create version"));
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
    // The update response omits localizations, so keep the ones already loaded
    // rather than blanking the content table.
    replaceVersion({ ...res.data.data, localizations: editingVersion.value.localizations });
    showVersionStylesheetModal.value = false;
    notify.success("Stylesheet updated");
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to update the stylesheet"));
  } finally {
    savingVersionStylesheet.value = false;
  }
}

async function activateVersion(v: TemplateVersion) {
  if (!v.localizations?.length) {
    const proceed = await confirm({
      title: `Activate v${v.version}?`,
      message:
        "This version has no content. Activating it means every send from this template will fail until a language is added.",
      confirmText: "Activate anyway",
      variant: "warning",
    });
    if (!proceed) return;
  }
  try {
    const res = await templatesApi.activateVersion(templateId, v.id);
    template.value = res.data.data;
    notify.success(`Version ${v.version} is now live`);
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to activate version"));
  }
}

async function deleteVersion(v: TemplateVersion) {
  const count = v.localizations?.length ?? 0;
  const confirmed = await confirm({
    title: `Delete v${v.version}`,
    message: count
      ? `Its ${count} language${count === 1 ? "" : "s"} of content will be deleted with it. This cannot be undone.`
      : "This cannot be undone.",
    confirmText: "Delete",
    variant: "danger",
  });
  if (!confirmed) return;
  try {
    await templatesApi.deleteVersion(templateId, v.id);
    versions.value = versions.value.filter((x) => x.id !== v.id);
    if (selectedVersion.value?.id === v.id) selectedVersion.value = versions.value[0] || null;
    notify.success("Version deleted");
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to delete version"));
  }
}

function openCreateLoc(language?: string) {
  editingLoc.value = null;
  locForm.value = {
    language: language || "",
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
      updateLocalizations((list) =>
        list.map((l) => (l.id === editingLoc.value!.id ? res.data.data : l))
      );
      notify.success("Content updated");
    } else {
      const res = await templatesApi.createLocalization(
        templateId,
        selectedVersion.value.id,
        locForm.value
      );
      updateLocalizations((list) => [...list, res.data.data]);
      notify.success(`${res.data.data.language} added`);
    }
    showLocModal.value = false;
  } catch (e: any) {
    notify.error(
      apiMessage(e, editingLoc.value ? "Failed to update the content" : "Failed to add the language")
    );
  } finally {
    savingLoc.value = false;
  }
}

async function deleteLoc(l: TemplateLocalization) {
  const isDefault = l.language === template.value?.default_language;
  const confirmed = await confirm({
    title: `Delete ${l.language}`,
    message: isDefault
      ? `${l.language} is this template's default language. Removing it means sends that do not name a language will fail.`
      : "This cannot be undone.",
    confirmText: "Delete",
    variant: "danger",
  });
  if (!confirmed) return;
  try {
    await templatesApi.deleteLocalization(l.id);
    updateLocalizations((list) => list.filter((x) => x.id !== l.id));
    notify.success(`${l.language} deleted`);
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to delete the content"));
  }
}

function openEditor(l: TemplateLocalization) {
  router.push(
    `/templates/${templateId}/versions/${selectedVersion.value?.id}/localizations/${l.id}/edit`
  );
}

async function renderPreview() {
  if (!selectedVersion.value || !previewLang.value) return;
  previewLoading.value = true;
  previewError.value = "";

  let data: Record<string, any> = {};
  try {
    data = JSON.parse(previewData.value || "{}");
  } catch {
    previewError.value = "Sample data is not valid JSON";
    previewLoading.value = false;
    return;
  }

  try {
    const res = await templatesApi.previewLocalization(templateId, selectedVersion.value.id, {
      language: previewLang.value,
      template_data: data,
    });
    preview.value = res.data.data;
  } catch (e: any) {
    previewError.value = apiMessage(e, "Failed to render preview");
  } finally {
    previewLoading.value = false;
  }
}

function openPreview(lang: string) {
  previewLang.value = lang;
  preview.value = null;
  previewError.value = "";
  previewData.value =
    template.value?.sample_data || selectedVersion.value?.sample_data || "{}";
  showPreview.value = true;
  renderPreview();
}

function openSendTest() {
  sendTestForm.value = {
    to: "",
    from: "",
    language: template.value?.default_language || localizations.value[0]?.language || "",
    data:
      template.value?.sample_data ||
      selectedVersion.value?.sample_data ||
      '{\n  "name": "John",\n  "company": "Acme"\n}',
  };
  showSendTest.value = true;
}

async function sendTest() {
  sendingTest.value = true;
  try {
    const to = sendTestForm.value.to
      .split(",")
      .map((e) => e.trim())
      .filter(Boolean);
    await templatesApi.sendTest(templateId, {
      to,
      from: sendTestForm.value.from || undefined,
      language: sendTestForm.value.language || undefined,
      template_data: JSON.parse(sendTestForm.value.data || "{}"),
    });
    showSendTest.value = false;
    notify.success(`Test sent to ${to.join(", ")}`);
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to send the test"));
  } finally {
    sendingTest.value = false;
  }
}

function openEditTemplate() {
  if (!template.value) return;
  templateForm.value = {
    name: template.value.name,
    display_name: template.value.display_name,
    sample_data: template.value.sample_data || "",
    default_language: template.value.default_language || "en",
    description: template.value.description || "",
  };
  showTemplateEditModal.value = true;
}

async function saveTemplate() {
  if (!template.value || !templateForm.value.display_name.trim() || !templateForm.value.name.trim()) return;
  savingTemplate.value = true;
  try {
    await templatesApi.update(template.value.id, templateForm.value);
    showTemplateEditModal.value = false;
    await loadAll();
    notify.success("Template updated");
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to update template"));
  } finally {
    savingTemplate.value = false;
  }
}

function closeActiveModal() {
  showVersionStylesheetModal.value = false;
  showPreview.value = false;
  showSendTest.value = false;
  showLocModal.value = false;
  showTemplateEditModal.value = false;
}

const { watchClickStart, confirmClickEnd } = useModalSafeClose(closeActiveModal);

onMounted(loadAll);
</script>

<template>
  <div>
    <div class="page-header">
      <div>
        <h1>{{ template?.display_name || "Template" }}</h1>
        <p class="crumb">
          <RouterLink to="/templates">Templates</RouterLink>
          <span aria-hidden="true"> / </span>
          <span>{{ template?.display_name || "…" }}</span>
        </p>
      </div>
      <div class="header-actions">
        <button
          class="btn btn-primary"
          :disabled="!template?.active_version_id"
          :title="template?.active_version_id ? '' : 'Activate a version before sending'"
          @click="openSendTest"
        >
          Send test
        </button>
        <button v-if="wsStore.canEdit" class="btn btn-secondary" @click="openEditTemplate">
          Settings
        </button>
      </div>
    </div>

    <div v-if="loading" class="loading-page"><div class="spinner"></div></div>

    <template v-else-if="template">
      <TemplateSummary :template="template" :versions="versions" :languages="activeLanguages" />

      <div class="stack">
        <VersionList
          v-model:new-stylesheet-id="newVersionStylesheetId"
          :versions="versions"
          :stylesheets="stylesheets"
          :selected-id="selectedVersion?.id ?? null"
          :active-id="template.active_version_id ?? null"
          :can-edit="wsStore.canEdit"
          :creating="creatingVersion"
          @select="selectVersion"
          @create="createVersion"
          @edit-stylesheet="openEditVersionStylesheet"
          @activate="activateVersion"
          @delete="deleteVersion"
        />

        <LocalizationList
          v-if="selectedVersion"
          :version="selectedVersion"
          :localizations="localizations"
          :languages="languages"
          :default-language="template.default_language"
          :can-edit="wsStore.canEdit"
          @add="openCreateLoc"
          @edit="openEditLoc"
          @open-editor="openEditor"
          @preview="openPreview"
          @delete="deleteLoc"
        />
      </div>
    </template>

    <div v-else class="card">
      <div class="empty-state">
        <h3>Template not found</h3>
        <button class="btn btn-secondary" @click="router.push('/templates')">
          Back to templates
        </button>
      </div>
    </div>

    <div
      v-if="showLocModal"
      class="modal-overlay"
      @mousedown="watchClickStart"
      @mouseup="confirmClickEnd"
    >
      <LocalizationModal
        v-model:form="locForm"
        :editing="editingLoc"
        :languages="languages"
        :taken="localizations.map((l) => l.language)"
        :saving="savingLoc"
        @close="showLocModal = false"
        @save="saveLoc"
      />
    </div>

    <div
      v-if="showSendTest"
      class="modal-overlay"
      @mousedown="watchClickStart"
      @mouseup="confirmClickEnd"
    >
      <SendTestModal
        v-model:form="sendTestForm"
        :localizations="localizations"
        :sending="sendingTest"
        @close="showSendTest = false"
        @send="sendTest"
      />
    </div>

    <div
      v-if="showPreview"
      class="modal-overlay"
      @mousedown="watchClickStart"
      @mouseup="confirmClickEnd"
    >
      <PreviewModal
        v-model:data="previewData"
        :language="previewLang"
        :preview="preview"
        :loading="previewLoading"
        :error="previewError"
        @close="showPreview = false"
        @render="renderPreview"
      />
    </div>

    <div
      v-if="showVersionStylesheetModal && editingVersion"
      class="modal-overlay"
      @mousedown="watchClickStart"
      @mouseup="confirmClickEnd"
    >
      <VersionStylesheetModal
        v-model:stylesheet-id="editVersionStylesheetId"
        :version="editingVersion"
        :stylesheets="stylesheets"
        :saving="savingVersionStylesheet"
        @close="showVersionStylesheetModal = false"
        @save="saveVersionStylesheet"
      />
    </div>

    <TemplateModal
      :editing="template"
      :is-visible="showTemplateEditModal"
      :saving="savingTemplate"
      :form="templateForm"
      @close="showTemplateEditModal = false"
      @save="saveTemplate"
    />
  </div>
</template>

<style scoped>
.crumb {
  margin-top: 4px;
  font-size: 12px;
  color: var(--text-tertiary);
}

.crumb a {
  color: var(--text-tertiary);
  text-decoration: none;
}

.crumb a:hover {
  color: var(--primary-600);
  text-decoration: underline;
}

.header-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.stack {
  display: flex;
  flex-direction: column;
  gap: 24px;
}
</style>
