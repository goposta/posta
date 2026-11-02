<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import { formSubmissionsApi, messagesApi } from "../../api/messages";
import { apiMessage } from "../../composables/apiError";
import { useNotificationStore } from "../../stores/notification";
import Pagination from "../../components/Pagination.vue";
import type {
  Form,
  FormField,
  Message,
  Pageable,
  SubmissionFieldFilter,
  SubmissionFieldSummary,
  SubmissionQuery,
  SubmissionSummary,
} from "../../api/types";

// A form's responses: a per-question summary and a table, over one set of
// filters, with CSV and JSON export of exactly what is shown.

const props = defineProps<{ form: Form; shareUrl?: string }>();
const notify = useNotificationStore();

const view = ref<"summary" | "table">("summary");
const loading = ref(false);
const summary = ref<SubmissionSummary | null>(null);
const rows = ref<Message[]>([]);
const pageable = ref<Pageable | null>(null);
const page = ref(0);

const status = ref("");
const search = ref("");
const after = ref("");
const before = ref("");
const fieldFilters = ref<SubmissionFieldFilter[]>([]);
const tag = ref("");
const priority = ref("");
const knownTags = ref<string[]>([]);

// Archived and hidden questions can still be filtered on: old answers keep them.
const questions = computed(() => (props.form.fields || []).filter((f) => f.type !== "section"));
const activeQuestions = computed(() =>
  (props.form.fields || []).filter((f) => !f.archived && f.type !== "hidden" && f.type !== "section"),
);
const hasSchema = computed(() => (props.form.fields || []).length > 0);
const byKey = computed(() => {
  const out: Record<string, FormField> = {};
  (props.form.fields || []).forEach((f) => (out[f.key] = f));
  return out;
});

const RANGE_TYPES = ["number", "scale", "rating", "date", "time"];

function query(): SubmissionQuery {
  return {
    status: status.value,
    q: search.value.trim(),
    tag: tag.value,
    priority: priority.value,
    // Dates are picked in local time; the day bounds are inclusive.
    after: after.value ? new Date(`${after.value}T00:00:00`).toISOString() : "",
    before: before.value ? new Date(`${before.value}T23:59:59`).toISOString() : "",
    filters: fieldFilters.value.filter((f) => f.key && f.value !== ""),
  };
}

async function load() {
  loading.value = true;
  try {
    const q = query();
    const [sumRes, listRes] = await Promise.all([
      formSubmissionsApi.summary(props.form.id, q),
      formSubmissionsApi.list(props.form.id, { ...q, page: page.value, size: 25 }),
    ]);
    summary.value = sumRes.data.data;
    rows.value = listRes.data.data;
    pageable.value = listRes.data.pageable;
  } catch (e: unknown) {
    notify.error(apiMessage(e, "Failed to load responses"));
  } finally {
    loading.value = false;
  }
}

let searchTimer: ReturnType<typeof setTimeout> | undefined;
watch(search, () => {
  clearTimeout(searchTimer);
  searchTimer = setTimeout(reload, 300);
});
watch([status, after, before, tag, priority, fieldFilters], reload, { deep: true });

function reload() {
  page.value = 0;
  load();
}

function goPage(p: number) {
  page.value = p;
  load();
}

// ─── Field filters ────────────────────────────────────────────────────────

function addFilter() {
  const first = activeQuestions.value[0];
  if (!first) return;
  fieldFilters.value.push({ key: first.key, op: "eq", value: "" });
}

function filterField(f: SubmissionFieldFilter) {
  return byKey.value[f.key];
}

function onFilterKey(f: SubmissionFieldFilter) {
  f.value = "";
  if (!RANGE_TYPES.includes(filterField(f)?.type || "")) f.op = "eq";
}

function inputType(f: SubmissionFieldFilter): string {
  const t = filterField(f)?.type;
  if (t === "number" || t === "scale" || t === "rating") return "number";
  if (t === "date" || t === "time") return t;
  return "text";
}

function clearFilters() {
  status.value = "";
  search.value = "";
  after.value = "";
  before.value = "";
  tag.value = "";
  priority.value = "";
  fieldFilters.value = [];
}

const filtered = computed(
  () =>
    !!(
      status.value ||
      search.value.trim() ||
      after.value ||
      before.value ||
      tag.value ||
      priority.value ||
      fieldFilters.value.some((f) => f.value !== "")
    ),
);

