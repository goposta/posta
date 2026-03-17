<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { languagesApi } from '../../api/languages'
import type { Language, LanguageInput, Pageable } from '../../api/types'
import { useNotificationStore } from '../../stores/notification'
import { useConfirm } from '../../composables/useConfirm'
import { useModalSafeClose } from '../../composables/useModalSafeClose';

const notify = useNotificationStore()
const { confirm } = useConfirm()

const languages = ref<Language[]>([])
const pageable = ref<Pageable>({ current_page: 0, size: 20, total_pages: 0, total_elements: 0, empty: true })
const loading = ref(true)

const showModal = ref(false)
const editing = ref<Language | null>(null)
const saving = ref(false)

const form = ref<LanguageInput>({
  code: '',
  name: '',
})

function resetForm() {
  form.value = { code: '', name: '' }
  editing.value = null
}

function openCreate() {
  resetForm()
  showModal.value = true
}

function openEdit(lang: Language) {
  editing.value = lang
  form.value = {
    code: lang.code,
    name: lang.name,
  }
  showModal.value = true
}

function closeModal() {
  showModal.value = false
  resetForm()
}

async function loadLanguages(page = 0) {
  loading.value = true
  try {
    const res = await languagesApi.list(page, pageable.value.size)
    languages.value = res.data.data
    pageable.value = res.data.pageable
  } catch {
    notify.error('Failed to load languages')
  } finally {
    loading.value = false
  }
}

async function saveLanguage() {
  if (!form.value.code.trim() || !form.value.name.trim()) return
  saving.value = true
  try {
    if (editing.value) {
      await languagesApi.update(editing.value.id, form.value)
      notify.success('Language updated')
    } else {
      await languagesApi.create(form.value)
      notify.success('Language created')
    }
    closeModal()
    await loadLanguages(pageable.value.current_page)
  } catch {
    notify.error(editing.value ? 'Failed to update language' : 'Language code already exists')
  } finally {
    saving.value = false
  }
}

async function deleteLanguage(lang: Language) {
  const confirmed = await confirm({
    title: 'Delete Language',
    message: `Are you sure you want to delete "${lang.name} (${lang.code})"?`,
    confirmText: 'Delete',
    variant: 'danger',
  })
  if (!confirmed) return
  try {
    await languagesApi.delete(lang.id)
    notify.success('Language deleted')
    await loadLanguages(pageable.value.current_page)
  } catch {
    notify.error('Failed to delete language')
  }
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
}
const { watchClickStart, confirmClickEnd } = useModalSafeClose(() => {
  closeModal()
}); 
onMounted(() => loadLanguages())
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Languages</h1>
      <button class="btn btn-primary" @click="openCreate">Add Language</button>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <div v-else class="card">
      <div v-if="languages.length === 0" class="empty-state">
        <h3>No Languages</h3>
        <p>Add languages to use for template localizations.</p>
      </div>

      <template v-else>
        <div class="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>Code</th>
                <th>Name</th>
                <th>Created</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="lang in languages" :key="lang.id">
                <td><span class="badge badge-neutral">{{ lang.code }}</span></td>
                <td>{{ lang.name }}</td>
                <td>{{ formatDate(lang.created_at) }}</td>
                <td>
                  <div class="flex gap-2">
                    <button class="btn btn-secondary btn-sm" @click="openEdit(lang)">Edit</button>
                    <button class="btn btn-danger btn-sm" @click="deleteLanguage(lang)">Delete</button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="pagination">
          <span class="pagination-info">
            Page {{ pageable.current_page + 1 }} of {{ pageable.total_pages }} ({{ pageable.total_elements }} languages)
          </span>
          <div class="pagination-buttons">
            <button
              class="btn btn-secondary btn-sm"
              :disabled="pageable.current_page === 0"
              @click="loadLanguages(pageable.current_page - 1)"
            >
              Previous
            </button>
            <button
              class="btn btn-secondary btn-sm"
              :disabled="pageable.current_page >= pageable.total_pages - 1"
              @click="loadLanguages(pageable.current_page + 1)"
            >
              Next
            </button>
          </div>
        </div>
      </template>
    </div>

    <!-- Create/Edit Language Modal -->
    <div v-if="showModal" class="modal-overlay" @mousedown="watchClickStart" 
      @mouseup="confirmClickEnd">
      <div class="modal" style="max-width: 480px;" @mousedown.stop @mouseup.stop>
        <div class="modal-header">
          <h3>{{ editing ? 'Edit Language' : 'Add Language' }}</h3>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label class="form-label">Code</label>
            <input v-model="form.code" class="form-input" placeholder="e.g. en, fr, de" maxlength="10" />
            <span class="form-hint">ISO 639-1 language code</span>
          </div>
          <div class="form-group">
            <label class="form-label">Name</label>
            <input v-model="form.name" class="form-input" placeholder="e.g. English, French" />
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="closeModal">Cancel</button>
          <button
            class="btn btn-primary"
            :disabled="saving || !form.code.trim() || !form.name.trim()"
            @click="saveLanguage"
          >
            {{ saving ? 'Saving...' : (editing ? 'Update' : 'Create') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
