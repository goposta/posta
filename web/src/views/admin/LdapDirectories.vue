<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ldapApi, type LDAPDirectory, type LDAPDirectoryInput, type LDAPTestResult, type MappableRole } from '../../api/ldap'
import { adminApi } from '../../api/admin'
import type { AdminWorkspaceRow } from '../../api/types'
import { useNotificationStore } from '../../stores/notification'
import { useLicenseStore } from '../../stores/license'
import { useConfirm } from '../../composables/useConfirm'
import { apiMessage } from '../../composables/apiError'
import EnterpriseLock from '../../components/EnterpriseLock.vue'

// LDAP / Active Directory sign-in. People sign in with their directory email
// and password; Posta asks the directories only after the local password
// check fails, so local accounts keep working when a directory is down.

const notify = useNotificationStore()
const license = useLicenseStore()
const { confirm } = useConfirm()

const loading = ref(true)
const directories = ref<LDAPDirectory[]>([])
const workspaces = ref<AdminWorkspaceRow[]>([])
const tests = ref<Record<number, LDAPTestResult | 'running'>>({})

const workspaceName = computed(() => Object.fromEntries(workspaces.value.map((w) => [w.id, w.display_name])))

async function load() {
  loading.value = true
  try {
    const [dirRes, wsRes] = await Promise.all([ldapApi.list(), adminApi.listWorkspaces(0, 200).catch(() => null)])
    directories.value = dirRes.data.data
    if (wsRes) workspaces.value = wsRes.data.data.filter((w) => !w.system)
  } catch (e: any) {
    notify.error(apiMessage(e, 'Failed to load directories'))
  } finally {
    loading.value = false
  }
}

// ---- Directory editor ----

const showEditor = ref(false)
const editing = ref<LDAPDirectory | null>(null)
const saving = ref(false)
const blank = (): LDAPDirectoryInput & { bind_password: string } => ({
  display_name: '',
  host: '',
  port: undefined,
  tls_mode: 'starttls',
  ca_cert_pem: '',
  insecure_skip_tls: false,
  timeout_seconds: 10,
  bind_dn: '',
  bind_password: '',
  user_base_dn: '',
  user_filter: '',
  attr_email: '',
  attr_name: '',
  group_base_dn: '',
  group_filter: '',
  member_attr: '',
  nested_groups: false,
  enabled: true,
})
const draft = ref(blank())

function openEditor(dir: LDAPDirectory | null) {
  editing.value = dir
  draft.value = dir
    ? {
        display_name: dir.display_name,
        host: dir.host,
        port: dir.port,
        tls_mode: dir.tls_mode,
        ca_cert_pem: dir.ca_cert_pem,
        insecure_skip_tls: dir.insecure_skip_tls,
        timeout_seconds: dir.timeout_seconds,
        bind_dn: dir.bind_dn,
        bind_password: '',
        user_base_dn: dir.user_base_dn,
        user_filter: dir.user_filter,
        attr_email: dir.attr_email,
        attr_name: dir.attr_name,
        group_base_dn: dir.group_base_dn,
        group_filter: dir.group_filter,
        member_attr: dir.member_attr,
        nested_groups: dir.nested_groups,
        enabled: dir.enabled,
      }
    : blank()
  showEditor.value = true
}

const canSave = computed(
  () => !!draft.value.display_name.trim() && !!draft.value.host.trim() && !!draft.value.user_base_dn.trim(),
)

async function save() {
  saving.value = true
  try {
    const input = { ...draft.value, port: draft.value.port || undefined }
    if (editing.value) await ldapApi.update(editing.value.id, input)
    else await ldapApi.create(input)
    notify.success(editing.value ? 'Directory saved.' : 'Directory added. Test it before people sign in.')
    showEditor.value = false
    await load()
  } catch (e: any) {
    notify.error(apiMessage(e, 'Failed to save the directory'))
  } finally {
    saving.value = false
  }
}

