<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, shallowRef } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { templatesApi } from '../../api/templates'
import type {
  Template,
  TemplateVersion,
  TemplateLocalization,
  TemplatePreview,
} from '../../api/types'
import { useNotificationStore } from '../../stores/notification'

// CodeMirror
import { EditorView, placeholder as cmPlaceholder } from '@codemirror/view'
import { EditorState } from '@codemirror/state'
import { html } from '@codemirror/lang-html'
import { json } from '@codemirror/lang-json'
import { oneDark } from '@codemirror/theme-one-dark'
import { basicSetup } from 'codemirror'

const route = useRoute()
const router = useRouter()
const notify = useNotificationStore()

const templateId = Number(route.params.id)
const versionId = Number(route.params.versionId)
const localizationId = Number(route.params.localizationId)

const template = ref<Template | null>(null)
const version = ref<TemplateVersion | null>(null)
const localization = ref<TemplateLocalization | null>(null)
const loading = ref(true)
const saving = ref(false)
const hasChanges = ref(false)

// Editor state
const activeTab = ref<'html' | 'text' | 'subject'>('html')
const subjectValue = ref('')
const htmlValue = ref('')
const textValue = ref('')
const sampleDataValue = ref('{\n  "name": "John",\n  "company": "Acme"\n}')

// Preview
const preview = ref<TemplatePreview | null>(null)
const previewLoading = ref(false)
const previewError = ref('')
const previewTab = ref<'html' | 'text'>('html')
const showPreviewPanel = ref(true)

// CodeMirror editor refs
const htmlEditorRef = ref<HTMLElement | null>(null)
const textEditorRef = ref<HTMLElement | null>(null)
const subjectEditorRef = ref<HTMLElement | null>(null)
const sampleDataEditorRef = ref<HTMLElement | null>(null)

const htmlEditor = shallowRef<EditorView | null>(null)
const textEditor = shallowRef<EditorView | null>(null)
const subjectEditor = shallowRef<EditorView | null>(null)
const sampleDataEditor = shallowRef<EditorView | null>(null)

function createEditor(
  container: HTMLElement,
  content: string,
  lang: 'html' | 'json' | 'text',
  onChange: (value: string) => void,
  placeholderText?: string,
): EditorView {
  const extensions = [
    basicSetup,
    EditorView.lineWrapping,
    EditorView.updateListener.of((update) => {
      if (update.docChanged) {
        onChange(update.state.doc.toString())
      }
    }),
  ]

  if (lang === 'html') extensions.push(html())
  if (lang === 'json') extensions.push(json())
  if (placeholderText) extensions.push(cmPlaceholder(placeholderText))

  // Always use dark theme for code editors — looks better for code
  extensions.push(oneDark)
  extensions.push(editorTheme)

  const state = EditorState.create({
    doc: content,
    extensions,
  })

  return new EditorView({ state, parent: container })
}

const editorTheme = EditorView.theme({
  '&': {
    fontSize: '13px',
    height: '100%',
  },
  '.cm-scroller': {
    fontFamily: '"JetBrains Mono", "Fira Code", "Cascadia Code", monospace',
  },
  '.cm-content': {
    minHeight: '100px',
  },
})

let debounceTimer: ReturnType<typeof setTimeout> | null = null

function schedulePreview() {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => renderPreview(), 600)
}

async function renderPreview() {
  if (!template.value) return
  previewLoading.value = true
  previewError.value = ''

  let data: Record<string, any> = {}
  try {
    data = JSON.parse(sampleDataValue.value)
  } catch {
    previewError.value = 'Invalid JSON in sample data'
    previewLoading.value = false
    return
  }

  try {
    const res = await templatesApi.previewLocalization(templateId, versionId, {
      language: localization.value?.language || template.value.default_language,
      template_data: data,
    })
    preview.value = res.data.data
  } catch (e: any) {
    previewError.value = e.response?.data?.error?.message || 'Failed to render preview'
  } finally {
    previewLoading.value = false
  }
}

