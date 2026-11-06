<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { teamsApi } from "../../api/messages";
import { workspaceApi } from "../../api/workspaces";
import { useNotificationStore } from "../../stores/notification";
import { useLicenseStore } from "../../stores/license";
import { useConfirm } from "../../composables/useConfirm";
import { apiMessage } from "../../composables/apiError";
import EnterpriseLock from "../../components/EnterpriseLock.vue";
import type { TeamDetail, WorkspaceMember } from "../../api/types";

const route = useRoute();
const router = useRouter();
const notify = useNotificationStore();
const license = useLicenseStore();
const { confirm } = useConfirm();

const id = Number(route.params.id);
const team = ref<TeamDetail | null>(null);
const workspaceMembers = ref<WorkspaceMember[]>([]);
const loading = ref(true);
const notFound = ref(false);
const saving = ref(false);

const addUserId = ref<number | "">("");
const showEditor = ref(false);
const draft = ref({ name: "", email: "" });

const memberIds = computed(() => (team.value?.members ?? []).map((m) => m.user_id));
const addable = computed(() => workspaceMembers.value.filter((m) => !memberIds.value.includes(m.user_id)));

async function load() {
  loading.value = true;
  try {
    const [teamRes, membersRes] = await Promise.all([teamsApi.get(id), workspaceApi.listMembers()]);
    team.value = teamRes.data.data;
    workspaceMembers.value = membersRes.data.data || [];
  } catch (e: any) {
    if (e?.response?.status === 404) notFound.value = true;
    else notify.error(apiMessage(e, "Failed to load the team"));
  } finally {
    loading.value = false;
  }
}

// The API replaces the whole team, so every change sends name, address and members.
async function save(changes: { name?: string; email?: string; user_ids?: number[] }, success: string) {
  const t = team.value;
  if (!t) return false;
  saving.value = true;
  try {
    await teamsApi.update(t.id, {
      name: changes.name ?? t.name,
      email: changes.email ?? t.email,
      user_ids: changes.user_ids ?? memberIds.value,
    });
    notify.success(success);
    await load();
    return true;
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to save the team"));
    return false;
  } finally {
    saving.value = false;
  }
}

async function addMember() {
  const userId = addUserId.value;
  if (userId === "") return;
  const who = workspaceMembers.value.find((m) => m.user_id === userId);
  if (await save({ user_ids: [...memberIds.value, userId] }, `${who?.name || who?.email || "Member"} added.`)) {
    addUserId.value = "";
  }
}

async function removeMember(userId: number, name: string) {
  const ok = await confirm({
    title: `Remove ${name} from ${team.value?.name}?`,
    message: "They stop being notified when a rule notifies this team, and the team's work leaves their “My work” view. Submissions already assigned to them stay assigned.",
    confirmText: "Remove",
    variant: "danger",
  });
  if (!ok) return;
  await save({ user_ids: memberIds.value.filter((x) => x !== userId) }, `${name} removed.`);
}

function openEditor() {
  if (!team.value) return;
  draft.value = { name: team.value.name, email: team.value.email };
  showEditor.value = true;
}

async function saveDetails() {
  if (await save({ name: draft.value.name.trim(), email: draft.value.email.trim() }, "Team saved.")) {
    showEditor.value = false;
  }
}

async function remove() {
  const t = team.value;
  if (!t) return;
  const ok = await confirm({
    title: `Delete the team "${t.name}"?`,
    message: "Submissions routed to it keep no team. A team that routing rules still use can't be deleted.",
    confirmText: "Delete team",
    variant: "danger",
  });
  if (!ok) return;
  try {
    await teamsApi.delete(t.id);
    notify.success("Team deleted.");
    router.push("/teams");
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to delete the team"));
  }
}

function roleLabel(role: string) {
  return role ? role.charAt(0).toUpperCase() + role.slice(1) : "—";
}

onMounted(async () => {
  await license.load();
  if (license.licensed) await load();
  else loading.value = false;
});
</script>

