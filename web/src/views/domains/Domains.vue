<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { domainsApi } from '../../api/domains'
import type { Domain, DnsRecord, Pageable } from '../../api/types'
import { useNotificationStore } from '../../stores/notification'
import { useConfirm } from '../../composables/useConfirm'
import { useModalSafeClose } from '../../composables/useModalSafeClose';
import { useWorkspaceStore } from '../../stores/workspace'

const notify = useNotificationStore()
const wsStore = useWorkspaceStore()
const { confirm } = useConfirm()

const domains = ref<Domain[]>([])
const pageable = ref<Pageable | null>(null)
const loading = ref(true)
const currentPage = ref(0)

const showAddModal = ref(false)
const newDomain = ref('')
const saving = ref(false)

const expandedDomainId = ref<number | null>(null)
const dnsLoading = ref(false)
const dnsRecordsDomain = ref<Domain | null>(null)
const verifying = ref(false)

type RecordKey = 'verification' | 'spf' | 'dkim' | 'dmarc'

interface RecordRow {
  key: RecordKey
  label: string
  description: string
  record: DnsRecord
  verified: boolean
}

const recordDescriptions: Record<RecordKey, { label: string; description: string }> = {
  verification: {
    label: 'Ownership Verification',
    description: 'Proves you control this domain. Posta will not send for an unverified domain when strict mode is enabled.',
  },
  spf: {
    label: 'SPF',
    description: 'Authorizes Posta’s mail servers to send on your behalf. Required to avoid spam filtering.',
  },
  dkim: {
    label: 'DKIM',
    description: 'Cryptographically signs each message so receivers can verify it wasn’t tampered with.',
  },
  dmarc: {
    label: 'DMARC',
    description: 'Tells receivers how to handle mail that fails SPF/DKIM. Start with p=none for monitoring.',
  },
}

const recordRows = computed<RecordRow[]>(() => {
  const d = dnsRecordsDomain.value
  if (!d || !d.dns_records) return []
  return [
    { key: 'verification', ...recordDescriptions.verification, record: d.dns_records.verification, verified: !!d.ownership_verified },
    { key: 'spf',          ...recordDescriptions.spf,          record: d.dns_records.spf,          verified: !!d.spf_verified },
    { key: 'dkim',         ...recordDescriptions.dkim,         record: d.dns_records.dkim,         verified: !!d.dkim_verified },
    { key: 'dmarc',        ...recordDescriptions.dmarc,        record: d.dns_records.dmarc,        verified: !!d.dmarc_verified },
  ]
})

function hostShortForm(fullHost: string, domain: string): string {
  if (!fullHost || !domain) return fullHost
  if (fullHost === domain) return '@'
  const suffix = '.' + domain
  if (fullHost.endsWith(suffix)) return fullHost.slice(0, -suffix.length)
  return fullHost
}

async function copy(text: string, label = 'Value') {
  try {
    await navigator.clipboard.writeText(text)
    notify.success(`${label} copied`)
  } catch {
    notify.error('Copy failed')
  }
}

async function copyAllRecords() {
  const d = dnsRecordsDomain.value
  if (!d || !d.dns_records) return
  const lines = ['Type\tHost\tValue']
  for (const row of recordRows.value) {
    lines.push(`${row.record.type}\t${row.record.name}\t${row.record.value}`)
  }
  await copy(lines.join('\n'), 'All records')
}

async function fetchDomains() {
  loading.value = true
  try {
    const res = await domainsApi.list(currentPage.value)
    domains.value = res.data.data
    pageable.value = res.data.pageable
  } catch {
    notify.error('Failed to load domains')
  } finally {
    loading.value = false
  }
}

async function addDomain() {
  if (!newDomain.value.trim()) return
  saving.value = true
  try {
    await domainsApi.create(newDomain.value.trim())
    notify.success('Domain added')
    showAddModal.value = false
    newDomain.value = ''
    await fetchDomains()
  } catch {
    notify.error('Failed to add domain')
  } finally {
    saving.value = false
  }
}

async function verifyDomain(domain: Domain) {
  verifying.value = true
  try {
    await domainsApi.verify(domain.id)
    notify.success(`Verification initiated for ${domain.domain}`)
    await fetchDomains()
    if (expandedDomainId.value === domain.id) {
      const res = await domainsApi.get(domain.id)
      dnsRecordsDomain.value = res.data.data
    }
  } catch {
    notify.error('Verification failed')
  } finally {
    verifying.value = false
  }
}

