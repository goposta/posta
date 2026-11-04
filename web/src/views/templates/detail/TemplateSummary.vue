<script setup lang="ts">
import { computed, ref } from "vue";
import type { Template, TemplateVersion } from "../../../api/types";

const props = defineProps<{
  template: Template;
  versions: TemplateVersion[];
  languages: string[];
}>();

// Which value was just copied: "name" or "id".
const copied = ref<string | null>(null);

async function copy(what: "name" | "id") {
  try {
    await navigator.clipboard.writeText(what === "name" ? props.template.name : String(props.template.id));
    copied.value = what;
    setTimeout(() => (copied.value = null), 2000);
  } catch {
    // Clipboard access can be denied; the value is selectable either way.
  }
}

const activeVersion = computed(() =>
  props.versions.find((v) => v.id === props.template.active_version_id)
);

function formatDate(value?: string | null) {
  return value ? new Date(value).toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" }) : "—";
}
</script>

<template>
  <div class="summary">
    <div class="facts">
      <div class="fact">
        <span class="k">Name</span>
        <span class="v">
          <code class="id" title="What you send as template in the API">{{ template.name }}</code>
          <button class="copy" type="button" :title="copied === 'name' ? 'Copied' : 'Copy template name'" @click="copy('name')">
            {{ copied === "name" ? "Copied" : "Copy" }}
          </button>
        </span>
      </div>

      <div class="fact">
        <span class="k">ID</span>
        <span class="v">
          <code class="id">{{ template.id }}</code>
          <button class="copy" type="button" :title="copied === 'id' ? 'Copied' : 'Copy template ID'" @click="copy('id')">
            {{ copied === "id" ? "Copied" : "Copy" }}
          </button>
        </span>
      </div>

      <div class="fact">
        <span class="k">Active version</span>
        <span class="v">
          <span v-if="activeVersion" class="badge badge-success">v{{ activeVersion.version }}</span>
          <span v-else class="warn" title="This template cannot be sent until a version is active">
            none — cannot send
          </span>
        </span>
      </div>

      <div class="fact">
        <span class="k">Languages</span>
        <span class="v langs">
          <span
            v-for="lang in languages"
            :key="lang"
            class="badge"
            :class="lang === template.default_language ? 'badge-info' : 'badge-neutral'"
            :title="lang === template.default_language ? 'Default language' : ''"
          >
            {{ lang }}
          </span>
          <span v-if="!languages.length" class="muted">none yet</span>
        </span>
      </div>

      <div class="fact">
        <span class="k">Last edited</span>
        <span class="v">
          {{ formatDate(template.updated_at || template.created_at) }}
          <span v-if="template.last_edited_by?.name" class="muted">
            by {{ template.last_edited_by.name }}
          </span>
        </span>
      </div>
    </div>

    <p v-if="template.description" class="description">{{ template.description }}</p>
  </div>
</template>

<style scoped>
.summary {
  padding: 16px 18px;
  margin-bottom: 24px;
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  background: var(--bg-primary);
}

.facts {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(190px, 1fr));
  gap: 16px;
}

.fact {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.k {
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--text-tertiary);
}

.v {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  font-size: 13px;
  color: var(--text-primary);
}

.langs {
  gap: 4px;
}

.id {
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: "JetBrains Mono", "Fira Code", monospace;
  font-size: 12px;
  padding: 2px 8px;
  border-radius: var(--radius);
  border: 1px solid var(--border-primary);
  background: var(--bg-tertiary);
  user-select: all;
}

.copy {
  border: 0;
  background: none;
  padding: 0;
  font: inherit;
  font-size: 12px;
  color: var(--text-tertiary);
  cursor: pointer;
}

.copy:hover {
  color: var(--primary-600);
  text-decoration: underline;
}

.muted {
  color: var(--text-tertiary);
  font-size: 12px;
}

.warn {
  color: var(--warning-text);
  font-weight: 600;
  font-size: 12px;
}

.description {
  margin: 14px 0 0;
  padding-top: 14px;
  border-top: 1px solid var(--border-secondary);
  font-size: 13px;
  line-height: 1.6;
  color: var(--text-secondary);
}
</style>
