<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { stylesheetsApi } from '../../api/stylesheets'
import type { StyleSheet, StyleSheetInput, Pageable } from '../../api/types'
import { useNotificationStore } from '../../stores/notification'
import { useConfirm } from '../../composables/useConfirm'
import { useModalSafeClose } from '../../composables/useModalSafeClose';

const notify = useNotificationStore()
const { confirm } = useConfirm()

const stylesheets = ref<StyleSheet[]>([])
const pageable = ref<Pageable>({ current_page: 0, size: 20, total_pages: 0, total_elements: 0, empty: true })
const loading = ref(true)

const showModal = ref(false)
const editing = ref<StyleSheet | null>(null)
const saving = ref(false)

const form = ref<StyleSheetInput>({
  name: '',
  css: '',
})

function resetForm() {
  form.value = { name: '', css: '' }
  editing.value = null
}

function openCreate() {
  resetForm()
  showModal.value = true
}

function openEdit(sheet: StyleSheet) {
  editing.value = sheet
  form.value = {
    name: sheet.name,
    css: sheet.css,
  }
  showModal.value = true
}

function closeModal() {
  showModal.value = false
  resetForm()
}

async function loadStylesheets(page = 0) {
  loading.value = true
  try {
    const res = await stylesheetsApi.list(page, pageable.value.size)
    stylesheets.value = res.data.data
    pageable.value = res.data.pageable
  } catch {
    notify.error('Failed to load stylesheets')
  } finally {
    loading.value = false
  }
}

async function saveStylesheet() {
  if (!form.value.name.trim()) return
  saving.value = true
  try {
    if (editing.value) {
      await stylesheetsApi.update(editing.value.id, form.value)
      notify.success('Stylesheet updated')
    } else {
      await stylesheetsApi.create(form.value)
      notify.success('Stylesheet created')
    }
    closeModal()
    await loadStylesheets(pageable.value.current_page)
  } catch {
    notify.error(editing.value ? 'Failed to update stylesheet' : 'Failed to create stylesheet')
  } finally {
    saving.value = false
  }
}

async function deleteStylesheet(sheet: StyleSheet) {
  const confirmed = await confirm({
    title: 'Delete Stylesheet',
    message: `Are you sure you want to delete "${sheet.name}"? Templates using this stylesheet will lose their styling.`,
    confirmText: 'Delete',
    variant: 'danger',
  })
  if (!confirmed) return
  try {
    await stylesheetsApi.delete(sheet.id)
    notify.success('Stylesheet deleted')
    await loadStylesheets(pageable.value.current_page)
  } catch {
    notify.error('Failed to delete stylesheet')
  }
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
}
const { watchClickStart, confirmClickEnd } = useModalSafeClose(() => {
  closeModal()
}); 
onMounted(() => loadStylesheets())
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Stylesheets</h1>
      <button class="btn btn-primary" @click="openCreate">Create Stylesheet</button>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <div v-else class="card">
      <div v-if="stylesheets.length === 0" class="empty-state">
        <h3>No Stylesheets</h3>
        <p>Create a stylesheet to reuse CSS across your email templates.</p>
      </div>

      <template v-else>
        <div class="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Created</th>
                <th>Updated</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="sheet in stylesheets" :key="sheet.id">
                <td>{{ sheet.name }}</td>
                <td>{{ formatDate(sheet.created_at) }}</td>
                <td>
                  <span v-if="sheet.updated_at">{{ formatDate(sheet.updated_at) }}</span>
                  <span v-else class="text-muted">&mdash;</span>
                </td>
                <td>
                  <div class="flex gap-2">
                    <button class="btn btn-secondary btn-sm" @click="openEdit(sheet)">Edit</button>
                    <button class="btn btn-danger btn-sm" @click="deleteStylesheet(sheet)">Delete</button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="pagination">
          <span class="pagination-info">
            Page {{ pageable.current_page + 1 }} of {{ pageable.total_pages }} ({{ pageable.total_elements }} stylesheets)
          </span>
          <div class="pagination-buttons">
            <button
              class="btn btn-secondary btn-sm"
              :disabled="pageable.current_page === 0"
              @click="loadStylesheets(pageable.current_page - 1)"
            >
              Previous
            </button>
            <button
              class="btn btn-secondary btn-sm"
              :disabled="pageable.current_page >= pageable.total_pages - 1"
              @click="loadStylesheets(pageable.current_page + 1)"
            >
              Next
            </button>
          </div>
        </div>
      </template>
    </div>

    <!-- Create/Edit Stylesheet Modal -->
    <div v-if="showModal" class="modal-overlay" @mousedown="watchClickStart" 
      @mouseup="confirmClickEnd">
      <div class="modal" style="max-width: 640px;" @mousedown.stop @mouseup.stop>
        <div class="modal-header">
          <h3>{{ editing ? 'Edit Stylesheet' : 'Create Stylesheet' }}</h3>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label class="form-label">Name</label>
            <input v-model="form.name" class="form-input" placeholder="e.g. Brand Styles" />
          </div>
          <div class="form-group">
            <label class="form-label">CSS</label>
            <textarea v-model="form.css" class="form-textarea css-editor" rows="12" placeholder="body { font-family: sans-serif; }"></textarea>
            <span class="form-hint">This CSS will be injected as a &lt;style&gt; block into templates that use this stylesheet.</span>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="closeModal">Cancel</button>
          <button
            class="btn btn-primary"
            :disabled="saving || !form.name.trim()"
            @click="saveStylesheet"
          >
            {{ saving ? 'Saving...' : (editing ? 'Update' : 'Create') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.css-editor {
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 13px;
}
</style>
