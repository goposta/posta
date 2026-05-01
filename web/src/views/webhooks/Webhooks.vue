<script setup lang="ts">
import { ref } from 'vue'
import { webhooksApi } from '../../api/webhooks'
import type { Webhook, WebhookInput } from '../../api/types'
import Pagination from '../../components/Pagination.vue'
import { usePagination } from '../../composables/usePagination'
import { useNotificationStore } from '../../stores/notification'
import { useConfirm } from '../../composables/useConfirm'
import { useModalSafeClose } from '../../composables/useModalSafeClose';
import { useWorkspaceStore } from '../../stores/workspace'

const notify = useNotificationStore()
const wsStore = useWorkspaceStore()
const { confirm } = useConfirm()

const webhooks = ref<Webhook[]>([])
const loading = ref(true)

const showModal = ref(false)
const form = ref<WebhookInput>({
  url: '',
  events: [],
  filters: [],
})
const filterInput = ref('')
const saving = ref(false)

const createdWebhook = ref<Webhook | null>(null)
const showSecretModal = ref(false)
const copied = ref(false)

const availableEvents = ['email.sent', 'email.failed', 'email.inbound']

const { pageable, goToPage } = usePagination(async (page) => {
  loading.value = true
  try {
    const res = await webhooksApi.list(page)
    webhooks.value = res.data.data
    pageable.value = res.data.pageable
  } catch (e) {
    console.error('Failed to load webhooks', e)
  } finally {
    loading.value = false
  }
})

function openCreate() {
  form.value = { url: '', events: [], filters: [] }
  filterInput.value = ''
  showModal.value = true
}

function addFilter() {
  const val = filterInput.value.trim().toLowerCase()
  if (val && !form.value.filters.includes(val)) {
    form.value.filters.push(val)
  }
  filterInput.value = ''
}

function removeFilter(index: number) {
  form.value.filters.splice(index, 1)
}

function toggleEvent(event: string) {
  const idx = form.value.events.indexOf(event)
  if (idx >= 0) {
    form.value.events.splice(idx, 1)
  } else {
    form.value.events.push(event)
  }
}

async function createWebhook() {
  if (!form.value.url.trim() || form.value.events.length === 0) return
  saving.value = true
  try {
    const res = await webhooksApi.create(form.value)
    createdWebhook.value = res.data.data
    showModal.value = false
    showSecretModal.value = true
    notify.success('Webhook created')
    await goToPage(pageable.value.current_page)
  } catch {
    notify.error('Failed to create webhook')
  } finally {
    saving.value = false
  }
}

function copySecret() {
  if (!createdWebhook.value?.secret) return
  navigator.clipboard.writeText(createdWebhook.value.secret)
  copied.value = true
  setTimeout(() => (copied.value = false), 2000)
}

function closeSecretModal() {
  showSecretModal.value = false
  createdWebhook.value = null
  copied.value = false
}

