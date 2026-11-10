<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import NameField from "../../components/NameField.vue";
import ChatHomeEditor from "./ChatHomeEditor.vue";
import ChatWidgetPreview, { type PreviewConfig } from "./ChatWidgetPreview.vue";
import { assetPath, cloneHome, homeError } from "./chatHome";
import { chatWidgetsApi } from "../../api/chat";
import type { ChatHome, ChatHoursRange, ChatLauncher, ChatTeaser, ChatWidget, ChatWidgetAsset, ChatWidgetSnippet } from "../../api/types";
import { useNotificationStore } from "../../stores/notification";
import { useWorkspaceStore } from "../../stores/workspace";
import { apiMessage } from "../../composables/apiError";
import { useConfirm } from "../../composables/useConfirm";
import { useLicenseStore } from "../../stores/license";

// Auto-assign is Posta Enterprise: read-only without a license that allows changes.
const license = useLicenseStore();
license.load();

const route = useRoute();
const router = useRouter();
const notify = useNotificationStore();
const wsStore = useWorkspaceStore();
const { confirm } = useConfirm();
const id = Number(route.params.id);

const loading = ref(true);
const saving = ref(false);
const loadFailed = ref(false);
const widget = ref<ChatWidget | null>(null);
const home = ref<ChatHome>(cloneHome(null));
const assets = ref<ChatWidgetAsset[]>([]);
const launcher = ref<Required<ChatLauncher>>({ label: "", icon: "", offset_x: 0, offset_y: 0, hide_on_mobile: false });
const teaser = ref<ChatTeaser>({ enabled: false, text: "", delay_seconds: 5, paths: [] });
const teaserPaths = ref("");

function loadLauncher(w: ChatWidget) {
  const l = w.launcher ?? {};
  launcher.value = {
    label: l.label ?? "",
    icon: l.icon || "",
    offset_x: l.offset_x ?? 0,
    offset_y: l.offset_y ?? 0,
    hide_on_mobile: !!l.hide_on_mobile,
  };
  const t = w.teaser;
  teaser.value = { enabled: !!t?.enabled, text: t?.text ?? "", delay_seconds: t?.delay_seconds ?? 0, paths: t?.paths ?? [] };
  teaserPaths.value = (t?.paths ?? []).join(", ");
}
const snippet = ref<ChatWidgetSnippet | null>(null);
const originsText = ref("");

// Availability: business hours are edited as rows; no rows means always.
const browserZone = Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC";
const timezone = ref(browserZone);
const hours = ref<ChatHoursRange[]>([]);
const DAYS = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"];
const TIME_PATTERN = /^([01]\d|2[0-3]):[0-5]\d$|^24:00$/;
const hoursValid = computed(() =>
  hours.value.every((r) => TIME_PATTERN.test(r.from) && TIME_PATTERN.test(r.to) && r.to > r.from),
);
const previewOffline = ref(false);

function loadAvailability(w: ChatWidget) {
  timezone.value = w.business_hours?.timezone || browserZone;
  hours.value = (w.business_hours?.days ?? []).map((r) => ({ ...r }));
}

function addHours() {
  const last = hours.value[hours.value.length - 1];
  const day = last ? (last.day + 1) % 7 : 1;
  hours.value.push({ day, from: last?.from ?? "09:00", to: last?.to ?? "17:00" });
}

function removeHours(i: number) {
  hours.value.splice(i, 1);
}

const TABS = [
  { key: "appearance", label: "Appearance" },
  { key: "home", label: "Home" },
  { key: "availability", label: "Availability" },
  { key: "settings", label: "Settings" },
  { key: "identity", label: "Identity" },
  { key: "embed", label: "Embed" },
] as const;
type Tab = (typeof TABS)[number]["key"];
function tabFromQuery(value: unknown): Tab | undefined {
  return TABS.find((t) => t.key === value)?.key;
}
const tab = ref<Tab>(tabFromQuery(route.query.tab) ?? "appearance");
watch(
  () => route.query.tab,
  (value) => {
    const t = tabFromQuery(value);
    if (t) tab.value = t;
  },
);
function selectTab(t: Tab) {
  if (t === tab.value) return;
  tab.value = t;
  router.replace({ query: { tab: t } });
}

