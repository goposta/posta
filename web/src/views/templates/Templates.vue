<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useRouter } from "vue-router";
import { templatesApi } from "../../api/templates";
import { languagesApi } from "../../api/languages";
import type {
  Template,
  TemplateListItem,
  TemplateDraft,
  TemplateExport,
  Language,
  Pageable,
} from "../../api/types";
import { useNotificationStore } from "../../stores/notification";
import { useConfirm } from "../../composables/useConfirm";
import { useModalSafeClose } from "../../composables/useModalSafeClose";
import { useWorkspaceStore } from "../../stores/workspace";
import { apiMessage } from "../../composables/apiError";
import { usePagination } from '@/composables/usePagination'
import Pagination from '@/components/Pagination.vue'
import TemplateModal from "@/components/TemplateModal.vue";

const router = useRouter();
const notify = useNotificationStore();
const wsStore = useWorkspaceStore();
const { confirm } = useConfirm();

const templates = ref<TemplateListItem[]>([]);
const loading = ref(true);
const search = ref("");
let searchTimeout: ReturnType<typeof setTimeout> | null = null;
  
  const showModal = ref(false);
  const editing = ref<Template | null>(null);
const saving = ref(false);

const languages = ref<Language[]>([]);

const form = ref<TemplateDraft>({
  name: "",
  sample_data: "",
  default_language: "en",
  description: "",
});

async function loadLanguages() {
  try {
    const res = await languagesApi.list(0, 100);
    languages.value = res.data.data;
  } catch {
    // Non-critical
  }
}

function resetForm() {
  form.value = { name: "", sample_data: "", default_language : languages.value.find(l => l.is_default)?.code || 'en', description: "" };
  editing.value = null;
}

function openCreate() {
  resetForm();
  showModal.value = true;
}

function openEdit(template: Template) {
  editing.value = template;
  form.value = {
    name: template.name,
    sample_data: template.sample_data || "",
    default_language: template.default_language || "en",
    description: template.description || "",
  };
  showModal.value = true;
}

function closeModal() {
  showModal.value = false;
  resetForm();
}

const { pageable, goToPage } = usePagination(async (page) => {
  loading.value = true
  try {
    const res = await templatesApi.list(page, pageable.value.size, search.value);
    templates.value = res.data.data;
    pageable.value = res.data.pageable;
  } catch (e: any) {
    notify.error(apiMessage(e, 'Failed to load templates'))
  } finally {
    loading.value = false
  }
})
function onSearchInput() {
  if (searchTimeout) clearTimeout(searchTimeout);
  searchTimeout = setTimeout(() => goToPage(0), 300);
}

async function saveTemplate() {
  if (!form.value.name.trim()) return;
  saving.value = true;
  try {
    if (editing.value) {
      await templatesApi.update(editing.value.id, form.value);
      notify.success("Template updated");
    } else {
      await templatesApi.create(form.value);
      notify.success("Template created");
    }
    closeModal();
    await goToPage(pageable.value.current_page);
  } catch (e: any) {
    notify.error(
      apiMessage(e, editing.value ? "Failed to update template" : "Failed to create template")
    );
  } finally {
    saving.value = false;
  }
}

async function deleteTemplate(template: Template) {
  const confirmed = await confirm({
    title: "Delete Template",
    message: `Are you sure you want to delete "${template.display_name}"? This action cannot be undone.`,
    confirmText: "Delete",
    variant: "danger",
  });
  if (!confirmed) return;
  try {
    await templatesApi.delete(template.id);
    notify.success("Template deleted");
    await goToPage(pageable.value.current_page);
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to delete template"));
  }
}

async function exportTemplate(template: Template) {
  try {
    const res = await templatesApi.exportTemplate(template.id);
    const data = res.data.data;
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: "application/json" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `${data.name}.json`;
    a.click();
    URL.revokeObjectURL(url);
    notify.success("Template exported");
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to export template"));
  }
}

const importInput = ref<HTMLInputElement | null>(null);
  const importing = ref(false);

function triggerImport() {
  importInput.value?.click();
}

async function handleImportFile(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  if (!file) return;
  
  importing.value = true;
  try {
    const name = file.name.toLowerCase();
    if (name.endsWith(".html") || name.endsWith(".htm")) {
      await templatesApi.importHTML(file);
      notify.success("HTML template imported");
    } else {
      const text = await file.text();
      const data: TemplateExport = JSON.parse(text);
      await templatesApi.importTemplate(data);
      notify.success("Template imported");
    }
    await goToPage(pageable.value.current_page);
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to import template. Check the file format."));
  } finally {
    importing.value = false;
    input.value = "";
  }
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}
const { watchClickStart, confirmClickEnd } = useModalSafeClose(() => {
  closeModal();
});