async function save() {
  if (!localization.value) return
  saving.value = true
  try {
    const res = await templatesApi.updateLocalization(localizationId, {
      subject_template: subjectValue.value,
      html_template: htmlValue.value,
      text_template: textValue.value,
    })
    localization.value = res.data.data
    hasChanges.value = false
    notify.success('Localization saved')
    renderPreview()
  } catch (e: any) {
    notify.error(e.response?.data?.error?.message || 'Failed to save')
  } finally {
    saving.value = false
  }
}

function handleKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === 's') {
    e.preventDefault()
    save()
  }
}

function initEditors() {
  if (htmlEditorRef.value) {
    htmlEditor.value = createEditor(htmlEditorRef.value, htmlValue.value, 'html', (val) => {
      htmlValue.value = val
      hasChanges.value = true
      schedulePreview()
    }, '<html>...</html>')
  }
  if (textEditorRef.value) {
    textEditor.value = createEditor(textEditorRef.value, textValue.value, 'text', (val) => {
      textValue.value = val
      hasChanges.value = true
      schedulePreview()
    }, 'Plain text version...')
  }
  if (subjectEditorRef.value) {
    subjectEditor.value = createEditor(subjectEditorRef.value, subjectValue.value, 'text', (val) => {
      subjectValue.value = val
      hasChanges.value = true
      schedulePreview()
    }, 'e.g. Welcome {{name}}!')
  }
  if (sampleDataEditorRef.value) {
    sampleDataEditor.value = createEditor(sampleDataEditorRef.value, sampleDataValue.value, 'json', (val) => {
      sampleDataValue.value = val
      schedulePreview()
    }, '{"key": "value"}')
  }
}

function destroyEditors() {
  htmlEditor.value?.destroy()
  textEditor.value?.destroy()
  subjectEditor.value?.destroy()
  sampleDataEditor.value?.destroy()
}

onMounted(async () => {
  document.addEventListener('keydown', handleKeydown)

  try {
    const [tmplRes, versionsRes, locsRes] = await Promise.all([
      templatesApi.list(0, 100),
      templatesApi.listVersions(templateId),
      templatesApi.listLocalizations(templateId, versionId),
    ])

    template.value = tmplRes.data.data.find((t: Template) => t.id === templateId) || null
    version.value = (versionsRes.data.data || []).find((v: TemplateVersion) => v.id === versionId) || null

    const locs: TemplateLocalization[] = locsRes.data.data || []
    localization.value = locs.find((l) => l.id === localizationId) || null

    if (!template.value || !version.value || !localization.value) {
      notify.error('Template, version, or localization not found')
      loading.value = false
      return
    }

    // Populate editor values
    subjectValue.value = localization.value.subject_template
    htmlValue.value = localization.value.html_template
    textValue.value = localization.value.text_template

    if (template.value.sample_data) {
      sampleDataValue.value = template.value.sample_data
    } else if (version.value.sample_data) {
      sampleDataValue.value = version.value.sample_data
    }

    loading.value = false

    // Initialize editors after DOM update
    setTimeout(() => {
      initEditors()
      renderPreview()
    }, 0)
  } catch {
    notify.error('Failed to load template data')
    loading.value = false
  }
})

onBeforeUnmount(() => {
  document.removeEventListener('keydown', handleKeydown)
  destroyEditors()
  if (debounceTimer) clearTimeout(debounceTimer)
})

function goBack() {
  router.push(`/templates/${templateId}/versions`)
}

function switchToVisualBuilder() {
  router.push(`/templates/${templateId}/versions/${versionId}/localizations/${localizationId}/builder`)
}
</script>

