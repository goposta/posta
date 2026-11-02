<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { onBeforeRouteLeave } from "vue-router";
import { formsApi } from "../../api/messages";
import { useNotificationStore } from "../../stores/notification";
import { useLicenseStore } from "../../stores/license";
import { apiMessage } from "../../composables/apiError";
import type { Form, FormField, FormFieldOption, FormFieldRole, FormFieldType } from "../../api/types";

// The question builder. Edits stay local until Save, and the preview on the
// right is the production widget rendering the unsaved questions, so what you
// see here is what respondents get.

const props = defineProps<{ form: Form }>();
const emit = defineEmits<{ saved: [form: Form] }>();
const notify = useNotificationStore();

interface DraftOption extends FormFieldOption {
  /** The value follows the label until it is saved or edited by hand. */
  auto?: boolean;
}

interface Draft extends Omit<FormField, "options"> {
  uid: number;
  /** The key exists in the stored schema: it cannot be renamed or removed, only archived. */
  saved: boolean;
  /** The key was typed by hand, so it stops following the label. */
  keyTouched: boolean;
  options?: DraftOption[];
}

const TYPES: { value: FormFieldType; label: string }[] = [
  { value: "text", label: "Short answer" },
  { value: "textarea", label: "Paragraph" },
  { value: "select", label: "Multiple choice" },
  { value: "multiselect", label: "Checkboxes" },
  { value: "checkbox", label: "Single checkbox" },
  { value: "scale", label: "Linear scale" },
  { value: "rating", label: "Rating" },
  { value: "email", label: "Email" },
  { value: "phone", label: "Phone" },
  { value: "number", label: "Number" },
  { value: "url", label: "Website" },
  { value: "date", label: "Date" },
  { value: "time", label: "Time" },
  { value: "hidden", label: "Hidden value" },
];
const typeLabel = (t: FormFieldType) => TYPES.find((x) => x.value === t)?.label || t;

// Mirrors roleTypes in services/messages/schema.go.
const ROLES: { value: FormFieldRole; label: string; types: FormFieldType[] }[] = [
  { value: "sender_email", label: "Sender email", types: ["email"] },
  { value: "sender_name", label: "Sender name", types: ["text"] },
  { value: "sender_phone", label: "Sender phone", types: ["phone"] },
  { value: "subject", label: "Subject", types: ["text", "select"] },
  { value: "body", label: "Message body", types: ["textarea", "text"] },
];

const TEXT_TYPES: FormFieldType[] = ["text", "textarea"];
const PLACEHOLDER_TYPES: FormFieldType[] = ["text", "textarea", "email", "phone", "number", "url"];
const CHOICE_TYPES: FormFieldType[] = ["select", "multiselect"];
const isSection = (d: { type: FormFieldType }) => d.type === "section";

let uidSeq = 0;
const drafts = ref<Draft[]>([]);
const unknownFields = ref<Form["unknown_fields"]>("keep");
const baseline = ref<Record<string, FormField>>({});
const baselineJSON = ref("");
const openUid = ref<number | null>(null);
const errorKey = ref("");
const saving = ref(false);
const showArchived = ref(false);

function toDraft(f: FormField): Draft {
  return {
    ...f,
    uid: ++uidSeq,
    saved: true,
    keyTouched: true,
    options: f.options?.map((o) => ({ ...o })),
  };
}

function reset(form: Form) {
  drafts.value = (form.fields || []).map(toDraft);
  unknownFields.value = form.unknown_fields || "keep";
  const byKey: Record<string, FormField> = {};
  (form.fields || []).forEach((f) => (byKey[f.key] = f));
  baseline.value = byKey;
  baselineJSON.value = JSON.stringify({ fields: serialize(), unknown: unknownFields.value });
  errorKey.value = "";
}

const active = computed(() => drafts.value.filter((d) => !d.archived));
const archived = computed(() => drafts.value.filter((d) => d.archived));
const questionCount = computed(() => active.value.filter((d) => !isSection(d)).length);

// ─── Sections and branching ───────────────────────────────────────────────

// Where a page may lead: a later section, or submit. Mirrors checkFlow in
// services/messages/schema.go, which only allows moving forward.
function laterSections(d: Draft) {
  const list = active.value;
  return list.slice(list.indexOf(d) + 1).filter(isSection);
}

// pageNumber is the page a card's question is on, counting from 1.
function pageNumber(d: Draft) {
  const list = active.value;
  const before = list.slice(0, list.indexOf(d) + 1);
  const opensWithSection = list.length > 0 && isSection(list[0]);
  return before.filter(isSection).length + (opensWithSection ? 0 : 1);
}

