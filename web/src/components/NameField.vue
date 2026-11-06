<script setup lang="ts">
import { computed } from 'vue'
import { toIdentifier } from '@/composables/identifier'

// The server derives the identifier from the typed name; once the resource exists its name cannot change.
interface Props {
  /** The resource being edited; absent while creating one. */
  existing?: { name: string; display_name?: string } | null
  /** What the resource is called, for the hint: "template", "form". */
  resource: string
  /** What the identifier is used for, like "Send with it as template in the API." */
  usage?: string
  placeholder?: string
  autofocus?: boolean
}
const props = withDefaults(defineProps<Props>(), {
  existing: null,
  usage: '',
  placeholder: '',
  autofocus: false,
})
const model = defineModel<string>({ default: '' })

const identifier = computed(() => toIdentifier(model.value))
</script>

<template>
  <div class="form-group">
    <label class="form-label">Name</label>
    <template v-if="existing">
      <input :value="existing.display_name || existing.name" class="form-input" disabled />
      <span class="form-hint">
        Saved as <code>{{ existing.name }}</code>. {{ usage }}
        A {{ resource }}'s name cannot change: create a new {{ resource }} to use another.
      </span>
    </template>
    <template v-else>
      <input v-model="model" class="form-input" :placeholder="placeholder" :autofocus="autofocus" />
      <span class="form-hint">
        <template v-if="identifier">Saved as <code>{{ identifier }}</code>.</template>
        {{ usage }} It cannot change later.
      </span>
    </template>
  </div>
</template>
