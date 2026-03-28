<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { templatesApi } from '../../api/templates'
import type {
  Template,
  TemplateVersion,
  TemplateLocalization,
} from '../../api/types'
import { useNotificationStore } from '../../stores/notification'
import grapesjs from 'grapesjs'
import type { Editor } from 'grapesjs'
import 'grapesjs/dist/css/grapes.min.css'
import grapesjsNewsletter from 'grapesjs-preset-newsletter'

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

const editorContainer = ref<HTMLElement | null>(null)
let editor: Editor | null = null

function initGrapesJS() {
  if (!editorContainer.value) return

  editor = grapesjs.init({
    container: editorContainer.value,
    height: '100%',
    width: 'auto',
    fromElement: false,
    storageManager: false,
    plugins: [grapesjsNewsletter],
    pluginsOpts: {
      [grapesjsNewsletter as any]: {
        modalTitleImport: 'Import HTML',
        modalTitleExport: 'Export HTML',
      },
    },
    canvas: {
      styles: [
        'https://fonts.googleapis.com/css2?family=Inter:wght@300;400;500;600;700&display=swap',
      ],
    },
    deviceManager: {
      devices: [
        { name: 'Desktop', width: '' },
        { name: 'Tablet', width: '768px', widthMedia: '992px' },
        { name: 'Mobile', width: '375px', widthMedia: '480px' },
      ],
    },
  })

  // Load existing content
  if (localization.value) {
    if (localization.value.builder_json) {
      try {
        const projectData = JSON.parse(localization.value.builder_json)
        editor.loadProjectData(projectData)
      } catch {
        // Fall back to loading raw HTML
        if (localization.value.html_template) {
          editor.setComponents(localization.value.html_template)
        }
      }
    } else if (localization.value.html_template) {
      editor.setComponents(localization.value.html_template)
    }
  }

  // Track changes
  editor.on('change:changesCount', () => {
    hasChanges.value = true
  })
}

async function save() {
  if (!editor || !localization.value) return
  saving.value = true

  try {
    const html = editor.getHtml()
    const css = editor.getCss()
    const fullHtml = injectStyles(html, css || '')
    const projectData = JSON.stringify(editor.getProjectData())

    const res = await templatesApi.updateLocalization(localizationId, {
      html_template: fullHtml,
      builder_json: projectData,
    })
    localization.value = res.data.data
    hasChanges.value = false
    notify.success('Template saved')
  } catch (e: any) {
    notify.error(e.response?.data?.error?.message || 'Failed to save')
  } finally {
    saving.value = false
  }
}

function injectStyles(html: string, css: string): string {
  if (!css) return html
  const styleTag = `<style>${css}</style>`
  if (html.includes('</head>')) {
    return html.replace('</head>', `${styleTag}</head>`)
  }
  if (html.includes('<body')) {
    return html.replace('<body', `${styleTag}<body`)
  }
  return styleTag + html
}

function handleKeydown(e: KeyboardEvent) {
  if ((e.metaKey || e.ctrlKey) && e.key === 's') {
    e.preventDefault()
    save()
  }
}

function goBack() {
  router.push(`/templates/${templateId}/versions`)
}

function switchToCodeEditor() {
  router.push(`/templates/${templateId}/versions/${versionId}/localizations/${localizationId}/edit`)
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

    loading.value = false

    // Initialize GrapesJS after DOM renders
    setTimeout(() => initGrapesJS(), 0)
  } catch {
    notify.error('Failed to load template data')
    loading.value = false
  }
})

onBeforeUnmount(() => {
  document.removeEventListener('keydown', handleKeydown)
  if (editor) {
    editor.destroy()
    editor = null
  }
})
</script>

<template>
  <div class="builder-page">
    <!-- Header -->
    <div class="builder-header">
      <div class="builder-header-left">
        <button class="btn btn-secondary btn-sm" @click="goBack">
          <span class="mdi mdi-arrow-left"></span> Back
        </button>
        <div class="builder-title">
          <h2>{{ template?.name || 'Template' }}</h2>
          <div class="builder-meta">
            <span class="badge badge-accent">Visual Builder</span>
            <span class="badge badge-info">{{ localization?.language }}</span>
            <span class="badge badge-neutral">v{{ version?.version }}</span>
            <span v-if="hasChanges" class="badge badge-warning">Unsaved</span>
          </div>
        </div>
      </div>
      <div class="builder-header-right">
        <button class="btn btn-secondary btn-sm" @click="switchToCodeEditor">
          <span class="mdi mdi-code-tags"></span> Code Editor
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

    <!-- GrapesJS Editor -->
    <div v-else-if="template && localization" ref="editorContainer" class="builder-editor"></div>

    <!-- Not found -->
    <div v-else class="empty-state">
      <h3>Not found</h3>
      <p>The template, version, or localization was not found.</p>
      <button class="btn btn-secondary" @click="goBack">Go Back</button>
    </div>
  </div>
</template>

<style scoped>
.builder-page {
  display: flex;
  flex-direction: column;
  height: calc(100vh - 60px);
  margin: -24px;
  overflow: hidden;
}

.builder-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  border-bottom: 1px solid var(--border-primary);
  background: var(--bg-primary);
  flex-shrink: 0;
  z-index: 10;
}

.builder-header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.builder-header-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.builder-title h2 {
  font-size: 15px;
  font-weight: 600;
  margin: 0;
  color: var(--text-primary);
}

.builder-meta {
  display: flex;
  gap: 6px;
  margin-top: 2px;
}

.builder-editor {
  flex: 1;
  overflow: hidden;
}

/* GrapesJS overrides for theme integration */
.builder-editor :deep(.gjs-one-bg) {
  background-color: var(--bg-secondary);
}

.builder-editor :deep(.gjs-two-color) {
  color: var(--text-primary);
}

.builder-editor :deep(.gjs-three-bg) {
  background-color: var(--bg-tertiary);
}

.builder-editor :deep(.gjs-four-color),
.builder-editor :deep(.gjs-four-color-h:hover) {
  color: var(--primary-600);
}

.badge-accent {
  background: var(--primary-50);
  color: var(--primary-600);
}

.badge-warning {
  background: var(--warning-50);
  color: var(--warning-600);
}
</style>