async function viewDnsRecords(domain: Domain) {
  if (expandedDomainId.value === domain.id) {
    expandedDomainId.value = null
    dnsRecordsDomain.value = null
    return
  }
  dnsLoading.value = true
  expandedDomainId.value = domain.id
  try {
    const res = await domainsApi.get(domain.id)
    dnsRecordsDomain.value = res.data.data
  } catch {
    notify.error('Failed to load DNS records')
    expandedDomainId.value = null
  } finally {
    dnsLoading.value = false
  }
}

async function deleteDomain(domain: Domain) {
  const confirmed = await confirm({
    title: 'Delete Domain',
    message: `Are you sure you want to delete "${domain.domain}"? This will remove all associated DNS records and verification status.`,
    confirmText: 'Delete',
    variant: 'danger',
  })
  if (!confirmed) return
  try {
    await domainsApi.delete(domain.id)
    notify.success('Domain deleted')
    if (expandedDomainId.value === domain.id) {
      expandedDomainId.value = null
      dnsRecordsDomain.value = null
    }
    await fetchDomains()
  } catch {
    notify.error('Failed to delete domain')
  }
}

function prevPage() {
  if (currentPage.value > 0) {
    currentPage.value--
    fetchDomains()
  }
}

function nextPage() {
  if (pageable.value && currentPage.value < pageable.value.total_pages - 1) {
    currentPage.value++
    fetchDomains()
  }
}
const { watchClickStart, confirmClickEnd } = useModalSafeClose(() => {
  showAddModal.value = false;
});
onMounted(fetchDomains)
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Domains</h1>
      <button v-if="wsStore.canEdit" class="btn btn-primary" @click="showAddModal = true">Add Domain</button>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <template v-else>
      <div class="card">
        <div class="table-wrapper" v-if="domains.length > 0">
          <table>
            <thead>
              <tr>
                <th>Domain</th>
                <th>Ownership</th>
                <th>SPF</th>
                <th>DKIM</th>
                <th>DMARC</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              <template v-for="domain in domains" :key="domain.id">
                <tr>
                  <td>{{ domain.domain }}</td>
                  <td>
                    <span v-if="domain.ownership_verified" class="verified">&#10003;</span>
                    <span v-else class="unverified">&#10005;</span>
                  </td>
                  <td>
                    <span v-if="domain.spf_verified" class="verified">&#10003;</span>
                    <span v-else class="unverified">&#10005;</span>
                  </td>
                  <td>
                    <span v-if="domain.dkim_verified" class="verified">&#10003;</span>
                    <span v-else class="unverified">&#10005;</span>
                  </td>
                  <td>
                    <span v-if="domain.dmarc_verified" class="verified">&#10003;</span>
                    <span v-else class="unverified">&#10005;</span>
                  </td>
                  <td>
                    <div class="flex gap-2">
                      <button v-if="wsStore.canEdit" class="btn btn-secondary btn-sm" @click="verifyDomain(domain)">Verify</button>
                      <button class="btn btn-secondary btn-sm" @click="viewDnsRecords(domain)">
                        {{ expandedDomainId === domain.id ? 'Hide DNS' : 'View DNS Records' }}
                      </button>
                      <button v-if="wsStore.canEdit" class="btn btn-danger btn-sm" @click="deleteDomain(domain)">Delete</button>
                    </div>
                  </td>
                </tr>
                <tr v-if="expandedDomainId === domain.id">
                  <td colspan="6" style="padding: 20px 16px; background: var(--bg-tertiary);">
                    <div v-if="dnsLoading" class="loading-page" style="min-height: 100px;">
                      <div class="spinner"></div>
                    </div>
                    <div v-else-if="dnsRecordsDomain && dnsRecordsDomain.dns_records" class="dns-panel">
                      <div class="dns-panel-header">
                        <div>
                          <h4 class="dns-panel-title">DNS records for {{ domain.domain }}</h4>
                          <p class="dns-panel-subtitle">
                            Add the records below at your DNS provider, then click
                            <strong>Re-check</strong>. Propagation is usually under 15 minutes but can take up to 48 hours.
                          </p>
                        </div>
                        <div class="dns-panel-actions">
                          <button class="btn btn-secondary btn-sm" @click="copyAllRecords">Copy all</button>
                          <button
                            v-if="wsStore.canEdit"
                            class="btn btn-primary btn-sm"
                            :disabled="verifying"
                            @click="verifyDomain(domain)"
                          >
                            {{ verifying ? 'Checking…' : 'Re-check' }}
                          </button>
                        </div>
                      </div>

                      <div class="dns-record-list">
                        <div
                          v-for="row in recordRows"
                          :key="row.key"
                          class="dns-record"
                          :class="{ 'dns-record-highlight': row.key === 'verification', 'dns-record-verified': row.verified }"
                        >
                          <div class="dns-record-head">
                            <div class="dns-record-head-left">
                              <span class="dns-label">{{ row.label }}</span>
                              <span class="dns-type-badge">{{ row.record.type }}</span>
                            </div>
                            <span v-if="row.verified" class="dns-status dns-status-ok">
                              &#10003; Verified
                            </span>
                            <span v-else class="dns-status dns-status-pending">
                              &#9679; Pending
                            </span>
                          </div>
                          <p class="dns-record-desc">{{ row.description }}</p>

                          <div class="dns-field">
                            <div class="dns-field-label">Host / Name</div>
                            <div class="dns-field-row">
                              <code class="dns-field-value">{{ row.record.name }}</code>
                              <button
                                type="button"
                                class="btn btn-ghost btn-xs"
                                @click="copy(row.record.name, 'Host')"
                              >Copy</button>
                            </div>
                            <div
                              v-if="hostShortForm(row.record.name, domain.domain) !== row.record.name"
                              class="dns-field-hint"
                            >
                              Some providers (Cloudflare, Route 53, GoDaddy) only accept the subdomain part —
                              use <code>{{ hostShortForm(row.record.name, domain.domain) }}</code> instead.
                            </div>
                          </div>

                          <div class="dns-field">
                            <div class="dns-field-label">Value</div>
                            <div class="dns-field-row">
                              <code class="dns-field-value dns-field-value-block">{{ row.record.value }}</code>
                              <button
                                type="button"
                                class="btn btn-ghost btn-xs"
                                @click="copy(row.record.value, 'Value')"
                              >Copy</button>
                            </div>
                          </div>

                          <div class="dns-field-hint dns-ttl-hint">
                            TTL: Auto (or 3600). Leave priority blank unless your provider requires one.
                          </div>
                        </div>
                      </div>
                    </div>
                    <div v-else class="text-muted text-sm">
                      No DNS records available for this domain.
                    </div>
                  </td>
                </tr>
              </template>
            </tbody>
          </table>
        </div>

        <div v-else class="empty-state">
          <h3>No domains</h3>
          <p>Add a domain to verify your sending identity.</p>
        </div>

        <div v-if="pageable && !pageable.empty" class="pagination">
          <span class="pagination-info">
            Page {{ pageable.current_page + 1 }} of {{ pageable.total_pages }}
            ({{ pageable.total_elements }} total)
          </span>
          <div class="pagination-buttons">
            <button class="btn btn-secondary btn-sm" :disabled="currentPage === 0" @click="prevPage">Previous</button>
            <button class="btn btn-secondary btn-sm" :disabled="currentPage >= pageable.total_pages - 1" @click="nextPage">Next</button>
          </div>
        </div>
      </div>
    </template>

    <!-- Add Domain Modal -->
    <div v-if="showAddModal" class="modal-overlay" @mousedown="watchClickStart" 
      @mouseup="confirmClickEnd">
      <div class="modal" @mousedown.stop @mouseup.stop>
        <div class="modal-header">
          <h3>Add Domain</h3>
        </div>
        <form @submit.prevent="addDomain">
          <div class="modal-body">
            <div class="form-group">
              <label class="form-label">Domain</label>
              <input v-model="newDomain" type="text" class="form-input" placeholder="example.com" required />
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" @click="showAddModal = false">Cancel</button>
            <button type="submit" class="btn btn-primary" :disabled="saving">
              {{ saving ? 'Adding...' : 'Add Domain' }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>
