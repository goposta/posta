<script setup lang="ts">
import type { StyleSheet, TemplateVersion } from "../../../api/types";

defineProps<{ version: TemplateVersion; stylesheets: StyleSheet[]; saving: boolean }>();
const emit = defineEmits<{ (e: "close"): void; (e: "save"): void }>();

const stylesheetId = defineModel<number | null>("stylesheetId", { default: null });
</script>

<template>
  <div class="modal" style="max-width: 440px" @mousedown.stop @mouseup.stop>
    <div class="modal-header">
      <h3>Stylesheet for v{{ version.version }}</h3>
    </div>
    <div class="modal-body">
      <div class="form-group">
        <label class="form-label" for="version-stylesheet">Stylesheet</label>
        <select id="version-stylesheet" v-model="stylesheetId" class="form-select">
          <option :value="null">No stylesheet</option>
          <option v-for="ss in stylesheets" :key="ss.id" :value="ss.id">{{ ss.display_name }}</option>
        </select>
        <span class="form-hint">
          Its CSS is inlined into every send from this version. Each version carries its own, so a
          style change can be tried on a draft before it goes live.
        </span>
      </div>
    </div>
    <div class="modal-footer">
      <button class="btn btn-secondary" @click="emit('close')">Cancel</button>
      <button class="btn btn-primary" :disabled="saving" @click="emit('save')">
        {{ saving ? "Saving…" : "Save" }}
      </button>
    </div>
  </div>
</template>
