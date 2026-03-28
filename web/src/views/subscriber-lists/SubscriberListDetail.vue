<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { subscriberListsApi } from '../../api/subscriberLists'
import { subscribersApi } from '../../api/subscribers'
import type { SubscriberListItem, Subscriber, FilterRule, Pageable } from '../../api/types'
import { useNotificationStore } from '../../stores/notification'
import { useConfirm } from '../../composables/useConfirm'
import { useModalSafeClose } from '../../composables/useModalSafeClose'

const route = useRoute()
const router = useRouter()
const notify = useNotificationStore()
const { confirm } = useConfirm()

const listId = Number(route.params.id)
const list = ref<SubscriberListItem | null>(null)
const members = ref<Subscriber[]>([])
const pageable = ref<Pageable>({ current_page: 0, size: 20, total_pages: 0, total_elements: 0, empty: true })
const loading = ref(true)
const saving = ref(false)

// Edit form
const editName = ref('')
const editDescription = ref('')

// Add member (static lists)
const showAddModal = ref(false)
const allSubscribers = ref<Subscriber[]>([])
const selectedSubscriberId = ref<number | null>(null)
const addingMember = ref(false)

// Dynamic preview
const previewMembers = ref<Subscriber[]>([])
const previewLoading = ref(false)
const previewCount = ref<number | null>(null)

const isDynamic = computed(() => list.value?.type === 'dynamic')

const operatorLabels: Record<string, string> = {
  eq: 'equals',
  neq: 'not equals',
  contains: 'contains',
  starts_with: 'starts with',
  ends_with: 'ends with',
  gt: 'greater than',
  lt: 'less than',
  in: 'in',
}

async function loadList() {
  try {
    const res = await subscriberListsApi.get(listId)
    list.value = res.data.data
    editName.value = list.value.name
    editDescription.value = list.value.description
  } catch {
    notify.error('Subscriber list not found')
    router.push({ name: 'subscriber-lists-page' })
  }
}

async function loadMembers(page = 0) {
  loading.value = true
  try {
    const res = await subscriberListsApi.listMembers(listId, page, pageable.value.size)
    members.value = res.data.data ?? []
    pageable.value = res.data.pageable
  } catch {
    notify.error('Failed to load members')
  } finally {
    loading.value = false
  }
}

async function saveDetails() {
  if (!list.value || !editName.value.trim()) return
  saving.value = true
  try {
    const res = await subscriberListsApi.update(listId, {
      name: editName.value.trim(),
      description: editDescription.value.trim(),
    })
    list.value = res.data.data
    notify.success('List updated')
  } catch (e: any) {
    notify.error(e?.response?.data?.error?.message || 'Failed to update list')
  } finally {
    saving.value = false
  }
}

async function openAddMember() {
  selectedSubscriberId.value = null
  showAddModal.value = true
  try {
    const res = await subscribersApi.list(0, 100)
    allSubscribers.value = res.data.data ?? []
  } catch {
    notify.error('Failed to load subscribers')
  }
}

async function addMember() {
  if (!selectedSubscriberId.value) return
  addingMember.value = true
  try {
    await subscriberListsApi.addMember(listId, selectedSubscriberId.value)
    notify.success('Member added')
    showAddModal.value = false
    await loadMembers(pageable.value.current_page)
    await loadList()
  } catch (e: any) {
    notify.error(e?.response?.data?.error?.message || 'Failed to add member')
  } finally {
    addingMember.value = false
  }
}

async function removeMember(subscriber: Subscriber) {
  const confirmed = await confirm({
    title: 'Remove Member',
    message: `Remove "${subscriber.email}" from this list?`,
    confirmText: 'Remove',
    variant: 'danger',
  })
  if (!confirmed) return
  try {
    await subscriberListsApi.removeMember(listId, subscriber.id)
    notify.success('Member removed')
    await loadMembers(pageable.value.current_page)
    await loadList()
  } catch {
    notify.error('Failed to remove member')
  }
}

async function previewSegment() {
  if (!list.value?.filter_rules) return
  previewLoading.value = true
  try {
    const res = await subscriberListsApi.previewSegment(list.value.filter_rules)
    previewMembers.value = res.data.data ?? []
    previewCount.value = res.data.pageable.total_elements
  } catch {
    notify.error('Failed to preview segment')
  } finally {
    previewLoading.value = false
  }
}

function statusBadgeClass(status: string): string {
  switch (status) {
    case 'subscribed': return 'badge badge-primary'
    case 'unsubscribed': return 'badge badge-neutral'
    case 'bounced': return 'badge badge-warning'
    case 'complained': return 'badge badge-warning'
    default: return 'badge'
  }
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
}

const { watchClickStart, confirmClickEnd } = useModalSafeClose(() => {
  showAddModal.value = false
})

onMounted(async () => {
  await loadList()
  await loadMembers()
})
</script>

