<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { templatesApi } from '../../api/templates'
import type { Template, TemplatePreview, TemplateLocalization, TemplateVersion } from '../../api/types'
import { useNotificationStore } from '../../stores/notification'

const route = useRoute()
const router = useRouter()
const notify = useNotificationStore()

const template = ref<Template | null>(null)
const version = ref<TemplateVersion | null>(null)
const localization = ref<TemplateLocalization | null>(null)
const preview = ref<TemplatePreview | null>(null)
const loading = ref(true)
const previewLoading = ref(false)
const previewError = ref('')
const activeTab = ref<'html' | 'text'>('html')
const sampleData = ref('{\n  "name": "John",\n  "company": "Acme"\n}')

const allTemplates = ref<Template[]>([])
const search = ref('')
const searchOpen = ref(false)

const searchResults = computed<Template[]>(() => {
  const q = search.value.trim().toLowerCase()
  if (!q) return allTemplates.value.slice(0, 10)
  return allTemplates.value.filter(t => t.name.toLowerCase().includes(q)).slice(0, 20)
})

let debounceTimer: ReturnType<typeof setTimeout> | null = null
watch(sampleData, () => {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => renderPreview(), 500)
})

watch(() => route.params.id, (id) => {
  if (id) loadTemplate(Number(id))
})

onMounted(() => {
  loadTemplate(Number(route.params.id))
})

async function loadTemplate(id: number) {
  loading.value = true
  template.value = null
  version.value = null
  localization.value = null
  preview.value = null

  try {
    if (allTemplates.value.length === 0) {
      const res = await templatesApi.list(0, 100)
      allTemplates.value = res.data.data
    }
    template.value = allTemplates.value.find((t: Template) => t.id === id) || null

    if (!template.value) {
      notify.error('Template not found')
      loading.value = false
      return
    }

    search.value = template.value.name

    if (template.value.sample_data) sampleData.value = template.value.sample_data

    if (template.value.active_version_id) {
      const versionsRes = await templatesApi.listVersions(id)
      version.value = (versionsRes.data.data || []).find((v: TemplateVersion) => v.id === template.value!.active_version_id) || null

      const locRes = await templatesApi.listLocalizations(id, template.value.active_version_id)
      const locs: TemplateLocalization[] = locRes.data.data || []
      localization.value =
        locs.find(l => l.language === template.value!.default_language) ||
        locs[0] || null
    }

    await renderPreview()
  } catch {
    notify.error('Failed to load template')
  } finally {
    loading.value = false
  }
}

function selectTemplate(t: Template) {
  searchOpen.value = false
  if (t.id === template.value?.id) return
  router.push(`/templates/${t.id}/preview`)
}

function onSearchBlur() {
  // Delay so click on a result still registers.
  setTimeout(() => { searchOpen.value = false }, 150)
}