const ACCENT_PATTERN = /^#[0-9a-fA-F]{6}$/;
const DEFAULT_ACCENT = "#7c3aed";
const accentValid = computed(() => !widget.value?.accent_color || ACCENT_PATTERN.test(widget.value.accent_color));
const avatarValid = computed(() => {
  const url = widget.value?.avatar_url?.trim();
  return !url || /^https:\/\/[^\s/]+/.test(url);
});
// The native colour input needs a full #rrggbb; it edits the same field.
const accentPicker = computed({
  get: () => (widget.value && ACCENT_PATTERN.test(widget.value.accent_color) ? widget.value.accent_color : DEFAULT_ACCENT),
  set: (v: string) => {
    if (widget.value) widget.value.accent_color = v;
  },
});
const previewConfig = computed<PreviewConfig>(() => {
  const w = widget.value!;
  const h = home.value;
  const offline = previewOffline.value;
  return {
    enabled: true,
    online: !offline,
    title: w.title,
    greeting: offline && w.offline_greeting ? w.offline_greeting : w.greeting,
    accent_color: accentValid.value ? w.accent_color : "",
    position: w.position,
    avatar_url: avatarValid.value ? w.avatar_url : "",
    require_email: offline || w.require_email,
    theme: w.theme,
    one_open_conversation: w.one_open_conversation,
    status: offline
      ? undefined
      : { reply_time: h.show_reply_time ? "minutes" : undefined, agents: h.show_agents && w.show_agent_names ? ["Jane", "Bob"] : undefined },
    new_chats: offline && w.offline_mode === "hide" ? false : undefined,
    home: h.enabled
      ? {
          ...h,
          logo_url: assetPath(h.logo_asset),
          items: (h.items ?? []).map((i) => ({ ...i, image_url: assetPath(i.image_asset) })),
        }
      : undefined,
  };
});

function withOrigin(s: ChatWidgetSnippet): ChatWidgetSnippet {
  if (!s.script_url.startsWith("/")) return s;
  const abs = window.location.origin + s.script_url;
  return { ...s, script_url: abs, html: s.html.split(`"${s.script_url}"`).join(`"${abs}"`) };
}

async function load() {
  loading.value = true;
  try {
    const [w, sn, as] = await Promise.all([chatWidgetsApi.get(id), chatWidgetsApi.snippet(id), chatWidgetsApi.assets(id)]);
    assets.value = as.data.data ?? [];
    widget.value = w.data.data;
    snippet.value = withOrigin(sn.data.data);
    originsText.value = (widget.value.allowed_origins ?? []).join(", ");
    loadAvailability(widget.value);
    home.value = cloneHome(widget.value.home);
    loadLauncher(widget.value);
  } catch {
    loadFailed.value = true;
  } finally {
    loading.value = false;
  }
}
onMounted(load);

function splitList(value: string) {
  return value
    .split(/[\s,]+/)
    .map((v) => v.trim())
    .filter(Boolean);
}

async function save() {
  const w = widget.value;
  if (!w) return;
  if (!accentValid.value) {
    notify.error("Accent colour must look like #7c3aed.");
    return;
  }
  if (!avatarValid.value) {
    notify.error("Avatar URL must start with https://.");
    return;
  }
  if (!hoursValid.value) {
    notify.error("Each business hours row needs a start time before its end time.");
    return;
  }
  const homeProblem = homeError(home.value);
  if (homeProblem) {
    notify.error(homeProblem);
    selectTab("home");
    return;
  }
  saving.value = true;
  try {
    const res = await chatWidgetsApi.update(id, {
      status: w.status,
      title: w.title,
      greeting: w.greeting,
      accent_color: w.accent_color,
      position: w.position,
      avatar_url: w.avatar_url.trim(),
      show_agent_names: w.show_agent_names,
      home: home.value,
      theme: w.theme,
      launcher: {
        ...launcher.value,
        offset_x: Math.max(0, Number(launcher.value.offset_x) || 0),
        offset_y: Math.max(0, Number(launcher.value.offset_y) || 0),
      },
      teaser: { ...teaser.value, delay_seconds: Math.max(0, Number(teaser.value.delay_seconds) || 0), paths: splitList(teaserPaths.value) },
      one_open_conversation: w.one_open_conversation,
      allowed_origins: splitList(originsText.value),
      strict_origin: w.strict_origin,
      require_name: w.require_name,
      require_email: w.require_email,
      scan_enabled: w.scan_enabled,
      flag_threshold: Number(w.flag_threshold),
      quarantine_threshold: Number(w.quarantine_threshold),
      reject_threshold: Number(w.reject_threshold),
      max_message_bytes: Number(w.max_message_bytes),
      allow_attachments: w.allow_attachments,
      auto_assign: w.auto_assign,
      max_open_per_agent: Math.max(0, Number(w.max_open_per_agent) || 0),
      retention_days: Math.max(0, Number(w.retention_days) || 0),
      business_hours: hours.value.length ? { timezone: timezone.value.trim(), days: hours.value } : {},
      offline_mode: w.offline_mode,
      offline_greeting: w.offline_greeting,
      reply_from: w.reply_from.trim(),
      reply_from_name: w.reply_from_name,
    });
    widget.value = res.data.data;
    originsText.value = (widget.value.allowed_origins ?? []).join(", ");
    loadAvailability(widget.value);
    home.value = cloneHome(widget.value.home);
    loadLauncher(widget.value);
    notify.success("Chat widget saved.");
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to save chat widget"));
  } finally {
    saving.value = false;
  }
}

