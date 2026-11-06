<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { emailsApi } from '../../api/emails'
import { apiKeysApi } from '../../api/apikeys'
import { smtpApi } from '../../api/smtp'
import { useNotificationStore } from '../../stores/notification'
import type { Email } from '../../api/types'
import Pagination from '../../components/Pagination.vue'
import MultiSelect from '../../components/MultiSelect.vue'
import { usePagination } from '../../composables/usePagination'
import { getToken, hasFlag, setToken, toggleFlag } from '../../composables/searchTokens'

const route = useRoute()
const router = useRouter()
const notify = useNotificationStore()
const loading = ref(true)
const emails = ref<Email[]>([])
const retryingId = ref<string | null>(null)
const showFilters = ref(false)

// The search string is the single source of truth, mirrored to the URL (?q=).
const queryText = ref<string>(typeof route.query.q === 'string' ? route.query.q : '')
// Optional sort key (e.g. 'subject', '-sent_at'); empty keeps the default order.
const sortValue = ref<string>(typeof route.query.sort === 'string' ? route.query.sort : '')

const load = async (page: number) => {
  loading.value = true
  try {
    const res = await emailsApi.list(page, pageable.value.size, queryText.value, sortValue.value)
    emails.value = res.data.data
    pageable.value = res.data.pageable
  } catch (e) {
    console.error('Failed to load emails', e)
  } finally {
    loading.value = false
  }
}

const { pageable, goToPage } = usePagination(load)

let searchTimeout: ReturnType<typeof setTimeout> | null = null
function clearSearchTimeout() {
  if (searchTimeout) {
    clearTimeout(searchTimeout)
    searchTimeout = null
  }
}

function applySearch() {
  // Drop a pending debounce so an immediate apply (Enter, a quick filter) isn't followed by a duplicate fire.
  clearSearchTimeout()
  router.replace({ query: { ...route.query, q: queryText.value || undefined, page: undefined } })
  load(0)
}

// Debounce free-text typing; discrete controls call applySearch directly.
function onSearchInput() {
  clearSearchTimeout()
  searchTimeout = setTimeout(applySearch, 300)
}

function resetSearch() {
  queryText.value = ''
  applySearch()
}

// Mirrored to the URL (?sort=); clicking a column cycles ascending, descending, then the default (newest first).
const sortField = computed(() => sortValue.value.replace(/^-/, ''))
const sortDesc = computed(() => sortValue.value.startsWith('-'))
function toggleSort(field: string) {
  let next: string
  if (sortField.value !== field) next = field
  else if (!sortDesc.value) next = '-' + field
  else next = ''
  sortValue.value = next
  router.replace({ query: { ...route.query, sort: next || undefined, page: undefined } })
  load(0)
}
function sortIcon(field: string) {
  if (sortField.value !== field) return 'mdi-unfold-more-horizontal th-sort__arrow--idle'
  return sortDesc.value ? 'mdi-arrow-down' : 'mdi-arrow-up'
}

const hasAttachment = computed({
  get: () => hasFlag(queryText.value, 'has:attachment') || hasFlag(queryText.value, 'has:attachments'),
  set: (on: boolean) => {
    let s = toggleFlag(queryText.value, 'has:attachments', false)
    s = toggleFlag(s, 'has:attachment', on)
    queryText.value = s
    applySearch()
  },
})
const afterDate = computed({
  get: () => getToken(queryText.value, 'after'),
  set: (v: string) => {
    queryText.value = setToken(queryText.value, 'after', v)
    applySearch()
  },
})
const beforeDate = computed({
  get: () => getToken(queryText.value, 'before'),
  set: (v: string) => {
    queryText.value = setToken(queryText.value, 'before', v)
    applySearch()
  },
})
const STATUS_OPTIONS = [
  { value: 'sent', label: 'Sent' },
  { value: 'queued', label: 'Queued' },
  { value: 'pending', label: 'Pending' },
  { value: 'processing', label: 'Processing' },
  { value: 'scheduled', label: 'Scheduled' },
  { value: 'failed', label: 'Failed' },
  { value: 'suppressed', label: 'Suppressed' },
]
const PROVIDER_OPTIONS = [
  'Gmail', 'Google Workspace', 'Outlook', 'Yahoo', 'Apple iCloud', 'Yandex',
  'Proton', 'Zoho', 'Fastmail', 'GMX', 'AOL', 'Other',
].map((p) => ({ value: p, label: p }))
const serverHosts = ref<string[]>([])
const keyNames = ref<string[]>([])
onMounted(async () => {
  const [servers, keys] = await Promise.allSettled([smtpApi.list(0, 100), apiKeysApi.list(0, 100)])
  if (servers.status === 'fulfilled') serverHosts.value = [...new Set(servers.value.data.data.map((s) => s.host))]
  if (keys.status === 'fulfilled') keyNames.value = keys.value.data.data.map((k) => k.name)
})
onBeforeUnmount(clearSearchTimeout)

