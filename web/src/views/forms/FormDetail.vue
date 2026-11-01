<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { formsApi } from "../../api/messages";
import { useNotificationStore } from "../../stores/notification";
import { apiMessage } from "../../composables/apiError";
import type { Form, FormSnippet } from "../../api/types";

const route = useRoute();
const router = useRouter();
const notify = useNotificationStore();
const id = Number(route.params.id);

const loading = ref(true);
const saving = ref(false);
const form = ref<Form | null>(null);
const snippet = ref<FormSnippet | null>(null);
const loadFailed = ref(false);
const tab = ref<"setup" | "responses" | "spam" | "notifications" | "embed">("setup");

const originsText = ref("");
// close_at is edited in the browser's local time and stored as UTC.
const closeAtLocal = ref("");
const appearance = ref({ accent: "", theme: "", header_image_url: "" });

function toLocalInput(iso?: string | null) {
  if (!iso) return "";
  const d = new Date(iso);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function setAppearanceAccent(e: Event) {
  appearance.value.accent = (e.target as HTMLInputElement).value;
}

function syncResponseFields(f: Form) {
  closeAtLocal.value = toLocalInput(f.close_at);
  appearance.value = {
    accent: f.appearance?.accent || "",
    // "light" is the default, which the select shows as its empty option.
    theme: f.appearance?.theme === "light" ? "" : f.appearance?.theme || "",
    header_image_url: f.appearance?.header_image_url || "",
  };
}
const notifyText = ref("");

async function load() {
  loading.value = true;
  try {
    const [formRes, snippetRes] = await Promise.all([formsApi.get(id), formsApi.snippet(id)]);
    form.value = formRes.data.data;
    snippet.value = snippetRes.data.data;
    originsText.value = (formRes.data.data.allowed_origins || []).join(", ");
    syncResponseFields(formRes.data.data);
    notifyText.value = (formRes.data.data.notify_emails || []).join(", ");
  } catch (e: any) {
    loadFailed.value = true;
    notify.error(apiMessage(e, "Failed to load form"));
  } finally {
    loading.value = false;
  }
}

function splitList(value: string) {
  return value
    .split(/[\s,]+/)
    .map((v) => v.trim())
    .filter(Boolean);
}

async function save() {
  if (!form.value) return;
  saving.value = true;
  try {
    const res = await formsApi.update(id, {
      name: form.value.name,
      description: form.value.description,
      status: form.value.status,
      allowed_origins: splitList(originsText.value),
      strict_origin: form.value.strict_origin,
      redirect_url: form.value.redirect_url,
      allow_attachments: form.value.allow_attachments,
      honeypot_field: form.value.honeypot_field,
      require_nonce: form.value.require_nonce,
      min_fill_seconds: form.value.min_fill_seconds,
      scan_enabled: form.value.scan_enabled,
      flag_threshold: form.value.flag_threshold,
      quarantine_threshold: form.value.quarantine_threshold,
      reject_threshold: form.value.reject_threshold,
      notify_enabled: form.value.notify_enabled,
      notify_emails: splitList(notifyText.value),
      notify_mode: form.value.notify_mode,
      notify_on_flagged: form.value.notify_on_flagged,
      reply_from: form.value.reply_from,
      reply_from_name: form.value.reply_from_name,
      retention_days: form.value.retention_days,
      confirmation_message: form.value.confirmation_message,
      closed_message: form.value.closed_message,
      close_at: closeAtLocal.value ? new Date(closeAtLocal.value).toISOString() : "",
      max_responses: Number(form.value.max_responses) || 0,
      allow_another_response: form.value.allow_another_response,
      appearance: {
        accent: appearance.value.accent,
        theme: appearance.value.theme as "" | "light" | "dark" | "auto",
        header_image_url: appearance.value.header_image_url.trim(),
      },
    });
    form.value = res.data.data;
    syncResponseFields(res.data.data);
    notify.success("Form saved.");
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to save form"));
  } finally {
    saving.value = false;
  }
}

async function rotateKey() {
  if (!confirm("Rotate the public key? Existing embeds stop working immediately.")) return;
  saving.value = true;
  try {
    const res = await formsApi.rotateKey(id);
    form.value = res.data.data;
    const snippetRes = await formsApi.snippet(id);
    snippet.value = snippetRes.data.data;
    notify.success("Public key rotated. Update your embed code.");
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to rotate key"));
  } finally {
    saving.value = false;
  }
}

async function remove() {
  if (!confirm("Delete this form? Its messages are deleted too.")) return;
  try {
    await formsApi.delete(id);
    router.push("/forms");
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to delete form"));
  }
}

function copy(text: string) {
  navigator.clipboard?.writeText(text);
  notify.success("Copied to clipboard.");
}

const endpoint = computed(() => snippet.value?.endpoint || "");

// Website widget builder. Options become data attributes on the placeholder;
// nothing is saved to the form.
const optionalWidgetFields = ["name", "phone", "subject"] as const;
const widget = ref({
  fields: ["name"] as string[],
  title: "",
  description: "",
  button: "",
  success: "",
  theme: "light",
  accent: "",
  lang: "",
});

function setAccent(e: Event) {
  widget.value.accent = (e.target as HTMLInputElement).value;
}

function escapeAttr(value: string) {
  return value.replace(/&/g, "&amp;").replace(/"/g, "&quot;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

const widgetScriptURL = computed(() =>
  endpoint.value.replace(/\/api\/v1\/f\/[^/]+$/, "") + "/widget/v1/posta-form.js",
);

const widgetSnippet = computed(() => {
  if (!form.value) return "";
  const w = widget.value;
  const attrs: [string, string][] = [["data-posta-form", form.value.public_key]];
  const fields = ["name", "email", "phone", "subject", "message"].filter(
    (f) => f === "email" || f === "message" || w.fields.includes(f),
  );
  if (fields.join(",") !== "name,email,message") attrs.push(["data-fields", fields.join(",")]);
  if (w.title.trim()) attrs.push(["data-title", w.title.trim()]);
  if (w.description.trim()) attrs.push(["data-description", w.description.trim()]);
  if (w.button.trim()) attrs.push(["data-button", w.button.trim()]);
  if (w.success.trim()) attrs.push(["data-success", w.success.trim()]);
  if (w.theme !== "light") attrs.push(["data-theme", w.theme]);
  if (w.accent) attrs.push(["data-accent", w.accent]);
  if (w.lang) attrs.push(["data-lang", w.lang]);
  const placeholder =
    attrs.length > 2
      ? `<div\n${attrs.map(([k, v]) => `  ${k}="${escapeAttr(v)}"`).join("\n")}\n></div>`
      : `<div ${attrs.map(([k, v]) => `${k}="${escapeAttr(v)}"`).join(" ")}></div>`;
  return `${placeholder}\n<script src="${escapeAttr(widgetScriptURL.value)}" async></` + `script>`;
});

onMounted(load);
</script>

<template>
  <div>
    <div class="page-header">
      <h1>{{ form?.name || "Form" }}</h1>
      <div style="display: flex; gap: 8px">
        <router-link
          v-if="form"
          :to="`/messages?form_id=${form.id}`"
          class="btn btn-secondary"
        >View messages</router-link>
        <button class="btn btn-danger" @click="remove">Delete</button>
        <button class="btn btn-secondary" @click="router.push('/forms')">Back</button>
      </div>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <div v-else-if="loadFailed" class="card">
      <div class="empty-state">
        <h3>Could not load this form</h3>
        <p>It may have been deleted, or it belongs to another workspace.</p>
        <button class="btn btn-secondary" @click="router.push('/forms')">Back to forms</button>
      </div>
    </div>

    <template v-else-if="form">
      <div class="card" style="margin-bottom: 20px">
        <div class="card-body" style="display: flex; gap: 8px; flex-wrap: wrap">
          <button
            v-for="t in (['setup', 'responses', 'spam', 'notifications', 'embed'] as const)"
            :key="t"
            class="btn"
            :class="tab === t ? 'btn-primary' : 'btn-secondary'"
            @click="tab = t"
          >
            {{ t }}
          </button>
        </div>
      </div>

      <div v-show="tab === 'setup'" class="card" style="margin-bottom: 20px">
        <div class="card-header"><h2>Setup</h2></div>
        <div class="card-body">
          <div class="form-group">
            <label class="form-label">Name</label>
            <input v-model="form.name" class="form-input" />
          </div>
          <div class="form-group">
            <label class="form-label">Description</label>
            <input v-model="form.description" class="form-input" />
          </div>
          <div class="form-group">
            <label class="form-label">Status</label>
            <select v-model="form.status" class="form-select">
              <option value="active">Active</option>
              <option value="paused">Paused</option>
              <option value="archived">Archived</option>
            </select>
          </div>
          <div class="form-group">
            <label class="form-label">Allowed origins</label>
            <input v-model="originsText" class="form-input" placeholder="https://example.com" />
            <small class="muted">
              Comma separated. Blank accepts any origin. A plain HTML form post is a CORS
              simple request, so this is a spam speed bump, not authentication.
            </small>
          </div>
          <div class="form-group">
            <label class="checkbox-label">
              <input v-model="form.strict_origin" type="checkbox" />
              Reject submissions with no Origin header
            </label>
          </div>
          <div class="form-group">
            <label class="form-label">Redirect after submit</label>
            <input v-model="form.redirect_url" class="form-input" placeholder="https://example.com/thanks" />
          </div>
          <div class="form-group">
            <label class="form-label">Honeypot field name</label>
            <input v-model="form.honeypot_field" class="form-input" />
          </div>
          <div class="form-group">
            <label class="checkbox-label">
              <input v-model="form.require_nonce" type="checkbox" />
              Require a signed nonce (blocks scripted posts, needs the JS snippet)
            </label>
          </div>
          <div class="form-group">
            <label class="form-label">Minimum fill time (seconds)</label>
            <input v-model.number="form.min_fill_seconds" type="number" class="form-input" min="0" max="120" />
          </div>
          <div class="form-group">
            <label class="checkbox-label">
              <input v-model="form.allow_attachments" type="checkbox" />
              Accept file attachments
            </label>
          </div>
          <div class="form-group">
            <label class="form-label">Reply sender</label>
            <input v-model="form.reply_from" class="form-input" placeholder="support@example.com" />
            <small class="muted">Must be on a verified domain in this workspace.</small>
          </div>
          <div class="form-group">
            <label class="form-label">Reply sender name</label>
            <input v-model="form.reply_from_name" class="form-input" placeholder="Acme Support" />
          </div>
          <div class="form-group">
            <label class="form-label">Retention (days, 0 uses the platform default)</label>
            <input v-model.number="form.retention_days" type="number" class="form-input" min="0" />
            <p class="muted" style="font-size: 12px; margin: 4px 0 0">
              Shortens how long this form's messages are kept. It cannot extend retention past the platform setting.
            </p>
          </div>
        </div>
      </div>

      <div v-show="tab === 'spam'" class="card" style="margin-bottom: 20px">
        <div class="card-header">
          <h2>Spam scanning</h2>
          <router-link to="/message-filters" class="btn btn-secondary">Manage filters</router-link>
        </div>
        <div class="card-body">
          <div class="form-group">
            <label class="checkbox-label">
              <input v-model="form.scan_enabled" type="checkbox" />
              Scan submissions
            </label>
          </div>
          <div class="form-group">
            <label class="form-label">Flag at score</label>
            <input v-model.number="form.flag_threshold" type="number" step="0.5" class="form-input" />
            <small class="muted">Stored and notified, marked for review.</small>
          </div>
          <div class="form-group">
            <label class="form-label">Quarantine at score</label>
            <input v-model.number="form.quarantine_threshold" type="number" step="0.5" class="form-input" />
            <small class="muted">Stored, no notification.</small>
          </div>
          <div class="form-group">
            <label class="form-label">Reject at score</label>
            <input v-model.number="form.reject_threshold" type="number" step="0.5" class="form-input" />
            <small class="muted">
              Stored for audit but never notified, never dispatched, and hidden from the
              inbox. The submitter still receives a normal acknowledgement.
            </small>
          </div>
        </div>
      </div>

      <div v-show="tab === 'responses'" class="card" style="margin-bottom: 20px">
        <div class="card-header"><h2>Responses</h2></div>
        <div class="card-body">
          <div class="form-group">
            <label class="form-label">Confirmation message</label>
            <textarea v-model="form.confirmation_message" class="form-input" rows="2" maxlength="500" placeholder="Thanks — your message has been received."></textarea>
            <small class="muted">Shown after someone submits the form.</small>
          </div>
          <div class="form-group">
            <label class="checkbox-label">
              <input v-model="form.allow_another_response" type="checkbox" />
              Show a link to submit another response
            </label>
          </div>
          <div class="form-group">
            <label class="form-label">Stop accepting responses after</label>
            <input v-model="closeAtLocal" class="form-input" type="datetime-local" />
            <small class="muted">Your local time. Leave empty to keep the form open.</small>
          </div>
          <div class="form-group">
            <label class="form-label">Response limit</label>
            <input v-model.number="form.max_responses" class="form-input" type="number" min="0" />
            <small class="muted">0 means no limit. Spam does not count.</small>
          </div>
          <div class="form-group">
            <label class="form-label">Closed message</label>
            <textarea v-model="form.closed_message" class="form-input" rows="2" maxlength="500" placeholder="This form is no longer accepting responses."></textarea>
            <small class="muted">
              Shown instead of the form once it closes, or while its status is Paused.
            </small>
          </div>

          <h3 style="margin: 24px 0 12px">Appearance</h3>
          <p class="muted" style="margin: 0 0 12px">
            Used by the form's share page and as the widget's default. Attributes on an embed take precedence.
          </p>
          <div class="form-group">
            <label class="form-label">Accent colour</label>
            <div style="display: flex; gap: 8px; align-items: center">
              <input
                type="color"
                :value="appearance.accent || '#9333ea'"
                aria-label="Accent colour"
                @input="setAppearanceAccent"
              />
              <button v-if="appearance.accent" class="btn btn-secondary" @click="appearance.accent = ''">Reset</button>
            </div>
          </div>
          <div class="form-group">
            <label class="form-label">Theme</label>
            <select v-model="appearance.theme" class="form-select">
              <option value="">Light (default)</option>
              <option value="dark">Dark</option>
              <option value="auto">Follow the visitor's system</option>
            </select>
          </div>
          <div class="form-group">
            <label class="form-label">Header image</label>
            <input v-model="appearance.header_image_url" class="form-input" placeholder="https://example.com/banner.png" />
            <small class="muted">An https image shown at the top of the share page.</small>
          </div>
        </div>
      </div>

      <div v-show="tab === 'notifications'" class="card" style="margin-bottom: 20px">
        <div class="card-header"><h2>Notifications</h2></div>
        <div class="card-body">
          <div class="form-group">
            <label class="checkbox-label">
              <input v-model="form.notify_enabled" type="checkbox" />
              Notify on new messages
            </label>
          </div>
          <div class="form-group">
            <label class="form-label">Recipients</label>
            <input v-model="notifyText" class="form-input" placeholder="team@example.com" />
            <small class="muted">Blank notifies workspace owners and admins.</small>
          </div>
          <div class="form-group">
            <label class="form-label">Delivery</label>
            <select v-model="form.notify_mode" class="form-select">
              <option value="immediate">Immediate</option>
              <option value="hourly">Hourly digest</option>
              <option value="daily">Daily digest</option>
              <option value="off">Off</option>
            </select>
          </div>
          <div class="form-group">
            <label class="checkbox-label">
              <input v-model="form.notify_on_flagged" type="checkbox" />
              Also notify for flagged messages
            </label>
          </div>
        </div>
      </div>

      <div v-show="tab === 'embed'" class="card" style="margin-bottom: 20px">
        <div class="card-header">
          <h2>Embed</h2>
          <button class="btn btn-secondary" @click="rotateKey">Rotate key</button>
        </div>
        <div class="card-body">
          <h3 style="margin: 0 0 4px">Share link</h3>
          <p class="muted" style="margin: 0 0 12px">
            A page Posta hosts for this form. Send it to people, or post it anywhere; no website needed.
          </p>
          <div class="form-group">
            <div style="display: flex; gap: 8px">
              <input class="form-input" :value="snippet?.share_url || ''" readonly />
              <button class="btn btn-secondary" @click="copy(snippet?.share_url || '')">Copy</button>
              <a class="btn btn-secondary" :href="snippet?.share_url" target="_blank" rel="noopener">Open</a>
            </div>
            <small class="muted">
              Add <code>?key=value</code> to prefill a question, for example <code>?plan=pro</code>.
              Questions marked as personal data are never prefilled.
            </small>
          </div>

          <hr style="margin: 24px 0; border: 0; border-top: 1px solid var(--border-primary)" />
          <h3 style="margin: 0 0 4px">Website widget <span class="badge badge-success">Recommended</span></h3>
          <p class="muted" style="margin: 0 0 16px">
            Paste two lines into any page. Posta renders a styled form, handles spam protection,
            and shows the result in place. Settings you change here later apply without editing your site.
          </p>
          <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 0 16px">
            <div class="form-group">
              <label class="form-label">Fields</label>
              <label v-for="f in optionalWidgetFields" :key="f" class="checkbox-label">
                <input v-model="widget.fields" type="checkbox" :value="f" />
                {{ f }}
              </label>
              <small class="muted">Email and message are always included.</small>
            </div>
            <div class="form-group">
              <label class="form-label">Title</label>
              <input v-model="widget.title" class="form-input" placeholder="Contact us" />
            </div>
            <div class="form-group">
              <label class="form-label">Description</label>
              <input v-model="widget.description" class="form-input" placeholder="We reply within a day." />
            </div>
            <div class="form-group">
              <label class="form-label">Button text</label>
              <input v-model="widget.button" class="form-input" placeholder="Send message" />
            </div>
            <div class="form-group">
              <label class="form-label">Success message</label>
              <input v-model="widget.success" class="form-input" placeholder="Thanks — your message has been received." />
            </div>
            <div class="form-group">
              <label class="form-label">Theme</label>
              <select v-model="widget.theme" class="form-select">
                <option value="light">Light</option>
                <option value="dark">Dark</option>
                <option value="auto">Follow the visitor's system</option>
              </select>
            </div>
            <div class="form-group">
              <label class="form-label">Accent colour</label>
              <div style="display: flex; gap: 8px; align-items: center">
                <input
                  type="color"
                  :value="widget.accent || '#9333ea'"
                  aria-label="Accent colour"
                  @input="setAccent"
                />
                <button v-if="widget.accent" class="btn btn-secondary" @click="widget.accent = ''">Reset</button>
              </div>
            </div>
            <div class="form-group">
              <label class="form-label">Language</label>
              <select v-model="widget.lang" class="form-select">
                <option value="">Visitor's browser</option>
                <option value="en">English</option>
                <option value="fr">French</option>
              </select>
            </div>
          </div>
          <div class="form-group">
            <label class="form-label">Embed code</label>
            <pre style="overflow-x: auto; padding: 12px; border-radius: 8px; background: var(--bg-secondary)">{{ widgetSnippet }}</pre>
            <button class="btn btn-primary" @click="copy(widgetSnippet)">Copy widget code</button>
          </div>
          <p v-if="!form.allowed_origins?.length" class="muted">
            Any website can show this form. To restrict it, list your site's origin
            (for example <code>https://example.com</code>) under Allowed origins on the Setup tab.
          </p>
          <p v-else class="muted">
            The widget only works on: <code>{{ form.allowed_origins.join(", ") }}</code>.
            Other sites see "This form is not available on this website."
          </p>

          <hr style="margin: 24px 0; border: 0; border-top: 1px solid var(--border-primary)" />
          <h3 style="margin: 0 0 12px">Build your own</h3>
          <div class="form-group">
            <label class="form-label">Endpoint</label>
            <div style="display: flex; gap: 8px">
              <input class="form-input" :value="endpoint" readonly />
              <button class="btn btn-secondary" @click="copy(endpoint)">Copy</button>
            </div>
          </div>
          <div class="form-group">
            <label class="form-label">HTML form</label>
            <pre style="overflow-x: auto; padding: 12px; border-radius: 8px; background: var(--bg-secondary)">{{ snippet?.html }}</pre>
            <button class="btn btn-secondary" @click="copy(snippet?.html || '')">Copy HTML</button>
          </div>
          <div class="form-group">
            <label class="form-label">fetch()</label>
            <pre style="overflow-x: auto; padding: 12px; border-radius: 8px; background: var(--bg-secondary)">{{ snippet?.fetch }}</pre>
            <button class="btn btn-secondary" @click="copy(snippet?.fetch || '')">Copy JS</button>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="card-body">
          <button class="btn btn-primary" :disabled="saving" @click="save">
            {{ saving ? "Saving..." : "Save changes" }}
          </button>
        </div>
      </div>
    </template>
  </div>
</template>