// Branching is a Posta Enterprise feature. Without a license (or once it has
// expired past grace) new branches cannot be added; existing ones keep working
// and can still be removed.
const license = useLicenseStore();
license.load();
const canBranch = computed(() => license.mutable);

const branchUids = ref(new Set<number>());
function branches(d: Draft) {
  return branchUids.value.has(d.uid) || (d.options || []).some((o) => o.goto);
}
function toggleBranch(d: Draft, on: boolean) {
  const next = new Set(branchUids.value);
  if (on) next.add(d.uid);
  else {
    next.delete(d.uid);
    (d.options || []).forEach((o) => (o.goto = undefined));
  }
  branchUids.value = next;
}

// A target that is no longer ahead (after a reorder or archive) would be
// refused on save; say so on the card instead.
function staleTarget(d: Draft, target?: string) {
  if (!target || target === "submit") return false;
  return !laterSections(d).some((s) => s.key === target);
}
function hasStaleTargets(d: Draft) {
  return isSection(d) ? staleTarget(d, d.next) : (d.options || []).some((o) => staleTarget(d, o.goto));
}

function sectionTitle(key?: string) {
  const s = drafts.value.find((d) => d.key === key);
  return s ? s.label || s.key : key;
}

// ─── Keys and option values ───────────────────────────────────────────────

function slug(text: string, max = 64) {
  const s = text
    .toLowerCase()
    .normalize("NFKD")
    .replace(/[̀-ͯ]/g, "")
    .replace(/[^a-z0-9]+/g, "_")
    .replace(/^_+|_+$/g, "")
    .slice(0, max)
    .replace(/_+$/, "");
  return s;
}

function uniqueKey(base: string, self?: Draft) {
  const root = slug(base) || "question";
  const taken = new Set(drafts.value.filter((d) => d !== self).map((d) => d.key));
  if (root !== props.form.honeypot_field && !taken.has(root)) return root;
  for (let n = 2; ; n++) {
    const candidate = `${root.slice(0, 60)}_${n}`;
    if (!taken.has(candidate)) return candidate;
  }
}

function onLabelInput(d: Draft) {
  if (!d.saved && !d.keyTouched) d.key = uniqueKey(d.label, d);
}

function onKeyInput(d: Draft) {
  d.keyTouched = true;
  d.key = d.key.toLowerCase().replace(/[^a-z0-9_]/g, "_").slice(0, 64);
}

function optionValue(d: Draft, o: DraftOption) {
  const root = slug(o.label || "", 64) || "option";
  const taken = new Set((d.options || []).filter((x) => x !== o).map((x) => x.value));
  if (!taken.has(root)) return root;
  for (let n = 2; ; n++) if (!taken.has(`${root}_${n}`)) return `${root}_${n}`;
}

function onOptionLabel(d: Draft, o: DraftOption) {
  if (o.auto) o.value = optionValue(d, o);
}

function addOption(d: Draft) {
  const o: DraftOption = { label: `Option ${(d.options?.length || 0) + 1}`, value: "", auto: true };
  d.options = [...(d.options || []), o];
  o.value = optionValue(d, o);
}

function removeOption(d: Draft, i: number) {
  d.options = (d.options || []).filter((_, j) => j !== i);
}

// ─── Questions ────────────────────────────────────────────────────────────

function applyTypeDefaults(d: Draft) {
  if (CHOICE_TYPES.includes(d.type) && !d.options?.length) {
    d.options = [];
    addOption(d);
    addOption(d);
  }
  if (!CHOICE_TYPES.includes(d.type)) d.display = "";
  if (d.type === "scale") {
    d.min = d.min === 0 ? 0 : 1;
    d.max = d.max && d.max >= 2 && d.max <= 10 ? d.max : 5;
  }
  if (d.type === "rating") d.max = d.max && d.max >= 3 && d.max <= 10 ? d.max : 5;
  if (d.role && !ROLES.find((r) => r.value === d.role)?.types.includes(d.type)) d.role = "";
  if (!d.saved && (d.type === "email" || d.type === "phone")) d.pii = true;
}

function newDraft(type: FormFieldType, label: string, extra: Partial<Draft> = {}): Draft {
  const d: Draft = { uid: ++uidSeq, saved: false, keyTouched: false, key: "", label, type, ...extra };
  d.key = uniqueKey(extra.key || label, d);
  applyTypeDefaults(d);
  return d;
}

async function addQuestion(type: FormFieldType = "text") {
  const d = newDraft(type, type === "section" ? "Untitled section" : "Untitled question");
  const at = openUid.value ? drafts.value.findIndex((x) => x.uid === openUid.value) + 1 : drafts.value.length;
  drafts.value.splice(at || drafts.value.length, 0, d);
  openUid.value = d.uid;
  await nextTick();
  (document.getElementById(`q-label-${d.uid}`) as HTMLInputElement | null)?.select();
}