// Multi-select filters — each stored as a comma-separated `key:` token.
function listFilter(key: string) {
  return computed<string[]>({
    get: () => {
      const raw = getToken(queryText.value, key)
      return raw ? raw.split(',').map((s) => s.trim()).filter(Boolean) : []
    },
    set: (vals: string[]) => {
      queryText.value = setToken(queryText.value, key, vals.join(','))
      applySearch()
    },
  })
}
const statuses = listFilter('status')
const providers = listFilter('provider')
const servers = listFilter('server')
const apiKeys = listFilter('key')

const serverOptions = computed(() => serverHosts.value.map((h) => ({ value: h, label: h })))
const keyOptions = computed(() => keyNames.value.map((k) => ({ value: k, label: k })))

const statusLabel = (v: string) => STATUS_OPTIONS.find((o) => o.value === v)?.label ?? v

const LIST_CHIPS = [
  { key: 'status', title: 'Status', filter: statuses },
  { key: 'provider', title: 'Provider', filter: providers },
  { key: 'server', title: 'Server', filter: servers },
  { key: 'key', title: 'API key', filter: apiKeys },
]

const activeChips = computed(() => {
  const chips: { key: string; label: string }[] = []
  for (const c of LIST_CHIPS) {
    for (const v of c.filter.value) {
      chips.push({ key: `${c.key}:${v}`, label: `${c.title}: ${c.key === 'status' ? statusLabel(v) : v}` })
    }
  }
  if (afterDate.value) chips.push({ key: 'after', label: `After: ${afterDate.value}` })
  if (beforeDate.value) chips.push({ key: 'before', label: `Before: ${beforeDate.value}` })
  if (hasAttachment.value) chips.push({ key: 'attachment', label: 'Has attachments' })
  return chips
})

function removeChip(key: string) {
  const i = key.indexOf(':')
  const list = LIST_CHIPS.find((c) => c.key === key.slice(0, i))
  if (list) list.filter.value = list.filter.value.filter((v) => v !== key.slice(i + 1))
  else if (key === 'attachment') hasAttachment.value = false
  else if (key === 'after') afterDate.value = ''
  else if (key === 'before') beforeDate.value = ''
}

function hasAttachments(email: Email): boolean {
  const a = email.attachments_json
  return !!a && a !== '' && a !== '[]'
}

async function retryEmail(e: Event, em: Email) {
  e.stopPropagation()
  if (retryingId.value) return
  retryingId.value = em.uuid
  try {
    const res = await emailsApi.retry(em.uuid)
    em.status = res.data.data.status as Email['status']
    em.error_message = ''
    notify.success('Email re-queued for delivery')
  } catch (err: any) {
    const msg = err.response?.data?.error?.message || 'Failed to retry email'
    notify.error(msg)
  } finally {
    retryingId.value = null
  }
}

function statusBadgeClass(status: string) {
  switch (status) {
    case 'sent': return 'badge badge-success'
    case 'failed': return 'badge badge-danger'
    case 'pending': return 'badge badge-warning'
    case 'queued': return 'badge badge-info'
    case 'processing': return 'badge badge-warning'
    case 'suppressed': return 'badge badge-secondary'
    case 'scheduled': return 'badge badge-info'
    default: return 'badge'
  }
}