<template>
  <div class="editor-page">
    <!-- Header -->
    <div class="editor-header">
      <div class="editor-header-left">
        <button class="btn btn-secondary btn-sm" @click="goBack">
          <span class="mdi mdi-arrow-left"></span> Back
        </button>
        <div class="editor-title">
          <h2>{{ template?.name || 'Template' }}</h2>
          <div class="editor-meta">
            <span class="badge badge-info">{{ localization?.language }}</span>
            <span class="badge badge-neutral">v{{ version?.version }}</span>
            <span v-if="hasChanges" class="badge badge-warning">Unsaved</span>
          </div>
        </div>
      </div>
      <div class="editor-header-right">
        <button class="btn btn-secondary btn-sm" @click="switchToVisualBuilder">
          <span class="mdi mdi-palette-outline"></span> Visual Builder
        </button>
        <button
          class="btn btn-secondary btn-sm"
          @click="showPreviewPanel = !showPreviewPanel"
        >
          {{ showPreviewPanel ? 'Hide Preview' : 'Show Preview' }}
        </button>
        <button
          class="btn btn-primary btn-sm"
          :disabled="saving || !hasChanges"
          @click="save"
        >
          {{ saving ? 'Saving...' : 'Save' }}
        </button>
      </div>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <!-- Editor Layout -->
    <div v-else-if="template && localization" class="editor-layout" :class="{ 'preview-hidden': !showPreviewPanel }">
      <!-- Left: Code Editor -->
      <div class="editor-pane">
        <!-- Tabs -->
        <div class="editor-tabs">
          <button
            class="editor-tab"
            :class="{ active: activeTab === 'html' }"
            @click="activeTab = 'html'"
          >
            HTML
          </button>
          <button
            class="editor-tab"
            :class="{ active: activeTab === 'text' }"
            @click="activeTab = 'text'"
          >
            Plain Text
          </button>
          <button
            class="editor-tab"
            :class="{ active: activeTab === 'subject' }"
            @click="activeTab = 'subject'"
          >
            Subject
          </button>
        </div>

        <!-- Editor panels -->
        <div class="editor-content">
          <div v-show="activeTab === 'html'" ref="htmlEditorRef" class="cm-container"></div>
          <div v-show="activeTab === 'text'" ref="textEditorRef" class="cm-container"></div>
          <div v-show="activeTab === 'subject'" ref="subjectEditorRef" class="cm-container"></div>
        </div>

        <!-- Sample Data (collapsible) -->
        <details class="sample-data-section">
          <summary class="sample-data-header">
            Sample Data (JSON)
          </summary>
          <div ref="sampleDataEditorRef" class="cm-container cm-container-sm"></div>
        </details>
      </div>

      <!-- Right: Preview -->
      <div v-if="showPreviewPanel" class="preview-pane">
        <div class="preview-header">
          <div class="preview-tabs">
            <button
              class="editor-tab"
              :class="{ active: previewTab === 'html' }"
              @click="previewTab = 'html'"
            >
              HTML
            </button>
            <button
              class="editor-tab"
              :class="{ active: previewTab === 'text' }"
              @click="previewTab = 'text'"
            >
              Text
            </button>
          </div>
          <button
            class="btn btn-secondary btn-sm"
            :disabled="previewLoading"
            @click="renderPreview"
          >
            {{ previewLoading ? 'Rendering...' : 'Refresh' }}
          </button>
        </div>

        <!-- Preview subject -->
        <div v-if="preview?.subject" class="preview-subject">
          <span class="preview-subject-label">Subject:</span>
          {{ preview.subject }}
        </div>

        <!-- Preview error -->
        <div v-if="previewError" class="preview-error">{{ previewError }}</div>

        <!-- Preview content -->
        <div class="preview-content">
          <div v-if="previewTab === 'html'">
            <iframe
              v-if="preview?.html"
              :srcdoc="preview.html"
              sandbox=""
              class="preview-iframe"
            ></iframe>
            <div v-else class="preview-empty">
              {{ previewLoading ? 'Rendering...' : 'No HTML preview available' }}
            </div>
          </div>
          <pre v-else class="preview-text">{{ preview?.text || (previewLoading ? 'Rendering...' : 'No text preview available') }}</pre>
        </div>
      </div>
    </div>

    <!-- Not found -->
    <div v-else class="empty-state">
      <h3>Not found</h3>
      <p>The template, version, or localization was not found.</p>
      <button class="btn btn-secondary" @click="goBack">Go Back</button>
    </div>
  </div>
