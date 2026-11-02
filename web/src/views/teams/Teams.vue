<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { teamsApi } from "../../api/messages";
import { workspaceApi } from "../../api/workspaces";
import { useNotificationStore } from "../../stores/notification";
import { apiMessage } from "../../composables/apiError";
import { useLicenseStore } from "../../stores/license";
import EnterpriseLock from "../../components/EnterpriseLock.vue";
import type { Team, WorkspaceMember } from "../../api/types";

// Teams group workspace members so routing rules can notify or assign them
// together. Any member can see them; owners and admins change them. Teams are
// a Posta Enterprise feature.

const notify = useNotificationStore();
const license = useLicenseStore();
const loading = ref(true);
const teams = ref<Team[]>([]);
const members = ref<WorkspaceMember[]>([]);

const editing = ref<Team | null>(null);
const showEditor = ref(false);
const saving = ref(false);
const draft = ref({ name: "", email: "", user_ids: [] as number[] });

const memberName = computed(() => {
  const out: Record<number, string> = {};
  members.value.forEach((m) => (out[m.user_id] = m.name || m.email));
  return out;
});

async function load() {
  loading.value = true;
  try {
    const [teamRes, memberRes] = await Promise.all([teamsApi.list(), workspaceApi.listMembers()]);
    teams.value = teamRes.data.data;
    members.value = memberRes.data.data || [];
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to load teams"));
  } finally {
    loading.value = false;
  }
}

function openEditor(team: Team | null) {
  editing.value = team;
  draft.value = {
    name: team?.name || "",
    email: team?.email || "",
    user_ids: (team?.members || []).map((m) => m.user_id),
  };
  showEditor.value = true;
}

function toggleMember(id: number) {
  const ids = draft.value.user_ids;
  draft.value.user_ids = ids.includes(id) ? ids.filter((x) => x !== id) : [...ids, id];
}

async function save() {
  saving.value = true;
  try {
    const payload = { name: draft.value.name.trim(), email: draft.value.email.trim(), user_ids: draft.value.user_ids };
    if (editing.value) await teamsApi.update(editing.value.id, payload);
    else await teamsApi.create(payload);
    showEditor.value = false;
    notify.success(editing.value ? "Team saved." : "Team created.");
    await load();
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to save team"));
  } finally {
    saving.value = false;
  }
}

async function remove(team: Team) {
  if (!confirm(`Delete the team "${team.name}"? Submissions routed to it keep no team.`)) return;
  try {
    await teamsApi.delete(team.id);
    notify.success("Team deleted.");
    await load();
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to delete team"));
  }
}

function memberList(team: Team) {
  const names = (team.members || []).map((m) => memberName.value[m.user_id] || m.user?.name || `User ${m.user_id}`);
  return names.length ? names.join(", ") : "No members";
}

onMounted(async () => {
  await license.load();
  if (license.licensed) await load();
});
</script>

<template>
  <div>
    <div class="page-header">
      <h1>Teams</h1>
      <button v-if="license.mutable" class="btn btn-primary" @click="openEditor(null)">New team</button>
    </div>

    <EnterpriseLock
      feature="Teams"
      description="Group members into teams like Sales or Billing. Routing rules notify a team or assign submissions to it, and its members see them under “My work” in Messages."
    >
      <div v-if="loading" class="loading-page"><div class="spinner"></div></div>

      <div v-else class="card">
        <div v-if="!teams.length" class="empty-state">
          <h3>No teams yet</h3>
          <p>
            A team is a group of members, like Sales or Billing. Form routing rules can notify a team or assign
            submissions to it, and everyone on it sees them under "My work" in Messages.
          </p>
          <button v-if="license.mutable" class="btn btn-primary" @click="openEditor(null)">Create a team</button>
        </div>
        <div v-else class="table-wrapper">
          <table>
            <thead>
              <tr>
                <th>Team</th>
                <th>Members</th>
                <th>Shared address</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="team in teams" :key="team.id">
                <td><strong>{{ team.name }}</strong></td>
                <td class="team-members">{{ memberList(team) }}</td>
                <td>{{ team.email || "—" }}</td>
                <td class="team-actions">
                  <router-link class="btn btn-ghost btn-sm" :to="`/messages?team_id=${team.id}`">Messages</router-link>
                  <template v-if="license.mutable">
                    <button class="btn btn-secondary btn-sm" @click="openEditor(team)">Edit</button>
                    <button class="btn btn-ghost btn-sm" @click="remove(team)">Delete</button>
                  </template>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </EnterpriseLock>

    <div v-if="showEditor" class="modal-overlay" @click.self="showEditor = false">
      <div class="modal">
        <div class="modal-header">
          <h3>{{ editing ? `Edit ${editing.name}` : "New team" }}</h3>
        </div>
        <div class="modal-body">
          <div class="form-group">
            <label class="form-label" for="team-name">Name</label>
            <input id="team-name" v-model="draft.name" class="form-input" placeholder="Sales" maxlength="80" />
          </div>
          <div class="form-group">
            <label class="form-label" for="team-email">Shared address <span class="muted">(optional)</span></label>
            <input id="team-email" v-model="draft.email" type="email" class="form-input" placeholder="sales@example.com" />
            <small class="muted">Notified along with the members when a rule notifies this team.</small>
          </div>
          <fieldset class="form-group team-pick">
            <legend class="form-label">Members</legend>
            <label v-for="m in members" :key="m.user_id" class="checkbox-label">
              <input type="checkbox" :checked="draft.user_ids.includes(m.user_id)" @change="toggleMember(m.user_id)" />
              {{ m.name || m.email }} <span class="muted">{{ m.email }}</span>
            </label>
          </fieldset>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="showEditor = false">Cancel</button>
          <button class="btn btn-primary" :disabled="saving || !draft.name.trim()" @click="save">
            {{ saving ? "Saving…" : editing ? "Save team" : "Create team" }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.team-members {
  color: var(--text-secondary);
  max-width: 420px;
}

.team-actions {
  display: flex;
  gap: 6px;
  justify-content: flex-end;
  white-space: nowrap;
}

.team-pick {
  border: 0;
  padding: 0;
  margin-inline: 0;
  display: grid;
  gap: 6px;
  max-height: 260px;
  overflow-y: auto;
}

.team-pick legend {
  padding: 0;
  margin-bottom: 6px;
}
</style>