function exportUrl(format: "csv" | "json") {
  return formSubmissionsApi.exportUrl(props.form.id, format, query());
}

// ─── Display ──────────────────────────────────────────────────────────────

function labelOf(field: FormField | undefined, value: unknown): string {
  if (value === null || value === undefined) return "";
  if (typeof value === "boolean") return value ? "Yes" : "No";
  if (Array.isArray(value)) return value.map((v) => labelOf(field, v)).join(", ");
  const s = String(value);
  return field?.options?.find((o) => o.value === s)?.label || s;
}

function answer(m: Message, key: string): string {
  if (m.data && key in m.data) return labelOf(byKey.value[key], m.data[key]);
  // Submissions from before the form had questions keep only their raw fields.
  return m.fields?.find((f) => f.key === key)?.value || "";
}

function when(iso: string) {
  return new Date(iso).toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });
}

function pct(n: number, of: number) {
  return of ? Math.round((n / of) * 100) : 0;
}

function fmt(n?: number) {
  return n === undefined ? "–" : Number.isInteger(n) ? String(n) : n.toFixed(1);
}

function isChoice(s: SubmissionFieldSummary) {
  return s.type === "select" || s.type === "multiselect" || s.type === "checkbox";
}

function isHistogram(s: SubmissionFieldSummary) {
  return s.type === "scale" || s.type === "rating";
}

function maxCount(s: SubmissionFieldSummary) {
  return Math.max(1, ...(s.options || []).map((o) => o.count));
}

function scaleTop(s: SubmissionFieldSummary) {
  const opts = s.options || [];
  return opts.length ? opts[opts.length - 1].value : "";
}

const PRIORITY_LABELS: Record<string, string> = { low: "Low", high: "High", urgent: "Urgent" };

onMounted(async () => {
  load();
  try {
    knownTags.value = (await messagesApi.tags()).data.data.map((t) => t.tag);
  } catch {
    /* the tag filter just stays hidden */
  }
});
</script>