async function deleteWebhook(webhook: Webhook) {
  const confirmed = await confirm({
    title: 'Delete Webhook',
    message: `Are you sure you want to delete the webhook for "${webhook.url}"?`,
    confirmText: 'Delete',
    variant: 'danger',
  })
  if (!confirmed) return
  try {
    await webhooksApi.delete(webhook.id)
    notify.success('Webhook deleted')
    await goToPage(pageable.value.current_page)
  } catch {
    notify.error('Failed to delete webhook')
  }
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  })
}
const { watchClickStart, confirmClickEnd } = useModalSafeClose(() => {
  showModal.value = false;
});
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Webhooks</h1>
      <button v-if="wsStore.canEdit" class="btn btn-primary" @click="openCreate">Add Webhook</button>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <template v-else>
      <div class="card">
        <div class="table-wrapper" v-if="webhooks.length > 0">
          <table>
            <thead>
              <tr>
                <th>URL</th>
                <th>Events</th>
                <th>Filters</th>
                <th>Created At</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="webhook in webhooks" :key="webhook.id">
                <td class="truncate" style="max-width: 300px;">{{ webhook.url }}</td>
                <td>
                  <div class="flex gap-2" style="flex-wrap: wrap;">
                    <span v-for="event in webhook.events" :key="event" class="badge badge-neutral">
                      {{ event }}
                    </span>
                  </div>
                </td>
                <td>
                  <div class="flex gap-2" style="flex-wrap: wrap;" v-if="webhook.filters && webhook.filters.length > 0">
                    <span v-for="filter in webhook.filters" :key="filter" class="badge badge-neutral">
                      {{ filter }}
                    </span>
                  </div>
                  <span v-else class="text-muted">All senders</span>
                </td>
                <td>{{ formatDate(webhook.created_at) }}</td>
                <td>
                  <button v-if="wsStore.canEdit" class="btn btn-danger btn-sm" @click="deleteWebhook(webhook)">Delete</button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div v-else class="empty-state">
          <h3>No webhooks</h3>
          <p>Add a webhook to receive event notifications.</p>
        </div>

        <Pagination :pageable="pageable" @page="goToPage" />
      </div>
    </template>

    <!-- Create Webhook Modal -->
    <div v-if="showModal" class="modal-overlay" @mousedown="watchClickStart" @mouseup="confirmClickEnd">
      <div class="modal" @mousedown.stop @mouseup.stop>
        <div class="modal-header">
          <h3>Add Webhook</h3>
        </div>
        <form @submit.prevent="createWebhook">
          <div class="modal-body">
            <div class="form-group">
              <label class="form-label">URL</label>
              <input v-model="form.url" type="url" class="form-input" placeholder="https://example.com/webhook" required />
            </div>
            <div class="form-group">
              <label class="form-label">Events</label>
              <div style="display: flex; flex-direction: column; gap: 8px; margin-top: 4px;">
                <label v-for="event in availableEvents" :key="event" class="checkbox-label">
                  <input
                    type="checkbox"
                    :checked="form.events.includes(event)"
                    @change="toggleEvent(event)"
                  />
                  {{ event }}
                </label>
              </div>
            </div>
            <div class="form-group">
              <label class="form-label">Sender Filters <span class="text-muted">(optional)</span></label>
              <p style="font-size: 0.85em; color: var(--text-muted); margin: 0 0 8px;">
                Restrict this webhook to specific senders. Enter an email address or domain. Leave empty to fire for all emails.
                For <code>email.inbound</code>, the filter matches the <strong>sender</strong> of the incoming message (the <code>From</code> address), not the recipient.
              </p>
              <div style="display: flex; gap: 8px;">
                <input
                  v-model="filterInput"
                  type="text"
                  class="form-input"
                  placeholder="e.g. example.com or user@example.com"
                  @keydown.enter.prevent="addFilter"
                />
                <button type="button" class="btn btn-secondary" @click="addFilter">Add</button>
              </div>
              <div v-if="form.filters.length > 0" style="display: flex; flex-wrap: wrap; gap: 6px; margin-top: 8px;">
                <span v-for="(filter, idx) in form.filters" :key="filter" class="badge badge-neutral" style="cursor: pointer;" @click="removeFilter(idx)">
                  {{ filter }} &times;
                </span>
              </div>
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" @click="showModal = false">Cancel</button>
            <button type="submit" class="btn btn-primary" :disabled="saving || form.events.length === 0">
              {{ saving ? 'Creating...' : 'Create Webhook' }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- Signing Secret Modal -->
    <div v-if="showSecretModal" class="modal-overlay">
      <div class="modal">
        <div class="modal-header">
          <h3>Webhook Signing Secret</h3>
        </div>
        <div class="modal-body">
          <p class="text-sm" style="color: var(--danger-600); font-weight: 500; margin-bottom: 12px;">
            Save this secret. It won't be shown again.
          </p>
          <div class="code-block">{{ createdWebhook?.secret }}</div>
          <p class="text-sm" style="margin-top: 12px; color: var(--text-muted);">
            Use this secret to verify webhook signatures. Each delivery includes an
            <code>X-Posta-Signature</code> header with an HMAC-SHA256 signature of the payload.
          </p>
          <button class="btn btn-secondary btn-sm mt-4" @click="copySecret">
            {{ copied ? 'Copied!' : 'Copy Secret' }}
          </button>
        </div>
        <div class="modal-footer">
          <button class="btn btn-primary" @click="closeSecretModal">Done</button>
        </div>
      </div>
    </div>
  </div>
</template>
