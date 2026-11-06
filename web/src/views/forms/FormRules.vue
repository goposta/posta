<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { formRulesApi, teamsApi } from "../../api/messages";
import { workspaceApi } from "../../api/workspaces";
import { webhooksApi } from "../../api/webhooks";
import { useNotificationStore } from "../../stores/notification";
import { apiMessage } from "../../composables/apiError";
import type {
  Form,
  FormField,
  FormRule,
  FormRuleInput,
  RuleAction,
  RuleActionType,
  RuleCondition,
  RuleOp,
  RuleTestResult,
  Team,
  Webhook,
  WorkspaceMember,
} from "../../api/types";

// Rules offer only this form's questions and this workspace's destinations: a value a visitor typed never becomes a destination.

const props = defineProps<{ form: Form }>();
const notify = useNotificationStore();

interface Draft extends FormRuleInput {
  uid: number;
  id?: number;
  hit_count: number;
  last_hit_at?: string | null;
  saved: string; // JSON of the last saved state, "" when new
}

let uidSeq = 0;
const loading = ref(true);
const drafts = ref<Draft[]>([]);
const openUid = ref<number | null>(null);
const busyUid = ref<number | null>(null);
const teams = ref<Team[]>([]);
const members = ref<WorkspaceMember[]>([]);
const webhooks = ref<Webhook[]>([]);
const testing = ref(false);
const test = ref<RuleTestResult | null>(null);

const BUILTINS = [
  { key: "$sender_email", label: "Sender email" },
  { key: "$origin", label: "Website it came from" },
  { key: "$referer", label: "Page it was sent from" },
  { key: "$spam_score", label: "Spam score" },
  { key: "$status", label: "Scan result" },
];

const questions = computed(() =>
  (props.form.fields || []).filter((f) => !f.archived && f.type !== "section"),
);
const schemaless = computed(() => questions.value.length === 0);

type Kind = "choice" | "checkbox" | "number" | "date" | "time" | "status" | "text";

function fieldDef(key: string): FormField | undefined {
  return questions.value.find((f) => f.key === key);
}

function kindOf(key: string): Kind {
  if (key === "$spam_score") return "number";
  if (key === "$status") return "status";
  const f = fieldDef(key);
  switch (f?.type) {
    case "select":
    case "multiselect":
      return "choice";
    case "checkbox":
      return "checkbox";
    case "number":
    case "scale":
    case "rating":
      return "number";
    case "date":
      return "date";
    case "time":
      return "time";
  }
  return "text";
}

const ANSWERED: [RuleOp, string][] = [
  ["exists", "is answered"],
  ["not_exists", "is not answered"],
];
const OPS: Record<Kind, [RuleOp, string][]> = {
  choice: [["in", "is any of"], ["not_in", "is none of"], ...ANSWERED],
  checkbox: [["eq", "is"]],
  number: [["eq", "equals"], ["gt", "is more than"], ["gte", "is at least"], ["lt", "is less than"], ["lte", "is at most"], ...ANSWERED],
  date: [["eq", "is"], ["gt", "is after"], ["gte", "is on or after"], ["lt", "is before"], ["lte", "is on or before"], ...ANSWERED],
  time: [["eq", "is"], ["gt", "is after"], ["lt", "is before"], ...ANSWERED],
  status: [["eq", "is"]],
  text: [["eq", "is"], ["neq", "is not"], ["contains", "contains"], ["not_contains", "does not contain"], ["matches", "matches pattern"], ...ANSWERED],
};

function opsFor(c: RuleCondition) {
  return OPS[kindOf(c.field)];
}

function needsValue(c: RuleCondition) {
  return c.op !== "exists" && c.op !== "not_exists";
}

function resetCondition(c: RuleCondition) {
  const kind = kindOf(c.field);
  c.op = OPS[kind][0][0];
  c.value = kind === "choice" ? [] : kind === "checkbox" ? "true" : kind === "status" ? "flagged" : "";
}

function toggleChoice(c: RuleCondition, value: string) {
  const list = Array.isArray(c.value) ? [...c.value] : [];
  const i = list.indexOf(value);
  if (i >= 0) list.splice(i, 1);
  else list.push(value);
  c.value = list;
}

function fieldLabel(key: string) {
  return fieldDef(key)?.label || BUILTINS.find((b) => b.key === key)?.label || key;
}

