<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { unsubscribeListsApi } from '../../api/unsubscribeLists'
import type { UnsubscribeListItem, UnsubscribeListInput } from '../../api/types'
import { useNotificationStore } from '../../stores/notification'
import { useConfirm } from '../../composables/useConfirm'
import { useModalSafeClose } from '../../composables/useModalSafeClose'
import { useWorkspaceStore } from '../../stores/workspace'
import { usePagination } from '@/composables/usePagination'
import Pagination from '@/components/Pagination.vue'

const router = useRouter()
const notify = useNotificationStore()
const wsStore = useWorkspaceStore()
const { confirm } = useConfirm()

const lists = ref<UnsubscribeListItem[]>([])
const loading = ref(true)
const showInfo = ref(false)

async function copyListId(id: number) {
  try {
    await navigator.clipboard.writeText(String(id))
    notify.success(`List ID ${id} copied`)
  } catch {
    notify.error('Failed to copy')
  }
}

const showModal = ref(false)
const editing = ref<UnsubscribeListItem | null>(null)
const saving = ref(false)

const form = ref<UnsubscribeListInput>({
  name: '',
  public_name: '',
  description: '',
  active: true,
})

function resetForm() {
  form.value = { name: '', public_name: '', description: '', active: true }
  editing.value = null
}

function openCreate() {
  resetForm()
  showModal.value = true
}

function openEdit(list: UnsubscribeListItem) {
  editing.value = list
  form.value = {
    name: list.name,
    public_name: list.public_name || '',
    description: list.description || '',
    active: list.active,
  }
  showModal.value = true
}

function closeModal() {
  showModal.value = false
  resetForm()
}

const { pageable, goToPage } = usePagination(async (page) => {
  loading.value = true
  try {
    const res = await unsubscribeListsApi.list(page, pageable.value.size)
    lists.value = res.data.data
    pageable.value = res.data.pageable
  } catch (e) {
    console.error('Failed to load unsubscribe lists', e)
  } finally {
    loading.value = false
  }
})

async function saveList() {
  if (!form.value.name.trim()) return
  saving.value = true
  try {
    if (editing.value) {
      await unsubscribeListsApi.update(editing.value.id, form.value)
      notify.success('Unsubscribe list updated')
    } else {
      await unsubscribeListsApi.create(form.value)
      notify.success('Unsubscribe list created')
    }
    closeModal()
    await goToPage(pageable.value.current_page)
  } catch {
    notify.error(editing.value ? 'Failed to update unsubscribe list' : 'A list with that name already exists')
  } finally {
    saving.value = false
  }
}

async function deleteList(list: UnsubscribeListItem) {
  const confirmed = await confirm({
    title: 'Delete Unsubscribe List',
    message: `Delete "${list.name}"? Existing per-list opt-outs that reference it will remain on the suppression list.`,
    confirmText: 'Delete',
    variant: 'danger',
  })
  if (!confirmed) return
  try {
    await unsubscribeListsApi.delete(list.id)
    notify.success('Unsubscribe list deleted')
    await goToPage(pageable.value.current_page)
  } catch {
    notify.error('Failed to delete unsubscribe list')
  }
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
}

const { watchClickStart, confirmClickEnd } = useModalSafeClose(() => {
  closeModal()
})
</script>

