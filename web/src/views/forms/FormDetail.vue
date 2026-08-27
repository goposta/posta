<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { formsApi } from "../../api/messages";
import type { Form, FormSnippet } from "../../api/types";

const route = useRoute();
const router = useRouter();
const id = Number(route.params.id);

const loading = ref(true);
const saving = ref(false);
const form = ref<Form | null>(null);
const snippet = ref<FormSnippet | null>(null);
const error = ref("");
const notice = ref("");
const tab = ref<"setup" | "spam" | "notifications" | "embed">("setup");

const originsText = ref("");
const notifyText = ref("");

async function load() {
  loading.value = true;
  try {
    const [formRes, snippetRes] = await Promise.all([formsApi.get(id), formsApi.snippet(id)]);
    form.value = formRes.data.data;
    snippet.value = snippetRes.data.data;
    originsText.value = (formRes.data.data.allowed_origins || []).join(", ");
    notifyText.value = (formRes.data.data.notify_emails || []).join(", ");
  } catch (e: any) {
    error.value = e?.response?.data?.error?.message || "Failed to load form";
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
  error.value = "";
  notice.value = "";
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
    });
    form.value = res.data.data;
    notice.value = "Form saved.";
  } catch (e: any) {
    error.value = e?.response?.data?.error?.message || "Failed to save form";
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
    notice.value = "Public key rotated. Update your embed code.";
  } catch (e: any) {
    error.value = e?.response?.data?.error?.message || "Failed to rotate key";
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
    error.value = e?.response?.data?.error?.message || "Failed to delete form";
  }
}

function copy(text: string) {
  navigator.clipboard?.writeText(text);
  notice.value = "Copied to clipboard.";
}

const endpoint = computed(() => snippet.value?.endpoint || "");

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

    <div v-if="error" class="alert alert-danger">{{ error }}</div>
    <div v-if="notice" class="alert alert-success">{{ notice }}</div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <template v-else-if="form">
      <div class="card" style="margin-bottom: 20px">
        <div class="card-body" style="display: flex; gap: 8px; flex-wrap: wrap">
          <button
            v-for="t in (['setup', 'spam', 'notifications', 'embed'] as const)"
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
            <label class="form-label">Retention (days, 0 uses the workspace default)</label>
            <input v-model.number="form.retention_days" type="number" class="form-input" min="0" />
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
          <div class="form-group">
            <label class="form-label">Endpoint</label>
            <div style="display: flex; gap: 8px">
              <input class="form-input" :value="endpoint" readonly />
              <button class="btn btn-secondary" @click="copy(endpoint)">Copy</button>
            </div>
          </div>
          <div class="form-group">
            <label class="form-label">HTML form</label>
            <pre style="overflow-x: auto; padding: 12px; border-radius: 8px; background: var(--bg-subtle)">{{ snippet?.html }}</pre>
            <button class="btn btn-secondary" @click="copy(snippet?.html || '')">Copy HTML</button>
          </div>
          <div class="form-group">
            <label class="form-label">fetch()</label>
            <pre style="overflow-x: auto; padding: 12px; border-radius: 8px; background: var(--bg-subtle)">{{ snippet?.fetch }}</pre>
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