const ACTIONS: { type: RuleActionType; label: string }[] = [
  { type: "notify_team", label: "Notify a team" },
  { type: "notify_members", label: "Notify members" },
  { type: "notify_emails", label: "Notify email addresses" },
  { type: "assign_team", label: "Route to a team" },
  { type: "assign_member", label: "Assign to a member" },
  { type: "add_tags", label: "Add tags" },
  { type: "set_priority", label: "Set priority" },
  { type: "webhook", label: "Send to a webhook" },
  { type: "suppress_default", label: "Don't notify the form's usual recipients" },
];

function newAction(type: RuleActionType): RuleAction {
  switch (type) {
    case "notify_team":
    case "assign_team":
      return { type, team_id: teams.value[0]?.id ?? null };
    case "notify_members":
      return { type, user_ids: [] };
    case "assign_member":
      return { type, user_ids: members.value[0] ? [members.value[0].user_id] : [] };
    case "notify_emails":
      return { type, emails: [] };
    case "add_tags":
      return { type, tags: [] };
    case "set_priority":
      return { type, priority: "high" };
    case "webhook":
      return { type, webhook_id: webhooks.value[0]?.id ?? null };
  }
  return { type };
}

function changeActionType(d: Draft, i: number, type: RuleActionType) {
  d.actions[i] = newAction(type);
}

function listText(list?: string[]) {
  return (list || []).join(", ");
}

function setList(a: RuleAction, key: "emails" | "tags", text: string) {
  a[key] = text
    .split(/[,;\s]+/)
    .map((s) => s.trim())
    .filter(Boolean);
}

function toggleMember(a: RuleAction, id: number) {
  const ids = a.user_ids || [];
  a.user_ids = ids.includes(id) ? ids.filter((x) => x !== id) : [...ids, id];
}

const teamName = (id?: number | null) => teams.value.find((t) => t.id === id)?.name || "a team";
const memberName = (id?: number) => {
  const m = members.value.find((x) => x.user_id === id);
  return m ? m.name || m.email : "a member";
};

function summary(d: Draft) {
  const when = d.conditions.length
    ? d.conditions
        .map((c) => {
          const op = opsFor(c).find(([o]) => o === c.op)?.[1] || c.op;
          const value = !needsValue(c) ? "" : Array.isArray(c.value) ? ` ${c.value.join(" or ")}` : ` ${c.value ?? ""}`;
          return `${fieldLabel(c.field)} ${op}${value}`;
        })
        .join(d.match === "any" ? " or " : " and ")
    : "Every submission";
  const then = d.actions
    .map((a) => {
      switch (a.type) {
        case "notify_team":
          return `notify ${teamName(a.team_id)}`;
        case "assign_team":
          return `route to ${teamName(a.team_id)}`;
        case "notify_members":
          return `notify ${(a.user_ids || []).map(memberName).join(", ") || "members"}`;
        case "assign_member":
          return `assign to ${memberName(a.user_ids?.[0])}`;
        case "notify_emails":
          return `email ${listText(a.emails) || "addresses"}`;
        case "add_tags":
          return `tag ${listText(a.tags)}`;
        case "set_priority":
          return `priority ${a.priority}`;
        case "webhook":
          return "call a webhook";
        case "suppress_default":
          return "skip the usual recipients";
      }
      return a.type;
    })
    .join(", ");
  return `${when} → ${then || "nothing yet"}`;
}

function input(d: Draft): FormRuleInput {
  return JSON.parse(
    JSON.stringify({ name: d.name, enabled: d.enabled, match: d.match, conditions: d.conditions, actions: d.actions, stop: d.stop }),
  );
}

function toDraft(r: FormRule): Draft {
  const d: Draft = {
    uid: ++uidSeq,
    id: r.id,
    name: r.name,
    enabled: r.enabled,
    match: r.match,
    conditions: r.conditions || [],
    actions: r.actions || [],
    stop: r.stop,
    hit_count: r.hit_count,
    last_hit_at: r.last_hit_at,
    saved: "",
  };
  d.saved = JSON.stringify(input(d));
  return d;
}

const dirty = (d: Draft) => JSON.stringify(input(d)) !== d.saved;

