<script setup lang="ts">
import type { StyleSheet, TemplateVersion } from "../../../api/types";

defineProps<{
  versions: TemplateVersion[];
  stylesheets: StyleSheet[];
  selectedId: number | null;
  activeId: number | null;
  canEdit: boolean;
  creating: boolean;
}>();

const emit = defineEmits<{
  (e: "select", v: TemplateVersion): void;
  (e: "create", stylesheetId: number | null): void;
  (e: "edit-stylesheet", v: TemplateVersion): void;
  (e: "activate", v: TemplateVersion): void;
  (e: "delete", v: TemplateVersion): void;
}>();

const newStylesheetId = defineModel<number | null>("newStylesheetId", { default: null });

function formatDate(value: string) {
  return new Date(value).toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" });
}
</script>

<template>
  <div class="card">
    <div class="card-header">
      <div>
        <h2>Versions</h2>
        <p class="hint">
          Only the active version is sent. Others are drafts you can edit and activate when ready.
        </p>
      </div>
      <div v-if="canEdit" class="new-version">
        <select v-model="newStylesheetId" class="form-select select-sm" aria-label="Stylesheet for the new version">
          <option :value="null">No stylesheet</option>
          <option v-for="ss in stylesheets" :key="ss.id" :value="ss.id">{{ ss.display_name }}</option>
        </select>
        <button class="btn btn-primary btn-sm" :disabled="creating" @click="emit('create', newStylesheetId)">
          {{ creating ? "Creating…" : "New version" }}
        </button>
      </div>
    </div>

    <div v-if="!versions.length" class="empty-state">
      <h3>No versions yet</h3>
      <p>Create one to start adding content.</p>
    </div>

    <div v-else class="table-wrapper">
      <table>
        <thead>
          <tr>
            <th>Version</th>
            <th>Stylesheet</th>
            <th>Languages</th>
            <th>Created</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="v in versions"
            :key="v.id"
            class="row"
            :class="{ selected: selectedId === v.id }"
            :aria-selected="selectedId === v.id"
            @click="emit('select', v)"
          >
            <td>
              <div class="version-cell">
                <strong>v{{ v.version }}</strong>
                <span v-if="activeId === v.id" class="badge badge-success">Active</span>
                <span v-else class="badge badge-neutral">Draft</span>
              </div>
              <span v-if="selectedId === v.id" class="viewing">Showing content below</span>
            </td>
            <td>
              <span v-if="v.stylesheet" class="badge badge-neutral">{{ v.stylesheet.display_name }}</span>
              <span v-else class="muted">&mdash;</span>
            </td>
            <td>{{ v.localizations?.length || 0 }}</td>
            <td>{{ formatDate(v.created_at) }}</td>
            <td class="actions">
              <div v-if="canEdit" class="flex gap-2" @click.stop>
                <button class="btn btn-secondary btn-sm" @click="emit('edit-stylesheet', v)">
                  Stylesheet
                </button>
                <button
                  v-if="activeId !== v.id"
                  class="btn btn-secondary btn-sm"
                  @click="emit('activate', v)"
                >
                  Activate
                </button>
                <button
                  v-if="activeId !== v.id"
                  class="btn btn-danger btn-sm"
                  @click="emit('delete', v)"
                >
                  Delete
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
.hint {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--text-tertiary);
}

.new-version {
  display: flex;
  align-items: center;
  gap: 8px;
}

.select-sm {
  width: auto;
  max-width: 170px;
  padding: 5px 8px;
  font-size: 12px;
}

.row {
  cursor: pointer;
}

.row.selected {
  background: var(--bg-tertiary);
  box-shadow: inset 3px 0 0 var(--primary-600);
}

.version-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.viewing {
  display: block;
  margin-top: 2px;
  font-size: 11px;
  color: var(--text-tertiary);
}

.muted {
  color: var(--text-tertiary);
}

.actions {
  text-align: right;
}
</style>
