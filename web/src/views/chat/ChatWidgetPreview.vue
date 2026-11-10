<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";

export interface PreviewConfig {
  enabled: true;
  online: boolean;
  title?: string;
  greeting?: string;
  accent_color?: string;
  position?: "left" | "right";
  avatar_url?: string;
  require_email?: boolean;
  theme?: "auto" | "light" | "dark";
  one_open_conversation?: boolean;
  status?: { reply_time?: string; agents?: string[] };
  new_chats?: boolean;
  home?: unknown;
}

type View = "home" | "messages" | "help" | "thread";

declare global {
  interface Window {
    PostaChat?: { preview?: (el: HTMLElement, cfg: PreviewConfig, opts?: { view?: View }) => void };
  }
}

const props = defineProps<{ config: PreviewConfig; homeEnabled: boolean; helpVisible: boolean }>();

const el = ref<HTMLElement | null>(null);
const state = ref<"loading" | "ready" | "failed" | "outdated">("loading");
const view = ref<View>(props.homeEnabled ? "home" : "thread");
const theme = ref<"widget" | "light" | "dark">("widget");

const views = computed<{ key: View; label: string }[]>(() =>
  props.homeEnabled
    ? [
        { key: "home", label: "Home" },
        { key: "messages", label: "Messages" },
        ...(props.helpVisible ? [{ key: "help" as View, label: "Help" }] : []),
        { key: "thread", label: "Conversation" },
      ]
    : [{ key: "thread", label: "Conversation" }],
);

let load: Promise<void> | null = null;
function loadWidget(): Promise<void> {
  if (window.PostaChat?.preview) return Promise.resolve();
  load ||= new Promise((resolve, reject) => {
    const s = document.createElement("script");
    // The script is cached for an hour, so a copy from before the preview existed would otherwise be reused.
    s.src = `/widget/v1/posta-chat.js?preview=${Date.now()}`;
    s.async = true;
    s.onload = () => (window.PostaChat?.preview ? resolve() : reject(new Error("outdated")));
    s.onerror = () => reject(new Error("widget failed to load"));
    document.head.appendChild(s);
  });
  return load;
}

function render() {
  if (state.value !== "ready" || !el.value) return;
  if (!views.value.some((v) => v.key === view.value)) view.value = views.value[0].key;
  const cfg = { ...props.config, theme: theme.value === "widget" ? props.config.theme : theme.value };
  window.PostaChat!.preview!(el.value, cfg, { view: view.value });
}

onMounted(async () => {
  try {
    await loadWidget();
    state.value = "ready";
    render();
  } catch (e) {
    state.value = e instanceof Error && e.message === "outdated" ? "outdated" : "failed";
  }
});

watch(() => [props.config, view.value, theme.value], render, { deep: true });
</script>

<template>
  <div class="card">
    <div class="card-header"><h2>Preview</h2></div>
    <div class="card-body">
      <div class="preview-controls">
        <div class="seg" role="group" aria-label="Preview view">
          <button v-for="v in views" :key="v.key" type="button" class="btn btn-sm"
            :class="view === v.key ? 'btn-primary' : 'btn-secondary'" :aria-pressed="view === v.key" @click="view = v.key">
            {{ v.label }}
          </button>
        </div>
        <select v-model="theme" class="form-select theme-select" aria-label="Preview theme">
          <option value="widget">Widget theme</option>
          <option value="light">Light</option>
          <option value="dark">Dark</option>
        </select>
      </div>
      <div class="preview-page" :class="{ dark: theme === 'dark' }">
        <div ref="el" class="preview-host"></div>
        <p v-if="state === 'failed'" class="muted preview-failed">
          The preview needs the widget script at <code>/widget/v1/posta-chat.js</code>, which could not be loaded.
        </p>
        <p v-else-if="state === 'outdated'" class="muted preview-failed">
          The server's widget script is older than this dashboard and has no preview. Rebuild and restart the server.
        </p>
      </div>
      <slot />
      <small class="muted">The real widget, drawn from your unsaved settings. Links do not open here.</small>
    </div>
  </div>
</template>

<style scoped>
.preview-controls {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  align-items: center;
  margin-bottom: 10px;
}
.seg {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
}
.theme-select {
  width: auto;
  margin-left: auto;
}
.preview-page {
  position: relative;
  height: 680px;
  border-radius: 10px;
  background: repeating-linear-gradient(135deg, var(--bg-secondary) 0 12px, var(--bg-tertiary, var(--bg-primary)) 12px 24px);
  overflow: hidden;
  isolation: isolate;
  margin-bottom: 8px;
}
.preview-page.dark {
  background: repeating-linear-gradient(135deg, #1f2937 0 12px, #1a2230 12px 24px);
}
.preview-host {
  position: absolute;
  inset: 0;
}
.preview-failed {
  padding: 16px;
}
</style>