async function load() {
  loading.value = true;
  try {
    const [ruleRes, teamRes, memberRes, hookRes] = await Promise.all([
      formRulesApi.list(props.form.id),
      teamsApi.list(),
      workspaceApi.listMembers(),
      webhooksApi.list(0, 100).catch(() => null),
    ]);
    drafts.value = ruleRes.data.data.map(toDraft);
    teams.value = teamRes.data.data;
    members.value = memberRes.data.data || [];
    webhooks.value = hookRes?.data.data || [];
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to load routing rules"));
  } finally {
    loading.value = false;
  }
}

function addRule() {
  const d: Draft = {
    uid: ++uidSeq,
    name: `Rule ${drafts.value.length + 1}`,
    enabled: true,
    match: "all",
    conditions: [],
    actions: [newAction(teams.value.length ? "notify_team" : "notify_emails")],
    stop: false,
    hit_count: 0,
    saved: "",
  };
  if (!schemaless.value) {
    const first = questions.value.find((q) => q.type === "select") || questions.value[0];
    const c: RuleCondition = { field: first.key, op: "in" };
    resetCondition(c);
    d.conditions.push(c);
  }
  drafts.value.push(d);
  openUid.value = d.uid;
}

function addCondition(d: Draft) {
  const c: RuleCondition = { field: schemaless.value ? "" : questions.value[0].key, op: "eq" };
  if (schemaless.value) c.value = "";
  else resetCondition(c);
  d.conditions.push(c);
}

async function save(d: Draft) {
  busyUid.value = d.uid;
  try {
    const res = d.id
      ? await formRulesApi.update(props.form.id, d.id, input(d))
      : await formRulesApi.create(props.form.id, input(d));
    const saved = toDraft(res.data.data);
    saved.uid = d.uid;
    drafts.value.splice(drafts.value.indexOf(d), 1, saved);
    notify.success("Rule saved.");
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to save rule"));
  } finally {
    busyUid.value = null;
  }
}

async function remove(d: Draft) {
  if (d.id && !confirm(`Delete the rule "${d.name}"?`)) return;
  try {
    if (d.id) await formRulesApi.delete(props.form.id, d.id);
    drafts.value = drafts.value.filter((x) => x !== d);
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to delete rule"));
  }
}

async function move(d: Draft, step: -1 | 1) {
  const list = [...drafts.value];
  const i = list.indexOf(d);
  const j = i + step;
  if (j < 0 || j >= list.length) return;
  [list[i], list[j]] = [list[j], list[i]];
  drafts.value = list;
  const ids = list.filter((x) => x.id).map((x) => x.id!);
  if (ids.length < 2) return;
  try {
    await formRulesApi.reorder(props.form.id, ids);
  } catch (e: any) {
    notify.error(apiMessage(e, "Failed to reorder rules"));
  }
}

// Test runs the rules as they are on screen, saved or not.
async function runTest() {
  testing.value = true;
  try {
    const res = await formRulesApi.test(props.form.id, drafts.value.map(input), 200);
    test.value = res.data.data;
  } catch (e: any) {
    notify.error(apiMessage(e, "Could not test the rules"));
  } finally {
    testing.value = false;
  }
}

function testCount(i: number) {
  return test.value?.rules[i]?.matches;
}

function when(iso?: string | null) {
  return iso ? new Date(iso).toLocaleString() : "";
}

onMounted(load);
</script>

