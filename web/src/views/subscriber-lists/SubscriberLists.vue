<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { subscriberListsApi } from '../../api/subscriberLists'
import type { SubscriberListItem, SubscriberListType, FilterRule, Pageable } from '../../api/types'
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

const lists = ref<SubscriberListItem[]>([])
const loading = ref(true)

// Create modal
const showModal = ref(false)
const formName = ref('')
const formDescription = ref('')
const formType = ref<SubscriberListType>('static')
const formFilterRules = ref<FilterRule[]>([])
const saving = ref(false)

const operatorOptions = [
  { value: 'eq', label: 'equals' },
  { value: 'neq', label: 'not equals' },
  { value: 'contains', label: 'contains' },
  { value: 'starts_with', label: 'starts with' },
  { value: 'ends_with', label: 'ends with' },
  { value: 'gt', label: 'greater than' },
  { value: 'lt', label: 'less than' },
  { value: 'in', label: 'in' },
]

const { pageable, goToPage } = usePagination(async (page) => {
  loading.value = true
  try {
    const res = await subscriberListsApi.list(page, pageable.value.size)
    lists.value = res.data.data ?? []
    pageable.value = res.data.pageable
  } catch (e) {
    console.error('Failed to load suscriber lists', e)
  } finally {
    loading.value = false
  }
})

function openCreate() {
  formName.value = ''
  formDescription.value = ''
  formType.value = 'static'
  formFilterRules.value = []
  showModal.value = true
}

function addFilterRule() {
  formFilterRules.value.push({ field: '', operator: 'eq', value: '' })
}

function removeFilterRule(index: number) {
  formFilterRules.value.splice(index, 1)
}

async function saveList() {
  if (!formName.value.trim()) return
  saving.value = true
  try {
    await subscriberListsApi.create({
      name: formName.value.trim(),
      description: formDescription.value.trim(),
      type: formType.value,
      filter_rules: formType.value === 'dynamic' ? formFilterRules.value : undefined,
    })
    notify.success('Subscriber list created')
    showModal.value = false
    await goToPage(pageable.value.current_page)
  } catch (e: any) {
    notify.error(e?.response?.data?.error?.message || 'Failed to create subscriber list')
  } finally {
    saving.value = false
  }
}

async function deleteList(list: SubscriberListItem) {
  const confirmed = await confirm({
    title: 'Delete List',
    message: `Are you sure you want to delete "${list.name}"? This action cannot be undone.`,
    confirmText: 'Delete',
    variant: 'danger',
  })
  if (!confirmed) return
  try {
    await subscriberListsApi.delete(list.id)
    notify.success('Subscriber list deleted')
    await goToPage(pageable.value.current_page)
  } catch {
    notify.error('Failed to delete subscriber list')
  }
}

function typeBadgeClass(listType: SubscriberListType): string {
  return listType === 'dynamic' ? 'badge badge-primary' : 'badge badge-neutral'
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
}

const { watchClickStart, confirmClickEnd } = useModalSafeClose(() => {
  showModal.value = false
})

</script>

<template>
  <div>
    <div class="page-header">
      <h1>Lists</h1>
      <button v-if="wsStore.canEdit" class="btn btn-primary" @click="openCreate">Create List</button>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <div v-else class="card">
      <div v-if="lists.length === 0" class="empty-state">
        <h3>No Lists</h3>
        <p>Create a segment to organize your subscribers.</p>
      </div>

      <template v-else>
        <div class="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Type</th>
                <th>Members</th>
                <th>Created</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="list in lists" :key="list.id">
                <td>
                  <a class="link" @click="router.push(`/subscriber-lists/${list.id}`)">{{ list.name }}</a>
                </td>
                <td><span :class="typeBadgeClass(list.type)">{{ list.type }}</span></td>
                <td>{{ list.member_count }}</td>
                <td>{{ formatDate(list.created_at) }}</td>
                <td>
                  <div style="display: flex; gap: 6px">
                    <button class="btn btn-secondary btn-sm" @click="router.push(`/subscriber-lists/${list.id}`)">View</button>
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

    <!-- Create Modal -->
    <div v-if="showModal" class="modal-overlay" @mousedown="watchClickStart" @mouseup="confirmClickEnd">
      <div class="modal" @mousedown.stop @mouseup.stop>
        <div class="modal-header">
          <h3>Create List</h3>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label class="form-label">Name</label>
            <input v-model="formName" class="form-input" placeholder="e.g. Active Users" @keyup.enter="saveList" />
          </div>
          <div class="form-group">
            <label class="form-label">Description</label>
            <input v-model="formDescription" class="form-input" placeholder="Optional description" />
          </div>
          <div class="form-group">
            <label class="form-label">Type</label>
            <select v-model="formType" class="form-select">
              <option value="static">Static</option>
              <option value="dynamic">Dynamic</option>
            </select>
          </div>

          <template v-if="formType === 'dynamic'">
            <div class="form-group">
              <label class="form-label">Filter Rules</label>
              <div v-for="(rule, index) in formFilterRules" :key="index" style="display: flex; gap: 8px; margin-bottom: 8px; align-items: center">
                <input v-model="rule.field" class="form-input" placeholder="Field" style="flex: 1" />
                <select v-model="rule.operator" class="form-select" style="flex: 1">
                  <option v-for="op in operatorOptions" :key="op.value" :value="op.value">{{ op.label }}</option>
                </select>
                <input v-model="rule.value" class="form-input" placeholder="Value" style="flex: 1" />
                <button class="btn btn-danger btn-sm" @click="removeFilterRule(index)">Remove</button>
              </div>
              <button class="btn btn-secondary btn-sm" @click="addFilterRule">Add Rule</button>
            </div>
          </template>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="showModal = false">Cancel</button>
          <button class="btn btn-primary" :disabled="saving || !formName.trim()" @click="saveList">
            {{ saving ? 'Creating...' : 'Create' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.link {
  color: var(--primary-600);
  cursor: pointer;
  font-weight: 500;
}
.link:hover {
  text-decoration: underline;
}
</style>
