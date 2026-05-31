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
  const isPersonal = computed(() => currentWorkspaceId.value === null)
  const isWorkspaceContext = computed(() => currentWorkspaceId.value !== null)
  const isWorkspaceAdmin = computed(() => currentRole.value === 'owner' || currentRole.value === 'admin')
  const canEdit = computed(() => {
    if (!currentWorkspaceId.value) return true // personal mode
    const role = currentRole.value
    return role === 'owner' || role === 'admin' || role === 'editor'
  })

  // The user's auto-provisioned personal workspace (workspace-only migration).
  const personalWorkspace = computed(() => workspaces.value.find((w) => w.is_personal) ?? null)
  // True when the active workspace is the user's personal one — used to hide
  // team-only affordances (Members, Plan & Billing).
  const currentWorkspaceIsPersonal = computed(() => currentWorkspace.value?.is_personal ?? false)

  const contextLabel = computed(() => currentWorkspace.value?.name ?? 'Personal')

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
      // After the workspace-only migration every user owns a personal workspace.
      // Land there by default (instead of legacy header-less "personal mode") so
      // the active context always maps to a real workspace.
      if (!valid) {
        setWorkspace(personalWorkspace.value?.id ?? null)
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
    isPersonal,
    isWorkspaceContext,
    isWorkspaceAdmin,
    canEdit,
    personalWorkspace,
    currentWorkspaceIsPersonal,
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
