<script setup lang="ts">
import { useRouter } from "vue-router";
import type { Email } from "../../api/types";

defineProps<{ emails: Email[] }>();
const router = useRouter();

function statusBadgeClass(status: string) {
  switch (status) {
    case "sent":
      return "badge badge-success";
    case "failed":
      return "badge badge-danger";
    case "pending":
    case "processing":
      return "badge badge-warning";
    case "queued":
    case "scheduled":
      return "badge badge-info";
    case "suppressed":
      return "badge badge-secondary";
    default:
      return "badge";
  }
}

function relativeTime(date: string | null) {
  if (!date) return "-";
  const then = new Date(date).getTime();
  if (Number.isNaN(then)) return "-";
  const secs = Math.floor((Date.now() - then) / 1000);
  if (secs < 60) return `${Math.max(0, secs)}s ago`;
  if (secs < 3600) return `${Math.floor(secs / 60)}m ago`;
  if (secs < 86400) return `${Math.floor(secs / 3600)}h ago`;
  return `${Math.floor(secs / 86400)}d ago`;
}
</script>

<template>
  <div class="card">
    <div class="card-header">
      <h2>Recent emails</h2>
      <button class="btn btn-secondary btn-sm" @click="router.push('/emails')">View all</button>
    </div>

    <div v-if="!emails.length" class="empty-state">
      <h3>No emails yet</h3>
      <p>Messages you send through the API, the relay, or a campaign appear here.</p>
    </div>

    <div v-else class="table-wrapper">
      <table>
        <thead>
          <tr>
            <th>Recipient</th>
            <th>Subject</th>
            <th>Status</th>
            <th>Sent</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="email in emails"
            :key="email.uuid"
            style="cursor: pointer"
            @click="router.push(`/emails/${email.uuid}`)"
          >
            <td class="cell-truncate">{{ email.recipients?.join(", ") || "-" }}</td>
            <td class="cell-truncate">{{ email.subject || "(no subject)" }}</td>
            <td><span :class="statusBadgeClass(email.status)">{{ email.status }}</span></td>
            <td :title="email.sent_at || email.created_at">
              {{ relativeTime(email.sent_at || email.created_at) }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
.btn-sm {
  padding: 5px 12px;
  font-size: 12px;
}

.cell-truncate {
  max-width: 220px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
