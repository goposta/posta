<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { bouncesApi, suppressionsApi } from '../../api/bounces'
import type { Bounce, Suppression, Pageable } from '../../api/types'
import { useNotificationStore } from '../../stores/notification'
import { useConfirm } from '../../composables/useConfirm'
import { useModalSafeClose } from '../../composables/useModalSafeClose';

const notify = useNotificationStore()
const { confirm } = useConfirm()

const activeTab = ref<'bounces' | 'suppressions'>('bounces')
const loading = ref(true)

const bounces = ref<Bounce[]>([])
const bouncesPageable = ref<Pageable | null>(null)
const bouncesPage = ref(0)

const suppressions = ref<Suppression[]>([])
const suppressionsPageable = ref<Pageable | null>(null)
const suppressionsPage = ref(0)

const showAddModal = ref(false)
const addForm = ref({ email: '', reason: '' })

onMounted(() => {
  loadBounces()
  loadSuppressions()
})

async function loadBounces() {
  loading.value = true
  try {
    const res = await bouncesApi.list(bouncesPage.value)
    bounces.value = res.data.data
    bouncesPageable.value = res.data.pageable
  } catch (e) {
    notify.error('Failed to load bounces')
  } finally {
    loading.value = false
  }
}

async function loadSuppressions() {
  try {
    const res = await suppressionsApi.list(suppressionsPage.value)
    suppressions.value = res.data.data
    suppressionsPageable.value = res.data.pageable
  } catch (e) {
    notify.error('Failed to load suppressions')
  }
}

function switchTab(tab: 'bounces' | 'suppressions') {
  activeTab.value = tab
}

function bounceBadgeClass(type: string) {
  switch (type) {
    case 'hard': return 'badge badge-danger'
    case 'soft': return 'badge badge-warning'
    case 'complaint': return 'badge badge-info'
    default: return 'badge'
  }
}

function formatDate(date: string) {
  return new Date(date).toLocaleString()
}

async function changebouncesPage(page: number) {
  bouncesPage.value = page
  await loadBounces()
}

async function changeSuppressionsPage(page: number) {
  suppressionsPage.value = page
  await loadSuppressions()
}

async function deleteSuppression(email: string) {
  const confirmed = await confirm({
    title: 'Remove Suppression',
    message: `Are you sure you want to remove "${email}" from the suppression list? This address will be able to receive emails again.`,
    confirmText: 'Remove',
    variant: 'warning',
  })
  if (!confirmed) return
  try {
    await suppressionsApi.delete(email)
    notify.success('Suppression removed')
    await loadSuppressions()
  } catch (e) {
    notify.error('Failed to remove suppression')
  }
}

function openAddModal() {
  addForm.value = { email: '', reason: '' }
  showAddModal.value = true
}

async function addSuppression() {
  if (!addForm.value.email) {
    notify.error('Email is required')
    return
  }
  try {
    await suppressionsApi.create(addForm.value)
    notify.success('Suppression added')
    showAddModal.value = false
    await loadSuppressions()
  } catch (e) {
    notify.error('Failed to add suppression')
  }
}
const { watchClickStart, confirmClickEnd } = useModalSafeClose(() => {
  showAddModal.value = false;
});
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Bounces & Suppressions</h1>
    </div>

    <div class="tabs">
      <button class="tab" :class="{ active: activeTab === 'bounces' }" @click="switchTab('bounces')">Bounces</button>
      <button class="tab" :class="{ active: activeTab === 'suppressions' }" @click="switchTab('suppressions')">Suppressions</button>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <template v-else>
      <!-- Bounces Tab -->
      <div v-if="activeTab === 'bounces'" class="card">
        <div class="card-header">
          <h2>Bounces</h2>
        </div>
        <div v-if="bounces.length === 0" class="empty-state">
          <h3>No bounces</h3>
          <p>Bounced emails will appear here.</p>
        </div>
        <div v-else class="card-body">
          <table class="table">
            <thead>
              <tr>
                <th>Recipient</th>
                <th>Type</th>
                <th>Reason</th>
                <th>Date</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="bounce in bounces" :key="bounce.id">
                <td>{{ bounce.recipient }}</td>
                <td><span :class="bounceBadgeClass(bounce.type)">{{ bounce.type }}</span></td>
                <td>{{ bounce.reason }}</td>
                <td>{{ formatDate(bounce.created_at) }}</td>
              </tr>
            </tbody>
          </table>
          <div v-if="bouncesPageable && bouncesPageable.total_pages > 1" class="pagination">
            <button
              class="btn btn-sm btn-secondary"
              :disabled="bouncesPage === 0"
              @click="changebouncesPage(bouncesPage - 1)"
            >
              Previous
            </button>
            <span>Page {{ bouncesPage + 1 }} of {{ bouncesPageable.total_pages }}</span>
            <button
              class="btn btn-sm btn-secondary"
              :disabled="bouncesPage >= bouncesPageable.total_pages - 1"
              @click="changebouncesPage(bouncesPage + 1)"
            >
              Next
            </button>
          </div>
        </div>
      </div>

      <!-- Suppressions Tab -->
      <div v-if="activeTab === 'suppressions'" class="card">
        <div class="card-header">
          <h2>Suppressions</h2>
          <button class="btn btn-primary" @click="openAddModal">Add Suppression</button>
        </div>
        <div v-if="suppressions.length === 0" class="empty-state">
          <h3>No suppressions</h3>
          <p>Suppressed email addresses will appear here.</p>
        </div>
        <div v-else class="card-body">
          <table class="table">
            <thead>
              <tr>
                <th>Email</th>
                <th>Reason</th>
                <th>Created At</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="suppression in suppressions" :key="suppression.id">
                <td>{{ suppression.email }}</td>
                <td>{{ suppression.reason }}</td>
                <td>{{ formatDate(suppression.created_at) }}</td>
                <td>
                  <button class="btn btn-sm btn-danger" @click="deleteSuppression(suppression.email)">
                    Delete
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
          <div v-if="suppressionsPageable && suppressionsPageable.total_pages > 1" class="pagination">
            <button
              class="btn btn-sm btn-secondary"
              :disabled="suppressionsPage === 0"
              @click="changeSuppressionsPage(suppressionsPage - 1)"
            >
              Previous
            </button>
            <span>Page {{ suppressionsPage + 1 }} of {{ suppressionsPageable.total_pages }}</span>
            <button
              class="btn btn-sm btn-secondary"
              :disabled="suppressionsPage >= suppressionsPageable.total_pages - 1"
              @click="changeSuppressionsPage(suppressionsPage + 1)"
            >
              Next
            </button>
          </div>
        </div>
      </div>
    </template>

    <!-- Add Suppression Modal -->
    <div v-if="showAddModal" class="modal-overlay" @mousedown="watchClickStart" 
      @mouseup="confirmClickEnd">
      <div class="modal" @mousedown.stop @mouseup.stop>
        <div class="modal-header">
          <h2>Add Suppression</h2>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label class="form-label">Email</label>
            <input v-model="addForm.email" type="email" class="form-input" placeholder="user@example.com" />
          </div>
          <div class="form-group">
            <label class="form-label">Reason</label>
            <input v-model="addForm.reason" type="text" class="form-input" placeholder="Reason for suppression" />
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="showAddModal = false">Cancel</button>
          <button class="btn btn-primary" @click="addSuppression">Add</button>
        </div>
      </div>
    </div>
  </div>
</template>