async function renderPreview() {
  if (!template.value || !localization.value || !template.value.active_version_id) return
  previewLoading.value = true
  previewError.value = ''
  preview.value = null

  let data: Record<string, any> = {}
  try {
    data = JSON.parse(sampleData.value)
  } catch {
    previewError.value = 'Invalid JSON in sample data'
    previewLoading.value = false
    return
  }

  try {
    const res = await templatesApi.previewLocalization(
      template.value.id,
      template.value.active_version_id,
      { language: localization.value.language, template_data: data },
    )
    preview.value = res.data.data
  } catch (e: any) {
    previewError.value = e.response?.data?.error?.message || 'Failed to render template'
  } finally {
    previewLoading.value = false
  }
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Template Preview</h1>
      <button class="btn btn-secondary" @click="router.push('/templates')">Back to Templates</button>
    </div>

    <div class="template-search" style="margin-bottom: 16px; position: relative; max-width: 420px;">
      <input
        v-model="search"
        type="text"
        class="form-input"
        placeholder="Search templates by name..."
        @focus="searchOpen = true"
        @blur="onSearchBlur"
      />
      <div v-if="searchOpen && searchResults.length > 0" class="template-search-results">
        <button
          v-for="t in searchResults"
          :key="t.id"
          type="button"
          class="template-search-item"
          :class="{ active: t.id === template?.id }"
          @mousedown.prevent="selectTemplate(t)"
        >
          <span class="template-search-name">{{ t.name }}</span>
          <span v-if="t.description" class="template-search-desc">{{ t.description }}</span>
        </button>
      </div>
      <div v-else-if="searchOpen && search.trim()" class="template-search-results template-search-empty">
        No templates match "{{ search }}"
      </div>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <template v-else-if="template">
      <!-- Template Info -->
      <div class="card" style="margin-bottom: 24px;">
        <div class="card-header">
          <h2>{{ template.name }}</h2>
          <div class="flex gap-2">
            <span v-if="localization" class="badge badge-success">{{ localization.language }}</span>
            <span v-if="version" class="badge badge-neutral">v{{ version.version }}</span>
          </div>
        </div>
        <div class="card-body">
          <table>
            <tbody>
              <tr v-if="localization">
                <td class="info-label">Subject</td>
                <td><code>{{ localization.subject_template }}</code></td>
              </tr>
              <tr>
                <td class="info-label">Language</td>
                <td><span class="badge badge-info">{{ template.default_language }}</span></td>
              </tr>
              <tr v-if="version?.stylesheet">
                <td class="info-label">Stylesheet</td>
                <td><span class="badge badge-neutral">{{ version.stylesheet.name }}</span></td>
              </tr>
              <tr>
                <td class="info-label">Created</td>
                <td>{{ formatDate(template.created_at) }}</td>
              </tr>
              <tr v-if="template.updated_at">
                <td class="info-label">Updated</td>
                <td>{{ formatDate(template.updated_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- No active version warning -->
      <div v-if="!localization" class="card" style="margin-bottom: 24px;">
        <div class="card-body">
          <div class="text-muted" style="text-align: center; padding: 20px 0;">
            No localization available. Add a localization to the active version to preview this template.
          </div>
        </div>
      </div>

      <!-- Sample Data + Render -->
      <div v-if="localization" class="card" style="margin-bottom: 24px;">
        <div class="card-header">
          <h2>Sample Data</h2>
          <button
            class="btn btn-primary btn-sm"
            @click="renderPreview"
            :disabled="previewLoading"
          >
            {{ previewLoading ? 'Rendering...' : 'Render Preview' }}
          </button>
        </div>
        <div class="card-body">
          <textarea
            v-model="sampleData"
            class="form-textarea"
            rows="4"
            placeholder='{"name": "John", "company": "Acme"}'
          ></textarea>
          <span class="form-hint">
            JSON object with variables for your template. Use &#123;&#123;key&#125;&#125; or &#123;&#123;.key&#125;&#125; in templates.
          </span>
        </div>
      </div>

      <!-- Rendered Preview -->
      <div v-if="previewError" class="preview-error" style="margin-bottom: 24px;">
        {{ previewError }}
      </div>

      <template v-if="preview">
        <div class="card" style="margin-bottom: 24px;">
          <div class="card-header">
            <h2>Rendered Subject</h2>
          </div>
          <div class="card-body">
            <div class="rendered-subject">{{ preview.subject }}</div>
          </div>
        </div>

        <div class="card">
          <div class="card-header">
            <div class="tabs" style="margin-bottom: 0;">
              <button class="tab" :class="{ active: activeTab === 'html' }" @click="activeTab = 'html'">HTML Preview</button>
              <button class="tab" :class="{ active: activeTab === 'text' }" @click="activeTab = 'text'">Plain Text</button>
            </div>
          </div>
          <div class="card-body">
            <div v-if="activeTab === 'html'">
              <div v-if="preview.html" class="preview-html-frame">
                <iframe :srcdoc="preview.html" sandbox="" class="preview-iframe-full"></iframe>
              </div>
              <div v-else class="text-muted text-sm" style="padding: 20px 0; text-align: center;">
                No HTML template defined.
              </div>
            </div>
            <pre
              v-else
              class="preview-text-block"
            >{{ preview.text || 'No text template defined.' }}</pre>
          </div>
        </div>
      </template>

      <!-- Source Code -->
      <div v-if="localization" class="card" style="margin-top: 24px;">
        <div class="card-header">
          <h2>Template Source</h2>
        </div>
        <div class="card-body">
          <div v-if="localization.html_template" style="margin-bottom: 16px;">
            <div class="source-label">HTML Template</div>
            <pre class="code-block">{{ localization.html_template }}</pre>
          </div>
          <div v-if="localization.text_template" style="margin-bottom: 16px;">
            <div class="source-label">Text Template</div>
            <pre class="code-block">{{ localization.text_template }}</pre>
          </div>
          <div v-if="version?.stylesheet" style="margin-bottom: 16px;">
            <div class="source-label">Stylesheet: {{ version.stylesheet.name }}</div>
            <pre class="code-block">{{ version.stylesheet.css }}</pre>
          </div>
          <div v-if="!localization.html_template && !localization.text_template" class="text-muted text-sm">
            No template body defined.
          </div>
        </div>
      </div>
    </template>

    <div v-else class="empty-state">
      <h3>Template not found</h3>
      <p>The template you are looking for does not exist.</p>
    </div>
  </div>
</template>

<style scoped>
.info-label {
  font-weight: 600;
  width: 120px;
  color: var(--text-secondary);
}

.rendered-subject {
  font-size: 16px;
  font-weight: 500;
  color: var(--text-primary);
}

.preview-html-frame {
  border: 1px solid var(--border-primary);
  border-radius: var(--radius);
  overflow: hidden;
}

.preview-iframe-full {
  width: 100%;
  min-height: 400px;
  border: none;
  background: #fff;
}

.preview-text-block {
  white-space: pre-wrap;
  word-wrap: break-word;
  font-size: 14px;
  color: var(--text-secondary);
  line-height: 1.6;
  margin: 0;
}

.preview-error {
  padding: 12px 16px;
  background: var(--danger-50);
  color: var(--danger-600);
  border-radius: var(--radius);
  font-size: 13px;
}

.source-label {
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--text-muted);
  margin-bottom: 6px;
}

.template-search-results {
  position: absolute;
  top: 100%;
  left: 0;
  right: 0;
  margin-top: 4px;
  background: var(--bg-primary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
  z-index: 10;
  max-height: 320px;
  overflow-y: auto;
}

.template-search-item {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
  width: 100%;
  padding: 8px 12px;
  background: transparent;
  border: none;
  text-align: left;
  cursor: pointer;
  border-bottom: 1px solid var(--border-primary);
}
.template-search-item:last-child { border-bottom: none; }
.template-search-item:hover { background: var(--bg-hover, rgba(0, 0, 0, 0.04)); }
.template-search-item.active { background: var(--primary-50, rgba(0, 95, 204, 0.08)); }

.template-search-name {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-primary);
}

.template-search-desc {
  font-size: 12px;
  color: var(--text-muted);
}

.template-search-empty {
  padding: 12px;
  font-size: 13px;
  color: var(--text-muted);
  text-align: center;
}
</style>
