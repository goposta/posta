<script setup lang="ts">
import { computed } from "vue";
import type { ChatHome, ChatHomeItemKind } from "../../api/types";
import { MAX_HELP_LINKS, MAX_HOME_ITEMS, move, newID, newItem, urlError } from "./chatHome";

const home = defineModel<ChatHome>({ required: true });
defineProps<{ accent: string; disabled?: boolean }>();

const COLOR = /^#[0-9a-fA-F]{6}$/;
const colorValid = computed(() => !home.value.header_color || COLOR.test(home.value.header_color));
const items = computed(() => (home.value.items ??= []));
const links = computed(() => (home.value.help_links ??= []));
const hasMessage = computed(() => items.value.some((i) => i.kind === "message"));

const KIND_LABELS: Record<ChatHomeItemKind, string> = {
  message: "Message",
  link: "Link",
  card: "Card",
};

function add(kind: ChatHomeItemKind) {
  items.value.push(newItem(kind));
}

function addLink() {
  links.value.push({ id: newID(), title: "", description: "", url: "" });
}
</script>

<template>
  <div class="card">
    <div class="card-header"><h2>Home screen</h2></div>
    <div class="card-body">
      <div class="form-group">
        <label class="checkbox-label">
          <input v-model="home.enabled" type="checkbox" :disabled="disabled" />
          Show a home screen with Home, Messages and Help tabs
        </label>
        <small class="muted">
          When off, the widget opens straight into the chat, as before.
        </small>
      </div>

      <fieldset :disabled="disabled || !home.enabled" class="home-fields">
        <h3 class="section-title">Header</h3>
        <div class="form-group">
          <label class="form-label" for="home-headline">Headline</label>
          <input id="home-headline" v-model="home.headline" class="form-input" maxlength="120" placeholder="How can we help?" />
          <small class="muted">Shown under the greeting from the Appearance tab.</small>
        </div>
        <div class="form-group">
          <label class="form-label" for="home-color">Header colour</label>
          <div class="color-row">
            <input
              :value="colorValid && home.header_color ? home.header_color : accent"
              type="color"
              class="color-input"
              aria-label="Pick header colour"
              @input="home.header_color = ($event.target as HTMLInputElement).value"
            />
            <input id="home-color" v-model="home.header_color" class="form-input" maxlength="7" placeholder="Accent colour" />
          </div>
          <small v-if="!colorValid" class="field-error">Use a #RRGGBB colour, like #0f172a.</small>
          <small v-else class="muted">Blank uses the accent colour. The text colour is picked for contrast.</small>
        </div>

        <h3 class="section-title">Items</h3>
        <p v-if="!items.length" class="muted">No items: visitors reach the chat from the Messages tab.</p>
        <ol class="home-list">
          <li v-for="(it, i) in items" :key="it.id" class="home-item">
            <div class="home-item-head">
              <span class="badge badge-neutral">{{ KIND_LABELS[it.kind] }}</span>
              <span class="spacer"></span>
              <button type="button" class="btn btn-secondary btn-sm" :disabled="i === 0" :aria-label="`Move item ${i + 1} up`" @click="move(items, i, i - 1)">↑</button>
              <button type="button" class="btn btn-secondary btn-sm" :disabled="i === items.length - 1" :aria-label="`Move item ${i + 1} down`" @click="move(items, i, i + 1)">↓</button>
              <button type="button" class="btn btn-secondary btn-sm" :aria-label="`Remove item ${i + 1}`" @click="items.splice(i, 1)">Remove</button>
            </div>
            <div class="form-group">
              <label class="form-label" :for="`item-title-${it.id}`">Title</label>
              <input :id="`item-title-${it.id}`" v-model="it.title" class="form-input" maxlength="80"
                :placeholder="it.kind === 'message' ? 'Send us a message' : ''" />
              <small v-if="it.kind === 'message'" class="muted">Starts a chat, or reads "Continue the conversation" while one is open.</small>
            </div>
            <div v-if="it.kind === 'card'" class="form-group">
              <label class="form-label" :for="`item-body-${it.id}`">Text</label>
              <textarea :id="`item-body-${it.id}`" v-model="it.body" class="form-input" rows="2" maxlength="280"></textarea>
              <small class="muted">{{ (it.body ?? "").length }}/280, plain text.</small>
            </div>
            <template v-if="it.kind !== 'message'">
              <div class="form-group">
                <label class="form-label" :for="`item-url-${it.id}`">URL{{ it.kind === "card" ? " (optional)" : "" }}</label>
                <input :id="`item-url-${it.id}`" v-model="it.url" class="form-input" placeholder="https://example.com/faq" />
                <small v-if="urlError(it.url)" class="field-error">{{ urlError(it.url) }}</small>
              </div>
              <div v-if="it.kind === 'card' && it.url" class="form-group">
                <label class="form-label" :for="`item-label-${it.id}`">Link text</label>
                <input :id="`item-label-${it.id}`" v-model="it.link_label" class="form-input" maxlength="40" placeholder="Learn more" />
              </div>
              <label class="checkbox-label">
                <input v-model="it.new_tab" type="checkbox" />
                Open in a new tab (links to your own site always stay in the tab)
              </label>
            </template>
          </li>
        </ol>
        <div class="add-row">
          <button type="button" class="btn btn-secondary btn-sm" :disabled="hasMessage || items.length >= MAX_HOME_ITEMS" @click="add('message')">Add message</button>
          <button type="button" class="btn btn-secondary btn-sm" :disabled="items.length >= MAX_HOME_ITEMS" @click="add('link')">Add link</button>
          <button type="button" class="btn btn-secondary btn-sm" :disabled="items.length >= MAX_HOME_ITEMS" @click="add('card')">Add card</button>
          <small class="muted">{{ items.length }}/{{ MAX_HOME_ITEMS }}</small>
        </div>

        <h3 class="section-title">Help tab</h3>
        <div class="form-group">
          <label class="form-label" for="help-title">Heading</label>
          <input id="help-title" v-model="home.help_title" class="form-input" maxlength="80" placeholder="Help" />
        </div>
        <p v-if="!links.length" class="muted">No links: the Help tab is hidden.</p>
        <ol class="home-list">
          <li v-for="(l, i) in links" :key="l.id" class="home-item">
            <div class="home-item-head">
              <span class="muted">Link {{ i + 1 }}</span>
              <span class="spacer"></span>
              <button type="button" class="btn btn-secondary btn-sm" :disabled="i === 0" :aria-label="`Move link ${i + 1} up`" @click="move(links, i, i - 1)">↑</button>
              <button type="button" class="btn btn-secondary btn-sm" :disabled="i === links.length - 1" :aria-label="`Move link ${i + 1} down`" @click="move(links, i, i + 1)">↓</button>
              <button type="button" class="btn btn-secondary btn-sm" :aria-label="`Remove link ${i + 1}`" @click="links.splice(i, 1)">Remove</button>
            </div>
            <div class="link-grid">
              <input v-model="l.title" class="form-input" maxlength="80" placeholder="Title" aria-label="Title" />
              <input v-model="l.url" class="form-input" placeholder="https://example.com/help/shipping" aria-label="URL" />
            </div>
            <input v-model="l.description" class="form-input link-desc" maxlength="140" placeholder="Short description (optional)" aria-label="Description" />
            <small v-if="urlError(l.url)" class="field-error">{{ urlError(l.url) }}</small>
          </li>
        </ol>
        <div class="add-row">
          <button type="button" class="btn btn-secondary btn-sm" :disabled="links.length >= MAX_HELP_LINKS" @click="addLink">Add help link</button>
          <small class="muted">{{ links.length }}/{{ MAX_HELP_LINKS }}</small>
        </div>
      </fieldset>
    </div>
  </div>