async function remove(dir: LDAPDirectory) {
  const ok = await confirm({
    title: `Delete ${dir.display_name}?`,
    message:
      'People can no longer sign in through it. Accounts it created stay, with their data, but cannot sign in until another directory or a password reset lets them.',
    confirmText: 'Delete directory',
    variant: 'danger',
  })
  if (!ok) return
  try {
    await ldapApi.remove(dir.id)
    notify.success('Directory deleted.')
    await load()
  } catch (e: any) {
    notify.error(apiMessage(e, 'Failed to delete the directory'))
  }
}

async function test(dir: LDAPDirectory) {
  tests.value[dir.id] = 'running'
  try {
    tests.value[dir.id] = (await ldapApi.test(dir.id)).data.data
  } catch (e: any) {
    delete tests.value[dir.id]
    notify.error(apiMessage(e, 'Failed to test the directory'))
  }
}

// ---- Group mappings ----

const mappingDrafts = ref<Record<number, { group_dn: string; platform_admin: boolean; workspace_id: number | ''; workspace_role: MappableRole }>>({})

function mappingDraft(id: number) {
  if (!mappingDrafts.value[id]) {
    mappingDrafts.value[id] = { group_dn: '', platform_admin: false, workspace_id: '', workspace_role: 'viewer' }
  }
  return mappingDrafts.value[id]
}

function canAddMapping(id: number) {
  const m = mappingDraft(id)
  return !!m.group_dn.trim() && (m.platform_admin || m.workspace_id !== '')
}

async function addMapping(dir: LDAPDirectory) {
  const m = mappingDraft(dir.id)
  try {
    await ldapApi.addMapping(dir.id, {
      group_dn: m.group_dn.trim(),
      platform_admin: m.platform_admin,
      workspace_id: m.workspace_id === '' ? undefined : m.workspace_id,
      workspace_role: m.workspace_id === '' ? undefined : m.workspace_role,
    })
    delete mappingDrafts.value[dir.id]
    notify.success('Group mapped. It applies from the next sign-in.')
    await load()
  } catch (e: any) {
    notify.error(apiMessage(e, 'Failed to add the mapping'))
  }
}

async function removeMapping(dir: LDAPDirectory, mappingId: number) {
  try {
    await ldapApi.removeMapping(dir.id, mappingId)
    await load()
  } catch (e: any) {
    notify.error(apiMessage(e, 'Failed to remove the mapping'))
  }
}

