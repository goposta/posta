<script setup lang="ts">
import { computed } from "vue";
import type { Language, TemplateLocalization, TemplateLocalizationInput } from "../../../api/types";

const props = defineProps<{
  editing: TemplateLocalization | null;
  languages: Language[];
  taken: string[];
  saving: boolean;
}>();

const emit = defineEmits<{ (e: "close"): void; (e: "save"): void }>();

const form = defineModel<TemplateLocalizationInput>("form", { required: true });

// A language already on this version would be rejected by the unique index, so it is not offered.
const available = computed(() => props.languages.filter((l) => !props.taken.includes(l.code)));

const valid = computed(
  () =>
    (props.editing !== null || form.value.language.trim() !== "") &&
    form.value.subject_template.trim() !== ""
);
</script>

<template>
  <div class="modal" style="max-width: 640px" @mousedown.stop @mouseup.stop>
    <div class="modal-header">
      <h3>{{ editing ? `Edit ${editing.language}` : "Add a language" }}</h3>
    </div>

    <div class="modal-body">
      <div v-if="!editing" class="form-group">
        <label class="form-label" for="loc-language">Language</label>
        <select id="loc-language" v-model="form.language" class="form-select">
          <option value="" disabled>Select a language</option>
          <option v-for="lang in available" :key="lang.id" :value="lang.code">
            {{ lang.name }} ({{ lang.code }})
          </option>
        </select>
        <span class="form-hint">
          Drawn from the workspace's configured languages. Ones this version already has are not
          listed.
        </span>
      </div>

      <div class="form-group">
        <label class="form-label" for="loc-subject">Subject</label>
        <input
          id="loc-subject"
          v-model="form.subject_template"
          class="form-input"
          placeholder="Welcome {{name}}!"
        />
        <span class="form-hint">
          Template variables use <code v-pre>{{ name }}</code> and resolve against the data sent
          with the email.
        </span>
      </div>

      <div class="form-group">
        <label class="form-label" for="loc-html">HTML body</label>
        <textarea id="loc-html" v-model="form.html_template" class="form-textarea" rows="7"></textarea>
        <span class="form-hint">
          For anything beyond a quick change, "Edit content" opens the full editor with syntax
          highlighting and a live preview.
        </span>
      </div>

      <div class="form-group">
        <label class="form-label" for="loc-text">Plain text body</label>
        <textarea id="loc-text" v-model="form.text_template" class="form-textarea" rows="4"></textarea>
        <span class="form-hint">
          Shown by clients that do not render HTML. Leaving it empty is allowed but hurts
          deliverability.
        </span>
      </div>
    </div>

    <div class="modal-footer">
      <button class="btn btn-secondary" @click="emit('close')">Cancel</button>
      <button class="btn btn-primary" :disabled="saving || !valid" @click="emit('save')">
        {{ saving ? "Saving…" : editing ? "Save" : "Add" }}
      </button>
    </div>
  </div>
</template>
