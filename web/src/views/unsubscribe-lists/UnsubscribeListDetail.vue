<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { unsubscribeListsApi } from '../../api/unsubscribeLists'
import { suppressionsApi } from '../../api/bounces'
import type { UnsubscribeListItem, Suppression, Pageable } from '../../api/types'
import { useNotificationStore } from '../../stores/notification'
import { useConfirm } from '../../composables/useConfirm'
import { useWorkspaceStore } from '../../stores/workspace'
import Pagination from '@/components/Pagination.vue'

const route = useRoute()
const router = useRouter()
const notify = useNotificationStore()
const wsStore = useWorkspaceStore()
const { confirm } = useConfirm()

const listId = Number(route.params.id)
const list = ref<UnsubscribeListItem | null>(null)
const optouts = ref<Suppression[]>([])
const pageable = ref<Pageable>({ current_page: 0, size: 20, total_pages: 0, total_elements: 0, empty: true })
const loading = ref(true)

async function loadList() {
  try {
    const res = await unsubscribeListsApi.get(listId)
    list.value = res.data.data
  } catch {
    notify.error('Unsubscribe list not found')
    router.push({ name: 'unsubscribe-lists-page' })
  }
}

async function loadOptouts(page = 0) {
  loading.value = true
  try {
    const res = await suppressionsApi.list(page, pageable.value.size, listId)
    optouts.value = res.data.data ?? []
    pageable.value = res.data.pageable
  } catch {
    notify.error('Failed to load opt-outs')
  } finally {
    loading.value = false
  }
}

async function removeOptout(s: Suppression) {
  const confirmed = await confirm({
    title: 'Remove opt-out',
    message: `Resubscribe ${s.email} to this list? Mail scoped to this list will reach them again.`,
    confirmText: 'Remove',
    variant: 'danger',
  })
  if (!confirmed) return
  try {
    await suppressionsApi.delete(s.email, listId)
    notify.success('Opt-out removed')
    await loadOptouts(pageable.value.current_page)
  } catch {
    notify.error('Failed to remove opt-out')
  }
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' })
}

onMounted(async () => {
  await loadList()
  await loadOptouts(0)
})
</script>

<template>
  <div>
    <div class="page-header">
      <div>
        <a class="link" @click="router.push({ name: 'unsubscribe-lists-page' })">← Unsubscribe Lists</a>
        <h1 v-if="list">{{ list.name }}</h1>
      </div>
    </div>

    <div v-if="loading && !list" class="loading-page">
      <div class="spinner"></div>
    </div>

    <template v-else-if="list">
      <div class="card" style="margin-bottom: 16px;">
        <div style="display: flex; flex-wrap: wrap; gap: 32px;">
          <div>
            <div class="form-hint">Opt-outs</div>
            <div style="font-size: 22px; font-weight: 600;">{{ pageable.total_elements }}</div>
          </div>
          <div>
            <div class="form-hint">Public name</div>
            <div>{{ list.public_name || list.name }}</div>
          </div>
          <div>
            <div class="form-hint">Status</div>
            <div>
              <span :class="list.active ? 'badge badge-success' : 'badge badge-neutral'">
                {{ list.active ? 'Active' : 'Archived' }}
              </span>
            </div>
          </div>
          <div>
            <div class="form-hint">List ID</div>
            <div>{{ list.id }}</div>
          </div>
        </div>
        <p v-if="list.description" style="margin-top: 16px; color: var(--text-secondary, #6b7280);">{{ list.description }}</p>
      </div>

      <div class="card">
        <div class="card-header">
          <h3>Opted-out recipients</h3>
        </div>

        <div v-if="optouts.length === 0" class="empty-state">
          <h3>No opt-outs yet</h3>
          <p>Recipients who click the one-click unsubscribe on a send that references this list appear here.</p>
        </div>

        <template v-else>
          <div class="table-wrapper">
            <table>
              <thead>
                <tr>
                  <th>Email</th>
                  <th>Reason</th>
                  <th>Date</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="s in optouts" :key="s.id">
                  <td>{{ s.email }}</td>
                  <td>{{ s.reason || '—' }}</td>
                  <td>{{ formatDate(s.created_at) }}</td>
                  <td>
                    <button v-if="wsStore.canEdit" class="btn btn-secondary btn-sm" @click="removeOptout(s)">Remove</button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <Pagination :pageable="pageable" @page="loadOptouts" />
        </template>
      </div>
    </template>
  </div>
</template>