<template>
  <div>
    <div class="page-header">
      <div>
        <div class="breadcrumb">
          <router-link :to="{ name: 'subscriber-lists-page' }">Lists</router-link>
          <span class="separator">/</span>
          <span>{{ list?.name || '...' }}</span>
        </div>
        <h1>{{ list?.name || 'List Detail' }}</h1>
        <p v-if="list?.description" class="page-description">{{ list.description }}</p>
      </div>
      <button v-if="!isDynamic" class="btn btn-primary" @click="openAddMember">Add Member</button>
    </div>

    <!-- Edit Details -->
    <div class="card" style="margin-bottom: 24px">
      <div class="card-header">
        <h2>Details</h2>
      </div>
      <div class="card-body">
        <div class="form-group">
          <label class="form-label">Name</label>
          <input v-model="editName" class="form-input" />
        </div>
        <div class="form-group">
          <label class="form-label">Description</label>
          <input v-model="editDescription" class="form-input" />
        </div>
        <div style="margin-top: 12px">
          <button class="btn btn-primary" :disabled="saving || !editName.trim()" @click="saveDetails">
            {{ saving ? 'Saving...' : 'Save' }}
          </button>
        </div>
      </div>
    </div>

    <!-- Dynamic: Filter Rules -->
    <div v-if="isDynamic && list?.filter_rules" class="card" style="margin-bottom: 24px">
      <div class="card-header">
        <h2>Filter Rules</h2>
        <button class="btn btn-secondary" :disabled="previewLoading" @click="previewSegment">
          {{ previewLoading ? 'Loading...' : 'Preview Matches' }}
        </button>
      </div>
      <div class="card-body">
        <div v-if="list.filter_rules.length === 0" class="empty-state">
          <p>No filter rules defined.</p>
        </div>
        <table v-else>
          <thead>
            <tr>
              <th>Field</th>
              <th>Operator</th>
              <th>Value</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(rule, index) in list.filter_rules" :key="index">
              <td>{{ rule.field }}</td>
              <td>{{ operatorLabels[rule.operator] || rule.operator }}</td>
              <td>{{ typeof rule.value === 'object' ? JSON.stringify(rule.value) : String(rule.value) }}</td>
            </tr>
          </tbody>
        </table>

        <div v-if="previewCount !== null" style="margin-top: 16px">
          <p><strong>{{ previewCount }}</strong> subscribers match these rules.</p>
        </div>
      </div>
    </div>

    <!-- Dynamic: Preview Results -->
    <div v-if="isDynamic && previewMembers.length > 0" class="card" style="margin-bottom: 24px">
      <div class="card-header">
        <h2>Matching Subscribers</h2>
      </div>
      <div class="table-wrapper">
        <table>
          <thead>
            <tr>
              <th>Email</th>
              <th>Name</th>
              <th>Status</th>
              <th>Created</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="sub in previewMembers" :key="sub.id">
              <td>{{ sub.email }}</td>
              <td>{{ sub.name || '-' }}</td>
              <td><span :class="statusBadgeClass(sub.status)">{{ sub.status }}</span></td>
              <td>{{ formatDate(sub.created_at) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Static: Members Table -->
    <div v-if="!isDynamic" class="card">
      <div class="card-header">
        <h2>Members</h2>
      </div>

      <div v-if="loading" class="loading-page">
        <div class="spinner"></div>
      </div>

      <template v-else>
        <div v-if="members.length === 0" class="empty-state">
          <h3>No Members</h3>
          <p>Add subscribers to this list.</p>
        </div>

        <template v-else>
          <div class="table-wrapper">
            <table>
              <thead>
                <tr>
                  <th>Email</th>
                  <th>Name</th>
                  <th>Status</th>
                  <th>Created</th>
                  <th style="width: 1%"></th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="member in members" :key="member.id">
                  <td>{{ member.email }}</td>
                  <td>{{ member.name || '-' }}</td>
                  <td><span :class="statusBadgeClass(member.status)">{{ member.status }}</span></td>
                  <td>{{ formatDate(member.created_at) }}</td>
                  <td>
                    <button class="btn btn-danger btn-sm" @click="removeMember(member)">Remove</button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <div class="pagination">
            <span class="pagination-info">
              Page {{ pageable.current_page + 1 }} of {{ pageable.total_pages }} ({{ pageable.total_elements }} members)
            </span>
            <div class="pagination-buttons">
              <button class="btn btn-secondary btn-sm" :disabled="pageable.current_page === 0" @click="loadMembers(pageable.current_page - 1)">Previous</button>
              <button class="btn btn-secondary btn-sm" :disabled="pageable.current_page >= pageable.total_pages - 1" @click="loadMembers(pageable.current_page + 1)">Next</button>
            </div>
          </div>
        </template>
      </template>
    </div>

    <!-- Add Member Modal -->
    <div v-if="showAddModal" class="modal-overlay" @mousedown="watchClickStart" @mouseup="confirmClickEnd">
      <div class="modal" @mousedown.stop @mouseup.stop>
        <div class="modal-header">
          <h2>Add Member</h2>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label class="form-label">Subscriber</label>
            <select v-model="selectedSubscriberId" class="form-select">
              <option :value="null" disabled>Select a subscriber...</option>
              <option v-for="sub in allSubscribers" :key="sub.id" :value="sub.id">
                {{ sub.email }} {{ sub.name ? `(${sub.name})` : '' }}
              </option>
            </select>
          </div>
          <div v-if="allSubscribers.length === 0" class="empty-hint">
            No subscribers found. Create subscribers first.
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="showAddModal = false">Cancel</button>
          <button class="btn btn-primary" :disabled="addingMember || !selectedSubscriberId" @click="addMember">
            {{ addingMember ? 'Adding...' : 'Add' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.breadcrumb {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  margin-bottom: 4px;
  color: var(--text-secondary);
}

.breadcrumb a {
  color: var(--primary-600);
  text-decoration: none;
}

.breadcrumb a:hover {
  text-decoration: underline;
}

.breadcrumb .separator {
  color: var(--text-tertiary);
}

.page-description {
  color: var(--text-secondary);
  margin-top: 4px;
  font-size: 14px;
}

.empty-hint {
  color: var(--text-secondary);
  font-size: 13px;
  font-style: italic;
}
</style>