function addContactQuestions() {
  drafts.value.push(
    newDraft("text", "Name", { key: "name", role: "sender_name", pii: true }),
    newDraft("email", "Email", { key: "email", role: "sender_email", required: true }),
    newDraft("textarea", "Message", { key: "message", role: "body", required: true }),
  );
}

function duplicate(d: Draft) {
  const { uid: _uid, saved: _saved, keyTouched: _touched, key: _key, ...settings } = JSON.parse(JSON.stringify(d)) as Draft;
  const copy = newDraft(d.type, `${d.label} (copy)`, {
    ...settings,
    label: `${d.label} (copy)`,
    role: "", // a role belongs to one field
    archived: false,
    options: d.options?.map((o) => ({ ...o, auto: false })),
  });
  drafts.value.splice(drafts.value.indexOf(d) + 1, 0, copy);
  openUid.value = copy.uid;
}

function remove(d: Draft) {
  if (isSection(d)) {
    // Nothing may lead to a section that is gone.
    drafts.value.forEach((x) => {
      if (x.next === d.key) x.next = undefined;
      (x.options || []).forEach((o) => {
        if (o.goto === d.key) o.goto = undefined;
      });
    });
  }
  if (d.saved) {
    // Stored answers refer to the key, so it is kept, hidden from respondents.
    d.archived = true;
    d.role = "";
  } else {
    drafts.value = drafts.value.filter((x) => x !== d);
  }
  if (openUid.value === d.uid) openUid.value = null;
}

function restore(d: Draft) {
  // Back at the end of the form, where a question that reappears is easiest to spot.
  drafts.value = [...drafts.value.filter((x) => x !== d), { ...d, archived: false }];
}

function move(d: Draft, step: -1 | 1) {
  const list = active.value;
  const i = list.indexOf(d);
  const other = list[i + step];
  if (!other) return;
  const a = drafts.value.indexOf(d);
  const b = drafts.value.indexOf(other);
  drafts.value.splice(a, 1, other);
  drafts.value.splice(b, 1, d);
}

// Drag and drop reorders by the card's handle; the arrow buttons do the same
// from the keyboard.
const dragUid = ref<number | null>(null);
function onDrop(target: Draft) {
  const from = drafts.value.findIndex((x) => x.uid === dragUid.value);
  dragUid.value = null;
  if (from < 0) return;
  const [item] = drafts.value.splice(from, 1);
  drafts.value.splice(drafts.value.indexOf(target), 0, item);
}

function rolesFor(d: Draft) {
  return ROLES.filter((r) => r.types.includes(d.type));
}

function requiredNewly(d: Draft) {
  return d.saved && d.required && !baseline.value[d.key]?.required;
}

function typeChanged(d: Draft) {
  return d.saved && baseline.value[d.key] && baseline.value[d.key].type !== d.type;
}

// ─── Save ─────────────────────────────────────────────────────────────────

function num(v: unknown): number | undefined {
  if (v === "" || v === null || v === undefined) return undefined;
  const n = Number(v);
  return Number.isFinite(n) ? n : undefined;
}

function serialize(): FormField[] {
  return drafts.value.map((d) => {
    const f: FormField = { key: d.key.trim(), label: d.label.trim(), type: d.type };
    if (isSection(d)) {
      if (d.help?.trim()) f.help = d.help.trim();
      if (d.next) f.next = d.next;
      if (d.archived) f.archived = true;
      return f;
    }
    if (d.required && d.type !== "hidden") f.required = true;
    if (d.role) f.role = d.role;
    if (d.pii) f.pii = true;
    if (d.help?.trim()) f.help = d.help.trim();
    if (d.archived) f.archived = true;
    if (PLACEHOLDER_TYPES.includes(d.type) && d.placeholder?.trim()) f.placeholder = d.placeholder.trim();
    if (TEXT_TYPES.includes(d.type)) {
      f.min_len = num(d.min_len);
      f.max_len = num(d.max_len);
      if (d.type === "text" && d.pattern?.trim()) f.pattern = d.pattern.trim();
    }
    if (d.type === "number" || d.type === "scale") {
      f.min = num(d.min);
      f.max = num(d.max);
    }
    if (d.type === "rating") f.max = num(d.max);
    if (d.type === "scale") {
      if (d.min_label?.trim()) f.min_label = d.min_label.trim();
      if (d.max_label?.trim()) f.max_label = d.max_label.trim();
    }
    if (CHOICE_TYPES.includes(d.type)) {
      f.options = (d.options || []).map((o) => {
        const opt: FormFieldOption = { value: o.value.trim() };
        if (o.label?.trim()) opt.label = o.label.trim();
        if (d.type === "select" && o.goto) opt.goto = o.goto;
        return opt;
      });
      if (d.display) f.display = d.display;
    }
    return JSON.parse(JSON.stringify(f)); // drops undefined
  });
}

