import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import api from '../api/client'
import { workspaceApi } from '../api/workspaces'
import type { Workspace } from '../api/types'

const STORAGE_KEY = 'posta_workspace_id'

export const useWorkspaceStore = defineStore('workspace', () => {
  const workspaces = ref<Workspace[]>([])
  const currentWorkspaceId = ref<number | null>(
    (() => {
      const stored = localStorage.getItem(STORAGE_KEY)
      return stored ? Number(stored) : null
    })()
  )

  const currentWorkspace = computed(() =>
    workspaces.value.find((w) => w.id === currentWorkspaceId.value) ?? null
  )

  const currentRole = computed(() => currentWorkspace.value?.role ?? null)
  const isWorkspaceContext = computed(() => currentWorkspaceId.value !== null)
  const isWorkspaceAdmin = computed(() => currentRole.value === 'owner' || currentRole.value === 'admin')
  // No workspace means no permissions. The pre-workspace era granted edit rights
  // to a context with no workspace; every resource is scoped now, so a caller
  // without one can read and write nothing.
  const canEdit = computed(() => {
    const role = currentRole.value
    return role === 'owner' || role === 'admin' || role === 'editor'
  })

  // Ordinary workspaces, i.e. everything the user actually works in. The system
  // workspace is platform infrastructure and only platform admins can see it.
  const ownWorkspaces = computed(() => workspaces.value.filter((w) => !w.system))
  const systemWorkspace = computed(() => workspaces.value.find((w) => w.system) ?? null)
  const currentWorkspaceIsSystem = computed(() => currentWorkspace.value?.system ?? false)
  const hasWorkspace = computed(() => ownWorkspaces.value.length > 0)

  const contextLabel = computed(() => currentWorkspace.value?.name ?? 'No workspace')

  function setWorkspace(wsId: number | null) {
    currentWorkspaceId.value = wsId
    if (wsId) {
      localStorage.setItem(STORAGE_KEY, String(wsId))
    } else {
      localStorage.removeItem(STORAGE_KEY)
    }
  }

  async function fetchWorkspaces() {
    try {
      const res = await workspaceApi.list()
      workspaces.value = res.data.data ?? []
      const valid = currentWorkspaceId.value && workspaces.value.find((w) => w.id === currentWorkspaceId.value)
      // Land in an ordinary workspace, never the system one: a platform admin
      // opening the dashboard wants their own work, not platform infrastructure.
      if (!valid) {
        setWorkspace(ownWorkspaces.value[0]?.id ?? null)
      }
    } catch {
      workspaces.value = []
    }
  }

  function clear() {
    workspaces.value = []
    setWorkspace(null)
  }

  return {
    workspaces,
    currentWorkspaceId,
    currentWorkspace,
    currentRole,
    isWorkspaceContext,
    isWorkspaceAdmin,
    canEdit,
    ownWorkspaces,
    systemWorkspace,
    currentWorkspaceIsSystem,
    hasWorkspace,
    contextLabel,
    setWorkspace,
    fetchWorkspaces,
    clear,
  }
})

// Axios interceptor: inject X-Posta-Workspace-Id header when a workspace is selected
api.interceptors.request.use((config) => {
  const stored = localStorage.getItem(STORAGE_KEY)
  if (stored) {
    config.headers['X-Posta-Workspace-Id'] = stored
  }
  return config
})