<template>
  <div class="rules">
    <div class="card" style="margin-bottom: 16px">
      <div class="card-body rules-intro">
        <div>
          <h2 style="margin: 0 0 4px">Routing</h2>
          <p class="muted" style="margin: 0">
            Rules run in order on every submission that passes the spam check. Each rule that matches applies,
            unless an earlier one says to stop. The form's usual recipients are still notified unless a rule
            turns that off.
          </p>
        </div>
        <div class="rules-intro-actions">
          <button class="btn btn-secondary" :disabled="testing || !drafts.length" @click="runTest">
            {{ testing ? "Testing…" : "Test on recent submissions" }}
          </button>
          <button class="btn btn-primary" @click="addRule">+ Add rule</button>
        </div>
      </div>
      <div v-if="test" class="card-body rules-test-head">
        <strong>{{ test.matched }} of {{ test.scanned }}</strong> recent submissions match at least one rule.
        <span class="muted">Nothing was sent or changed.</span>
        <button class="btn btn-ghost btn-xs" @click="test = null">Hide</button>
      </div>
    </div>

    <div v-if="loading" class="loading-page"><div class="spinner"></div></div>

    <div v-else-if="!drafts.length" class="card">
      <div class="empty-state">
        <h3>No routing rules</h3>
        <p>
          Every submission goes to the form's usual recipients. Add a rule to send some of them elsewhere: for
          example, notify Sales when the department is "sales", or mark urgent requests as high priority.
        </p>
        <p v-if="!teams.length" class="muted">
          <router-link to="/teams">Create a team</router-link> first to route submissions to a group of members.
        </p>
      </div>
    </div>

    <div
      v-for="(d, i) in drafts"
      :key="d.uid"
      class="card rule-card"
      :class="{ open: openUid === d.uid, off: !d.enabled }"
    >
      <div class="rule-head" @click="openUid = openUid === d.uid ? null : d.uid">
        <span class="rule-pos">{{ i + 1 }}</span>
        <div class="rule-summary">
          <div class="rule-title">
            {{ d.name || "Untitled rule" }}
            <span v-if="!d.enabled" class="badge badge-neutral">Off</span>
            <span v-if="d.stop" class="badge badge-neutral">Stops</span>
            <span v-if="!d.id" class="badge badge-warning">Not saved</span>
            <span v-else-if="dirty(d)" class="badge badge-warning">Unsaved changes</span>
          </div>
          <div class="rule-line">{{ summary(d) }}</div>
          <div class="rule-meta muted">
            <template v-if="test && testCount(i) !== undefined">
              <strong>Would match {{ testCount(i) }} of {{ test.scanned }}</strong> ·
            </template>
            <template v-if="d.id">Matched {{ d.hit_count }} time{{ d.hit_count === 1 ? "" : "s" }}</template>
            <template v-if="d.last_hit_at">, last {{ when(d.last_hit_at) }}</template>
          </div>
        </div>
        <div class="rule-tools" @click.stop>
          <button class="btn btn-ghost btn-xs" :disabled="i === 0" aria-label="Move up" @click="move(d, -1)">↑</button>
          <button class="btn btn-ghost btn-xs" :disabled="i === drafts.length - 1" aria-label="Move down" @click="move(d, 1)">↓</button>
        </div>
      </div>

      <div v-if="openUid === d.uid" class="rule-body">
        <div class="rule-row">
          <div class="form-group" style="flex: 1 1 240px">
            <label class="form-label" :for="`rule-name-${d.uid}`">Name</label>
            <input :id="`rule-name-${d.uid}`" v-model="d.name" class="form-input" maxlength="120" />
          </div>
          <label class="checkbox-label"><input v-model="d.enabled" type="checkbox" /> On</label>
        </div>

        <h4 class="rule-section">When</h4>
        <p v-if="!d.conditions.length" class="muted rule-note">No conditions: this rule applies to every submission.</p>
        <div v-if="d.conditions.length > 1" class="form-group" style="max-width: 280px">
          <select v-model="d.match" class="form-select" aria-label="Match">
            <option value="all">All of these are true</option>
            <option value="any">Any of these is true</option>
          </select>
        </div>
        <div v-for="(c, ci) in d.conditions" :key="ci" class="cond">
          <select
            v-if="!schemaless"
            v-model="c.field"
            class="form-select cond-field"
            aria-label="Field"
            @change="resetCondition(c)"
          >
            <optgroup label="Questions">
              <option v-for="q in questions" :key="q.key" :value="q.key">{{ q.label || q.key }}</option>
            </optgroup>
            <optgroup label="About the submission">
              <option v-for="b in BUILTINS" :key="b.key" :value="b.key">{{ b.label }}</option>
            </optgroup>
          </select>
          <input
            v-else
            v-model="c.field"
            class="form-input cond-field"
            placeholder="field name, e.g. department"
            aria-label="Field"
            list="rule-builtins"
          />
          <select v-model="c.op" class="form-select cond-op" aria-label="Comparison">
            <option v-for="[op, label] in opsFor(c)" :key="op" :value="op">{{ label }}</option>
          </select>

          <template v-if="needsValue(c)">
            <div v-if="kindOf(c.field) === 'choice'" class="cond-choices">
              <label v-for="o in fieldDef(c.field)?.options || []" :key="o.value" class="chip-check">
                <input
                  type="checkbox"
                  :checked="Array.isArray(c.value) && c.value.includes(o.value)"
                  @change="toggleChoice(c, o.value)"
                />
                {{ o.label || o.value }}
              </label>
            </div>
            <select v-else-if="kindOf(c.field) === 'checkbox'" v-model="c.value" class="form-select cond-value" aria-label="Value">
              <option value="true">Checked</option>
              <option value="false">Not checked</option>
            </select>
            <select v-else-if="kindOf(c.field) === 'status'" v-model="c.value" class="form-select cond-value" aria-label="Value">
              <option value="received">Clean</option>
              <option value="flagged">Flagged as possible spam</option>
            </select>
            <input
              v-else
              v-model="c.value"
              class="form-input cond-value"
              :type="{ number: 'number', date: 'date', time: 'time' }[kindOf(c.field) as string] || 'text'"
              :step="kindOf(c.field) === 'number' ? 'any' : undefined"
              :placeholder="c.op === 'matches' ? 'regular expression' : 'value'"
              aria-label="Value"
            />
          </template>
          <button class="btn btn-ghost btn-xs" aria-label="Remove condition" @click="d.conditions.splice(ci, 1)">×</button>
        </div>
        <datalist id="rule-builtins">
          <option v-for="b in BUILTINS" :key="b.key" :value="b.key">{{ b.label }}</option>
        </datalist>
        <button class="btn btn-secondary btn-xs" @click="addCondition(d)">+ Condition</button>

        <h4 class="rule-section">Then</h4>
        <div v-for="(a, ai) in d.actions" :key="ai" class="act">
          <select
            :value="a.type"
            class="form-select act-type"
            aria-label="Action"
            @change="changeActionType(d, ai, ($event.target as HTMLSelectElement).value as RuleActionType)"
          >
            <option v-for="t in ACTIONS" :key="t.type" :value="t.type">{{ t.label }}</option>
          </select>

          <template v-if="a.type === 'notify_team' || a.type === 'assign_team'">
            <select v-if="teams.length" v-model="a.team_id" class="form-select act-value" aria-label="Team">
              <option v-for="t in teams" :key="t.id" :value="t.id">{{ t.name }}</option>
            </select>
            <span v-else class="muted act-value"><router-link to="/teams">Create a team</router-link> first.</span>
          </template>
          <select
            v-else-if="a.type === 'assign_member'"
            :value="a.user_ids?.[0]"
            class="form-select act-value"
            aria-label="Member"
            @change="a.user_ids = [Number(($event.target as HTMLSelectElement).value)]"
          >
            <option v-for="m in members" :key="m.user_id" :value="m.user_id">{{ m.name || m.email }}</option>
          </select>
          <div v-else-if="a.type === 'notify_members'" class="cond-choices act-value">
            <label v-for="m in members" :key="m.user_id" class="chip-check">
              <input type="checkbox" :checked="(a.user_ids || []).includes(m.user_id)" @change="toggleMember(a, m.user_id)" />
              {{ m.name || m.email }}
            </label>
          </div>
          <input
            v-else-if="a.type === 'notify_emails'"
            :value="listText(a.emails)"
            class="form-input act-value"
            placeholder="sales@example.com, lead@example.com"
            aria-label="Addresses"
            @change="setList(a, 'emails', ($event.target as HTMLInputElement).value)"
          />
          <input
            v-else-if="a.type === 'add_tags'"
            :value="listText(a.tags)"
            class="form-input act-value"
            placeholder="lead, follow up"
            aria-label="Tags"
            @change="setList(a, 'tags', ($event.target as HTMLInputElement).value)"
          />
          <select v-else-if="a.type === 'set_priority'" v-model="a.priority" class="form-select act-value" aria-label="Priority">
            <option value="urgent">Urgent</option>
            <option value="high">High</option>
            <option value="normal">Normal</option>
            <option value="low">Low</option>
          </select>
          <template v-else-if="a.type === 'webhook'">
            <select v-if="webhooks.length" v-model="a.webhook_id" class="form-select act-value" aria-label="Webhook">
              <option v-for="w in webhooks" :key="w.id" :value="w.id">{{ w.url }}</option>
            </select>
            <span v-else class="muted act-value"><router-link to="/webhooks">Add a webhook</router-link> first.</span>
          </template>
          <span v-else class="act-value"></span>
          <button class="btn btn-ghost btn-xs" :disabled="d.actions.length <= 1" aria-label="Remove action" @click="d.actions.splice(ai, 1)">×</button>
        </div>
        <button class="btn btn-secondary btn-xs" @click="d.actions.push(newAction('add_tags'))">+ Action</button>
        <p class="muted rule-note">
          Addresses are typed here by you. A rule can choose between destinations, but never sends to an address
          a visitor entered.
        </p>

        <div class="rule-foot">
          <label class="checkbox-label" title="Later rules are not checked when this one matches">
            <input v-model="d.stop" type="checkbox" /> Stop here when this rule matches
          </label>
          <div class="rule-foot-actions">
            <button class="btn btn-ghost btn-sm" @click="remove(d)">Delete</button>
            <button class="btn btn-primary btn-sm" :disabled="busyUid === d.uid || (!!d.id && !dirty(d))" @click="save(d)">
              {{ busyUid === d.uid ? "Saving…" : "Save rule" }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <div v-if="test && test.samples.length" class="card">
      <div class="card-header"><h2>Recent matches</h2></div>
      <div class="table-wrapper">
        <table>
          <thead>
            <tr>
              <th>Received</th>
              <th>From</th>
              <th>Rules</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="s in test.samples" :key="s.message_uuid">
              <td><router-link :to="`/messages/${s.message_uuid}`">{{ when(s.received_at) }}</router-link></td>
              <td>{{ s.sender || s.subject || "—" }}</td>
              <td>{{ s.matched.map((i) => drafts[i]?.name || `Rule ${i + 1}`).join(", ") }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<style scoped>
.rules-intro {
  display: flex;
  gap: 16px;
  align-items: flex-start;
  justify-content: space-between;
  flex-wrap: wrap;
}

.rules-intro p {
  max-width: 640px;
}

.rules-intro-actions {
  display: flex;
  gap: 8px;
}

.rules-test-head {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
  border-top: 1px solid var(--border-secondary);
}

.rule-card {
  margin-bottom: 12px;
  border-left: 3px solid transparent;
}

.rule-card.open {
  border-left-color: var(--primary-600);
}

.rule-card.off .rule-summary {
  opacity: 0.6;
}

.rule-head {
  display: flex;
  gap: 12px;
  align-items: flex-start;
  padding: 14px 16px;
  cursor: pointer;
}

.rule-pos {
  flex: none;
  width: 24px;
  height: 24px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  font-size: 12px;
  font-weight: 600;
  background: var(--bg-tertiary);
  color: var(--text-secondary);
}

.rule-summary {
  flex: 1;
  min-width: 0;
}

.rule-title {
  display: flex;
  gap: 6px;
  align-items: center;
  flex-wrap: wrap;
  font-weight: 600;
}

.rule-line {
  margin-top: 2px;
  font-size: 13px;
  color: var(--text-secondary);
  overflow-wrap: anywhere;
}

.rule-meta {
  margin-top: 2px;
  font-size: 12px;
}

.rule-tools {
  display: flex;
  gap: 4px;
}

.rule-body {
  padding: 0 16px 16px 52px;
}

.rule-row {
  display: flex;
  gap: 16px;
  align-items: center;
  flex-wrap: wrap;
}

.rule-section {
  margin: 16px 0 8px;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--text-tertiary);
}

.rule-note {
  font-size: 13px;
  margin: 6px 0;
}

.cond,
.act {
  display: flex;
  gap: 8px;
  align-items: flex-start;
  flex-wrap: wrap;
  margin-bottom: 8px;
}

.cond-field,
.act-type {
  flex: 0 1 220px;
}

.cond-op {
  flex: 0 1 170px;
}

.cond-value,
.act-value {
  flex: 1 1 200px;
  min-width: 0;
}

.cond-choices {
  flex: 1 1 260px;
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.chip-check {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 5px 10px;
  border: 1px solid var(--border-primary);
  border-radius: 999px;
  font-size: 13px;
  cursor: pointer;
}

.chip-check:has(input:checked) {
  border-color: var(--primary-500);
  background: var(--primary-50);
}

.rule-foot {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  margin-top: 16px;
  padding-top: 12px;
  border-top: 1px dashed var(--border-primary);
}

.rule-foot-actions {
  display: flex;
  gap: 8px;
}

@media (max-width: 640px) {
  .rule-body {
    padding-left: 16px;
  }
}
</style>