const dirty = computed(() => JSON.stringify({ fields: serialize(), unknown: unknownFields.value }) !== baselineJSON.value);

async function save() {
  saving.value = true;
  errorKey.value = "";
  try {
    const res = await formsApi.update(props.form.id, { fields: serialize(), unknown_fields: unknownFields.value });
    reset(res.data.data);
    emit("saved", res.data.data);
    notify.success("Questions saved.");
  } catch (e: unknown) {
    const message = apiMessage(e, "Failed to save questions");
    // Schema errors name the field: open that card so the problem is in view.
    const key = /(?:field|section)(?: key)? "([^"]+)"/.exec(message)?.[1];
    const target = key && drafts.value.find((d) => d.key === key);
    if (target) {
      errorKey.value = key!;
      if (target.archived) showArchived.value = true;
      else openUid.value = target.uid;
    }
    notify.error(message);
  } finally {
    saving.value = false;
  }
}

function discard() {
  if (!confirm("Discard unsaved changes to the questions?")) return;
  reset(props.form);
  openUid.value = null;
}

onBeforeRouteLeave(() => {
  if (dirty.value && !confirm("Leave without saving your question changes?")) return false;
});

function beforeUnload(e: BeforeUnloadEvent) {
  if (dirty.value) e.preventDefault();
}

// The parent saves other tabs without touching fields; resync only when the
// stored schema moved and nothing here is unsaved.
watch(
  () => props.form.schema_version,
  () => {
    if (!dirty.value) reset(props.form);
  },
);

// ─── Live preview ─────────────────────────────────────────────────────────

declare global {
  interface Window {
    PostaForms?: { preview: (el: HTMLElement, cfg: unknown) => void };
  }
}

const previewEl = ref<HTMLElement | null>(null);
const previewState = ref<"loading" | "ready" | "failed">("loading");
let widgetLoad: Promise<void> | null = null;

function loadWidget(): Promise<void> {
  if (window.PostaForms?.preview) return Promise.resolve();
  widgetLoad ||= new Promise((resolve, reject) => {
    const s = document.createElement("script");
    s.src = "/widget/v1/posta-form.js";
    s.async = true;
    s.onload = () => (window.PostaForms?.preview ? resolve() : reject(new Error("old widget")));
    s.onerror = () => reject(new Error("widget failed to load"));
    document.head.appendChild(s);
  });
  return widgetLoad;
}

let previewTimer: ReturnType<typeof setTimeout> | undefined;
function renderPreview() {
  clearTimeout(previewTimer);
  previewTimer = setTimeout(() => {
    if (previewState.value !== "ready" || !previewEl.value) return;
    const fields = serialize().filter((f) => !f.archived);
    window.PostaForms!.preview(previewEl.value, {
      fields: fields.length ? fields : undefined,
      appearance: props.form.appearance,
      confirmation_message: props.form.confirmation_message,
      allow_another_response: props.form.allow_another_response,
      honeypot_field: props.form.honeypot_field,
    });
  }, 120);
}

watch(
  () => [drafts.value, props.form.name, props.form.description, props.form.appearance],
  renderPreview,
  { deep: true },
);

onMounted(async () => {
  reset(props.form);
  window.addEventListener("beforeunload", beforeUnload);
  try {
    await loadWidget();
    previewState.value = "ready";
    renderPreview();
  } catch {
    previewState.value = "failed";
  }
});

onBeforeUnmount(() => {
  clearTimeout(previewTimer);
  window.removeEventListener("beforeunload", beforeUnload);
});
</script>