</template>

<style scoped>
.home-fields {
  border: 0;
  padding: 0;
  margin: 0;
  min-width: 0;
}
.home-fields:disabled {
  opacity: 0.6;
}
.section-title {
  font-size: 14px;
  font-weight: 600;
  margin: 20px 0 8px;
}
.color-row {
  display: flex;
  gap: 8px;
  align-items: center;
}
.color-input {
  width: 44px;
  height: 38px;
  padding: 2px;
  border: 1px solid var(--border-primary);
  border-radius: 8px;
  background: transparent;
  cursor: pointer;
}
.field-error {
  color: var(--danger-600, #dc2626);
}
.home-list {
  list-style: none;
  padding: 0;
  margin: 0 0 8px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.home-item {
  border: 1px solid var(--border-primary);
  border-radius: 10px;
  padding: 12px;
}
.home-item-head {
  display: flex;
  gap: 6px;
  align-items: center;
  margin-bottom: 8px;
}
.spacer {
  flex: 1;
}
.add-row {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}
.link-grid {
  display: grid;
  grid-template-columns: 1fr 2fr;
  gap: 8px;
}
.link-desc {
  margin-top: 8px;
}
@media (max-width: 640px) {
  .link-grid {
    grid-template-columns: 1fr;
  }
}
</style>