onMounted(() => {
  loadLanguages();
});
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Templates</h1>
    </div>

    <div class="card">
      <div class="card-header">
        <input
          v-model="search"
          class="form-input"
          placeholder="Search by name or description..."
          style="max-width: 320px"
          @input="onSearchInput"
        />
        <div class="flex gap-2">
          <input
            ref="importInput"
            type="file"
            accept=".json,.html,.htm"
            style="display: none"
            @change="handleImportFile"
          />
          <button class="btn btn-secondary" @click="router.push('/templates/preview')">
            Preview Template
          </button>
          <button
            v-if="wsStore.canEdit"
            class="btn btn-secondary"
            :disabled="importing"
            @click="triggerImport"
          >
            {{ importing ? "Importing..." : "Import" }}
          </button>
          <button v-if="wsStore.canEdit" class="btn btn-primary" @click="openCreate">
            Create Template
          </button>
        </div>
      </div>

      <div v-if="loading" class="loading-page">
        <div class="spinner"></div>
      </div>

      <div v-else>
        <div v-if="templates.length === 0" class="empty-state">
          <h3>{{ search ? "No Results" : "No Templates" }}</h3>
          <p>
            {{
              search
                ? "No templates match your search."
                : "Create your first template to reuse email layouts."
            }}
          </p>
        </div>

        <template v-else>
          <div class="table-wrapper">
            <table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Languages</th>
                  <th>Version</th>
                  <th>Last edited</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="tmpl in templates"
                  :key="tmpl.id"
                  class="cursor-pointer"
                  @click="
                    router.push({ name: 'template-detail', params: { id: tmpl.id } })
                  "
                >
                  <td>
                    <div>{{ tmpl.display_name }} <code class="tmpl-name">{{ tmpl.name }}</code></div>
                    <div v-if="tmpl.description" class="text-muted text-sm">
                      {{ tmpl.description }}
                    </div>
                  </td>
                  <td>
                    <div class="langs">
                      <span
                        v-for="lang in tmpl.languages"
                        :key="lang"
                        class="badge"
                        :class="lang === tmpl.default_language ? 'badge-success' : 'badge-neutral'"
                        :title="lang === tmpl.default_language ? 'Default language' : ''"
                      >
                        {{ lang }}
                      </span>
                      <span v-if="!tmpl.languages.length" class="text-muted text-sm">
                        No content yet
                      </span>
                    </div>
                  </td>
                  <td>
                    <span v-if="tmpl.active_version" class="badge badge-neutral">
                      v{{ tmpl.active_version.version }}
                    </span>
                    <span v-else class="text-muted" title="This template cannot be sent until a version is active">
                      &mdash;
                    </span>
                  </td>
                  <td>
                    <div>{{ formatDate(tmpl.updated_at || tmpl.created_at) }}</div>
                    <div v-if="tmpl.last_edited_by?.name" class="text-muted text-sm">
                      by {{ tmpl.last_edited_by.name }}
                    </div>
                  </td>
                  <td>
                    <div class="flex gap-2">
                      <button
                        class="btn btn-secondary btn-sm"
                        @click.stop="
                          router.push({
                            name: 'template-preview',
                            params: { id: tmpl.id },
                          })
                        "
                      >
                        Preview
                      </button>
                      <button
                        v-if="wsStore.canEdit"
                        class="btn btn-secondary btn-sm"
                        @click.stop="openEdit(tmpl)"
                      >
                        Edit
                      </button>
                      <button
                        class="btn btn-secondary btn-sm"
                        @click.stop="exportTemplate(tmpl)"
                      >
                        Export
                      </button>
                      <button
                        v-if="wsStore.canEdit"
                        class="btn btn-danger btn-sm"
                        @click.stop="deleteTemplate(tmpl)"
                      >
                        Delete
                      </button>
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

<Pagination :pageable="pageable" @page="goToPage" />
  
        </template>
      </div>
    </div>

    <!-- Create/Edit Template Modal -->
     <TemplateModal :editing="editing" :is-visible="showModal" :saving="saving" :form="form"
      @close="closeModal" @save="saveTemplate" />
   
  </div>
</template>

<style scoped>
.tmpl-name {
  margin-left: 6px;
  font-size: 12px;
  color: var(--text-muted);
}

.langs {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
  max-width: 220px;
}
</style>
