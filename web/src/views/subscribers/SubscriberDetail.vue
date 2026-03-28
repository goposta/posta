<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { subscribersApi } from '../../api/subscribers'
import type { Subscriber, SubscriberStatus } from '../../api/types'
import { useNotificationStore } from '../../stores/notification'
import { useConfirm } from '../../composables/useConfirm'

const route = useRoute()
const router = useRouter()
const notify = useNotificationStore()
const { confirm } = useConfirm()

const loading = ref(true)
const saving = ref(false)
const subscriber = ref<Subscriber | null>(null)

const editForm = ref({
  email: '',
  name: '',
  status: '' as SubscriberStatus,
  custom_fields: '',
})

onMounted(async () => {
  try {
    const id = Number(route.params.id)
    const res = await subscribersApi.get(id)
    subscriber.value = res.data.data
    editForm.value = {
      email: subscriber.value.email,
      name: subscriber.value.name,
      status: subscriber.value.status,
      custom_fields: JSON.stringify(subscriber.value.custom_fields || {}, null, 2),
    }
  } catch {
    notify.error('Failed to load subscriber')
  } finally {
    loading.value = false
  }
})

async function saveSubscriber() {
  if (!subscriber.value) return
  saving.value = true
  try {
    let customFields: Record<string, any> = {}
    if (editForm.value.custom_fields.trim()) {
      customFields = JSON.parse(editForm.value.custom_fields)
    }
    const res = await subscribersApi.update(subscriber.value.id, {
      email: editForm.value.email.trim(),
      name: editForm.value.name.trim(),
      status: editForm.value.status,
      custom_fields: customFields,
    })
    subscriber.value = res.data.data
    notify.success('Subscriber updated')
  } catch (e: any) {
    if (e instanceof SyntaxError) {
      notify.error('Invalid JSON in custom fields')
    } else {
      notify.error(e?.response?.data?.error?.message || 'Failed to update subscriber')
    }
  } finally {
    saving.value = false
  }
}

async function deleteSubscriber() {
  if (!subscriber.value) return
  const confirmed = await confirm({
    title: 'Delete Subscriber',
    message: `Are you sure you want to delete "${subscriber.value.email}"? This action cannot be undone.`,
    confirmText: 'Delete',
    variant: 'danger',
  })
  if (!confirmed) return
  try {
    await subscribersApi.delete(subscriber.value.id)
    notify.success('Subscriber deleted')
    router.push('/subscribers')
  } catch {
    notify.error('Failed to delete subscriber')
  }
}

function statusBadgeClass(status: SubscriberStatus): string {
  switch (status) {
    case 'subscribed': return 'badge badge-primary'
    case 'unsubscribed': return 'badge badge-neutral'
    case 'bounced': return 'badge badge-warning'
    case 'complained': return 'badge badge-warning'
    default: return 'badge'
  }
}

function formatDate(date: string | null) {
  if (!date) return '-'
  return new Date(date).toLocaleString()
}

const customFieldEntries = (fields: Record<string, any>) => {
  if (!fields) return []
  return Object.entries(fields)
}
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Subscriber Detail</h1>
      <div style="display: flex; gap: 8px">
        <button class="btn btn-secondary" @click="router.push('/subscribers')">Back to Subscribers</button>
      </div>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <template v-else-if="subscriber">
      <div class="card" style="margin-bottom: 24px">
        <div class="card-header">
          <h2>{{ subscriber.email }}</h2>
          <span :class="statusBadgeClass(subscriber.status)">{{ subscriber.status }}</span>
        </div>
        <div class="card-body">
          <div class="form-group">
            <label class="form-label">Email</label>
            <input v-model="editForm.email" type="email" class="form-input" />
          </div>
          <div class="form-group">
            <label class="form-label">Name</label>
            <input v-model="editForm.name" type="text" class="form-input" />
          </div>
          <div class="form-group">
            <label class="form-label">Status</label>
            <select v-model="editForm.status" class="form-select">
              <option value="subscribed">Subscribed</option>
              <option value="unsubscribed">Unsubscribed</option>
              <option value="bounced">Bounced</option>
              <option value="complained">Complained</option>
            </select>
          </div>
          <div class="form-group">
            <label class="form-label">Custom Fields (JSON)</label>
            <textarea v-model="editForm.custom_fields" class="form-input" rows="6"></textarea>
          </div>
          <div style="display: flex; gap: 8px; margin-top: 16px">
            <button class="btn btn-primary" :disabled="saving" @click="saveSubscriber">
              {{ saving ? 'Saving...' : 'Save Changes' }}
            </button>
            <button class="btn btn-danger" @click="deleteSubscriber">Delete</button>
          </div>
        </div>
      </div>

      <div class="card" style="margin-bottom: 24px">
        <div class="card-header">
          <h2>Custom Fields</h2>
        </div>
        <div class="card-body">
          <div v-if="customFieldEntries(subscriber.custom_fields).length === 0" class="empty-state">
            <p>No custom fields set.</p>
          </div>
          <table v-else>
            <thead>
              <tr>
                <th>Field</th>
                <th>Value</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="[key, value] in customFieldEntries(subscriber.custom_fields)" :key="key">
                <td style="font-weight: 600">{{ key }}</td>
                <td>{{ typeof value === 'object' ? JSON.stringify(value) : String(value) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div class="card">
        <div class="card-header">
          <h2>Details</h2>
        </div>
        <div class="card-body">
          <table>
            <tbody>
              <tr>
                <td style="font-weight: 600; width: 160px">Subscribed At</td>
                <td>{{ formatDate(subscriber.subscribed_at) }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600">Unsubscribed At</td>
                <td>{{ formatDate(subscriber.unsubscribed_at) }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600">Created At</td>
                <td>{{ formatDate(subscriber.created_at) }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600">Updated At</td>
                <td>{{ formatDate(subscriber.updated_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </template>

    <div v-else class="empty-state">
      <h3>Subscriber not found</h3>
      <p>The subscriber you are looking for does not exist.</p>
    </div>
  </div>
</template>