<template>
  <div>
    <!-- Filters: one row above the views, applied to summary, table and export alike. -->
    <div class="card" style="margin-bottom: 16px">
      <div class="card-body">
        <div class="rf-row">
          <div class="rf-views" role="tablist">
            <button
              v-for="v in (['summary', 'table'] as const)"
              :key="v"
              class="btn btn-sm"
              :class="view === v ? 'btn-primary' : 'btn-secondary'"
              role="tab"
              :aria-selected="view === v"
              @click="view = v"
            >
              {{ v === "summary" ? "Summary" : "Table" }}
            </button>
          </div>
          <input v-model="search" class="form-input rf-search" placeholder="Search answers" />
          <input v-model="after" type="date" class="form-input rf-date" aria-label="From" />
          <span class="muted">–</span>
          <input v-model="before" type="date" class="form-input rf-date" aria-label="To" />
          <select v-model="status" class="form-select rf-status" aria-label="Status">
            <option value="">Responses</option>
            <option value="flagged">Flagged only</option>
            <option value="quarantined">Quarantined (spam)</option>
          </select>
          <select v-model="priority" class="form-select rf-status" aria-label="Priority">
            <option value="">Any priority</option>
            <option value="urgent">Urgent</option>
            <option value="high">High</option>
            <option value="normal">Normal</option>
            <option value="low">Low</option>
          </select>
          <select v-if="knownTags.length" v-model="tag" class="form-select rf-status" aria-label="Tag">
            <option value="">Any tag</option>
            <option v-for="t in knownTags" :key="t" :value="t">{{ t }}</option>
          </select>
          <div class="rf-export">
            <a class="btn btn-secondary btn-sm" :href="exportUrl('csv')" download>Download CSV</a>
            <a class="btn btn-ghost btn-sm" :href="exportUrl('json')" download>JSON</a>
          </div>
        </div>

        <div v-if="hasSchema" class="rf-fields">
          <div v-for="(f, i) in fieldFilters" :key="i" class="rf-field">
            <select v-model="f.key" class="form-select" aria-label="Question" @change="onFilterKey(f)">
              <option v-for="q in questions" :key="q.key" :value="q.key">
                {{ q.label || q.key }}{{ q.archived ? " (archived)" : "" }}
              </option>
            </select>
            <select v-if="RANGE_TYPES.includes(filterField(f)?.type || '')" v-model="f.op" class="form-select rf-op" aria-label="Comparison">
              <option value="eq">is</option>
              <option value="gte">at least</option>
              <option value="lte">at most</option>
            </select>
            <span v-else class="muted rf-is">is</span>
            <select
              v-if="filterField(f)?.options?.length"
              v-model="f.value"
              class="form-select"
              aria-label="Value"
            >
              <option value="">Any</option>
              <option v-for="o in filterField(f)!.options" :key="o.value" :value="o.value">{{ o.label || o.value }}</option>
            </select>
            <select v-else-if="filterField(f)?.type === 'checkbox'" v-model="f.value" class="form-select" aria-label="Value">
              <option value="">Any</option>
              <option value="true">Checked</option>
              <option value="false">Not checked</option>
            </select>
            <input
              v-else
              v-model.lazy="f.value"
              class="form-input"
              aria-label="Value"
              :type="inputType(f)"
            />
            <button class="btn btn-ghost btn-xs" aria-label="Remove filter" @click="fieldFilters.splice(i, 1)">×</button>
          </div>
          <button class="btn btn-ghost btn-sm" :disabled="!activeQuestions.length" @click="addFilter">+ Filter by answer</button>
          <button v-if="filtered" class="btn btn-ghost btn-sm" @click="clearFilters">Clear filters</button>
        </div>
        <small class="muted">
          Downloads include exactly what is filtered here. Questions marked as personal data are exported only for workspace admins.
        </small>
      </div>
    </div>

    <div v-if="loading && !summary" class="loading-page"><div class="spinner"></div></div>

    <div v-else-if="summary && summary.total === 0" class="card">
      <div class="empty-state">
        <h3>{{ filtered ? "No responses match these filters" : "No responses yet" }}</h3>
        <template v-if="!filtered">
          <p>Share this link, or embed the form on your site from the Share &amp; embed tab.</p>
          <p v-if="props.shareUrl" class="share-line">
            <a :href="props.shareUrl" target="_blank" rel="noopener">{{ props.shareUrl }}</a>
          </p>
        </template>
        <button v-else class="btn btn-secondary" @click="clearFilters">Clear filters</button>
      </div>
    </div>

    <!-- ─── Summary ─── -->
    <template v-else-if="summary && view === 'summary'">
      <div class="rs-head">
        <div class="rs-total">{{ summary.total.toLocaleString() }}</div>
        <div class="muted">{{ summary.total === 1 ? "response" : "responses" }}{{ filtered ? " match" : "" }}</div>
      </div>
      <p v-if="summary.truncated" class="muted" style="font-size: 13px">
        Summarized from the newest {{ summary.total.toLocaleString() }} responses. Narrow the dates to cover older ones.
      </p>
      <div v-if="!hasSchema" class="card">
        <div class="card-body muted">
          This form has no questions, so there is nothing to chart. Add questions on the Questions tab, or use the Table view.
        </div>
      </div>

      <div v-for="s in summary.fields" :key="s.key" class="card rs-card">
        <div class="card-body">
          <div class="rs-title">{{ s.label || s.key }}</div>
          <div class="rs-sub">
            {{ s.answered.toLocaleString() }} {{ s.answered === 1 ? "response" : "responses" }}
            <span v-if="s.answered < summary.total"> · {{ summary.total - s.answered }} skipped</span>
          </div>

          <!-- Choices: horizontal bars, labelled with count and share. -->
          <div v-if="isChoice(s) && s.options" class="rs-bars">
            <div
              v-for="o in s.options"
              :key="o.value"
              class="rs-bar-row"
              :title="`${o.label}: ${o.count} (${pct(o.count, s.answered)}%)`"
            >
              <div class="rs-bar-label">{{ o.label }}</div>
              <div class="rs-bar-track">
                <div class="rs-bar" :style="{ width: `${(o.count / maxCount(s)) * 100}%` }"></div>
              </div>
              <div class="rs-bar-value">
                {{ o.count }} <span class="muted">({{ pct(o.count, s.answered) }}%)</span>
              </div>
            </div>
            <p v-if="s.type === 'multiselect'" class="rs-note">Respondents could pick more than one, so shares add up past 100%.</p>
          </div>

          <!-- Scale and rating: a histogram anchored to the baseline, plus the mean. -->
          <div v-else-if="isHistogram(s) && s.options" class="rs-hist-wrap">
            <div class="rs-mean">
              <span class="rs-mean-value">{{ fmt(s.mean) }}</span>
              <span class="muted"> average{{ s.type === "rating" ? ` of ${scaleTop(s)} stars` : ` on ${s.options[0].value}–${scaleTop(s)}` }}</span>
            </div>
            <div class="rs-hist" role="img" :aria-label="s.options.map((o) => `${o.label}: ${o.count}`).join(', ')">
              <div
                v-for="o in s.options"
                :key="o.value"
                class="rs-col"
                :title="`${s.type === 'rating' ? o.label + ' ★' : o.label}: ${o.count} (${pct(o.count, s.answered)}%)`"
              >
                <div class="rs-col-count">{{ o.count || "" }}</div>
                <div class="rs-col-plot">
                  <div class="rs-col-bar" :style="{ height: `${(o.count / maxCount(s)) * 100}%` }"></div>
                </div>
                <div class="rs-col-label">{{ o.label }}{{ s.type === "rating" ? "★" : "" }}</div>
              </div>
            </div>
          </div>

          <!-- Numbers: the figures themselves, no chart. -->
          <div v-else-if="s.type === 'number'" class="rs-stats">
            <div><span class="rs-stat">{{ fmt(s.mean) }}</span><span class="muted">average</span></div>
            <div><span class="rs-stat">{{ fmt(s.min) }}</span><span class="muted">lowest</span></div>
            <div><span class="rs-stat">{{ fmt(s.max) }}</span><span class="muted">highest</span></div>
          </div>

          <!-- Text: the latest answers. -->
          <template v-else>
            <p v-if="s.samples_hidden" class="rs-note">
              Personal data: answers are shown to workspace admins only. Open a response to see it.
            </p>
            <ul v-else-if="s.samples?.length" class="rs-samples">
              <li v-for="(a, i) in s.samples" :key="i">{{ a }}</li>
            </ul>
            <p v-if="(s.samples?.length || 0) < s.answered && !s.samples_hidden" class="rs-note">
              Latest {{ s.samples?.length }} of {{ s.answered }}. See all in the Table view.
            </p>
          </template>
        </div>
      </div>
    </template>

    <!-- ─── Table ─── -->
    <div v-else-if="summary && view === 'table'" class="card rt-card">
      <div class="rt-scroll">
        <table class="rt">
          <thead>
            <tr>
              <th>Submitted</th>
              <template v-if="hasSchema">
                <th v-for="q in activeQuestions" :key="q.key">{{ q.label || q.key }}</th>
              </template>
              <template v-else>
                <th>Email</th>
                <th>Name</th>
                <th>Subject</th>
                <th>Message</th>
              </template>
              <th>Priority</th>
              <th>Tags</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="m in rows" :key="m.uuid">
              <td class="rt-when">
                <router-link :to="`/messages/${m.uuid}`">{{ when(m.created_at) }}</router-link>
                <span v-if="m.status !== 'received'" class="badge badge-warning" style="margin-left: 6px">{{ m.status }}</span>
              </td>
              <template v-if="hasSchema">
                <td v-for="q in activeQuestions" :key="q.key" :class="{ 'rt-num': ['number', 'scale', 'rating'].includes(q.type) }">
                  {{ answer(m, q.key) }}
                </td>
              </template>
              <template v-else>
                <td>{{ m.sender_email }}</td>
                <td>{{ m.sender_name }}</td>
                <td>{{ m.subject }}</td>
                <td class="rt-long">{{ m.body }}</td>
              </template>
              <td>{{ PRIORITY_LABELS[m.priority] || "" }}</td>
              <td class="rt-tags">{{ (m.tags || []).join(", ") }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-if="pageable && pageable.total_pages > 1" class="card-body">
        <Pagination :pageable="pageable" @page="goPage" />
      </div>
    </div>
  </div>
</template>

<style scoped>
/* Single-series charts: one brand hue, validated against both surfaces
   (#9333EA on light, #A855F7 on dark). Text stays in text tokens. */
.rs-card,
.rt-scroll {
  --rs-mark: #9333ea;
}

:global([data-theme="dark"]) .rs-card {
  --rs-mark: #a855f7;
}

.rf-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.rf-views {
  display: flex;
  gap: 4px;
  margin-right: 8px;
}

.rf-search {
  flex: 1 1 180px;
  min-width: 140px;
}

.rf-date {
  width: 150px;
}

.rf-status {
  width: 190px;
}

.rf-export {
  display: flex;
  gap: 4px;
  margin-left: auto;
}

.rf-fields {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin: 12px 0 8px;
}

.rf-field {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 6px;
  border: 1px solid var(--border-primary);
  border-radius: 8px;
}

.rf-field .form-select,
.rf-field .form-input {
  width: auto;
  max-width: 200px;
  padding-top: 4px;
  padding-bottom: 4px;
}

.rf-op {
  width: 100px;
}

.rf-is {
  font-size: 13px;
}

.rs-head {
  display: flex;
  align-items: baseline;
  gap: 8px;
  margin: 4px 0 12px;
}

.rs-total {
  font-size: 32px;
  font-weight: 700;
  line-height: 1;
  color: var(--text-primary);
}

.rs-card {
  margin-bottom: 16px;
}

.rs-title {
  font-weight: 600;
  font-size: 15px;
  color: var(--text-primary);
}

.rs-sub {
  font-size: 12px;
  color: var(--text-muted);
  margin: 2px 0 14px;
}

.rs-bars {
  display: grid;
  gap: 6px;
}

.rs-bar-row {
  display: grid;
  grid-template-columns: minmax(80px, 200px) minmax(0, 1fr) 90px;
  align-items: center;
  gap: 10px;
  font-size: 13px;
  padding: 2px 0;
  border-radius: 4px;
}

.rs-bar-row:hover {
  background: var(--bg-hover, rgba(0, 0, 0, 0.03));
}

.rs-bar-label {
  color: var(--text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.rs-bar-track {
  height: 12px;
}

.rs-bar {
  height: 100%;
  min-width: 2px;
  background: var(--rs-mark);
  border-radius: 0 4px 4px 0;
}

.rs-bar-value {
  color: var(--text-primary);
  font-variant-numeric: tabular-nums;
}

.rs-hist-wrap {
  display: grid;
  gap: 10px;
}

.rs-mean-value {
  font-size: 24px;
  font-weight: 700;
  color: var(--text-primary);
}

.rs-hist {
  display: flex;
  align-items: flex-end;
  gap: 2px;
  max-width: 560px;
}

.rs-col {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  min-width: 24px;
  max-width: 64px;
  border-radius: 4px;
}

.rs-col:hover {
  background: var(--bg-hover, rgba(0, 0, 0, 0.03));
}

.rs-col-count {
  font-size: 12px;
  color: var(--text-secondary);
  height: 18px;
  font-variant-numeric: tabular-nums;
}

.rs-col-plot {
  width: 100%;
  height: 120px;
  display: flex;
  align-items: flex-end;
  justify-content: center;
  border-bottom: 1px solid var(--border-primary);
}

.rs-col-bar {
  width: 70%;
  min-height: 0;
  background: var(--rs-mark);
  border-radius: 4px 4px 0 0;
}

.rs-col-label {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 4px;
}

.rs-stats {
  display: flex;
  gap: 32px;
}

.rs-stats > div {
  display: flex;
  flex-direction: column;
}

.rs-stat {
  font-size: 22px;
  font-weight: 700;
  color: var(--text-primary);
  font-variant-numeric: tabular-nums;
}

.rs-samples {
  list-style: none;
  margin: 0;
  padding: 0;
  max-height: 260px;
  overflow: auto;
}

.rs-samples li {
  padding: 8px 10px;
  font-size: 13px;
  color: var(--text-primary);
  background: var(--bg-secondary);
  border-radius: 6px;
  margin-bottom: 4px;
  white-space: pre-wrap;
  word-break: break-word;
}

.rs-note {
  margin: 8px 0 0;
  font-size: 12px;
  color: var(--text-muted);
}

/* A wide table scrolls inside its card instead of widening the page:
   inline-size containment keeps the content from sizing the card. */
.rt-card {
  contain: inline-size;
}

.rt-scroll {
  overflow-x: auto;
}

.rt {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.rt th,
.rt td {
  padding: 10px 12px;
  text-align: left;
  border-bottom: 1px solid var(--border-primary);
  vertical-align: top;
  max-width: 280px;
}

.rt th {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
  white-space: nowrap;
  position: sticky;
  top: 0;
  background: var(--bg-primary);
}

.rt td {
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
}

.share-line {
  overflow-wrap: anywhere;
}

.rt-tags {
  color: var(--text-secondary);
}

.rt-when {
  white-space: nowrap;
}

.rt-num {
  font-variant-numeric: tabular-nums;
}

.rt-long {
  white-space: nowrap;
}
</style>
