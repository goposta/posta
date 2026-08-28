<script setup lang="ts">
import { computed } from "vue";
import { useRouter } from "vue-router";
import type { DashboardFeatures } from "../../api/types";

const props = defineProps<{ features?: DashboardFeatures | null }>();

const router = useRouter();

const actions = computed(() => {
  const items = [
    { to: "/emails", icon: "send", label: "Send an email", hint: "Compose or replay a message", tone: "primary" },
    { to: "/templates", icon: "file-text", label: "New template", hint: "Reusable, versioned, localizable", tone: "primary" },
    { to: "/campaigns", icon: "mail", label: "New campaign", hint: "Send to a subscriber list", tone: "primary" },
    { to: "/domains", icon: "globe", label: "Add domain", hint: "Verify ownership, SPF, DKIM, DMARC", tone: "success" },
    { to: "/api-keys", icon: "key", label: "Create API key", hint: "Scoped credentials for your app", tone: "info" },
    { to: "/webhooks", icon: "link", label: "Add webhook", hint: "Receive delivery events", tone: "info" },
  ];
  if (props.features?.messages) {
    items.splice(3, 0, {
      to: "/forms",
      icon: "edit-3",
      label: "New form",
      hint: "Receive messages from your website",
      tone: "primary",
    });
  }
  return items;
});

const icons: Record<string, string> = {
  send: '<path d="M16.5 1.5L8.25 9.75M16.5 1.5l-5.25 15-3-6.75L1.5 6.75l15-5.25z"/>',
  "file-text": '<path d="M10.5 1.5H4.5a1.5 1.5 0 00-1.5 1.5v12a1.5 1.5 0 001.5 1.5h9a1.5 1.5 0 001.5-1.5V6l-4.5-4.5z"/><path d="M10.5 1.5V6H15"/>',
  mail: '<rect x="2" y="3.5" width="14" height="11" rx="2"/><path d="M2 5.5l7 5 7-5"/>',
  globe: '<circle cx="9" cy="9" r="7"/><path d="M2 9h14M9 2a11 11 0 013 7 11 11 0 01-3 7 11 11 0 01-3-7 11 11 0 013-7z"/>',
  key: '<path d="M15.5 2.5l-2 2m1 1l-2 2-3-3 2-2m-3.18 3.18a4 4 0 10-5.64 5.64 4 4 0 005.64-5.64z"/>',
  link: '<path d="M7.5 10.5a3.75 3.75 0 005.3.45l2.25-2.25a3.75 3.75 0 00-5.3-5.3l-1.29 1.28"/><path d="M10.5 7.5a3.75 3.75 0 00-5.3-.45L2.96 9.3a3.75 3.75 0 005.3 5.3l1.28-1.28"/>',
  "edit-3": '<path d="M12 2.25l3.75 3.75L6 15.75H2.25V12L12 2.25z"/>',
};
</script>

<template>
  <div class="quick-actions">
    <button
      v-for="a in actions"
      :key="a.to"
      class="qa"
      :class="`qa-${a.tone}`"
      :title="a.hint"
      @click="router.push(a.to)"
    >
      <svg
        class="qa-icon"
        width="16"
        height="16"
        viewBox="0 0 18 18"
        fill="none"
        stroke="currentColor"
        stroke-width="1.5"
        stroke-linecap="round"
        stroke-linejoin="round"
        aria-hidden="true"
        v-html="icons[a.icon]"
      ></svg>
      <span>{{ a.label }}</span>
    </button>
  </div>
</template>

<style scoped>
.quick-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 20px;
}

.qa {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 7px 13px 7px 10px;
  border-radius: 8px;
  background: var(--bg-primary);
  border: 1px solid var(--border-primary);
  color: var(--text-primary);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s, transform 0.15s;
}

.qa:hover {
  border-color: var(--qa-color);
  background: var(--bg-hover);
  transform: translateY(-1px);
}

.qa-icon {
  flex: none;
  color: var(--qa-color);
}

.qa-primary { --qa-color: var(--primary-500); }
.qa-success { --qa-color: var(--success-600); }
.qa-info { --qa-color: var(--primary-400); }

@media (max-width: 719px) {
  .quick-actions {
    flex-wrap: nowrap;
    overflow-x: auto;
    scrollbar-width: none;
    padding-bottom: 2px;
  }
  .quick-actions::-webkit-scrollbar {
    display: none;
  }
  .qa {
    flex: 0 0 auto;
  }
}
</style>