<template>
  <div class="qb">
    <div class="qb-editor">
      <div class="card qb-bar">
        <div class="card-body qb-bar-body">
          <div>
            <strong>{{ questionCount }} {{ questionCount === 1 ? "question" : "questions" }}</strong>
            <span v-if="form.schema_version" class="muted"> · version {{ form.schema_version }}</span>
            <span v-if="dirty" class="badge badge-warning" style="margin-left: 8px">Unsaved</span>
          </div>
          <div class="qb-bar-actions">
            <button class="btn btn-secondary btn-sm" :disabled="!dirty || saving" @click="discard">Discard</button>
            <button class="btn btn-primary btn-sm" :disabled="!dirty || saving" @click="save">
              {{ saving ? "Saving…" : "Save questions" }}
            </button>
          </div>
        </div>
      </div>

      <div v-if="!drafts.length" class="card">
        <div class="empty-state">
          <h3>No questions yet</h3>
          <p>
            Without questions this form accepts any fields it is sent, and the widget shows name, email and message.
            Add questions to validate answers and store them as typed data.
          </p>
          <div style="display: flex; gap: 8px; justify-content: center; flex-wrap: wrap">
            <button class="btn btn-primary" @click="addQuestion()">Add a question</button>
            <button class="btn btn-secondary" @click="addContactQuestions">Start with name, email and message</button>
          </div>
        </div>
      </div>

      <div
        v-for="(d, i) in active"
        :key="d.uid"
        class="card qcard"
        :class="{ open: openUid === d.uid, error: errorKey === d.key, dragging: dragUid === d.uid, section: isSection(d) }"
        @dragover.prevent
        @drop="onDrop(d)"
      >
        <div class="qcard-head" @click="openUid = openUid === d.uid ? null : d.uid">
          <span
            class="qcard-handle"
            draggable="true"
            title="Drag to reorder"
            aria-hidden="true"
            @click.stop
            @dragstart="dragUid = d.uid"
            @dragend="dragUid = null"
          >⋮⋮</span>
          <div class="qcard-summary">
            <div class="qcard-title">
              {{ d.label || (d.type === "hidden" ? d.key : isSection(d) ? "Untitled section" : "Untitled question") }}
              <span v-if="d.required" class="qcard-req" title="Required">*</span>
            </div>
            <div v-if="isSection(d)" class="qcard-meta">
              Page {{ pageNumber(d) }}
              <template v-if="d.next === 'submit'"> · then submit</template>
              <template v-else-if="d.next"> · then “{{ sectionTitle(d.next) }}”</template>
              <span v-if="hasStaleTargets(d)" class="badge badge-warning">Check where it leads</span>
            </div>
            <div v-else class="qcard-meta">
              {{ typeLabel(d.type) }} · <code>{{ d.key }}</code>
              <span v-if="branches(d)" class="badge badge-info">Branches</span>
              <span v-if="hasStaleTargets(d)" class="badge badge-warning">Check where it leads</span>
              <span v-if="d.role" class="badge badge-info">{{ ROLES.find((r) => r.value === d.role)?.label }}</span>
              <span v-if="d.pii" class="badge badge-neutral">Personal data</span>
            </div>
          </div>
          <div class="qcard-tools" @click.stop>
            <button class="btn btn-ghost btn-xs" :disabled="i === 0" title="Move up" aria-label="Move up" @click="move(d, -1)">↑</button>
            <button class="btn btn-ghost btn-xs" :disabled="i === active.length - 1" title="Move down" aria-label="Move down" @click="move(d, 1)">↓</button>
            <button class="btn btn-ghost btn-xs" title="Duplicate" @click="duplicate(d)">Duplicate</button>
            <button class="btn btn-ghost btn-xs" :title="d.saved ? 'Archive: hidden from respondents, answers kept' : 'Delete'" @click="remove(d)">
              {{ d.saved ? "Archive" : "Delete" }}
            </button>
          </div>
        </div>

        <div v-if="openUid === d.uid && isSection(d)" class="qcard-body">
          <div class="form-group">
            <label class="form-label" :for="`q-label-${d.uid}`">Section title</label>
            <input :id="`q-label-${d.uid}`" v-model="d.label" class="form-input" @input="onLabelInput(d)" />
          </div>
          <div class="form-group">
            <label class="form-label">Description <span class="muted">(optional)</span></label>
            <textarea v-model="d.help" class="form-input" rows="2" placeholder="Shown at the top of the page"></textarea>
          </div>
          <div class="form-group">
            <label class="form-label">After this section</label>
            <select v-model="d.next" class="form-select" style="max-width: 360px" :disabled="!canBranch && !d.next">
              <option :value="undefined">Continue to the next section</option>
              <option v-for="s in laterSections(d)" :key="s.uid" :value="s.key">Go to “{{ s.label || s.key }}”</option>
              <option value="submit">Submit the form</option>
            </select>
            <small v-if="staleTarget(d, d.next)" class="form-hint warn">
              “{{ sectionTitle(d.next) }}” is no longer after this section. Pick where this page leads again.
            </small>
            <small v-else-if="!canBranch" class="form-hint">
              <span class="ee-badge">Enterprise</span> Sending respondents to another section needs a Posta Enterprise license.
            </small>
            <small v-else class="form-hint">A branching question on this page takes precedence for the answers it covers.</small>
          </div>
          <div class="qcard-foot">
            <div class="qcard-key">
              <label class="form-label" style="margin: 0">Key</label>
              <input
                v-model="d.key"
                class="form-input"
                :disabled="d.saved"
                title="Branches point to sections by this key"
                spellcheck="false"
                @input="onKeyInput(d)"
              />
            </div>
          </div>
        </div>

        <div v-if="openUid === d.uid && !isSection(d)" class="qcard-body">
          <div class="qgrid">
            <div class="form-group">
              <label class="form-label" :for="`q-label-${d.uid}`">Question</label>
              <input :id="`q-label-${d.uid}`" v-model="d.label" class="form-input" @input="onLabelInput(d)" />
            </div>
            <div class="form-group">
              <label class="form-label">Type</label>
              <select v-model="d.type" class="form-select" @change="applyTypeDefaults(d)">
                <option v-for="t in TYPES" :key="t.value" :value="t.value">{{ t.label }}</option>
              </select>
              <small v-if="typeChanged(d)" class="form-hint warn">Answers already stored keep their old type.</small>
            </div>
          </div>

          <div class="form-group">
            <label class="form-label">Description <span class="muted">(optional)</span></label>
            <input v-model="d.help" class="form-input" placeholder="Shown under the question" />
          </div>

          <!-- Choices -->
          <div v-if="CHOICE_TYPES.includes(d.type)" class="form-group">
            <label class="form-label">Options</label>
            <div v-for="(o, oi) in d.options" :key="oi" class="qopt">
              <span class="qopt-mark" :class="d.type === 'multiselect' ? 'box' : 'dot'" aria-hidden="true"></span>
              <input v-model="o.label" class="form-input" :placeholder="`Option ${oi + 1}`" @input="onOptionLabel(d, o)" />
              <input
                v-model="o.value"
                class="form-input qopt-value"
                title="The value stored and matched by rules"
                @input="o.auto = false"
              />
              <button class="btn btn-ghost btn-xs" :disabled="(d.options?.length || 0) <= 1" aria-label="Remove option" @click="removeOption(d, oi)">×</button>
              <select
                v-if="d.type === 'select' && branches(d)"
                v-model="o.goto"
                class="form-select qopt-goto"
                :class="{ stale: staleTarget(d, o.goto) }"
                :aria-label="`After choosing ${o.label || o.value}`"
              >
                <option :value="undefined">Continue</option>
                <option v-for="s in laterSections(d)" :key="s.uid" :value="s.key">Go to “{{ s.label || s.key }}”</option>
                <option value="submit">Submit the form</option>
              </select>
            </div>
            <label
              v-if="d.type === 'select' && (laterSections(d).length || branches(d))"
              class="checkbox-label"
              style="margin-top: 8px"
            >
              <input
                type="checkbox"
                :checked="branches(d)"
                :disabled="!canBranch && !branches(d)"
                @change="toggleBranch(d, ($event.target as HTMLInputElement).checked)"
              />
              Go to a section based on the answer
              <span v-if="!canBranch" class="ee-badge" title="Branching needs a Posta Enterprise license">Enterprise</span>
            </label>
            <small v-if="d.type === 'select' && !laterSections(d).length && !branches(d)" class="form-hint">
              Add a section below this question to send respondents to different pages by their answer.
            </small>
            <button class="btn btn-secondary btn-xs" style="margin-top: 6px" @click="addOption(d)">Add option</button>
            <small class="form-hint">The second box is the stored value. Keep it stable once answers come in.</small>
          </div>
          <div v-if="CHOICE_TYPES.includes(d.type)" class="form-group">
            <label class="form-label">Show as</label>
            <select v-model="d.display" class="form-select">
              <option value="">Automatic</option>
              <template v-if="d.type === 'select'">
                <option value="radio">Radio buttons</option>
                <option value="dropdown">Dropdown</option>
              </template>
              <template v-else>
                <option value="checkboxes">Checkboxes</option>
                <option value="list">List box</option>
              </template>
            </select>
          </div>

          <!-- Scale and rating -->
          <div v-if="d.type === 'scale'" class="qgrid">
            <div class="form-group">
              <label class="form-label">From</label>
              <select v-model.number="d.min" class="form-select">
                <option :value="0">0</option>
                <option :value="1">1</option>
              </select>
            </div>
            <div class="form-group">
              <label class="form-label">To</label>
              <select v-model.number="d.max" class="form-select">
                <option v-for="n in 9" :key="n" :value="n + 1">{{ n + 1 }}</option>
              </select>
            </div>
            <div class="form-group">
              <label class="form-label">Low label <span class="muted">(optional)</span></label>
              <input v-model="d.min_label" class="form-input" placeholder="Not likely" />
            </div>
            <div class="form-group">
              <label class="form-label">High label <span class="muted">(optional)</span></label>
              <input v-model="d.max_label" class="form-input" placeholder="Very likely" />
            </div>
          </div>
          <div v-if="d.type === 'rating'" class="form-group">
            <label class="form-label">Stars</label>
            <select v-model.number="d.max" class="form-select" style="max-width: 120px">
              <option v-for="n in 8" :key="n" :value="n + 2">{{ n + 2 }}</option>
            </select>
          </div>

          <!-- Limits -->
          <div v-if="PLACEHOLDER_TYPES.includes(d.type)" class="form-group">
            <label class="form-label">Placeholder <span class="muted">(optional)</span></label>
            <input v-model="d.placeholder" class="form-input" />
          </div>
          <div v-if="TEXT_TYPES.includes(d.type)" class="qgrid">
            <div class="form-group">
              <label class="form-label">Min length</label>
              <input v-model="d.min_len" type="number" min="0" class="form-input" />
            </div>
            <div class="form-group">
              <label class="form-label">Max length</label>
              <input v-model="d.max_len" type="number" min="1" max="8000" class="form-input" :placeholder="d.type === 'textarea' ? '8000' : '200'" />
            </div>
          </div>
          <div v-if="d.type === 'number'" class="qgrid">
            <div class="form-group">
              <label class="form-label">Minimum</label>
              <input v-model="d.min" type="number" step="any" class="form-input" />
            </div>
            <div class="form-group">
              <label class="form-label">Maximum</label>
              <input v-model="d.max" type="number" step="any" class="form-input" />
            </div>
          </div>
          <div v-if="d.type === 'text'" class="form-group">
            <label class="form-label">Pattern <span class="muted">(optional)</span></label>
            <input v-model="d.pattern" class="form-input" placeholder="[A-Z]{2}[0-9]{4}" spellcheck="false" />
            <small class="form-hint">A regular expression the whole answer must match. Checked by the server.</small>
          </div>
          <p v-if="d.type === 'hidden'" class="muted" style="margin: 0 0 12px; font-size: 13px">
            Not shown to respondents. Filled from a prefilled link (<code>?{{ d.key }}=value</code>) or an embed's
            <code>data-value-{{ d.key }}</code> attribute.
          </p>

          <div class="qcard-foot">
            <label v-if="d.type !== 'hidden'" class="checkbox-label">
              <input v-model="d.required" type="checkbox" /> Required
            </label>
            <label
              class="checkbox-label"
              :title="`Hidden in notification emails and never prefilled from links${form.pii_encrypted ? '; encrypted at rest, so not filterable or searchable' : ''}`"
            >
              <input v-model="d.pii" type="checkbox" /> Personal data
            </label>
            <div v-if="rolesFor(d).length" class="qcard-role">
              <label class="form-label" style="margin: 0">Use as</label>
              <select v-model="d.role" class="form-select">
                <option value="">Nothing</option>
                <option v-for="r in rolesFor(d)" :key="r.value" :value="r.value">{{ r.label }}</option>
              </select>
            </div>
            <div class="qcard-key">
              <label class="form-label" style="margin: 0">Key</label>
              <input
                v-model="d.key"
                class="form-input"
                :disabled="d.saved"
                :title="d.saved ? 'Saved keys cannot change: stored answers refer to them' : 'The name answers are stored under'"
                spellcheck="false"
                @input="onKeyInput(d)"
              />
            </div>
          </div>
          <small v-if="requiredNewly(d)" class="form-hint warn">
            Embeds or integrations that don't send <code>{{ d.key }}</code> will now be rejected.
          </small>
        </div>
      </div>

      <div v-if="drafts.length" class="qb-add">
        <button class="btn btn-secondary" @click="addQuestion()">+ Add question</button>
        <button class="btn btn-secondary" @click="addQuestion('section')">+ Add section</button>
      </div>

      <div v-if="archived.length" class="card">
        <div class="card-body">
          <button class="btn btn-ghost btn-sm" @click="showArchived = !showArchived">
            {{ showArchived ? "▾" : "▸" }} Archived questions ({{ archived.length }})
          </button>
          <div v-if="showArchived" style="margin-top: 8px">
            <p class="muted" style="font-size: 13px; margin: 0 0 8px">
              Hidden from respondents. Their stored answers are kept, so their keys stay reserved.
            </p>
            <div v-for="d in archived" :key="d.uid" class="qarch" :class="{ error: errorKey === d.key }">
              <span>{{ d.label || d.key }} <code class="muted">{{ d.key }}</code></span>
              <button class="btn btn-secondary btn-xs" @click="restore(d)">Restore</button>
            </div>
          </div>
        </div>
      </div>

      <div v-if="drafts.length" class="card">
        <div class="card-body">
          <div class="form-group" style="margin: 0">
            <label class="form-label">Fields the form doesn't ask for</label>
            <select v-model="unknownFields" class="form-select" style="max-width: 360px">
              <option value="keep">Keep them with the submission</option>
              <option value="drop">Drop them silently</option>
              <option value="reject">Reject the submission</option>
            </select>
            <small class="form-hint">
              Applies to submissions from your own HTML or the API that send extra fields. Hidden values count as asked for.
            </small>
          </div>
        </div>
      </div>
    </div>

    <aside class="qb-preview">
      <div class="qb-preview-head">
        <span>Preview</span>
        <small class="muted">Unsaved changes included</small>
      </div>
      <p v-if="previewState === 'failed'" class="muted" style="font-size: 13px">
        The preview needs the widget script at <code>/widget/v1/posta-form.js</code>, which could not be loaded.
      </p>
      <p v-else-if="previewState === 'loading'" class="muted" style="font-size: 13px">Loading preview…</p>
      <div ref="previewEl" :data-title="form.name" :data-description="form.description || null"></div>
    </aside>
  </div>