async function rotateKey() {
  const okToRotate = await confirm({
    title: "Rotate public key",
    message: "Existing embeds stop working immediately. You will need to update the script tag on your website.",
    confirmText: "Rotate key",
    variant: "warning",
  });
  if (!okToRotate) return;
  saving.value = true;
  try {
    const res = await chatWidgetsApi.rotateKey(id);
    widget.value = res.data.data;
    snippet.value = withOrigin((await chatWidgetsApi.snippet(id)).data.data);
    notify.success("Public key rotated. Update your embed code.");
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to rotate key"));
  } finally {
    saving.value = false;
  }
}

async function remove() {
  const okToDelete = await confirm({
    title: "Delete chat widget",
    message: "The widget disappears from every website that embeds it. Its conversations are kept.",
    confirmText: "Delete",
  });
  if (!okToDelete) return;
  try {
    await chatWidgetsApi.delete(id);
    router.push("/chat-widgets");
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to delete chat widget"));
  }
}

// The secret is returned once, when generated, and never again.
const identitySecret = ref("");
const SIGNING_LANGS = ["Go", "Node.js", "PHP", "Python"] as const;
type SigningLang = (typeof SIGNING_LANGS)[number];
const signingLang = ref<SigningLang>("Node.js");
const signingSnippets: Record<SigningLang, string> = {
  Go: `import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func postaChatSignature(secret, userID, email string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(userID + "." + strings.ToLower(email)))
	return hex.EncodeToString(mac.Sum(nil))
}`,
  "Node.js": `import { createHmac } from "node:crypto";

function postaChatSignature(secret, userId, email) {
  return createHmac("sha256", secret)
    .update(\`\${userId}.\${email.toLowerCase()}\`)
    .digest("hex");
}`,
  PHP: `function posta_chat_signature(string $secret, string $userId, string $email): string {
    return hash_hmac('sha256', $userId . '.' . strtolower($email), $secret);
}`,
  Python: `import hashlib
import hmac

def posta_chat_signature(secret: str, user_id: str, email: str) -> str:
    message = f"{user_id}.{email.lower()}".encode()
    return hmac.new(secret.encode(), message, hashlib.sha256).hexdigest()`,
};
// The closing tag is split so it does not end this component's own script block.
const SCRIPT_CLOSE = "</" + "script>";
const identifySnippet = `<script>
  // After posta-chat.js has loaded, for a signed-in user.
  // Compute the signature on your server; never put the secret in the page.
  PostaChat.identify({
    user_id: "u_123",
    email: "jane@example.com",
    name: "Jane",
    signature: "<hex HMAC-SHA256 from your server>",
  });
${SCRIPT_CLOSE}`;

