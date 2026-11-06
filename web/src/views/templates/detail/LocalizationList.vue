<script setup lang="ts">
import { computed } from "vue";
import type { Language, TemplateLocalization, TemplateVersion } from "../../../api/types";

const props = defineProps<{
  version: TemplateVersion;
  localizations: TemplateLocalization[];
  languages: Language[];
  defaultLanguage: string;
  canEdit: boolean;
}>();

const emit = defineEmits<{
  (e: "add", language?: string): void;
  (e: "edit", l: TemplateLocalization): void;
  (e: "open-editor", l: TemplateLocalization): void;
  (e: "preview", language: string): void;
  (e: "delete", l: TemplateLocalization): void;
}>();

const missing = computed(() => {
  const have = new Set(props.localizations.map((l) => l.language));
  return props.languages.filter((l) => !have.has(l.code));
});

const hasDefault = computed(() =>
  props.localizations.some((l) => l.language === props.defaultLanguage)
);

function formatDate(l: TemplateLocalization) {
  return new Date(l.updated_at || l.created_at).toLocaleDateString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
  });
}
</script>

<template>
  <div class="card">
    <div class="card-header">
      <div>
        <h2>Content — v{{ version.version }}</h2>
        <p class="hint">The subject and body sent for each language.</p>
      </div>
      <button v-if="canEdit" class="btn btn-primary btn-sm" @click="emit('add')">Add language</button>
    </div>

    <div v-if="!localizations.length" class="empty-state">
      <h3>No content yet</h3>
      <p>Add a language to define the subject and body this version sends.</p>
      <button v-if="canEdit" class="btn btn-primary" @click="emit('add', defaultLanguage)">
        Add {{ defaultLanguage }}
      </button>
    </div>

    <template v-else>
      <div v-if="!hasDefault" class="notice">
        <strong>No content for {{ defaultLanguage }}.</strong>
        <span>
          That is this template's default language — the one used when a send does not ask for a
          specific one, so those sends will fail.
        </span>
        <button v-if="canEdit" class="btn btn-secondary btn-sm" @click="emit('add', defaultLanguage)">
          Add {{ defaultLanguage }}
        </button>
      </div>

      <div class="table-wrapper">
        <table>
          <thead>
            <tr>
              <th>Language</th>
              <th>Subject</th>
              <th>Updated</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="l in localizations" :key="l.id">
              <td>
                <strong>{{ l.language }}</strong>
                <span v-if="l.language === defaultLanguage" class="badge badge-info default-tag">
                  default
                </span>
              </td>
              <td class="subject">{{ l.subject_template }}</td>
              <td>{{ formatDate(l) }}</td>
              <td class="actions">
                <div class="flex gap-2">
                  <button class="btn btn-secondary btn-sm" @click="emit('preview', l.language)">
                    Preview
                  </button>
                  <button
                    v-if="canEdit"
                    class="btn btn-primary btn-sm"
                    title="Full editor with syntax highlighting and a live preview"
                    @click="emit('open-editor', l)"
                  >
                    Edit content
                  </button>
                  <button
                    v-if="canEdit"
                    class="btn btn-secondary btn-sm"
                    title="Change the subject line without leaving this page"
                    @click="emit('edit', l)"
                  >
                    Subject
                  </button>
                  <button v-if="canEdit" class="btn btn-danger btn-sm" @click="emit('delete', l)">
                    Delete
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-if="canEdit && missing.length" class="missing">
        <span class="missing-label">Not translated yet</span>
        <button
          v-for="lang in missing"
          :key="lang.id"
          class="missing-chip"
          type="button"
          :title="`Add ${lang.name} content to this version`"
          @click="emit('add', lang.code)"
        >
          + {{ lang.code }}
        </button>
      </div>
    </template>
  </div>
</template>

<style scoped>
.hint {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--text-tertiary);
}

.notice {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin: 0 16px 12px;
  padding: 10px 14px;
  border: 1px solid var(--warning-500);
  border-radius: var(--radius);
  background: var(--warning-50);
  font-size: 12px;
  color: var(--text-secondary);
}

.notice strong {
  color: var(--text-primary);
}

.default-tag {
  margin-left: 6px;
}

.subject {
  max-width: 340px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text-secondary);
}

.actions {
  text-align: right;
}

.missing {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
  padding: 12px 16px;
  border-top: 1px solid var(--border-secondary);
}

.missing-label {
  font-size: 12px;
  color: var(--text-tertiary);
  margin-right: 2px;
}

.missing-chip {
  padding: 3px 10px;
  border: 1px dashed var(--border-input);
  border-radius: 999px;
  background: none;
  font: inherit;
  font-size: 12px;
  color: var(--text-secondary);
  cursor: pointer;
}

.missing-chip:hover {
  border-style: solid;
  border-color: var(--primary-600);
  color: var(--primary-600);
}
</style>