</template>

<style scoped>
.editor-page {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 60px);
  margin: -24px;
  overflow: hidden;
}

.editor-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  border-bottom: 1px solid var(--border-primary);
  background: var(--bg-primary);
  flex-shrink: 0;
}

.editor-header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.editor-header-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.editor-title h2 {
  font-size: 15px;
  font-weight: 600;
  margin: 0;
  color: var(--text-primary);
}

.editor-meta {
  display: flex;
  gap: 6px;
  margin-top: 2px;
}

.editor-layout {
  display: grid;
  grid-template-columns: 1fr 1fr;
  flex: 1;
  overflow: hidden;
}

.editor-layout.preview-hidden {
  grid-template-columns: 1fr;
}

.editor-pane {
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border-right: 1px solid var(--border-primary);
}

.editor-tabs {
  display: flex;
  border-bottom: 1px solid var(--border-primary);
  background: var(--bg-secondary);
  flex-shrink: 0;
}

.editor-tab {
  padding: 8px 16px;
  font-size: 13px;
  font-weight: 500;
  color: var(--text-secondary);
  background: none;
  border: none;
  border-bottom: 2px solid transparent;
  cursor: pointer;
  transition: color 0.15s, border-color 0.15s;
}

.editor-tab:hover {
  color: var(--text-primary);
}

.editor-tab.active {
  color: var(--primary-600);
  border-bottom-color: var(--primary-600);
}

.editor-content {
  flex: 1;
  overflow: hidden;
}

.cm-container {
  height: 100%;
  overflow: auto;
}

.cm-container :deep(.cm-editor) {
  height: 100%;
}

.cm-container-sm {
  height: 150px;
}

.cm-container-sm :deep(.cm-editor) {
  height: 150px;
}

.sample-data-section {
  border-top: 1px solid var(--border-primary);
  flex-shrink: 0;
}

.sample-data-header {
  padding: 8px 16px;
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-muted);
  background: var(--bg-secondary);
  cursor: pointer;
  user-select: none;
}

.sample-data-header:hover {
  color: var(--text-secondary);
}

/* Preview pane */
.preview-pane {
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: var(--bg-primary);
}

.preview-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid var(--border-primary);
  background: var(--bg-secondary);
  flex-shrink: 0;
  padding-right: 12px;
}

.preview-tabs {
  display: flex;
}

.preview-subject {
  padding: 8px 16px;
  font-size: 13px;
  color: var(--text-primary);
  background: var(--bg-tertiary);
  border-bottom: 1px solid var(--border-primary);
  flex-shrink: 0;
}

.preview-subject-label {
  font-weight: 600;
  color: var(--text-secondary);
  margin-right: 6px;
}

.preview-error {
  padding: 8px 16px;
  background: var(--danger-50);
  color: var(--danger-600);
  font-size: 13px;
  flex-shrink: 0;
}

.preview-content {
  flex: 1;
  overflow: auto;
}

.preview-iframe {
  width: 100%;
  height: 100%;
  min-height: 400px;
  border: none;
  background: #fff;
}

.preview-text {
  padding: 16px;
  margin: 0;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 13px;
  color: var(--text-secondary);
  white-space: pre-wrap;
  word-wrap: break-word;
  line-height: 1.6;
}

.preview-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 200px;
  color: var(--text-muted);
  font-size: 14px;
}

.badge-warning {
  background: var(--warning-50);
  color: var(--warning-600);
}
</style>
