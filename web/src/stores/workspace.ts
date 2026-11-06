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
  // Every resource is workspace-scoped, so a caller without a workspace can read and write nothing.
  const canEdit = computed(() => {
    const role = currentRole.value
    return role === 'owner' || role === 'admin' || role === 'editor'
  })

  // The system workspace is platform infrastructure, visible only to platform admins.
  const ownWorkspaces = computed(() => workspaces.value.filter((w) => !w.system))
  const systemWorkspace = computed(() => workspaces.value.find((w) => w.system) ?? null)
  const currentWorkspaceIsSystem = computed(() => currentWorkspace.value?.system ?? false)
  const hasWorkspace = computed(() => ownWorkspaces.value.length > 0)

  const contextLabel = computed(() => currentWorkspace.value?.display_name ?? 'No workspace')

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
      // Never land in the system workspace: a platform admin opening the dashboard wants their own work.
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

api.interceptors.request.use((config) => {
  const stored = localStorage.getItem(STORAGE_KEY)
  if (stored) {
    config.headers['X-Posta-Workspace-Id'] = stored
  }
  return config
})