async function rotateIdentitySecret() {
  const w = widget.value;
  if (!w) return;
  const sure = await confirm({
    title: w.has_identity_secret ? "Rotate identity secret" : "Generate identity secret",
    message: w.has_identity_secret
      ? "Signatures made with the current secret stop verifying at once; those visitors carry on anonymously until your server signs with the new secret."
      : "Your server signs each signed-in user's ID and email with this secret, so their chats follow them across devices.",
    confirmText: w.has_identity_secret ? "Rotate secret" : "Generate secret",
    variant: w.has_identity_secret ? "warning" : "info",
  });
  if (!sure) return;
  saving.value = true;
  try {
    const res = await chatWidgetsApi.rotateIdentitySecret(id);
    identitySecret.value = res.data.data.secret;
    w.has_identity_secret = true;
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to create identity secret"));
  } finally {
    saving.value = false;
  }
}

function copy(text: string) {
  navigator.clipboard?.writeText(text);
  notify.success("Copied to clipboard.");
}
</script>

<template>
  <div>
    <div class="page-header">
      <h1>{{ widget?.display_name || "Chat widget" }}</h1>
      <div style="display: flex; gap: 8px">
        <router-link v-if="widget" :to="`/chat?widget_id=${widget.id}`" class="btn btn-secondary">Conversations</router-link>
        <button v-if="wsStore.canEdit" class="btn btn-danger" @click="remove">Delete</button>
        <button class="btn btn-secondary" @click="router.push('/chat-widgets')">Back</button>
      </div>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <div v-else-if="loadFailed" class="card">
      <div class="empty-state">
        <h3>Could not load this chat widget</h3>
        <p>It may have been deleted, or it belongs to another workspace.</p>
        <button class="btn btn-secondary" @click="router.push('/chat-widgets')">Back to chat widgets</button>
      </div>
    </div>

    <template v-else-if="widget">
      <div class="card" style="margin-bottom: 20px">
        <div class="card-body" style="display: flex; gap: 8px; flex-wrap: wrap">
          <button
            v-for="t in TABS"
            :key="t.key"
            class="btn"
            :class="tab === t.key ? 'btn-primary' : 'btn-secondary'"
            @click="selectTab(t.key)"
          >
            {{ t.label }}
          </button>
        </div>
      </div>

      <div v-show="tab === 'appearance'" class="appearance-grid" style="margin-bottom: 20px">
        <div class="card">
          <div class="card-header"><h2>Appearance</h2></div>
          <div class="card-body">
            <div class="form-group">
              <label class="form-label">Title</label>
              <input v-model="widget.title" class="form-input" maxlength="80" placeholder="Chat with us" />
            </div>
            <div class="form-group">
              <label class="form-label">Greeting</label>
              <textarea v-model="widget.greeting" class="form-input" rows="3" maxlength="500"
                placeholder="Hi! How can we help?"></textarea>
              <small class="muted">The first line visitors see when they open the chat. <code>{name}</code> becomes the visitor's first name when it is known, and disappears otherwise.</small>
            </div>
            <div class="form-group">
              <label class="form-label">Accent colour</label>
              <div style="display: flex; gap: 8px; align-items: center">
                <input v-model="accentPicker" type="color" class="color-input" aria-label="Pick accent colour" />
                <input v-model="widget.accent_color" class="form-input" placeholder="#7c3aed" maxlength="7" />
              </div>
              <small v-if="!accentValid" class="field-error">Use a #RRGGBB colour, like #7c3aed.</small>
            </div>
            <div class="form-group">
              <label class="form-label">Position</label>
              <select v-model="widget.position" class="form-select">
                <option value="right">Bottom right</option>
                <option value="left">Bottom left</option>
              </select>
            </div>
            <div class="form-group">
              <label class="form-label" for="widget-theme">Theme</label>
              <select id="widget-theme" v-model="widget.theme" class="form-select">
                <option value="auto">Follow the visitor's system</option>
                <option value="light">Light</option>
                <option value="dark">Dark</option>
              </select>
            </div>
            <div class="form-group">
              <label class="form-label">Avatar URL</label>
              <input v-model="widget.avatar_url" class="form-input" placeholder="https://example.com/logo.png" />
              <small v-if="!avatarValid" class="field-error">Must be an https:// URL.</small>
              <small v-else class="muted">Shown in the chat header. https only. For the home screen, upload a logo on the Home tab instead: it is served from Posta.</small>
            </div>
            <div class="form-group">
              <label class="checkbox-label">
                <input v-model="widget.show_agent_names" type="checkbox" />
                Show visitors the name of the agent answering
              </label>
            </div>

            <h3 class="section-title">Button</h3>
            <div class="form-group">
              <label class="form-label" for="launcher-label">Label</label>
              <input id="launcher-label" v-model="launcher.label" class="form-input" maxlength="40" placeholder="Need help?" />
              <small class="muted">Shown next to the closed button. Blank shows the button alone.</small>
            </div>
            <div class="form-group">
              <label class="form-label" for="launcher-icon">Icon</label>
              <select id="launcher-icon" v-model="launcher.icon" class="form-select">
                <option value="">Chat bubble</option>
                <option value="help">Question mark</option>
                <option value="mail">Envelope</option>
              </select>
            </div>
            <div class="offset-grid">
              <div class="form-group">
                <label class="form-label" for="launcher-x">From the side (px)</label>
                <input id="launcher-x" v-model.number="launcher.offset_x" type="number" min="0" max="200" class="form-input" placeholder="20" />
              </div>
              <div class="form-group">
                <label class="form-label" for="launcher-y">From the bottom (px)</label>
                <input id="launcher-y" v-model.number="launcher.offset_y" type="number" min="0" max="200" class="form-input" placeholder="20" />
              </div>
            </div>
            <small class="muted">0 keeps the default of 20 pixels. Raise it to clear a cookie banner or another floating button.</small>
            <div class="form-group" style="margin-top: 12px">
              <label class="checkbox-label">
                <input v-model="launcher.hide_on_mobile" type="checkbox" />
                Hide the button on phones
              </label>
              <small class="muted">Links with <code>data-posta-chat-open</code> and <code>PostaChat.open()</code> still open the chat.</small>
            </div>

            <h3 class="section-title">Teaser</h3>
            <div class="form-group">
              <label class="checkbox-label">
                <input v-model="teaser.enabled" type="checkbox" :disabled="!license.mutable" />
                Show a message bubble over the closed button
              </label>
              <small v-if="!license.licensed" class="muted">The teaser is part of Posta Enterprise.</small>
              <small v-else-if="license.readOnly" class="muted">The Enterprise license has expired: the teaser keeps showing, but can't be changed until it is renewed.</small>
            </div>
            <template v-if="teaser.enabled">
              <div class="form-group">
                <label class="form-label" for="teaser-text">Text</label>
                <input id="teaser-text" v-model="teaser.text" class="form-input" maxlength="140" :disabled="!license.mutable"
                  placeholder="Questions about our plans? We're here." />
              </div>
              <div class="form-group">
                <label class="form-label" for="teaser-delay">Show after (seconds)</label>
                <input id="teaser-delay" v-model.number="teaser.delay_seconds" type="number" min="0" max="120" class="form-input" :disabled="!license.mutable" />
              </div>
              <div class="form-group">
                <label class="form-label" for="teaser-paths">On pages</label>
                <input id="teaser-paths" v-model="teaserPaths" class="form-input" :disabled="!license.mutable" placeholder="/pricing, /docs/*" />
                <small class="muted">Comma separated paths; end one with <code>*</code> to match everything under it. Blank shows it on every page. A visitor who closes it doesn't see it again until you change the text.</small>
              </div>
            </template>
          </div>
        </div>

        <ChatWidgetPreview :config="previewConfig" :home-enabled="home.enabled"
          :help-visible="!!home.help_links?.length">
          <label class="checkbox-label" style="margin: 8px 0">
            <input v-model="previewOffline" type="checkbox" />
            Show as offline (no agent online, or outside business hours)
          </label>
        </ChatWidgetPreview>
      </div>

      <div v-show="tab === 'home'" class="appearance-grid" style="margin-bottom: 20px">
        <div class="home-col">
          <ChatHomeEditor v-model="home" v-model:assets="assets" :widget-id="widget.id" :accent="accentPicker"
            :disabled="!wsStore.canEdit" />
          <div class="card">
            <div class="card-header"><h2>Conversations</h2></div>
            <div class="card-body">
              <label class="checkbox-label">
                <input v-model="widget.one_open_conversation" type="checkbox" />
                Visitors can have one open conversation at a time
              </label>
              <small class="muted">Turn it off to let visitors start a new conversation from the Messages tab while another is open.</small>
            </div>
          </div>
        </div>
        <ChatWidgetPreview v-if="tab === 'home'" :config="previewConfig" :home-enabled="home.enabled"
          :help-visible="!!home.help_links?.length" />
      </div>

      <div v-show="tab === 'availability'" class="card" style="margin-bottom: 20px">
        <div class="card-header"><h2>Availability</h2></div>
        <div class="card-body">
          <p class="muted" style="margin-top: 0">
            The widget is online when at least one agent has set themselves online and the time is within business hours.
          </p>

          <h3 class="section-title">Business hours</h3>
          <div class="form-group">
            <label class="form-label">Time zone</label>
            <input v-model="timezone" class="form-input" placeholder="Europe/Paris" />
            <small class="muted">An IANA time zone, such as Europe/Paris or America/New_York.</small>
          </div>
          <p v-if="!hours.length" class="muted">Always available: no hours set.</p>
          <div v-for="(r, i) in hours" :key="i" class="hours-row">
            <select v-model.number="r.day" class="form-select" aria-label="Day">
              <option v-for="(d, n) in DAYS" :key="n" :value="n">{{ d }}</option>
            </select>
            <input v-model="r.from" type="time" class="form-input" aria-label="From" />
            <span class="muted">to</span>
            <input v-model="r.to" type="time" class="form-input" aria-label="To" />
            <button class="btn btn-secondary btn-sm" type="button" @click="removeHours(i)">Remove</button>
          </div>
          <small v-if="!hoursValid" class="field-error">Each row needs a start time before its end time.</small>
          <div style="margin-top: 8px">
            <button class="btn btn-secondary btn-sm" type="button" @click="addHours">Add hours</button>
          </div>

          <h3 class="section-title">When no one is available</h3>
          <div class="form-group">
            <label class="checkbox-label">
              <input v-model="widget.offline_mode" type="radio" value="email" />
              Take messages and reply by email (visitors must give their email)
            </label>
            <label class="checkbox-label">
              <input v-model="widget.offline_mode" type="radio" value="hide" />
              Hide the widget
            </label>
            <small class="muted">With a home screen, visitors who already have a conversation can still open it and read replies; no one can start a new chat.</small>
          </div>
          <div class="form-group">
            <label class="form-label">Offline greeting</label>
            <textarea v-model="widget.offline_greeting" class="form-input" rows="2" maxlength="500"
              placeholder="We're away right now. Leave a message and we'll reply by email."></textarea>
            <small class="muted">Shown instead of the greeting while the widget is offline. Blank keeps the greeting.</small>
          </div>

          <h3 class="section-title">Replies to visitors who left</h3>
          <div class="form-group">
            <label class="form-label">Reply from (email)</label>
            <input v-model="widget.reply_from" type="email" class="form-input" placeholder="support@example.com" />
            <small class="muted">
              Must be on a verified domain of this workspace. When a visitor has left and gave an email, agents' replies are
              emailed to them from this address. Without it, those replies wait in the chat until the visitor returns.
            </small>
          </div>
          <div class="form-group">
            <label class="form-label">Reply from (name)</label>
            <input v-model="widget.reply_from_name" class="form-input" maxlength="80" placeholder="Acme Support" />
          </div>
        </div>
      </div>

      <div v-show="tab === 'settings'" class="card" style="margin-bottom: 20px">
        <div class="card-header"><h2>Settings</h2></div>
        <div class="card-body">
          <NameField :existing="widget" resource="chat widget" usage="Identifies the widget in the API." />
          <div class="form-group">
            <label class="form-label">Status</label>
            <select v-model="widget.status" class="form-select">
              <option value="active">Active</option>
              <option value="paused">Paused (hidden on websites)</option>
              <option value="archived">Archived</option>
            </select>
          </div>
          <div class="form-group">
            <label class="form-label">Allowed origins</label>
            <input v-model="originsText" class="form-input" placeholder="https://example.com" />
            <small class="muted">
              Comma separated. Blank allows any website. This stops other sites from using your
              widget in a browser; it is a speed bump, not authentication.
            </small>
          </div>
          <div class="form-group">
            <label class="checkbox-label">
              <input v-model="widget.strict_origin" type="checkbox" />
              Reject requests with no Origin header (recommended: the widget only runs in browsers)
            </label>
          </div>

          <h3 class="section-title">Before the first message</h3>
          <div class="form-group">
            <label class="checkbox-label">
              <input v-model="widget.require_name" type="checkbox" />
              Ask for the visitor's name
            </label>
          </div>
          <div class="form-group">
            <label class="checkbox-label">
              <input v-model="widget.require_email" type="checkbox" />
              Ask for the visitor's email
            </label>
            <small class="muted">What visitors type is not verified. It is shown to agents and never used to link conversations.</small>
          </div>

          <h3 class="section-title">Spam</h3>
          <div class="form-group">
            <label class="checkbox-label">
              <input v-model="widget.scan_enabled" type="checkbox" />
              Scan the first message of each conversation
            </label>
          </div>
          <div class="threshold-grid">
            <div class="form-group">
              <label class="form-label">Flag at score</label>
              <input v-model.number="widget.flag_threshold" type="number" class="form-input" min="0" max="100" step="0.5" />
            </div>
            <div class="form-group">
              <label class="form-label">Quarantine at score</label>
              <input v-model.number="widget.quarantine_threshold" type="number" class="form-input" min="0" max="100" step="0.5" />
            </div>
            <div class="form-group">
              <label class="form-label">Reject at score</label>
              <input v-model.number="widget.reject_threshold" type="number" class="form-input" min="0" max="100" step="0.5" />
            </div>
          </div>
          <small class="muted">Quarantined conversations stay out of the default list and notify no one. Rejected ones are not stored.</small>
          <div class="form-group" style="margin-top: 16px">
            <label class="form-label">Maximum message size (bytes)</label>
            <input v-model.number="widget.max_message_bytes" type="number" class="form-input" min="256" />
          </div>

          <h3 class="section-title">Files</h3>
          <div class="form-group">
            <label class="checkbox-label">
              <input v-model="widget.allow_attachments" type="checkbox" />
              Let visitors and agents send images and PDFs (up to 5 MB)
            </label>
            <small class="muted">Needs blob storage on the server (<code>POSTA_BLOB_PROVIDER</code>); without it, files are refused.</small>
          </div>

          <h3 class="section-title">Assignment</h3>
          <div class="form-group">
            <label class="checkbox-label">
              <input v-model="widget.auto_assign" type="checkbox" :disabled="!license.mutable" />
              Assign each new conversation to the next online agent in turn
            </label>
            <small v-if="!license.licensed" class="muted">
              Auto-assign is part of Posta Enterprise. Without it, agents claim conversations themselves.
            </small>
            <small v-else-if="license.readOnly" class="muted">
              The Enterprise license has expired: auto-assign keeps working, but can't be changed until it is renewed.
            </small>
            <small v-else class="muted">When nobody is online, conversations stay unassigned.</small>
          </div>
          <div v-if="widget.auto_assign" class="form-group">
            <label class="form-label" for="max-open">Maximum open conversations per agent</label>
            <input id="max-open" v-model.number="widget.max_open_per_agent" type="number" class="form-input" min="0" max="1000" :disabled="!license.mutable" />
            <small class="muted">Agents holding this many open conversations are skipped. 0 means no limit.</small>
          </div>

          <h3 class="section-title">Retention</h3>
          <div class="form-group">
            <label class="form-label" for="retention-days">Keep closed and spam conversations for (days)</label>
            <input id="retention-days" v-model.number="widget.retention_days" type="number" class="form-input" min="0" max="3650" />
            <small class="muted">0 uses the platform's retention (<code>POSTA_CHAT_RETENTION_DAYS</code>). A widget can only shorten it; open conversations are never deleted.</small>
          </div>
        </div>
      </div>

      <div v-show="tab === 'identity'" class="card" style="margin-bottom: 20px">
        <div class="card-header"><h2>Identity verification</h2></div>
        <div class="card-body">
          <p class="muted" style="margin-top: 0">
            For users signed in to your website, your server signs their user ID and email with this widget's secret.
            A valid signature marks the email as verified and lets the same user see their chats on any device.
            Without it, what visitors type is shown as unverified and never linked to anyone.
          </p>
          <div class="form-group">
            <span v-if="widget.has_identity_secret" class="badge badge-success">Secret set</span>
            <span v-else class="badge badge-secondary">No secret yet</span>
          </div>
          <button v-if="wsStore.canEdit" class="btn btn-secondary" :disabled="saving" @click="rotateIdentitySecret">
            {{ widget.has_identity_secret ? "Rotate secret" : "Generate secret" }}
          </button>

          <div v-if="identitySecret" class="secret-box" role="status">
            <strong>Copy this secret now. It will not be shown again.</strong>
            <div class="secret-row">
              <code class="secret">{{ identitySecret }}</code>
              <button class="btn btn-primary btn-sm" @click="copy(identitySecret)">Copy</button>
            </div>
            <small class="muted">Keep it on your server only, never in the page.</small>
          </div>

          <h3 class="section-title">1. Sign on your server</h3>
          <p class="muted">signature = hex HMAC-SHA256(secret, user_id + "." + lowercase(email))</p>
          <div class="lang-tabs" role="tablist" aria-label="Server language">
            <button v-for="l in SIGNING_LANGS" :key="l" role="tab" :aria-selected="signingLang === l"
              class="btn btn-sm" :class="signingLang === l ? 'btn-primary' : 'btn-secondary'" @click="signingLang = l">{{ l }}</button>
          </div>
          <pre class="snippet">{{ signingSnippets[signingLang] }}</pre>
          <button class="btn btn-secondary btn-sm" @click="copy(signingSnippets[signingLang])">Copy</button>

          <h3 class="section-title">2. Identify in the page</h3>
          <pre class="snippet">{{ identifySnippet }}</pre>
          <button class="btn btn-secondary btn-sm" @click="copy(identifySnippet)">Copy</button>
        </div>
      </div>

      <div v-show="tab === 'embed'" class="card" style="margin-bottom: 20px">
        <div class="card-header"><h2>Embed</h2></div>
        <div class="card-body">
          <p class="muted" style="margin-top: 0">
            Paste this before <code>&lt;/body&gt;</code> on every page that should show the chat.
            If your site sends a Content-Security-Policy, allow this Posta origin in
            <code>script-src</code> and <code>connect-src</code>, and in <code>img-src</code> when the home screen shows images.
          </p>
          <pre class="snippet">{{ snippet?.html }}</pre>
          <div style="display: flex; gap: 8px; flex-wrap: wrap">
            <button class="btn btn-primary" @click="copy(snippet?.html || '')">Copy embed code</button>
            <button v-if="wsStore.canEdit" class="btn btn-secondary" :disabled="saving" @click="rotateKey">Rotate key</button>
          </div>
          <div class="form-group" style="margin-top: 16px">
            <label class="form-label">Public key</label>
            <code>{{ widget.public_key }}</code>
          </div>
        </div>
      </div>

      <div v-if="wsStore.canEdit" v-show="tab !== 'embed' && tab !== 'identity'" class="card">
        <div class="card-body">
          <button class="btn btn-primary" :disabled="saving" @click="save">
            {{ saving ? "Saving..." : "Save changes" }}
          </button>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.secret-box {
  margin-top: 16px;
  padding: 14px 16px;
  border-radius: 10px;
  border: 1px solid var(--border-primary);
  background: var(--bg-secondary);
}
.secret-row {
  display: flex;
  gap: 8px;
  align-items: center;
  margin: 8px 0;
}
.secret {
  word-break: break-all;
  font-size: 13px;
}
.lang-tabs {
  display: flex;
  gap: 4px;
  margin-bottom: 8px;
}

.appearance-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 20px;
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

.hours-row {
  display: grid;
  grid-template-columns: minmax(120px, 160px) minmax(100px, 130px) auto minmax(100px, 130px) auto;
  gap: 8px;
  align-items: center;
  margin-bottom: 8px;
}

@media (max-width: 640px) {
  .hours-row {
    grid-template-columns: 1fr 1fr;
  }
}

.section-title {
  font-size: 14px;
  font-weight: 600;
  margin: 20px 0 8px;
}

.threshold-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(140px, 1fr));
  gap: 12px;
}

.snippet {
  white-space: pre-wrap;
  word-break: break-all;
  font-size: 12px;
  padding: 12px;
  border-radius: 8px;
  background: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  margin: 0 0 12px;
}

.offset-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}
.home-col {
  display: flex;
  flex-direction: column;
  gap: 20px;
  min-width: 0;
}
</style>
