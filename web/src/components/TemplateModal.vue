<script setup lang="ts">
import { languagesApi } from '@/api/languages';
import { Language, Template, TemplateDraft } from '@/api/types';
import { IDENTIFIER_HINT, toIdentifier } from '@/composables/identifier';
import { useModalSafeClose } from '@/composables/useModalSafeClose';
import { onMounted, ref, watch } from 'vue';

interface Props {
  form: TemplateDraft
  isVisible?: boolean
  saving?: boolean 
  editing?: Template | null
}
const props = withDefaults(defineProps<Props>(), {
  isVisible: false,
  saving: false,
  editing: null
})
const languages = ref<Language[]>([]);

async function loadLanguages() {
  try {
    const res = await languagesApi.list(0, 100);
    languages.value = res.data.data;
  } catch {
    // Non-critical
  }
}

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'save'): void
}>()

const { watchClickStart, confirmClickEnd } = useModalSafeClose(() => {
  emit('close')
});

onMounted(() => {
  loadLanguages();
});

// A new template's name follows its display name until edited by hand; an
// existing one keeps its name, which callers send.
const nameEdited = ref(false)
watch(() => props.isVisible, (visible) => {
  if (visible) nameEdited.value = !!props.editing
})
function syncName() {
  if (!nameEdited.value) props.form.name = toIdentifier(props.form.display_name)
}
</script>

<template>
  <div v-if="isVisible" class="modal-overlay" @mousedown="watchClickStart" @mouseup="confirmClickEnd">
    <div class="modal" style="max-width: 560px" @mousedown.stop @mouseup.stop>
      <div class="modal-header">
        <h3>{{ editing ? "Edit Template" : "Create Template" }}</h3>
      </div>

      <div class="modal-body">
        <div class="form-group">
          <label class="form-label">Display name</label>
          <input v-model="form.display_name" class="form-input" placeholder="e.g. Welcome Email" @input="syncName" />
        </div>
        <div class="form-group">
          <label class="form-label">Name</label>
          <input v-model="form.name" class="form-input mono" placeholder="welcome-email" :title="IDENTIFIER_HINT"
            @input="nameEdited = true" />
          <span v-if="editing && form.name !== editing.name" class="form-hint form-hint-warning">
            Callers that send with <code>{{ editing.name }}</code> stop finding this template.
          </span>
          <span v-else class="form-hint">What you send as <code>template</code> in the API. {{ IDENTIFIER_HINT }}</span>
        </div>
        <div class="form-group">
          <label class="form-label">Description</label>
          <input v-model="form.description" class="form-input" placeholder="e.g. Sent after user registration" />
        </div>
        <div class="form-group">
          <label class="form-label">Default Language</label>
          <select v-model="form.default_language" class="form-select">
            <option v-for="lang in languages" :key="lang.id" :value="lang.code">
              {{ lang.name }} ({{ lang.code }})
            </option>
          </select>
          <span class="form-hint">Fallback language when no localization matches the requested language</span>
        </div>
        <div class="form-group">
          <label class="form-label">Sample Data (JSON)</label>
          <textarea v-model="form.sample_data" class="form-textarea" rows="3"
            placeholder='{"name": "John", "company": "Acme"}'></textarea>
          <span class="form-hint">Default sample data for previewing template localizations</span>
        </div>
      </div>

      <div class="modal-footer">
        <button class="btn btn-secondary" @click="emit('close')">Cancel</button>
        <button class="btn btn-primary" :disabled="saving || !form.display_name.trim() || !form.name.trim()" @click="emit('save')">
          {{ saving ? "Saving..." : editing ? "Update" : "Create" }}
        </button>
      </div>
    </div>
  </div>
</template>