<template>
  <div>
    <div class="page-header">
      <div>
        <h1>{{ team?.name || "Team" }}</h1>
        <p v-if="team" class="team-subtitle">
          {{ team.email ? `Shared address ${team.email}` : "No shared address" }}
        </p>
      </div>
      <div class="header-actions">
        <router-link class="btn btn-secondary" to="/teams">Back</router-link>
        <template v-if="team && license.mutable">
          <button class="btn btn-secondary" @click="openEditor">Edit</button>
          <button class="btn btn-danger" @click="remove">Delete</button>
        </template>
      </div>
    </div>

    <EnterpriseLock
      feature="Teams"
      description="Group members into teams like Sales or Billing. Routing rules notify a team or assign submissions to it, and its members see them under “My work” in Messages."
    >
      <div v-if="loading" class="loading-page"><div class="spinner"></div></div>

      <div v-else-if="notFound || !team" class="card">
        <div class="empty-state">
          <h3>Team not found</h3>
          <router-link class="btn btn-secondary" to="/teams">Back to teams</router-link>
        </div>
      </div>

      <template v-else>
        <div class="stats-grid team-stats">
          <router-link class="stat-card" :to="`/messages?team_id=${team.id}`">
            <div class="stat-label">Open submissions</div>
            <div class="stat-value">{{ team.open_messages }}</div>
          </router-link>
          <router-link class="stat-card" :to="`/messages?team_id=${team.id}`">
            <div class="stat-label">All submissions</div>
            <div class="stat-value">{{ team.messages }}</div>
          </router-link>
          <div class="stat-card">
            <div class="stat-label">Members</div>
            <div class="stat-value">{{ team.members.length }}</div>
          </div>
        </div>

        <div class="card section-card">
          <div class="card-header">
            <h2>Members ({{ team.members.length }})</h2>
          </div>
          <div class="card-body">
            <div v-if="license.mutable" class="add-member">
              <select v-model="addUserId" class="form-select" aria-label="Workspace member to add" :disabled="!addable.length">
                <option value="" disabled>{{ addable.length ? "Add a workspace member…" : "Every workspace member is on this team" }}</option>
                <option v-for="m in addable" :key="m.user_id" :value="m.user_id">
                  {{ m.name || m.email }}{{ m.name ? ` — ${m.email}` : "" }}
                </option>
              </select>
              <button class="btn btn-primary" :disabled="saving || addUserId === ''" @click="addMember">Add</button>
            </div>

            <p v-if="!team.members.length" class="muted empty-members">
              No members yet. Rules that notify this team reach only its shared address{{ team.email ? "" : ", which isn't set" }}.
            </p>
            <div v-else class="table-wrapper">
              <table>
                <thead>
                  <tr>
                    <th>Name</th>
                    <th>Email</th>
                    <th>Workspace role</th>
                    <th v-if="license.mutable"></th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="m in team.members" :key="m.user_id">
                    <td><strong>{{ m.name || "—" }}</strong></td>
                    <td class="muted">{{ m.email }}</td>
                    <td><span class="badge badge-secondary">{{ roleLabel(m.role) }}</span></td>
                    <td v-if="license.mutable" class="row-actions">
                      <button class="btn btn-ghost btn-sm" :disabled="saving" @click="removeMember(m.user_id, m.name || m.email)">
                        Remove
                      </button>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>

        <div class="card section-card">
          <div class="card-header">
            <h2>Routing rules ({{ team.rules.length }})</h2>
          </div>
          <div class="card-body">
            <p v-if="!team.rules.length" class="muted">
              No routing rule notifies or assigns this team yet. Add one on a form's Routing tab.
            </p>
            <div v-else class="table-wrapper">
              <table>
                <thead>
                  <tr>
                    <th>Rule</th>
                    <th>Form</th>
                    <th>Status</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="r in team.rules" :key="r.id">
                    <td><strong>{{ r.name }}</strong></td>
                    <td>
                      <router-link :to="`/forms/${r.form_id}?tab=routing`">{{ r.form_name }}</router-link>
                    </td>
                    <td>
                      <span v-if="r.enabled" class="badge badge-success">On</span>
                      <span v-else class="badge badge-neutral">Off</span>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
      </template>
    </EnterpriseLock>

    <div v-if="showEditor" class="modal-overlay" @click.self="showEditor = false">
      <div class="modal">
        <div class="modal-header">
          <h3>Edit {{ team?.name }}</h3>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label class="form-label" for="team-name">Name</label>
            <input id="team-name" v-model="draft.name" class="form-input" maxlength="80" />
          </div>
          <div class="form-group">
            <label class="form-label" for="team-email">Shared address <span class="muted">(optional)</span></label>
            <input id="team-email" v-model="draft.email" type="email" class="form-input" placeholder="sales@example.com" />
            <small class="muted">Notified along with the members when a rule notifies this team.</small>
          </div>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="showEditor = false">Cancel</button>
          <button class="btn btn-primary" :disabled="saving || !draft.name.trim()" @click="saveDetails">
            {{ saving ? "Saving…" : "Save team" }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.team-subtitle {
  margin: 4px 0 0;
  color: var(--text-muted);
  font-size: 13px;
}

.header-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.team-stats {
  margin-bottom: 20px;
}

.team-stats a.stat-card {
  color: inherit;
  text-decoration: none;
}

.section-card {
  margin-bottom: 20px;
}

.add-member {
  display: flex;
  gap: 8px;
  margin-bottom: 16px;
  max-width: 560px;
}

.add-member select {
  flex: 1;
  min-width: 0;
}

.empty-members {
  margin: 0;
}

.row-actions {
  text-align: right;
}
</style>
