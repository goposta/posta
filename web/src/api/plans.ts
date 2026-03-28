import api from './client'
import type { ApiResponse, PaginatedResponse, Plan, PlanInput } from './types'

export const plansApi = {
  list(page = 0, size = 20) {
    return api.get<PaginatedResponse<Plan>>('/admin/plans', { params: { page, size } })
  },
  get(id: number) {
    return api.get<ApiResponse<Plan>>(`/admin/plans/${id}`)
  },
  create(data: PlanInput) {
    return api.post<ApiResponse<Plan>>('/admin/plans', data)
  },
  update(id: number, data: Partial<PlanInput> & { is_active?: boolean; is_default?: boolean }) {
    return api.put<ApiResponse<Plan>>(`/admin/plans/${id}`, data)
  },
  delete(id: number, force = false) {
    return api.delete(`/admin/plans/${id}`, { params: force ? { force: true } : {} })
  },
  setDefault(id: number) {
    return api.patch<ApiResponse<Plan>>(`/admin/plans/${id}/default`)
  },
  assignToWorkspace(workspaceId: number, planId: number) {
    return api.post<ApiResponse<{ message: string }>>(`/admin/workspaces/${workspaceId}/plan`, { plan_id: planId })
  },
  getWorkspacePlan(workspaceId: number) {
    return api.get<ApiResponse<Plan | null>>(`/admin/workspaces/${workspaceId}/plan`)
  },
}
