<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useRoute, useRouter } from "vue-router";
import { serversApi } from "../../api/servers";
import { useNotificationStore } from "../../stores/notification";
import { useConfirm } from "../../composables/useConfirm";
import type { SharedServer, SharedServerInput } from "../../api/types";
import { useModalSafeClose } from '../../composables/useModalSafeClose';

const route = useRoute();
const router = useRouter();
const notify = useNotificationStore();
const { confirm } = useConfirm();

const server = ref<SharedServer | null>(null);
const loading = ref(true);

const showModal = ref(false);
const form = ref<SharedServerInput>({
  name: "",
  host: "",
  port: 587,
  username: "",
  password: "",
  encryption: "starttls",
  max_retries: 0,
  allowed_domains: [],
  security_mode: "permissive",
});
const allowedDomainsText = ref("");
const saving = ref(false);

async function fetchServer() {
  loading.value = true;
  try {
    const id = Number(route.params.id);
    const res = await serversApi.get(id);
    server.value = res.data.data;
  } catch {
    notify.error("Failed to load server");
  } finally {
    loading.value = false;
  }
}

function openEdit() {
  if (!server.value) return;
  form.value = {
    name: server.value.name,
    host: server.value.host,
    port: server.value.port,
    username: server.value.username,
    password: "",
    encryption: server.value.encryption,
    max_retries: server.value.max_retries ?? 0,
    allowed_domains: server.value.allowed_domains ?? [],
    security_mode: server.value.security_mode ?? "permissive",
  };
  allowedDomainsText.value = (server.value.allowed_domains ?? []).join(", ");
  showModal.value = true;
}

async function save() {
  if (!server.value) return;
  saving.value = true;
  const data: SharedServerInput = {
    ...form.value,
    allowed_domains: allowedDomainsText.value
      .split(",")
      .map((d) => d.trim().toLowerCase())
      .filter((d) => d.length > 0),
  };
  try {
    await serversApi.update(server.value.id, data);
    notify.success("Server updated");
    showModal.value = false;
    await fetchServer();
  } catch {
    notify.error("Failed to update server");
  } finally {
    saving.value = false;
  }
}

async function toggleStatus() {
  if (!server.value) return;
  try {
    if (server.value.status !== "disabled") {
      await serversApi.disable(server.value.id);
      notify.success(`"${server.value.name}" disabled`);
    } else {
      await serversApi.enable(server.value.id);
      notify.success(`"${server.value.name}" enabled`);
    }
    await fetchServer();
  } catch {
    notify.error("Failed to update server status");
  }
}

async function testServer() {
  if (!server.value) return;
  try {
    const res = await serversApi.test(server.value.id);
    if (res.data.data.success) {
      notify.success("Connection test successful");
    } else {
      notify.error("Connection test failed: " + res.data.data.message);
    }
    await fetchServer();
  } catch {
    notify.error("Connection test failed");
  }
}

async function deleteServer() {
  if (!server.value) return;
  const confirmed = await confirm({
    title: "Delete Shared Server",
    message: `Are you sure you want to delete "${server.value.name}"? Any accounts relying on this server will stop receiving email delivery.`,
    confirmText: "Delete",
    variant: "danger",
  });
  if (!confirmed) return;
  try {
    await serversApi.delete(server.value.id);
    notify.success("Server deleted");
    router.push("/admin/servers");
  } catch {
    notify.error("Failed to delete server");
  }
}

function formatDate(date: string | null) {
  if (!date) return "-";
  return new Date(date).toLocaleString();
}

const { watchClickStart, confirmClickEnd } = useModalSafeClose(() => {
  showModal.value = false;
});

onMounted(fetchServer);
</script>

