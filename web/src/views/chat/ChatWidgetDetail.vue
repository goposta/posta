<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import NameField from "../../components/NameField.vue";
import { chatWidgetsApi } from "../../api/chat";
import type { ChatHoursRange, ChatWidget, ChatWidgetSnippet } from "../../api/types";
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
// The preview is drawn by the dashboard, not the widget script; its accent custom property is validated above before use.
const previewStyle = computed(() => ({
  "--chat-accent": accentValid.value && widget.value?.accent_color ? widget.value.accent_color : DEFAULT_ACCENT,
}));

function withOrigin(s: ChatWidgetSnippet): ChatWidgetSnippet {
  if (!s.script_url.startsWith("/")) return s;
  const abs = window.location.origin + s.script_url;
  return { ...s, script_url: abs, html: s.html.split(`"${s.script_url}"`).join(`"${abs}"`) };
}

async function load() {
  loading.value = true;
  try {
    const [w, sn] = await Promise.all([chatWidgetsApi.get(id), chatWidgetsApi.snippet(id)]);
    widget.value = w.data.data;
    snippet.value = withOrigin(sn.data.data);
    originsText.value = (widget.value.allowed_origins ?? []).join(", ");
    loadAvailability(widget.value);
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
              <small class="muted">The first line visitors see when they open the chat.</small>
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
              <label class="form-label">Avatar URL</label>
              <input v-model="widget.avatar_url" class="form-input" placeholder="https://example.com/logo.png" />
              <small v-if="!avatarValid" class="field-error">Must be an https:// URL.</small>
              <small v-else class="muted">Shown in the panel header. https only.</small>
            </div>
            <div class="form-group">
              <label class="checkbox-label">
                <input v-model="widget.show_agent_names" type="checkbox" />
                Show visitors the name of the agent answering
              </label>
            </div>
          </div>
        </div>

        <div class="card">
          <div class="card-header"><h2>Preview</h2></div>
          <div class="card-body">
            <div class="preview-page" :class="`pos-${widget.position}`" :style="previewStyle">
              <div class="preview-panel">
                <div class="preview-header">
                  <img v-if="avatarValid && widget.avatar_url" :src="widget.avatar_url" alt="" class="preview-avatar" />
                  <span>{{ widget.title || "Chat with us" }}</span>
                </div>
                <div class="preview-body">
                  <div class="preview-bubble preview-agent">
                    {{ previewOffline ? widget.offline_greeting || widget.greeting || "Hi! How can we help?" : widget.greeting || "Hi! How can we help?" }}
                  </div>
                  <div v-if="!previewOffline" class="preview-bubble preview-visitor">Do you ship to Canada?</div>
                </div>
                <div v-if="previewOffline" class="preview-composer">Your email</div>
                <div class="preview-composer">Type a message…</div>
              </div>
              <div class="preview-launcher" aria-hidden="true">
                <svg width="22" height="22" viewBox="0 0 18 18" fill="none">
                  <path d="M15.75 11.25a1.5 1.5 0 01-1.5 1.5H5.25L2.25 15.75V3.75a1.5 1.5 0 011.5-1.5h10.5a1.5 1.5 0 011.5 1.5v7.5z"
                    stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" />
                </svg>
              </div>
            </div>
            <label class="checkbox-label" style="margin: 8px 0">
              <input v-model="previewOffline" type="checkbox" />
              Show as offline (no agent online, or outside business hours)
            </label>
            <small class="muted">An approximation; the real widget follows the visitor's language and screen size.</small>
          </div>
        </div>
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
            <code>script-src</code> and <code>connect-src</code>.
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

.preview-page {
  position: relative;
  height: 380px;
  border-radius: 10px;
  background: repeating-linear-gradient(135deg, #f3f4f6 0 12px, #eef0f3 12px 24px);
  overflow: hidden;
  margin-bottom: 8px;
}

.preview-panel {
  position: absolute;
  bottom: 76px;
  width: 260px;
  border-radius: 12px;
  background: #fff;
  box-shadow: 0 8px 24px rgba(15, 23, 42, 0.18);
  overflow: hidden;
  display: flex;
  flex-direction: column;
  color: #111827;
  font-size: 13px;
}

.preview-launcher {
  position: absolute;
  bottom: 16px;
  width: 48px;
  height: 48px;
  border-radius: 50%;
  background: var(--chat-accent);
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 4px 12px rgba(15, 23, 42, 0.25);
}

.pos-right .preview-panel,
.pos-right .preview-launcher {
  right: 16px;
}

.pos-left .preview-panel,
.pos-left .preview-launcher {
  left: 16px;
}

.preview-header {
  background: var(--chat-accent);
  color: #fff;
  padding: 12px 14px;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 8px;
}

.preview-avatar {
  width: 24px;
  height: 24px;
  border-radius: 50%;
  object-fit: cover;
  background: #fff;
}

.preview-body {
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-height: 140px;
}

.preview-bubble {
  max-width: 80%;
  padding: 8px 10px;
  border-radius: 10px;
  white-space: pre-wrap;
  word-break: break-word;
}

.preview-agent {
  background: #f3f4f6;
  align-self: flex-start;
}

.preview-visitor {
  background: var(--chat-accent);
  color: #fff;
  align-self: flex-end;
}

.preview-composer {
  border-top: 1px solid #e5e7eb;
  padding: 10px 12px;
  color: #9ca3af;
}
</style>