function formatDate(date: string | null) {
  if (!date) return '-'
  // Rendered in the viewer's local timezone; the short tz label (e.g. GMT+3) keeps the time unambiguous.
  return new Date(date).toLocaleString(undefined, { timeZoneName: 'short' })
}
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Emails</h1>
      <button class="btn btn-secondary" @click="router.push('/templates/preview')">Preview Template</button>
    </div>

    <div class="card">
      <div class="card-body filters">
        <div class="filters__bar">
          <div class="filters__search">
            <span class="mdi mdi-magnify filters__search-icon"></span>
            <input
              v-model="queryText"
              class="form-input filters__search-input"
              placeholder="Search — from: to: subject: template: provider: server: key: has:attachment after: before: status:"
              @input="onSearchInput"
              @keyup.enter="applySearch"
            />
          </div>
          <button
            class="btn btn-secondary filters__toggle"
            :class="{ 'filters__toggle--active': showFilters }"
            @click="showFilters = !showFilters"
          >
            <span class="mdi mdi-tune-variant"></span>
            <span class="filters__toggle-label">Filters</span>
            <span v-if="activeChips.length" class="filters__badge">{{ activeChips.length }}</span>
          </button>
        </div>

        <div v-if="activeChips.length" class="filters__chips">
          <button
            v-for="c in activeChips"
            :key="c.key"
            class="filters__chip"
            @click="removeChip(c.key)"
          >
            {{ c.label }}
            <span class="mdi mdi-close"></span>
          </button>
          <button class="filters__chip filters__chip--clear" @click="resetSearch">Clear all</button>
        </div>

        <div v-show="showFilters" class="filters__panel">
          <div class="filters__group">
            <span class="filters__group-title">Delivery</span>
            <div class="filters__row">
              <div class="filters__field">
                <label class="form-label">Status</label>
                <MultiSelect v-model="statuses" :options="STATUS_OPTIONS" placeholder="Any status" plural="statuses" />
              </div>
              <div class="filters__field">
                <label class="form-label">Provider (first recipient)</label>
                <MultiSelect v-model="providers" :options="PROVIDER_OPTIONS" placeholder="Any provider" plural="providers" />
              </div>
              <div class="filters__field">
                <label class="form-label">SMTP server</label>
                <MultiSelect v-model="servers" :options="serverOptions" placeholder="Any server" plural="servers" />
              </div>
              <div class="filters__field">
                <label class="form-label">API key</label>
                <MultiSelect v-model="apiKeys" :options="keyOptions" placeholder="Any key" plural="keys" />
              </div>
            </div>
          </div>

          <div class="filters__group">
            <span class="filters__group-title">Date &amp; content</span>
            <div class="filters__row">
              <div class="filters__field">
                <label class="form-label">After</label>
                <input v-model="afterDate" type="date" class="form-input" />
              </div>
              <div class="filters__field">
                <label class="form-label">Before</label>
                <input v-model="beforeDate" type="date" class="form-input" />
              </div>
              <label class="filters__check">
                <input v-model="hasAttachment" type="checkbox" />
                <span>Has attachments</span>
              </label>
              <button class="btn btn-secondary filters__reset" @click="resetSearch">Reset</button>
            </div>
          </div>
        </div>
      </div>

      <div v-if="loading" class="loading-page">
        <div class="spinner"></div>
      </div>

      <template v-else>
        <div v-if="emails.length === 0" class="empty-state">
          <h3>No emails found</h3>
          <p v-if="queryText">No emails match your search. Try adjusting the filters.</p>
          <p v-else>Emails sent through the API will appear here.</p>
        </div>
        <template v-else>
          <div class="table-wrapper">
            <table>
              <thead>
                <tr>
                  <th class="th-sort" :class="{ 'th-sort--active': sortField === 'subject' }" @click="toggleSort('subject')">
                    <span class="th-sort__inner">Subject<span class="mdi th-sort__arrow" :class="sortIcon('subject')"></span></span>
                  </th>
                  <th class="th-sort" :class="{ 'th-sort--active': sortField === 'sender' }" @click="toggleSort('sender')">
                    <span class="th-sort__inner">From<span class="mdi th-sort__arrow" :class="sortIcon('sender')"></span></span>
                  </th>
                  <th>Recipients</th>
                  <th class="th-sort" :class="{ 'th-sort--active': sortField === 'template' }" @click="toggleSort('template')">
                    <span class="th-sort__inner">Template<span class="mdi th-sort__arrow" :class="sortIcon('template')"></span></span>
                  </th>
                  <th class="th-sort" :class="{ 'th-sort--active': sortField === 'status' }" @click="toggleSort('status')">
                    <span class="th-sort__inner">Status<span class="mdi th-sort__arrow" :class="sortIcon('status')"></span></span>
                  </th>
                  <th class="th-sort" :class="{ 'th-sort--active': sortField === 'sent_at' }" @click="toggleSort('sent_at')">
                    <span class="th-sort__inner">Sent At<span class="mdi th-sort__arrow" :class="sortIcon('sent_at')"></span></span>
                  </th>
                  <th class="col-actions"></th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="email in emails"
                  :key="email.uuid"
                  style="cursor: pointer"
                  @click="router.push(`/emails/${email.uuid}`)"
                >
                  <td>
                    <span class="subject-cell">
                      <span class="subject-cell__clip">
                        <span
                          v-if="hasAttachments(email)"
                          class="mdi mdi-paperclip"
                          title="Has attachments"
                        ></span>
                      </span>
                      <span>{{ email.subject }}</span>
                    </span>
                  </td>
                  <td>{{ email.sender }}</td>
                  <td>{{ email.recipients.join(', ') }}</td>
                  <td>{{ email.template_name || 'N/A' }}</td>
                  <td><span :class="statusBadgeClass(email.status)">{{ email.status }}</span></td>
                  <td>{{ formatDate(email.sent_at) }}</td>
                  <td class="col-actions">
                    <button
                      v-if="email.status === 'failed'"
                      class="btn btn-secondary btn-sm"
                      :disabled="retryingId === email.uuid"
                      @click="retryEmail($event, email)"
                    >
                      {{ retryingId === email.uuid ? 'Retrying...' : 'Retry' }}
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <Pagination :pageable="pageable" @page="goToPage" />
        </template>
      </template>
    </div>
  </div>
