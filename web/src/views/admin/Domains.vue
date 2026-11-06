<script setup lang="ts">
import { ref } from "vue";
import { useRouter } from "vue-router";
import { adminApi } from "../../api/admin";
import { useNotificationStore } from "../../stores/notification";
import { apiMessage } from "../../composables/apiError";
import { usePagination } from "../../composables/usePagination";
import Pagination from "../../components/Pagination.vue";
import type { AdminDomainRow } from "../../api/types";

const router = useRouter();
const notify = useNotificationStore();

const domains = ref<AdminDomainRow[]>([]);
const loading = ref(true);
const search = ref("");
const status = ref<"" | "verified" | "unverified">("");

let searchTimer: ReturnType<typeof setTimeout> | undefined;

const { pageable, goToPage } = usePagination(async (page) => {
  loading.value = true;
  try {
    const res = await adminApi.listDomains(page, pageable.value.size, search.value.trim(), status.value);
    domains.value = res.data.data;
    pageable.value = res.data.pageable;
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to load domains"));
  } finally {
    loading.value = false;
  }
});

function onSearchInput() {
  if (searchTimer) clearTimeout(searchTimer);
  searchTimer = setTimeout(() => goToPage(0), 300);
}

function setStatus(next: typeof status.value) {
  status.value = next;
  goToPage(0);
}

// Ownership and SPF/DKIM/DMARC are shown apart: the first gates sending, the rest decide how well mail is received.
function ownershipBadge(d: AdminDomainRow) {
  return d.ownership_verified ? "badge badge-success" : "badge badge-warning";
}

function dnsSummary(d: AdminDomainRow) {
  const passed = [d.spf_verified, d.dkim_verified, d.dmarc_verified].filter(Boolean).length;
  return `${passed}/3`;
}

function dnsBadge(d: AdminDomainRow) {
  const passed = [d.spf_verified, d.dkim_verified, d.dmarc_verified].filter(Boolean).length;
  if (passed === 3) return "badge badge-success";
  if (passed === 0) return "badge badge-neutral";
  return "badge badge-warning";
}

function formatDate(value: string) {
  return new Date(value).toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}
</script>

<template>
  <div>
    <div class="page-header">
      <div>
        <h1>Domains</h1>
        <p class="page-subtitle">Every workspace's sending domains and their DNS state</p>
      </div>
    </div>

    <div class="card">
      <div class="card-body toolbar">
        <input
          v-model="search"
          class="form-input search"
          placeholder="Search by domain name…"
          @input="onSearchInput"
        />
        <div class="chips">
          <button class="chip" :class="{ active: status === '' }" @click="setStatus('')">All</button>
          <button class="chip" :class="{ active: status === 'verified' }" @click="setStatus('verified')">
            Verified
          </button>
          <button class="chip" :class="{ active: status === 'unverified' }" @click="setStatus('unverified')">
            Unverified
          </button>
        </div>
        <span class="count">
          {{ pageable.total_elements }} domain{{ pageable.total_elements === 1 ? "" : "s" }}
        </span>
      </div>

      <div v-if="loading && !domains.length" class="card-body">
        <div class="spinner"></div>
      </div>

      <div v-else-if="!domains.length" class="empty-state">
        <h3>{{ search.trim() || status ? "No domains match these filters" : "No domains yet" }}</h3>
        <p v-if="!search.trim() && !status">Domains appear here once a workspace adds one.</p>
      </div>

      <template v-else>
        <div class="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>Domain</th>
                <th>Workspace</th>
                <th>Owner</th>
                <th>Ownership</th>
                <th>SPF / DKIM / DMARC</th>
                <th>Added</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="d in domains"
                :key="d.id"
                style="cursor: pointer"
                @click="router.push(`/admin/domains/${d.id}`)"
              >
                <td class="mono">{{ d.domain }}</td>
                <td>{{ d.workspace_name || (d.workspace_id ? `#${d.workspace_id}` : "—") }}</td>
                <td class="muted-cell">{{ d.owner_email || `#${d.owner_id}` }}</td>
                <td>
                  <span :class="ownershipBadge(d)">
                    {{ d.ownership_verified ? "verified" : "unverified" }}
                  </span>
                </td>
                <td><span :class="dnsBadge(d)">{{ dnsSummary(d) }}</span></td>
                <td class="muted-cell">{{ formatDate(d.created_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <Pagination :pageable="pageable" @page="goToPage" />
      </template>
    </div>
  </div>
</template>

<style scoped>
.page-subtitle {
  font-size: 13px;
  color: var(--text-tertiary);
  margin-top: 4px;
}

.toolbar {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}

.search {
  flex: 1 1 260px;
  max-width: 360px;
}

.chips {
  display: flex;
  gap: 6px;
}

.chip {
  padding: 6px 12px;
  border-radius: 999px;
  border: 1px solid var(--border-primary);
  background: var(--bg-primary);
  color: var(--text-secondary);
  font: inherit;
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
}

.chip:hover {
  background: var(--bg-hover);
}

.chip.active {
  border-color: var(--primary-500);
  background: var(--primary-50);
  color: var(--primary-600);
}

.count {
  margin-left: auto;
  font-size: 12px;
  color: var(--text-tertiary);
}

.mono {
  font-family: "SFMono-Regular", Consolas, monospace;
  font-size: 13px;
}

.muted-cell {
  color: var(--text-tertiary);
  font-size: 13px;
}
</style>