</template>

<style scoped>
.qb {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(300px, 420px);
  gap: 20px;
  align-items: start;
  margin-bottom: 20px;
}

@media (max-width: 1100px) {
  .qb {
    grid-template-columns: minmax(0, 1fr);
  }
}

.qb-editor {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
}

.qb-editor > .card {
  margin: 0;
}

.qb-bar {
  position: sticky;
  top: 56px; /* flush under the topbar, so cards never show through a gap */
  z-index: 5;
  box-shadow: var(--shadow-sm, 0 1px 3px rgba(0, 0, 0, 0.06));
}

.qb .form-hint {
  display: block;
}

.qb-bar-body {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  padding-top: 10px;
  padding-bottom: 10px;
}

.qb-bar-actions {
  display: flex;
  gap: 8px;
}

.qcard {
  border-left: 3px solid transparent;
  transition: border-color 0.15s, box-shadow 0.15s;
}

.qcard.open {
  border-left-color: var(--primary-600);
}

/* A section starts a page: set it apart from the questions around it. */
.qcard.section {
  margin-top: 12px;
  background: var(--bg-secondary);
  border-top: 3px solid var(--primary-500);
}

.qcard.error {
  border-left-color: var(--danger, #dc2626);
}

.qcard.dragging {
  opacity: 0.5;
}

.qcard-head {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 14px;
  cursor: pointer;
}

.qcard-handle {
  cursor: grab;
  color: var(--text-muted);
  letter-spacing: -2px;
  user-select: none;
  padding: 2px;
}

.qcard-summary {
  flex: 1;
  min-width: 0;
}

.qcard-title {
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.qcard-req {
  color: var(--danger, #dc2626);
}

.qcard-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 2px;
}

.qcard-tools {
  display: flex;
  gap: 2px;
  flex-shrink: 0;
}

.qcard-body {
  padding: 4px 16px 14px 38px;
  border-top: 1px solid var(--border-primary);
  padding-top: 14px;
}

.qgrid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 0 12px;
}

.qopt {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}

.qopt .form-input {
  flex: 1;
  min-width: 0;
}

.qopt .qopt-value {
  flex: 0 1 160px;
  font-family: var(--font-mono, ui-monospace, monospace);
  font-size: 12px;
  color: var(--text-muted);
}

.qopt .qopt-goto {
  flex: 0 1 190px;
  min-width: 0;
}

.qopt .qopt-goto.stale {
  border-color: var(--warning-600);
}

.qopt-mark {
  width: 14px;
  height: 14px;
  border: 1.5px solid var(--text-muted);
  flex-shrink: 0;
}

.qopt-mark.dot {
  border-radius: 50%;
}

.qopt-mark.box {
  border-radius: 3px;
}

.qcard-foot {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
  padding-top: 12px;
  border-top: 1px dashed var(--border-primary);
}

.qcard-foot .checkbox-label {
  margin: 0;
}

.qcard-role,
.qcard-key {
  display: flex;
  align-items: center;
  gap: 8px;
}

.qcard-key {
  margin-left: auto;
}

.qcard-key .form-input {
  width: 170px;
  font-family: var(--font-mono, ui-monospace, monospace);
  font-size: 12px;
}

.form-hint.warn {
  display: block;
  color: var(--warning-text, #b45309);
  margin-top: 6px;
}

.qb-add {
  display: flex;
  justify-content: center;
  gap: 8px;
}

.qarch {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 6px 0;
  border-top: 1px solid var(--border-primary);
  font-size: 13px;
}

.qarch.error {
  color: var(--danger, #dc2626);
}

.qb-preview {
  position: sticky;
  top: 72px;
  max-height: calc(100vh - 96px);
  overflow: auto;
  padding: 16px;
  border-radius: 12px;
  background: var(--bg-primary);
  border: 1px solid var(--border-primary);
}

.qb-preview-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  margin-bottom: 12px;
  font-weight: 600;
}

@media (max-width: 1100px) {
  .qb-preview {
    position: static;
    max-height: none;
  }
}
</style>