function grants(m: { platform_admin: boolean; workspace_id: number | null; workspace_role: string }) {
  const out: string[] = []
  if (m.platform_admin) out.push('Platform admin')
  if (m.workspace_id) out.push(`${m.workspace_role} in ${workspaceName.value[m.workspace_id] || `workspace #${m.workspace_id}`}`)
  return out.join(' · ')
}

function testOf(id: number) {
  const t = tests.value[id]
  return t === 'running' ? null : t
}

onMounted(async () => {
  await license.load()
  if (license.licensed) await load()
  else loading.value = false
})
</script>

<template>
  <div>
    <div class="page-header">
      <div>
        <h1>LDAP Directories</h1>
        <p class="ldap-intro">
          People sign in with their directory email and password. Posta asks a directory only when the local
          password does not match, so local accounts keep working if it is down.
        </p>
      </div>
      <button v-if="license.mutable" class="btn btn-primary" @click="openEditor(null)">Add directory</button>
    </div>

    <EnterpriseLock
      feature="LDAP / Active Directory sign-in"
      description="Let people sign in with their LDAP or Active Directory account. Posta creates their account on first sign-in, and directory groups can make them platform admins or members of workspaces."
    >
      <div v-if="loading" class="loading-page"><div class="spinner"></div></div>

      <div v-else-if="!directories.length" class="card">
        <div class="empty-state">
          <h3>No directories yet</h3>
          <p>Add your LDAP server or Active Directory domain controller, test the connection, then map groups to access.</p>
          <button v-if="license.mutable" class="btn btn-primary" @click="openEditor(null)">Add directory</button>
        </div>
      </div>

      <div v-for="dir in directories" v-else :key="dir.id" class="card ldap-card">
        <div class="card-header">
          <div class="ldap-title">
            <h2>{{ dir.display_name }}</h2>
            <span v-if="dir.enabled" class="badge badge-success">On</span>
            <span v-else class="badge badge-neutral">Off</span>
            <span class="badge badge-secondary">{{ dir.tls_mode }}</span>
            <span v-if="dir.insecure_skip_tls" class="badge badge-warning">Certificate not checked</span>
          </div>
          <div class="ldap-actions">
            <button class="btn btn-secondary btn-sm" :disabled="tests[dir.id] === 'running'" @click="test(dir)">
              {{ tests[dir.id] === 'running' ? 'Testing…' : 'Test connection' }}
            </button>
            <template v-if="license.mutable">
              <button class="btn btn-secondary btn-sm" @click="openEditor(dir)">Edit</button>
              <button class="btn btn-ghost btn-sm" @click="remove(dir)">Delete</button>
            </template>
          </div>
        </div>
        <div class="card-body">
          <div
            v-if="testOf(dir.id)"
            class="app-banner ldap-test"
            :class="testOf(dir.id)?.ok ? 'app-banner--success' : 'app-banner--danger'"
            role="status"
          >
            <div class="app-banner-content">
              <p class="app-banner-text">{{ testOf(dir.id)?.ok ? testOf(dir.id)?.message : testOf(dir.id)?.error }}</p>
            </div>
          </div>

          <dl class="ldap-facts">
            <dt>Server</dt>
            <dd><code>{{ dir.host }}:{{ dir.port }}</code></dd>
            <dt>Service account</dt>
            <dd>
              <code v-if="dir.bind_dn">{{ dir.bind_dn }}</code><span v-else class="muted">Anonymous</span>
              <span v-if="dir.bind_dn && !dir.bind_password_set" class="badge badge-warning">No password</span>
            </dd>
            <dt>Users</dt>
            <dd>
              <code>{{ dir.user_base_dn }}</code> · <code>{{ dir.user_filter || `(${dir.attr_email || 'mail'}=%s)` }}</code>
            </dd>
            <dt>Groups</dt>
            <dd>
              <template v-if="dir.group_base_dn || dir.group_filter">
                <code>{{ dir.group_base_dn || dir.user_base_dn }}</code> · <code>{{ dir.group_filter || '(member=%s)' }}</code>
              </template>
              <template v-else><code>{{ dir.member_attr || 'memberOf' }}</code> on the user</template>
              <span v-if="dir.nested_groups" class="muted"> · nested</span>
            </dd>
          </dl>

          <h3 class="ldap-subtitle">Group mappings</h3>
          <p class="muted ldap-help">
            Applied at every sign-in to accounts this directory created: members of a group become platform admins
            or join a workspace. Access a mapping does not govern is left as set by hand.
          </p>
          <div v-if="dir.mappings?.length" class="table-wrapper">
            <table>
              <thead>
                <tr>
                  <th>Group</th>
                  <th>Grants</th>
                  <th v-if="license.mutable"></th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="m in dir.mappings" :key="m.id">
                  <td><code>{{ m.group_dn }}</code></td>
                  <td>{{ grants(m) }}</td>
                  <td v-if="license.mutable" class="ldap-row-actions">
                    <button class="btn btn-ghost btn-sm" @click="removeMapping(dir, m.id)">Remove</button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <p v-else class="muted ldap-help">No mappings: directory accounts sign in as plain users of their own workspace.</p>

          <div v-if="license.mutable" class="ldap-mapping-form">
            <input
              v-model="mappingDraft(dir.id).group_dn"
              class="form-input"
              placeholder="cn=posta-admins,ou=groups,dc=example,dc=com, or posta-admins"
              aria-label="Directory group"
            />
            <label class="checkbox-label">
              <input v-model="mappingDraft(dir.id).platform_admin" type="checkbox" /> Platform admin
            </label>
            <select v-model="mappingDraft(dir.id).workspace_id" class="form-select" aria-label="Workspace">
              <option value="">No workspace</option>
              <option v-for="w in workspaces" :key="w.id" :value="w.id">{{ w.display_name }}</option>
            </select>
            <select
              v-model="mappingDraft(dir.id).workspace_role"
              class="form-select"
              aria-label="Workspace role"
              :disabled="mappingDraft(dir.id).workspace_id === ''"
            >
              <option value="admin">Admin</option>
              <option value="editor">Editor</option>
              <option value="viewer">Viewer</option>
            </select>
            <button class="btn btn-primary" :disabled="!canAddMapping(dir.id)" @click="addMapping(dir)">Add mapping</button>
          </div>
        </div>
      </div>
    </EnterpriseLock>

    <div v-if="showEditor" class="modal-overlay" @click.self="showEditor = false">
      <div class="modal ldap-modal">
        <div class="modal-header">
          <h3>{{ editing ? `Edit ${editing.display_name}` : 'Add a directory' }}</h3>
        </div>
        <div class="modal-body">
          <h4 class="ldap-section">Connection</h4>
          <div class="ldap-grid">
            <div class="form-group">
              <label class="form-label" for="ldap-name">Name</label>
              <input id="ldap-name" v-model="draft.display_name" class="form-input" placeholder="Corporate AD" />
            </div>
            <div class="form-group">
              <label class="form-label" for="ldap-host">Host</label>
              <input id="ldap-host" v-model="draft.host" class="form-input" placeholder="ldap.example.com" />
            </div>
            <div class="form-group">
              <label class="form-label" for="ldap-tls">Encryption</label>
              <select id="ldap-tls" v-model="draft.tls_mode" class="form-select">
                <option value="starttls">StartTLS (port 389)</option>
                <option value="ldaps">LDAPS (port 636)</option>
                <option value="none">None: development only</option>
              </select>
            </div>
            <div class="form-group">
              <label class="form-label" for="ldap-port">Port</label>
              <input
                id="ldap-port"
                v-model.number="draft.port"
                type="number"
                min="1"
                max="65535"
                class="form-input"
                :placeholder="draft.tls_mode === 'ldaps' ? '636' : '389'"
              />
            </div>
          </div>
          <div class="form-group">
            <label class="form-label" for="ldap-ca">Private CA certificate <span class="muted">(optional, PEM)</span></label>
            <textarea id="ldap-ca" v-model="draft.ca_cert_pem" class="form-input ldap-mono" rows="3" spellcheck="false" />
          </div>
          <label class="checkbox-label">
            <input v-model="draft.insecure_skip_tls" type="checkbox" /> Don't check the server certificate
            <span class="muted">(testing only: anyone on the network could read passwords)</span>
          </label>

          <h4 class="ldap-section">Service account</h4>
          <p class="muted ldap-help">Searches for the person signing in. Leave empty to search anonymously.</p>
          <div class="ldap-grid">
            <div class="form-group">
              <label class="form-label" for="ldap-binddn">Bind DN</label>
              <input id="ldap-binddn" v-model="draft.bind_dn" class="form-input" placeholder="cn=posta,ou=services,dc=example,dc=com" />
            </div>
            <div class="form-group">
              <label class="form-label" for="ldap-bindpw">Password</label>
              <input
                id="ldap-bindpw"
                v-model="draft.bind_password"
                type="password"
                class="form-input"
                autocomplete="new-password"
                :placeholder="editing?.bind_password_set ? 'Stored: leave empty to keep' : ''"
              />
            </div>
          </div>

          <h4 class="ldap-section">Users</h4>
          <div class="ldap-grid">
            <div class="form-group">
              <label class="form-label" for="ldap-base">Base DN</label>
              <input id="ldap-base" v-model="draft.user_base_dn" class="form-input" placeholder="ou=people,dc=example,dc=com" />
            </div>
            <div class="form-group">
              <label class="form-label" for="ldap-filter">Filter <span class="muted">(%s is the email)</span></label>
              <input id="ldap-filter" v-model="draft.user_filter" class="form-input" placeholder="(mail=%s)" />
            </div>
            <div class="form-group">
              <label class="form-label" for="ldap-attr-email">Email attribute</label>
              <input id="ldap-attr-email" v-model="draft.attr_email" class="form-input" placeholder="mail" />
            </div>
            <div class="form-group">
              <label class="form-label" for="ldap-attr-name">Name attribute</label>
              <input id="ldap-attr-name" v-model="draft.attr_name" class="form-input" placeholder="displayName" />
            </div>
          </div>

          <h4 class="ldap-section">Groups</h4>
          <p class="muted ldap-help">
            By default Posta reads the user's <code>memberOf</code>. Set a group base DN or filter to search for
            groups instead.
          </p>
          <div class="ldap-grid">
            <div class="form-group">
              <label class="form-label" for="ldap-member">Member attribute</label>
              <input id="ldap-member" v-model="draft.member_attr" class="form-input" placeholder="memberOf" />
            </div>
            <div class="form-group">
              <label class="form-label" for="ldap-gbase">Group base DN</label>
              <input id="ldap-gbase" v-model="draft.group_base_dn" class="form-input" placeholder="ou=groups,dc=example,dc=com" />
            </div>
            <div class="form-group">
              <label class="form-label" for="ldap-gfilter">Group filter <span class="muted">(%s is the user's DN)</span></label>
              <input id="ldap-gfilter" v-model="draft.group_filter" class="form-input" placeholder="(member=%s)" />
            </div>
          </div>
          <label class="checkbox-label">
            <input v-model="draft.nested_groups" type="checkbox" /> Include nested groups (Active Directory)
          </label>
          <label class="checkbox-label">
            <input v-model="draft.enabled" type="checkbox" /> People can sign in through this directory
          </label>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="showEditor = false">Cancel</button>
          <button class="btn btn-primary" :disabled="saving || !canSave" @click="save">
            {{ saving ? 'Saving…' : editing ? 'Save directory' : 'Add directory' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.ldap-modal {
  width: min(760px, calc(100vw - 32px));
  max-width: 760px;
}

.ldap-intro {
  margin: 4px 0 0;
  color: var(--text-muted);
  font-size: 13px;
  max-width: 640px;
}

.ldap-card {
  margin-bottom: 20px;
}

.ldap-title {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.ldap-actions {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.ldap-test {
  margin-bottom: 16px;
}

.ldap-facts {
  display: grid;
  grid-template-columns: max-content 1fr;
  gap: 8px 16px;
  margin: 0 0 20px;
  font-size: 14px;
}

.ldap-facts dt {
  color: var(--text-tertiary);
}

.ldap-facts dd {
  margin: 0;
  min-width: 0;
  overflow-wrap: anywhere;
}

.ldap-subtitle {
  margin: 0 0 4px;
  font-size: 15px;
}

.ldap-help {
  margin: 0 0 12px;
  font-size: 13px;
}

.ldap-mapping-form {
  display: grid;
  grid-template-columns: minmax(220px, 2fr) auto minmax(140px, 1fr) minmax(110px, auto) auto;
  gap: 8px;
  align-items: center;
  margin-top: 12px;
}

.ldap-row-actions {
  text-align: right;
}

.ldap-section {
  margin: 20px 0 8px;
  font-size: 14px;
}

.ldap-section:first-child {
  margin-top: 0;
}

.ldap-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 0 16px;
}

.ldap-mono {
  font-family: var(--font-mono, ui-monospace, monospace);
  font-size: 12px;
}

@media (max-width: 900px) {
  .ldap-mapping-form {
    grid-template-columns: 1fr;
  }
}
</style>
