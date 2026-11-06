<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { conversationsApi } from "../../api/chat";
import type { ChatAttachment } from "../../api/types";

// One file in a chat message. The download route needs the workspace header,
// so the file is fetched through the API client and shown from a blob URL,
// never from a plain <img src>.
const props = defineProps<{
  conversation: string;
  messageId: string;
  index: number;
  attachment: ChatAttachment;
}>();

const url = ref("");
const failed = ref(false);
const isImage = computed(() => props.attachment.content_type.startsWith("image/"));

function release() {
  if (url.value) URL.revokeObjectURL(url.value);
  url.value = "";
}

async function load() {
  release();
  failed.value = false;
  // Local, not yet stored messages have no file to fetch.
  if (!props.messageId || props.messageId.startsWith("local-")) return;
  try {
    const res = await conversationsApi.attachment(props.conversation, props.messageId, props.index);
    url.value = URL.createObjectURL(res.data);
  } catch {
    failed.value = true;
  }
}

function size(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${Math.round(n / 1024)} KB`;
  return `${(n / 1024 / 1024).toFixed(1)} MB`;
}

watch(() => [props.conversation, props.messageId, props.index], load, { immediate: true });
onBeforeUnmount(release);
</script>

<template>
  <div class="attachment">
    <a v-if="isImage && url" :href="url" target="_blank" rel="noopener" :title="`Open ${attachment.filename}`">
      <img :src="url" :alt="attachment.filename" class="thumb" />
    </a>
    <a v-else-if="url" :href="url" :download="attachment.filename" class="chip">
      <span class="chip-name">{{ attachment.filename }}</span>
      <span class="chip-size">{{ size(attachment.size) }}</span>
    </a>
    <span v-else class="chip" :class="{ failed }">
      <span class="chip-name">{{ attachment.filename }}</span>
      <span class="chip-size">{{ failed ? "unavailable" : messageId.startsWith("local-") ? "uploading…" : size(attachment.size) }}</span>
    </span>
  </div>
</template>

<style scoped>
.attachment {
  margin-top: 6px;
}
.thumb {
  display: block;
  max-width: 220px;
  max-height: 180px;
  border-radius: 8px;
  border: 1px solid var(--border-primary);
  object-fit: cover;
}
.chip {
  display: inline-flex;
  gap: 8px;
  align-items: center;
  padding: 6px 10px;
  border-radius: 8px;
  border: 1px solid var(--border-primary);
  background: var(--bg-secondary);
  color: var(--text-primary);
  text-decoration: none;
  font-size: 13px;
  max-width: 100%;
}
.chip-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.chip-size {
  color: var(--text-muted);
  flex: none;
}
.chip.failed .chip-size {
  color: var(--danger, #dc2626);
}
</style>
