<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { domainsApi } from '../../api/domains'
import type { Domain, Pageable } from '../../api/types'
import { useNotificationStore } from '../../stores/notification'
import { useConfirm } from '../../composables/useConfirm'
import { useModalSafeClose } from '../../composables/useModalSafeClose';

const notify = useNotificationStore()
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
  try {
    await domainsApi.verify(domain.id)
    notify.success(`Verification initiated for ${domain.domain}`)
    await fetchDomains()
  } catch {
    notify.error('Verification failed')
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
      <button class="btn btn-primary" @click="showAddModal = true">Add Domain</button>
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
                      <button class="btn btn-secondary btn-sm" @click="verifyDomain(domain)">Verify</button>
                      <button class="btn btn-secondary btn-sm" @click="viewDnsRecords(domain)">
                        {{ expandedDomainId === domain.id ? 'Hide DNS' : 'View DNS Records' }}
                      </button>
                      <button class="btn btn-danger btn-sm" @click="deleteDomain(domain)">Delete</button>
                    </div>
                  </td>
                </tr>
                <tr v-if="expandedDomainId === domain.id">
                  <td colspan="6" style="padding: 20px 16px; background: var(--bg-tertiary);">
                    <div v-if="dnsLoading" class="loading-page" style="min-height: 100px;">
                      <div class="spinner"></div>
                    </div>
                    <div v-else-if="dnsRecordsDomain && dnsRecordsDomain.dns_records">
                      <h4 style="margin-bottom: 12px; font-size: 14px; font-weight: 600; color: var(--text-secondary);">
                        Required DNS Records for {{ domain.domain }}
                      </h4>
                      <div class="dns-record dns-record-highlight">
                        <div class="dns-label">Ownership Verification</div>
                        <div class="dns-type">{{ dnsRecordsDomain.dns_records.verification.type }}</div>
                        <div class="dns-name">{{ dnsRecordsDomain.dns_records.verification.name }}</div>
                        <div class="dns-value">{{ dnsRecordsDomain.dns_records.verification.value }}</div>
                      </div>
                      <div class="dns-record">
                        <div class="dns-label">SPF</div>
                        <div class="dns-type">{{ dnsRecordsDomain.dns_records.spf.type }}</div>
                        <div class="dns-name">{{ dnsRecordsDomain.dns_records.spf.name }}</div>
                        <div class="dns-value">{{ dnsRecordsDomain.dns_records.spf.value }}</div>
                      </div>
                      <div class="dns-record">
                        <div class="dns-label">DKIM</div>
                        <div class="dns-type">{{ dnsRecordsDomain.dns_records.dkim.type }}</div>
                        <div class="dns-name">{{ dnsRecordsDomain.dns_records.dkim.name }}</div>
                        <div class="dns-value">{{ dnsRecordsDomain.dns_records.dkim.value }}</div>
                      </div>
                      <div class="dns-record">
                        <div class="dns-label">DMARC</div>
                        <div class="dns-type">{{ dnsRecordsDomain.dns_records.dmarc.type }}</div>
                        <div class="dns-name">{{ dnsRecordsDomain.dns_records.dmarc.name }}</div>
                        <div class="dns-value">{{ dnsRecordsDomain.dns_records.dmarc.value }}</div>
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
