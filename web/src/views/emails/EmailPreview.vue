<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { emailsApi } from '../../api/emails'
import type { EmailPreviewResponse } from '../../api/emails'
import { templatesApi } from '../../api/templates'
import type { Template } from '../../api/types'
import { useNotificationStore } from '../../stores/notification'

const router = useRouter()
const notify = useNotificationStore()

const templates = ref<Template[]>([])
const selectedTemplate = ref('')
const language = ref('')
const sampleData = ref('{\n  \n}')
const preview = ref<EmailPreviewResponse | null>(null)
const loading = ref(true)
const previewLoading = ref(false)
const previewError = ref('')
const activeTab = ref<'html' | 'text'>('html')

onMounted(async () => {
  try {
    const res = await templatesApi.list(0, 100)
    templates.value = res.data.data || []
  } catch {
    notify.error('Failed to load templates')
  } finally {
    loading.value = false
  }
})

watch(selectedTemplate, (name) => {
  const tmpl = templates.value.find(t => t.name === name)
  if (tmpl) {
    language.value = tmpl.default_language || ''
    if (tmpl.sample_data) sampleData.value = tmpl.sample_data
  }
  preview.value = null
  previewError.value = ''
})

async function renderPreview() {
  if (!selectedTemplate.value) {
    previewError.value = 'Select a template first'
    return
  }

  previewLoading.value = true
  previewError.value = ''
  preview.value = null

  let data: Record<string, any> = {}
  try {
    const trimmed = sampleData.value.trim()
    if (trimmed) data = JSON.parse(trimmed)
  } catch {
    previewError.value = 'Invalid JSON in template data'
    previewLoading.value = false
    return
  }

  try {
    const res = await emailsApi.preview({
      template: selectedTemplate.value,
      language: language.value || undefined,
      template_data: Object.keys(data).length > 0 ? data : undefined,
    })
    preview.value = res.data.data
  } catch (e: any) {
    previewError.value = e.response?.data?.error?.message || 'Failed to render preview'
  } finally {
    previewLoading.value = false
  }
}
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Email Preview</h1>
      <button class="btn btn-secondary" @click="router.push('/emails')">Back to Emails</button>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <template v-else>
      <!-- Template Selection -->
      <div class="card" style="margin-bottom: 24px;">
        <div class="card-header">
          <h2>Configuration</h2>
          <button
            class="btn btn-primary btn-sm"
            :disabled="previewLoading || !selectedTemplate"
            @click="renderPreview"
          >
            {{ previewLoading ? 'Rendering...' : 'Render Preview' }}
          </button>
        </div>
        <div class="card-body">
          <div class="form-grid">
            <div class="form-group">
              <label class="form-label">Template</label>
              <select v-model="selectedTemplate" class="form-select">
                <option value="" disabled>Select a template</option>
                <option v-for="t in templates" :key="t.id" :value="t.name">{{ t.name }}</option>
              </select>
            </div>
            <div class="form-group">
              <label class="form-label">Language</label>
              <input v-model="language" type="text" class="form-input" placeholder="e.g. en, fr, de" />
              <span class="form-hint">Leave empty to use the template default.</span>
            </div>
          </div>
          <div class="form-group" style="margin-top: 16px;">
            <label class="form-label">Template Data (JSON)</label>
            <textarea
              v-model="sampleData"
              class="form-textarea"
              rows="5"
              placeholder='{"name": "John", "company": "Acme"}'
              @keydown.ctrl.enter="renderPreview"
              @keydown.meta.enter="renderPreview"
            ></textarea>
            <span class="form-hint">JSON variables to inject into the template. Press Ctrl+Enter to render.</span>
          </div>
        </div>
      </div>

      <!-- Error -->
      <div v-if="previewError" class="preview-error" style="margin-bottom: 24px;">
        {{ previewError }}
      </div>

      <!-- Rendered Preview -->
      <template v-if="preview">
        <div class="card" style="margin-bottom: 24px;">
          <div class="card-header"><h2>Rendered Subject</h2></div>
          <div class="card-body">
            <div style="font-size: 16px; font-weight: 500; color: var(--text-primary);">{{ preview.subject }}</div>
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
                <iframe :srcdoc="preview.html" sandbox="" class="preview-iframe"></iframe>
              </div>
              <div v-else class="text-muted" style="text-align: center; padding: 20px 0;">
                No HTML content rendered.
              </div>
            </div>
            <pre
              v-else
              style="white-space: pre-wrap; word-wrap: break-word; font-size: 14px; color: var(--text-secondary); line-height: 1.6; margin: 0;"
            >{{ preview.text || 'No text content rendered.' }}</pre>
          </div>
        </div>
      </template>
    </template>
  </div>
</template>

<style scoped>
.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.preview-error {
  padding: 12px 16px;
  background: var(--danger-50);
  color: var(--danger-600);
  border-radius: var(--radius);
  font-size: 13px;
}

.preview-html-frame {
  border: 1px solid var(--border-primary);
  border-radius: var(--radius);
  overflow: hidden;
}

.preview-iframe {
  width: 100%;
  min-height: 400px;
  border: none;
  background: #fff;
}

@media (max-width: 640px) {
  .form-grid {
    grid-template-columns: 1fr;
  }
}
</style>
