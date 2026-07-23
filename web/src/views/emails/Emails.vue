<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { emailsApi } from '../../api/emails'
import { useNotificationStore } from '../../stores/notification'
import type { Email } from '../../api/types'
import Pagination from '../../components/Pagination.vue'
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
  // Drop any pending debounce so an immediate apply (Enter / a quick filter)
  // doesn't get followed by a duplicate fire.
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

// Column sorting — mirrored to the URL (?sort=). Clicking a column cycles
// ascending -> descending -> default (newest first).
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

// Surfaced quick filters — each just nudges a token in `queryText`.
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
const statusLabel = (v: string) => STATUS_OPTIONS.find((o) => o.value === v)?.label ?? v

// Status is a multi-select — stored as a comma-separated `status:` token.
const statuses = computed<string[]>({
  get: () => {
    const raw = getToken(queryText.value, 'status')
    return raw ? raw.split(',').map((s) => s.trim()).filter(Boolean) : []
  },
  set: (vals: string[]) => {
    queryText.value = setToken(queryText.value, 'status', vals.join(','))
    applySearch()
  },
})
function toggleStatus(value: string) {
  const cur = statuses.value
  statuses.value = cur.includes(value) ? cur.filter((s) => s !== value) : [...cur, value]
}
const statusSummary = computed(() => {
  const n = statuses.value.length
  if (n === 0) return 'Any status'
  if (n === 1) return statusLabel(statuses.value[0])
  return `${n} statuses`
})

// Close the status dropdown on an outside click.
const statusOpen = ref(false)
const statusRef = ref<HTMLElement | null>(null)
function onDocClick(e: MouseEvent) {
  if (statusOpen.value && statusRef.value && !statusRef.value.contains(e.target as Node)) {
    statusOpen.value = false
  }
}
onMounted(() => document.addEventListener('click', onDocClick))
onBeforeUnmount(() => {
  document.removeEventListener('click', onDocClick)
  clearSearchTimeout()
})

// Compact summary of the structured quick-filters that are active — shown as
// removable chips so the selection stays visible even when the panel is closed.
const activeChips = computed(() => {
  const chips: { key: string; label: string }[] = []
  for (const s of statuses.value) chips.push({ key: `status:${s}`, label: `Status: ${statusLabel(s)}` })
  if (afterDate.value) chips.push({ key: 'after', label: `After: ${afterDate.value}` })
  if (beforeDate.value) chips.push({ key: 'before', label: `Before: ${beforeDate.value}` })
  if (hasAttachment.value) chips.push({ key: 'attachment', label: 'Has attachments' })
  return chips
})

function removeChip(key: string) {
  if (key.startsWith('status:')) {
    const v = key.slice('status:'.length)
    statuses.value = statuses.value.filter((s) => s !== v)
  } else if (key === 'attachment') hasAttachment.value = false
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
  // Rendered in the viewer's local timezone; the short tz label (e.g. GMT+3)
  // keeps the time unambiguous.
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
              placeholder="Search — from: to: subject: template: has:attachment after: before: status:"
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
          <div ref="statusRef" class="filters__field">
            <label class="form-label">Status</label>
            <div class="msel">
              <button
                type="button"
                class="msel__button"
                :class="{ 'msel__button--empty': !statuses.length, 'msel__button--open': statusOpen }"
                @click="statusOpen = !statusOpen"
              >
                <span>{{ statusSummary }}</span>
                <span class="mdi mdi-chevron-down msel__chevron"></span>
              </button>
              <div v-if="statusOpen" class="msel__menu">
                <label v-for="o in STATUS_OPTIONS" :key="o.value" class="msel__option">
                  <input
                    type="checkbox"
                    :checked="statuses.includes(o.value)"
                    @change="toggleStatus(o.value)"
                  />
                  <span>{{ o.label }}</span>
                </label>
              </div>
            </div>
          </div>

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
/* Reserve a fixed gutter for the attachment clip so subjects always align,
   whether or not a row has an attachment. */
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

/* Sortable column headers */
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

/* Reserve a stable width for the action column so the table doesn't shift
   between pages/searches depending on whether a row has a Retry button. */
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
  flex-wrap: wrap;
  align-items: flex-end;
  gap: 14px 16px;
  padding-top: 2px;
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

.msel {
  position: relative;
}

.msel__button {
  width: 100%;
  height: 44px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 0 12px 0 14px;
  border: 1px solid var(--border-input);
  border-radius: var(--radius);
  background: var(--bg-input);
  color: var(--text-primary);
  font-size: 14px;
  font-family: inherit;
  cursor: pointer;
  text-align: left;
  transition: border-color var(--transition), box-shadow var(--transition);
}

.msel__button--open {
  border-color: var(--primary-600);
  box-shadow: var(--shadow-focus);
}

.msel__button--empty {
  color: var(--text-tertiary);
}

.msel__chevron {
  font-size: 18px;
  color: var(--text-tertiary);
  transition: transform var(--transition);
}

.msel__button--open .msel__chevron {
  transform: rotate(180deg);
}

.msel__menu {
  position: absolute;
  z-index: 20;
  top: calc(100% + 4px);
  left: 0;
  min-width: 100%;
  padding: 6px;
  display: flex;
  flex-direction: column;
  gap: 2px;
  background: var(--bg-secondary);
  border: 1px solid var(--border-input);
  border-radius: var(--radius);
  box-shadow: var(--shadow-lg);
}

.msel__option {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 8px;
  border-radius: var(--radius-sm);
  font-size: 14px;
  color: var(--text-secondary);
  white-space: nowrap;
  cursor: pointer;
}

.msel__option:hover {
  background: var(--bg-hover);
}

.msel__option input {
  width: 15px;
  height: 15px;
  cursor: pointer;
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
