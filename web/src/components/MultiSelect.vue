<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

const props = defineProps<{
  options: { value: string; label: string }[]
  placeholder: string
  // Summary for several selected items, e.g. "statuses" -> "3 statuses".
  plural: string
}>()
const model = defineModel<string[]>({ required: true })

const open = ref(false)
const root = ref<HTMLElement | null>(null)

const label = (v: string) => props.options.find((o) => o.value === v)?.label ?? v
const summary = computed(() => {
  const n = model.value.length
  if (n === 0) return props.placeholder
  if (n === 1) return label(model.value[0])
  return `${n} ${props.plural}`
})

function toggle(value: string) {
  const cur = model.value
  model.value = cur.includes(value) ? cur.filter((v) => v !== value) : [...cur, value]
}

function onDocClick(e: MouseEvent) {
  if (open.value && root.value && !root.value.contains(e.target as Node)) open.value = false
}
onMounted(() => document.addEventListener('click', onDocClick))
onBeforeUnmount(() => document.removeEventListener('click', onDocClick))
</script>

<template>
  <div ref="root" class="msel">
    <button
      type="button"
      class="msel__button"
      :class="{ 'msel__button--empty': !model.length, 'msel__button--open': open }"
      @click="open = !open"
    >
      <span class="msel__summary">{{ summary }}</span>
      <span class="mdi mdi-chevron-down msel__chevron"></span>
    </button>
    <div v-if="open" class="msel__menu">
      <label v-for="o in options" :key="o.value" class="msel__option">
        <input type="checkbox" :checked="model.includes(o.value)" @change="toggle(o.value)" />
        <span>{{ o.label }}</span>
      </label>
      <p v-if="!options.length" class="msel__empty">Nothing to choose from</p>
    </div>
  </div>
</template>

<style scoped>
.msel {
  position: relative;
}
.msel__button {
  width: 100%;
  height: 44px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 0 12px 0 14px;
  border: 1px solid var(--border-input);
  border-radius: var(--radius);
  background: var(--bg-input);
  color: var(--text-primary);
  font-size: 14px;
  font-family: inherit;
  cursor: pointer;
  text-align: left;
  transition: border-color var(--transition), box-shadow var(--transition);
}
.msel__button--open {
  border-color: var(--primary-600);
  box-shadow: var(--shadow-focus);
}
.msel__button--empty {
  color: var(--text-tertiary);
}
.msel__summary {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.msel__chevron {
  font-size: 18px;
  color: var(--text-tertiary);
  transition: transform var(--transition);
}
.msel__button--open .msel__chevron {
  transform: rotate(180deg);
}
.msel__menu {
  position: absolute;
  z-index: 20;
  top: calc(100% + 4px);
  left: 0;
  min-width: 100%;
  max-height: 280px;
  overflow-y: auto;
  padding: 6px;
  display: flex;
  flex-direction: column;
  gap: 2px;
  background: var(--bg-secondary);
  border: 1px solid var(--border-input);
  border-radius: var(--radius);
  box-shadow: var(--shadow-lg);
}
.msel__option {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 7px 8px;
  border-radius: var(--radius-sm);
  font-size: 14px;
  color: var(--text-secondary);
  white-space: nowrap;
  cursor: pointer;
}
.msel__option:hover {
  background: var(--bg-hover);
}
.msel__option input {
  width: 15px;
  height: 15px;
  cursor: pointer;
}
.msel__empty {
  margin: 0;
  padding: 7px 8px;
  font-size: 13px;
  color: var(--text-tertiary);
}
</style>
