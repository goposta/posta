<script setup lang="ts">
import { onMounted, ref } from "vue";
import { messagesApi, teamsApi } from "../../api/messages";
import { useNotificationStore } from "../../stores/notification";
import { apiMessage } from "../../composables/apiError";
import type { Message, MessagePriority, Team } from "../../api/types";

// Tags and priority on one message. Each change saves at once and hands the
// updated message back, so the page never holds unsaved triage.
const props = defineProps<{ message: Message }>();
const emit = defineEmits<{ updated: [message: Message] }>();

const notify = useNotificationStore();
const busy = ref(false);
const draft = ref("");
const suggestions = ref<string[]>([]);
const teams = ref<Team[]>([]);
const listId = `tag-suggestions-${props.message.uuid}`;

const PRIORITIES: { value: MessagePriority; label: string }[] = [
  { value: "low", label: "Low" },
  { value: "normal", label: "Normal" },
  { value: "high", label: "High" },
  { value: "urgent", label: "Urgent" },
];

onMounted(async () => {
  try {
    const res = await messagesApi.tags();
    suggestions.value = res.data.data.map((t) => t.tag);
  } catch {
    /* suggestions are a convenience */
  }
  try {
    teams.value = (await teamsApi.list()).data.data;
  } catch {
    /* without teams the picker stays hidden */
  }
});

async function setTeam(event: Event) {
  const raw = (event.target as HTMLSelectElement).value;
  await run(() => messagesApi.setTeam(props.message.uuid, raw ? Number(raw) : null), "Failed to change the team");
}

async function run(action: () => Promise<{ data: { data: Message } }>, failure: string) {
  busy.value = true;
  try {
    const res = await action();
    emit("updated", res.data.data);
    return true;
  } catch (e: any) {
    notify.error(apiMessage(e, failure));
    return false;
  } finally {
    busy.value = false;
  }
}

async function setPriority(event: Event) {
  const priority = (event.target as HTMLSelectElement).value as MessagePriority;
  await run(() => messagesApi.setPriority(props.message.uuid, priority), "Failed to set priority");
}

async function addTags() {
  const tags = draft.value
    .split(/[,;]/)
    .map((t) => t.trim())
    .filter(Boolean);
  if (!tags.length) return;
  if (await run(() => messagesApi.updateTags(props.message.uuid, { add: tags }), "Failed to add tag")) {
    draft.value = "";
    for (const t of tags) {
      const tag = t.toLowerCase();
      if (!suggestions.value.includes(tag)) suggestions.value.push(tag);
    }
  }
}

function removeTag(tag: string) {
  run(() => messagesApi.updateTags(props.message.uuid, { remove: [tag] }), "Failed to remove tag");
}

function onKey(event: KeyboardEvent) {
  if (event.key === "Enter" || event.key === ",") {
    event.preventDefault();
    addTags();
  }
}
</script>

<template>
  <div class="triage">
    <label class="triage-priority">
      <span class="triage-label">Priority</span>
      <select
        class="form-select"
        :value="message.priority || 'normal'"
        :disabled="busy"
        @change="setPriority"
      >
        <option v-for="p in PRIORITIES" :key="p.value" :value="p.value">{{ p.label }}</option>
      </select>
    </label>

    <label v-if="teams.length || message.team" class="triage-priority">
      <span class="triage-label">Team</span>
      <select class="form-select" :value="message.team_id || ''" :disabled="busy" @change="setTeam">
        <option value="">No team</option>
        <option v-for="t in teams" :key="t.id" :value="t.id">{{ t.name }}</option>
      </select>
    </label>

    <div class="triage-tags">
      <span class="triage-label">Tags</span>
      <div class="tag-row">
        <span v-for="tag in message.tags" :key="tag" class="tag-chip">
          {{ tag }}
          <button
            type="button"
            class="tag-remove"
            :disabled="busy"
            :aria-label="`Remove tag ${tag}`"
            @click="removeTag(tag)"
          >×</button>
        </span>
        <input
          v-model="draft"
          class="form-input tag-input"
          :list="listId"
          maxlength="32"
          placeholder="Add a tag…"
          :disabled="busy"
          aria-label="Add a tag"
          @keydown="onKey"
          @blur="addTags"
        />
        <datalist :id="listId">
          <option v-for="s in suggestions.filter((s) => !message.tags.includes(s))" :key="s" :value="s" />
        </datalist>
      </div>
    </div>
  </div>
</template>

<style scoped>
.triage {
  display: flex;
  flex-wrap: wrap;
  gap: 16px 24px;
  align-items: flex-start;
}

.triage-label {
  display: block;
  margin-bottom: 6px;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  font-weight: 600;
  color: var(--text-tertiary);
}

.triage-priority .form-select {
  width: 140px;
}

.triage-tags {
  flex: 1 1 280px;
  min-width: 0;
}

.tag-row {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
}

.tag-chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px 4px 3px 10px;
  border-radius: 999px;
  background: var(--bg-tertiary);
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.4;
}

.tag-remove {
  border: 0;
  background: none;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  color: var(--text-muted);
  font-size: 15px;
  line-height: 1;
  cursor: pointer;
}

.tag-remove:hover,
.tag-remove:focus-visible {
  background: var(--bg-secondary);
  color: var(--text-primary);
}

.tag-input {
  flex: 1 1 140px;
  min-width: 120px;
  max-width: 220px;
}
</style>