<template>
  <div>
    <div class="page-header">
      <div>
        <h1>{{ server?.name ?? "Server Details" }}</h1>
      </div>
      <div class="flex gap-2">
        <button class="btn btn-secondary" @click="router.push('/admin/servers')">
          Back
        </button>
      </div>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <template v-else-if="server">
      <!-- Info card -->
      <div class="card" style="margin-bottom: 24px">
        <div class="card-header">
          <h2>{{ server.name }}</h2>
          <div class="flex gap-2">
            <span class="badge badge-success" v-if="server.status === 'enabled'"
              >Enabled</span
            >
            <span class="badge badge-danger" v-else-if="server.status === 'invalid'"
              >Invalid</span
            >
            <span class="badge badge-neutral" v-else>Disabled</span>
            <span class="badge badge-warning" v-if="server.security_mode === 'strict'"
              >Strict</span
            >
            <span class="badge badge-neutral" v-else>Permissive</span>
          </div>
        </div>
        <div class="card-body">
          <table>
            <tbody>
              <tr>
                <td style="font-weight: 600; width: 160px">Host</td>
                <td>{{ server.host }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600">Port</td>
                <td>{{ server.port }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600">Encryption</td>
                <td>
                  <span class="badge badge-success" v-if="server.encryption === 'ssl'"
                    >SSL</span
                  >
                  <span
                    class="badge badge-info"
                    v-else-if="server.encryption === 'starttls'"
                    >STARTTLS</span
                  >
                  <span class="badge badge-neutral" v-else>None</span>
                </td>
              </tr>
              <tr>
                <td style="font-weight: 600">Username</td>
                <td>{{ server.username || "-" }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600">Max Retries</td>
                <td>{{ server.max_retries }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600">Security Mode</td>
                <td>
                  <span
                    class="badge badge-warning"
                    v-if="server.security_mode === 'strict'"
                    >Strict</span
                  >
                  <span class="badge badge-neutral" v-else>Permissive</span>
                  <span class="text-muted" style="margin-left: 8px; font-size: 13px">
                    {{
                      server.security_mode === "strict"
                        ? "Requires verified domain ownership"
                        : "Allows any matching domain"
                    }}
                  </span>
                </td>
              </tr>
              <tr>
                <td style="font-weight: 600">Allowed Domains</td>
                <td>
                  <span
                    v-if="server.allowed_domains && server.allowed_domains.length > 0"
                  >
                    <span
                      v-for="d in server.allowed_domains"
                      :key="d"
                      class="badge badge-neutral"
                      style="margin-right: 6px"
                      >{{ d }}</span
                    >
                  </span>
                  <span v-else class="text-muted">All domains</span>
                </td>
              </tr>
              <tr v-if="server.status === 'invalid' && server.validation_error">
                <td style="font-weight: 600">Validation Error</td>
                <td style="color: var(--color-danger, #e53e3e)">
                  {{ server.validation_error }}
                </td>
              </tr>
              <tr>
                <td style="font-weight: 600">Last Validated</td>
                <td>{{ formatDate(server.validated_at) }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600">Created</td>
                <td>{{ formatDate(server.created_at) }}</td>
              </tr>
              <tr>
                <td style="font-weight: 600">Updated</td>
                <td>{{ formatDate(server.updated_at) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- Stats -->
      <div class="stats-grid" style="margin-bottom: 24px">
        <div class="stat-card">
          <div class="stat-label">Sent</div>
          <div class="stat-value">{{ server.sent_count.toLocaleString() }}</div>
        </div>
        <div class="stat-card">
          <div class="stat-label">Failed</div>
          <div class="stat-value">{{ server.failed_count.toLocaleString() }}</div>
        </div>
      </div>

      <!-- Actions -->
      <div class="card">
        <div class="card-header"><h2>Actions</h2></div>
        <div class="card-body">
          <div class="flex gap-2">
            <button class="btn btn-primary" @click="openEdit">Edit</button>
            <button class="btn btn-secondary" @click="testServer">Test Connection</button>
            <button class="btn btn-secondary" @click="toggleStatus">
              {{ server.status !== "disabled" ? "Disable" : "Enable" }}
            </button>
            <button class="btn btn-danger" @click="deleteServer">Delete</button>
          </div>
        </div>
      </div>
    </template>

    <div v-else class="empty-state">
      <h3>Server not found</h3>
    </div>

    <!-- Edit Modal -->
    <div
      v-if="showModal"
      class="modal-overlay"
      @mousedown="watchClickStart"
      @mouseup="confirmClickEnd"
    >
      <div class="modal" @mousedown.stop @mouseup.stop>
        <div class="modal-header">
          <h3>Edit Shared Server</h3>
        </div>
        <form @submit.prevent="save">
          <div class="modal-body">
            <div class="form-group">
              <label class="form-label">Name</label>
              <input v-model="form.name" type="text" class="form-input" required />
            </div>
            <div class="form-group">
              <label class="form-label">Host</label>
              <input v-model="form.host" type="text" class="form-input" required />
            </div>
            <div class="form-group">
              <label class="form-label">Port</label>
              <input
                v-model.number="form.port"
                type="number"
                class="form-input"
                required
              />
            </div>
            <div class="form-group">
              <label class="form-label">Username</label>
              <input v-model="form.username" type="text" class="form-input" />
            </div>
            <div class="form-group">
              <label class="form-label">Password</label>
              <input
                v-model="form.password"
                type="password"
                class="form-input"
                placeholder="Leave blank to keep current"
              />
            </div>
            <div class="form-group">
              <label class="form-label">Encryption</label>
              <select v-model="form.encryption" class="form-select">
                <option value="none">None</option>
                <option value="starttls">STARTTLS (port 587)</option>
                <option value="ssl">SSL/TLS (port 465)</option>
              </select>
            </div>
            <div class="form-group">
              <label class="form-label">Max Retries</label>
              <input
                v-model.number="form.max_retries"
                type="number"
                class="form-input"
                min="0"
                max="10"
              />
              <span class="form-hint"
                >Number of times to retry a failed send. Set to 0 to disable
                retries.</span
              >
            </div>
            <div class="form-group">
              <label class="form-label">Allowed Domains</label>
              <input
                v-model="allowedDomainsText"
                type="text"
                class="form-input"
                placeholder="example.com, acme.org"
              />
              <span class="form-hint"
                >Comma-separated. Leave empty to allow all domains.</span
              >
            </div>
            <div class="form-group">
              <label class="form-label">Security Mode</label>
              <select v-model="form.security_mode" class="form-select">
                <option value="permissive">
                  Permissive — allow any user whose sender domain matches
                </option>
                <option value="strict">Strict — require verified domain ownership</option>
              </select>
              <span class="form-hint"
                >In strict mode, users must verify ownership of their sender domain before
                this server is available to them.</span
              >
            </div>
          </div>
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" @click="showModal = false">
              Cancel
            </button>
            <button type="submit" class="btn btn-primary" :disabled="saving">
              {{ saving ? "Saving..." : "Update" }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>
