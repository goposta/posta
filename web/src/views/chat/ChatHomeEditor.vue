<script setup lang="ts">
import { computed, ref } from "vue";
import { chatWidgetsApi } from "../../api/chat";
import type { ChatHome, ChatHomeItemKind, ChatWidgetAsset } from "../../api/types";
import { apiMessage } from "../../composables/apiError";
import { useNotificationStore } from "../../stores/notification";
import { assetPath, MAX_HELP_LINKS, MAX_HOME_ITEMS, move, newID, newItem, urlError } from "./chatHome";

const home = defineModel<ChatHome>({ required: true });
const assets = defineModel<ChatWidgetAsset[]>("assets", { required: true });
const props = defineProps<{ accent: string; widgetId: number; disabled?: boolean }>();
const notify = useNotificationStore();

const MAX_IMAGE_BYTES = 512 * 1024;
const fileInput = ref<HTMLInputElement | null>(null);
const uploading = ref(false);
let assign: ((id: string) => void) | null = null;

const inUse = computed(() => {
  const ids = new Set<string>();
  if (home.value.logo_asset) ids.add(home.value.logo_asset);
  for (const it of home.value.items ?? []) if (it.image_asset) ids.add(it.image_asset);
  return ids;
});

function pick(target: (id: string) => void) {
  assign = target;
  fileInput.value?.click();
}

async function onFile(e: Event) {
  const input = e.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = "";
  if (!file || !assign) return;
  if (file.size > MAX_IMAGE_BYTES) {
    notify.error("Images can be at most 512 KB.");
    return;
  }
  uploading.value = true;
  try {
    const res = await chatWidgetsApi.uploadAsset(props.widgetId, file);
    assets.value = [res.data.data, ...assets.value];
    assign(res.data.data.id);
  } catch (err: any) {
    notify.error(apiMessage(err, "Failed to upload the image"));
  } finally {
    uploading.value = false;
  }
}

async function removeAsset(id: string) {
  try {
    await chatWidgetsApi.deleteAsset(props.widgetId, id);
    assets.value = assets.value.filter((a) => a.id !== id);
  } catch (err: any) {
    notify.error(apiMessage(err, "Failed to delete the image"));
  }
}

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
          <span class="form-label">Logo</span>
          <div class="image-row">
            <img v-if="home.logo_asset" :src="assetPath(home.logo_asset)" alt="Logo" class="thumb"
              :style="{ background: colorValid && home.header_color ? home.header_color : accent }" />
            <button type="button" class="btn btn-secondary btn-sm" :disabled="uploading"
              @click="pick((id) => (home.logo_asset = id))">{{ home.logo_asset ? "Replace" : "Upload logo" }}</button>
            <button v-if="home.logo_asset" type="button" class="btn btn-secondary btn-sm" @click="home.logo_asset = ''">Remove</button>
          </div>
          <small class="muted">PNG, JPEG, GIF or WebP, up to 512 KB. Served from Posta, so a site with a Content Security Policy only needs Posta's origin in <code>img-src</code>.</small>
        </div>
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

        <div class="form-group">
          <label class="checkbox-label">
            <input v-model="home.show_reply_time" type="checkbox" />
            Show the typical reply time while online
          </label>
          <small class="muted">From the median first reply over 30 days, once 10 chats have been answered. Outside business hours the header says when you're back instead.</small>
        </div>
        <div class="form-group">
          <label class="checkbox-label">
            <input v-model="home.show_agents" type="checkbox" />
            Show the first names of online agents
          </label>
          <small class="muted">Up to three, and only while "Show visitors the name of the agent answering" is on in Appearance.</small>
        </div>

        <h3 class="section-title">Items</h3>
        <small v-if="!items.length" class="muted empty-note">No items: visitors reach the chat from the Messages tab.</small>
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
              <span class="form-label">Image (optional)</span>
              <div class="image-row">
                <img v-if="it.image_asset" :src="assetPath(it.image_asset)" alt="" class="thumb wide" />
                <button type="button" class="btn btn-secondary btn-sm" :disabled="uploading"
                  @click="pick((id) => (it.image_asset = id))">{{ it.image_asset ? "Replace" : "Upload image" }}</button>
                <button v-if="it.image_asset" type="button" class="btn btn-secondary btn-sm" @click="it.image_asset = ''">Remove</button>
              </div>
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
        <small v-if="!links.length" class="muted empty-note">No links: the Help tab is hidden.</small>
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
        <template v-if="assets.length">
          <h3 class="section-title">Uploaded images</h3>
          <ul class="asset-grid">
            <li v-for="a in assets" :key="a.id">
              <img :src="assetPath(a.id)" :alt="a.filename" class="thumb wide" />
              <span class="muted asset-name" :title="a.filename">{{ a.filename }}</span>
              <button type="button" class="btn btn-secondary btn-sm" :disabled="inUse.has(a.id)"
                :title="inUse.has(a.id) ? 'Shown on the home screen' : ''" @click="removeAsset(a.id)">Delete</button>
            </li>
          </ul>
        </template>
      </fieldset>
      <input ref="fileInput" type="file" accept="image/png,image/jpeg,image/gif,image/webp" class="file-input" @change="onFile" />
    </div>
  </div>
</template>

<style scoped>
.empty-note {
  display: block;
  margin-bottom: 8px;
}
.image-row {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}
.thumb {
  height: 36px;
  max-width: 120px;
  object-fit: contain;
  border-radius: 6px;
  border: 1px solid var(--border-primary);
  background: var(--bg-secondary);
}
.thumb.wide {
  width: 96px;
  height: 54px;
  object-fit: cover;
}
.asset-grid {
  list-style: none;
  padding: 0;
  margin: 0;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));
  gap: 10px;
}
.asset-grid li {
  display: flex;
  flex-direction: column;
  gap: 4px;
  align-items: flex-start;
}
.asset-name {
  font-size: 12px;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.file-input {
  display: none;
}
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
