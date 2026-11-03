<script setup lang="ts">
import { onMounted, ref } from "vue";
import { adminApi } from "../../api/admin";
import type { Announcement, NotificationSeverity } from "../../api/inbox";
import { useNotificationStore } from "../../stores/notification";
import { useConfirm } from "../../composables/useConfirm";
import { apiMessage } from "../../composables/apiError";
import EnterpriseLock from "../../components/EnterpriseLock.vue";
import { useLicenseStore } from "../../stores/license";

const notify = useNotificationStore();
const { confirm } = useConfirm();

const items = ref<Announcement[]>([]);
const loading = ref(true);
const sending = ref(false);

const title = ref("");
const message = ref("");
const link = ref("");
const severity = ref<NotificationSeverity>("info");

async function load() {
  loading.value = true;
  try {
    items.value = (await adminApi.listAnnouncements(0, 50)).data.data ?? [];
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to load announcements"));
  } finally {
    loading.value = false;
  }
}

async function send() {
  if (!title.value.trim()) return;
  const confirmed = await confirm({
    title: "Broadcast to every user",
    message:
      "This puts the notice in every active user's inbox immediately. You can retract it afterwards, which also removes it from their inboxes.",
    confirmText: "Broadcast",
  });
  if (!confirmed) return;

  sending.value = true;
  try {
    const res = await adminApi.createAnnouncement({
      title: title.value.trim(),
      message: message.value.trim(),
      link: link.value.trim() || undefined,
      severity: severity.value,
    });
    notify.success(`Delivered to ${res.data.data.recipients} user${res.data.data.recipients === 1 ? "" : "s"}`);
    title.value = "";
    message.value = "";
    link.value = "";
    severity.value = "info";
    await load();
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to broadcast"));
  } finally {
    sending.value = false;
  }
}

async function retract(a: Announcement) {
  const confirmed = await confirm({
    title: "Retract announcement",
    message: `"${a.title}" will be removed from every inbox it was delivered to.`,
    confirmText: "Retract",
    variant: "danger",
  });
  if (!confirmed) return;
  try {
    await adminApi.retractAnnouncement(a.id);
    notify.success("Announcement retracted");
    await load();
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to retract"));
  }
}

function formatDate(iso?: string | null) {
  return iso ? new Date(iso).toLocaleString() : "—";
}

const license = useLicenseStore();

onMounted(async () => {
  await license.load();
  if (license.licensed) await load();
});
</script>

<template>
  <div>
    <div class="page-header">
      <div>
        <h1>Announcements</h1>
        <p class="page-subtitle">
          Put a notice in every user's in-app inbox — planned maintenance, a breaking upgrade, a
          policy change.
        </p>
      </div>
    </div>

    <EnterpriseLock
      feature="Announcements"
      description="Put a notice in every user's in-app inbox, such as planned maintenance or a breaking upgrade. Warning and critical notices also appear on the workspace dashboard."
    >
    <div v-if="license.mutable" class="card" style="margin-bottom: 20px">
      <div class="card-header"><h2>New announcement</h2></div>
      <div class="card-body">
        <div class="form-group">
          <label class="form-label" for="ann-title">Title</label>
          <input id="ann-title" v-model="title" class="form-input" maxlength="200" placeholder="Scheduled maintenance on Sunday" />
        </div>
        <div class="form-group">
          <label class="form-label" for="ann-message">Message</label>
          <textarea id="ann-message" v-model="message" class="form-input" rows="3" maxlength="2000"
            placeholder="Posta will be unavailable between 02:00 and 03:00 UTC while the database is upgraded."></textarea>
        </div>
        <div class="form-row">
          <div class="form-group">
            <label class="form-label" for="ann-link">Link</label>
            <input id="ann-link" v-model="link" class="form-input" placeholder="/status (optional)" />
            <span class="form-hint">Where the notice takes the reader. Leave empty for a notice with no action.</span>
          </div>
          <div class="form-group">
            <label class="form-label" for="ann-sev">Severity</label>
            <select id="ann-sev" v-model="severity" class="form-select">
              <option value="info">Info</option>
              <option value="warning">Warning</option>
              <option value="critical">Critical</option>
            </select>
            <span class="form-hint">Warning and critical also appear on the workspace dashboard.</span>
          </div>
        </div>
        <button class="btn btn-primary" :disabled="sending || !title.trim()" @click="send">
          {{ sending ? "Broadcasting…" : "Broadcast" }}
        </button>
      </div>
    </div>

    <div class="card">
      <div class="card-header"><h2>Sent</h2></div>
      <div v-if="loading" class="loading-page"><div class="spinner"></div></div>
      <div v-else-if="!items.length" class="empty-state">
        <h3>Nothing broadcast yet</h3>
      </div>
      <table v-else>
        <thead>
          <tr>
            <th>Title</th>
            <th>Severity</th>
            <th>Recipients</th>
            <th>Sent</th>
            <th>By</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="a in items" :key="a.id">
            <td>
              <div class="ann-title">{{ a.title }}</div>
              <div class="ann-body">{{ a.message }}</div>
            </td>
            <td>
              <span class="badge" :class="a.severity === 'critical' ? 'badge-danger' : a.severity === 'warning' ? 'badge-warning' : 'badge-neutral'">
                {{ a.severity }}
              </span>
            </td>
            <td>{{ a.recipients }}</td>
            <td>{{ formatDate(a.sent_at) }}</td>
            <td>{{ a.author_name || `#${a.created_by}` }}</td>
            <td class="right">
              <button class="btn btn-secondary btn-sm" @click="retract(a)">Retract</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    </EnterpriseLock>
  </div>
</template>

<style scoped>
.page-subtitle {
  font-size: 13px;
  color: var(--text-tertiary);
  margin-top: 4px;
}

.form-row {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 16px;
}

.ann-title {
  font-weight: 600;
  color: var(--text-primary);
}

.ann-body {
  font-size: 12px;
  color: var(--text-tertiary);
  margin-top: 2px;
  max-width: 460px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.right {
  text-align: right;
}

.btn-sm {
  padding: 5px 12px;
  font-size: 12px;
}
</style>