<template>
  <div>
    <div class="page-header">
      <div style="display: flex; align-items: center; gap: 6px;">
        <h1>Unsubscribe Lists</h1>
        <button
          type="button"
          @click="showInfo = !showInfo"
          :title="showInfo ? 'Hide help' : 'How to use unsubscribe lists'"
          aria-label="Toggle help"
          style="background:transparent;border:0;cursor:pointer;color:var(--text-secondary,#6b7280);display:inline-flex;align-items:center;justify-content:center;padding:6px;border-radius:6px;"
        >
          <svg width="18" height="18" viewBox="0 0 18 18" fill="none"><circle cx="9" cy="9" r="7" stroke="currentColor" stroke-width="1.5"/><path d="M9 12.75V9M9 5.25h.007" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>
        </button>
      </div>
      <button v-if="wsStore.canEdit" class="btn btn-primary" @click="openCreate">Add List</button>
    </div>

    <div v-if="showInfo" class="card" style="margin-bottom:16px;display:flex;gap:12px;align-items:flex-start;">
      <svg width="20" height="20" viewBox="0 0 18 18" fill="none" style="flex-shrink:0;color:#9333ea;margin-top:2px;"><circle cx="9" cy="9" r="7" stroke="currentColor" stroke-width="1.5"/><path d="M9 12.75V9M9 5.25h.007" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>
      <div style="flex:1;min-width:0;">
        <h3 style="margin-top:0;margin-bottom:8px;">Where to use Unsubscribe Lists</h3>
        <p style="margin:0 0 8px 0;">
          Reference a list from a transactional send with <code>unsubscribe.list_id</code>.<br>
          Posta mints the signed one-click URL and emits the <code>List-Unsubscribe</code> header.<br>
          A click opts the recipient out of <strong>that list only</strong> and their receipts, password resets, and other transactional mail keep flowing.
        </p>
        <pre class="code-block" style="margin:0;font-size:12px;line-height:1.5;"><code>POST /api/v1/emails/send
{
  "to": ["user@example.com"],
  "subject": "Product update",
  "html": "...",
  "unsubscribe": { "list_id": 42 }
}</code></pre>
        <p style="margin:10px 0 0 0;color:var(--text-secondary,#6b7280);font-size:13px;">
          Click a list name below to see who opted out and to resubscribe individual addresses.
        </p>
      </div>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <div v-else class="card">
      <div v-if="lists.length === 0" class="empty-state">
        <h3>No Unsubscribe Lists</h3>
        <p>Create a list, then reference it from a send with <code>unsubscribe.list_id</code>. A one-click then opts the recipient out of that list only.</p>
      </div>

      <template v-else>
        <div class="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Public Name</th>
                <th>Description</th>
                <th>Status</th>
                <th>Created</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="list in lists" :key="list.id">
                <td>
                  <a class="link" @click="router.push(`/unsubscribe-lists/${list.id}`)">{{ list.name }}</a>
                  <span
                    class="badge badge-neutral"
                    :title="`Copy list ID ${list.id}`"
                    style="cursor:pointer;user-select:none;"
                    @click.stop="copyListId(list.id)"
                  >#{{ list.id }}</span>
                </td>
                <td>{{ list.public_name || '—' }}</td>
                <td>{{ list.description || '—' }}</td>
                <td>
                  <span v-if="list.active" class="badge badge-success">Active</span>
                  <span v-else class="badge badge-neutral">Archived</span>
                </td>
                <td>{{ formatDate(list.created_at) }}</td>
                <td>
                  <div class="flex gap-2">
                    <button class="btn btn-secondary btn-sm" @click="router.push(`/unsubscribe-lists/${list.id}`)">View</button>
                    <button v-if="wsStore.canEdit" class="btn btn-secondary btn-sm" @click="openEdit(list)">Edit</button>
                    <button v-if="wsStore.canEdit" class="btn btn-danger btn-sm" @click="deleteList(list)">Delete</button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <Pagination :pageable="pageable" @page="goToPage" />
      </template>
    </div>

    <!-- Create/Edit Unsubscribe List Modal -->
    <div v-if="showModal" class="modal-overlay" @mousedown="watchClickStart" @mouseup="confirmClickEnd">
      <div class="modal" style="max-width: 480px;" @mousedown.stop @mouseup.stop>
        <div class="modal-header">
          <h3>{{ editing ? 'Edit Unsubscribe List' : 'Add Unsubscribe List' }}</h3>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label class="form-label">Name</label>
            <input v-model="form.name" class="form-input" placeholder="e.g. Product updates" />
            <span class="form-hint">Internal label, unique within your account.</span>
          </div>
          <div class="form-group">
            <label class="form-label">Public name</label>
            <input v-model="form.public_name" class="form-input" placeholder="Shown to recipients (optional)" />
            <span class="form-hint">Displayed on the unsubscribe confirmation page. Falls back to the name.</span>
          </div>
          <div class="form-group">
            <label class="form-label">Description</label>
            <input v-model="form.description" class="form-input" placeholder="Optional" />
          </div>
          <div v-if="editing" class="form-group">
            <label class="checkbox-label">
              <input type="checkbox" v-model="form.active" />
              Active
            </label>
            <span class="form-hint">Archive a list to retire it without deleting its opt-out history.</span>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="closeModal">Cancel</button>
          <button class="btn btn-primary" :disabled="saving || !form.name.trim()" @click="saveList">
            {{ saving ? 'Saving...' : (editing ? 'Update' : 'Create') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
