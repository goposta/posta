<script setup lang="ts">
import { onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { adminApi } from "../../api/admin";
import { plansApi } from "../../api/plans";
import { useNotificationStore } from "../../stores/notification";
import { apiMessage } from "../../composables/apiError";
import { usePagination } from "../../composables/usePagination";
import Pagination from "../../components/Pagination.vue";
import type { AdminWorkspaceRow, Plan } from "../../api/types";

const router = useRouter();
const notify = useNotificationStore();

const workspaces = ref<AdminWorkspaceRow[]>([]);
const plans = ref<Plan[]>([]);
const loading = ref(true);
const search = ref("");
const planFilter = ref(0);

let searchTimer: ReturnType<typeof setTimeout> | undefined;

const { pageable, goToPage } = usePagination(async (page) => {
  loading.value = true;
  try {
    const res = await adminApi.listWorkspaces(page, pageable.value.size, search.value.trim(), planFilter.value);
    workspaces.value = res.data.data;
    pageable.value = res.data.pageable;
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to load workspaces"));
  } finally {
    loading.value = false;
  }
});

function onSearchInput() {
  if (searchTimer) clearTimeout(searchTimer);
  searchTimer = setTimeout(() => goToPage(0), 300);
}

function formatDate(value: string) {
  return new Date(value).toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}

onMounted(async () => {
  try {
    plans.value = (await plansApi.list(0, 100)).data.data;
  } catch {
    // The plan filter is optional; the list still works without it.
  }
});
</script>

<template>
  <div>
    <div class="page-header">
      <div>
        <h1>Workspaces</h1>
        <p class="page-subtitle">Every workspace on the platform, with its owner, plan and members</p>
      </div>
    </div>

    <div class="card">
      <div class="card-body toolbar">
        <input
          v-model="search"
          class="form-input search"
          placeholder="Search by display name or name…"
          @input="onSearchInput"
        />
        <select v-model.number="planFilter" class="form-select plan-filter" @change="goToPage(0)">
          <option :value="0">All plans</option>
          <option v-for="plan in plans" :key="plan.id" :value="plan.id">{{ plan.name }}</option>
        </select>
        <span class="count">
          {{ pageable.total_elements }} workspace{{ pageable.total_elements === 1 ? "" : "s" }}
        </span>
      </div>

      <div v-if="loading && !workspaces.length" class="card-body">
        <div class="spinner"></div>
      </div>

      <div v-else-if="!workspaces.length" class="empty-state">
        <h3>{{ search.trim() || planFilter ? "No workspaces match these filters" : "No workspaces yet" }}</h3>
      </div>

      <template v-else>
        <div class="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>Display name</th>
                <th>Name</th>
                <th>Owner</th>
                <th>Plan</th>
                <th>Members</th>
                <th>Created</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="ws in workspaces"
                :key="ws.id"
                style="cursor: pointer"
                @click="router.push(`/admin/workspaces/${ws.id}`)"
              >
                <td>
                  {{ ws.display_name }}
                  <span v-if="ws.system" class="badge badge-info" style="margin-left: 6px">system</span>
                </td>
                <td class="mono">{{ ws.name }}</td>
                <td class="muted-cell">{{ ws.owner_email || `#${ws.owner_id}` }}</td>
                <td>
                  <span v-if="ws.plan_name" class="badge badge-neutral">{{ ws.plan_name }}</span>
                  <span v-else class="muted-cell">Default</span>
                </td>
                <td>{{ ws.member_count }}</td>
                <td class="muted-cell">{{ formatDate(ws.created_at) }}</td>
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

.plan-filter {
  max-width: 200px;
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
