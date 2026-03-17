<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useRouter } from "vue-router";
import { templatesApi } from "../../api/templates";
import { languagesApi } from "../../api/languages";
import type {
  Template,
  TemplateInput,
  TemplateExport,
  Language,
  Pageable,
} from "../../api/types";
import { useNotificationStore } from "../../stores/notification";
import { useConfirm } from "../../composables/useConfirm";
import { useModalSafeClose } from "../../composables/useModalSafeClose";

const router = useRouter();
const notify = useNotificationStore();
const { confirm } = useConfirm();

const templates = ref<Template[]>([]);
const pageable = ref<Pageable>({
  current_page: 0,
  size: 20,
  total_pages: 0,
  total_elements: 0,
  empty: true,
});
const loading = ref(true);

const showModal = ref(false);
const editing = ref<Template | null>(null);
const saving = ref(false);

const languages = ref<Language[]>([]);

const form = ref<TemplateInput>({
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
  form.value = { name: "", sample_data: "", default_language: "en", description: "" };
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

async function loadTemplates(page = 0) {
  loading.value = true;
  try {
    const res = await templatesApi.list(page, pageable.value.size);
    templates.value = res.data.data;
    pageable.value = res.data.pageable;
  } catch {
    notify.error("Failed to load templates");
  } finally {
    loading.value = false;
  }
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
    await loadTemplates(pageable.value.current_page);
  } catch {
    notify.error(
      editing.value ? "Failed to update template" : "Failed to create template"
    );
  } finally {
    saving.value = false;
  }
}

async function deleteTemplate(template: Template) {
  const confirmed = await confirm({
    title: "Delete Template",
    message: `Are you sure you want to delete "${template.name}"? This action cannot be undone.`,
    confirmText: "Delete",
    variant: "danger",
  });
  if (!confirmed) return;
  try {
    await templatesApi.delete(template.id);
    notify.success("Template deleted");
    await loadTemplates(pageable.value.current_page);
  } catch {
    notify.error("Failed to delete template");
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
  } catch {
    notify.error("Failed to export template");
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
    const text = await file.text();
    const data: TemplateExport = JSON.parse(text);
    await templatesApi.importTemplate(data);
    notify.success("Template imported");
    await loadTemplates(pageable.value.current_page);
  } catch {
    notify.error("Failed to import template. Please check the file format.");
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
  closeModal()
});

onMounted(() => {
  loadTemplates();
  loadLanguages();
});
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Templates</h1>
      <div class="flex gap-2">
        <input
          ref="importInput"
          type="file"
          accept=".json"
          style="display: none"
          @change="handleImportFile"
        />
        <button class="btn btn-secondary" :disabled="importing" @click="triggerImport">
          {{ importing ? "Importing..." : "Import" }}
        </button>
        <button class="btn btn-primary" @click="openCreate">Create Template</button>
      </div>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <div v-else class="card">
      <div v-if="templates.length === 0" class="empty-state">
        <h3>No Templates</h3>
        <p>Create your first template to reuse email layouts.</p>
      </div>

      <template v-else>
        <div class="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Language</th>
                <th>Active Version</th>
                <th>Created</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="tmpl in templates"
                :key="tmpl.id"
                class="cursor-pointer"
                @click="router.push({ name: 'template-detail', params: { id: tmpl.id } })"
              >
                <td>
                  <div>{{ tmpl.name }}</div>
                  <div v-if="tmpl.description" class="text-muted text-sm">
                    {{ tmpl.description }}
                  </div>
                </td>
                <td>
                  <span class="badge badge-neutral">{{ tmpl.default_language }}</span>
                </td>
                <td>
                  <span v-if="tmpl.active_version_id" class="badge badge-success"
                    >v{{ tmpl.active_version?.version || "?" }}</span
                  >
                  <span v-else class="text-muted">&mdash;</span>
                </td>
                <td>{{ formatDate(tmpl.created_at) }}</td>
                <td>
                  <div class="flex gap-2">
                    <button
                      class="btn btn-primary btn-sm"
                      @click.stop="
                        router.push({ name: 'template-detail', params: { id: tmpl.id } })
                      "
                    >
                      Versions
                    </button>
                    <button
                      class="btn btn-secondary btn-sm"
                      @click.stop="
                        router.push({ name: 'template-preview', params: { id: tmpl.id } })
                      "
                    >
                      Preview
                    </button>
                    <button class="btn btn-secondary btn-sm" @click.stop="openEdit(tmpl)">
                      Edit
                    </button>
                    <button
                      class="btn btn-secondary btn-sm"
                      @click.stop="exportTemplate(tmpl)"
                    >
                      Export
                    </button>
                    <button
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

        <div class="pagination">
          <span class="pagination-info">
            Page {{ pageable.current_page + 1 }} of {{ pageable.total_pages }} ({{
              pageable.total_elements
            }}
            templates)
          </span>
          <div class="pagination-buttons">
            <button
              class="btn btn-secondary btn-sm"
              :disabled="pageable.current_page === 0"
              @click="loadTemplates(pageable.current_page - 1)"
            >
              Previous
            </button>
            <button
              class="btn btn-secondary btn-sm"
              :disabled="pageable.current_page >= pageable.total_pages - 1"
              @click="loadTemplates(pageable.current_page + 1)"
            >
              Next
            </button>
          </div>
        </div>
      </template>
    </div>

    <!-- Create/Edit Template Modal -->
    <div
      v-if="showModal"
      class="modal-overlay"
      @mousedown="watchClickStart"
      @mouseup="confirmClickEnd"
    >
      <div class="modal" style="max-width: 560px" @mousedown.stop @mouseup.stop>
        <div class="modal-header">
          <h3>{{ editing ? "Edit Template" : "Create Template" }}</h3>
        </div>

        <div class="modal-body">
          <div class="form-group">
            <label class="form-label">Name</label>
            <input
              v-model="form.name"
              class="form-input"
              placeholder="e.g. Welcome Email"
            />
          </div>
          <div class="form-group">
            <label class="form-label">Description</label>
            <input
              v-model="form.description"
              class="form-input"
              placeholder="e.g. Sent after user registration"
            />
          </div>
          <div class="form-group">
            <label class="form-label">Default Language</label>
            <select v-model="form.default_language" class="form-select">
              <option v-for="lang in languages" :key="lang.id" :value="lang.code">
                {{ lang.name }} ({{ lang.code }})
              </option>
            </select>
            <span class="form-hint"
              >Fallback language when no localization matches the requested language</span
            >
          </div>
          <div class="form-group">
            <label class="form-label">Sample Data (JSON)</label>
            <textarea
              v-model="form.sample_data"
              class="form-textarea"
              rows="3"
              placeholder='{"name": "John", "company": "Acme"}'
            ></textarea>
            <span class="form-hint"
              >Default sample data for previewing template localizations</span
            >
          </div>
        </div>

        <div class="modal-footer">
          <button class="btn btn-secondary" @click="closeModal">Cancel</button>
          <button
            class="btn btn-primary"
            :disabled="saving || !form.name.trim()"
            @click="saveTemplate"
          >
            {{ saving ? "Saving..." : editing ? "Update" : "Create" }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