</template>

<style scoped>
.subject-cell {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.subject-cell__clip {
  flex: 0 0 16px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted, var(--text-tertiary));
}

.th-sort {
  cursor: pointer;
  user-select: none;
  white-space: nowrap;
}

.th-sort__inner {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.th-sort__arrow {
  font-size: 15px;
  line-height: 1;
}

.th-sort__arrow--idle {
  opacity: 0;
  transition: opacity var(--transition);
}

.th-sort:hover .th-sort__arrow--idle {
  opacity: 0.45;
}

.th-sort--active {
  color: var(--text-primary);
}

.col-actions {
  width: 96px;
  text-align: right;
}

.filters {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.filters__bar {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: center;
}

.filters__search {
  position: relative;
  flex: 1 1 240px;
  min-width: 200px;
}

.filters__search-icon {
  position: absolute;
  left: 12px;
  top: 50%;
  transform: translateY(-50%);
  font-size: 18px;
  color: var(--text-muted, var(--text-tertiary));
  pointer-events: none;
}

.filters__search-input {
  height: 44px;
  padding-left: 38px;
}

.filters__toggle {
  flex: 0 0 auto;
  height: 44px;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  white-space: nowrap;
}

.filters__toggle .mdi {
  font-size: 18px;
}

.filters__toggle--active {
  border-color: var(--primary-600);
  color: var(--primary-600);
}

.filters__badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 18px;
  height: 18px;
  padding: 0 5px;
  border-radius: 999px;
  background: var(--primary-600);
  color: #fff;
  font-size: 11px;
  font-weight: 600;
  line-height: 1;
}

.filters__chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.filters__chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  font-size: 13px;
  border-radius: 999px;
  border: 1px solid var(--border-primary);
  background: var(--bg-tertiary);
  color: var(--text-secondary);
  cursor: pointer;
  transition: background var(--transition), border-color var(--transition);
}

.filters__chip:hover {
  background: var(--bg-hover);
}

.filters__chip .mdi {
  font-size: 14px;
  color: var(--text-tertiary);
}

.filters__chip--clear {
  border-style: dashed;
  background: transparent;
}

.filters__panel {
  display: flex;
  flex-direction: column;
  gap: 18px;
  padding-top: 2px;
}

.filters__group {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.filters__group-title {
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--text-tertiary);
}

.filters__row {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 14px 16px;
}

.filters__field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  flex: 1 1 150px;
  min-width: 140px;
  max-width: 220px;
}

.filters__field .form-label {
  margin: 0;
}

.filters__field .form-input,
.filters__field .form-select {
  height: 44px;
}

.filters__check {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 44px;
  white-space: nowrap;
  cursor: pointer;
  user-select: none;
  color: var(--text-secondary);
  font-size: 14px;
}

.filters__check input {
  width: 16px;
  height: 16px;
  cursor: pointer;
}

.filters__reset {
  height: 44px;
  align-self: flex-end;
  margin-left: auto;
}

@media (max-width: 640px) {
  .filters__toggle {
    flex: 1 1 auto;
    justify-content: center;
  }
}
</style>
