import api from './client'
import type { ApiResponse, PaginatedResponse, Domain } from './types'

export const domainsApi = {
  list(page = 0, size = 20) {
    return api.get<PaginatedResponse<Domain>>('/workspaces/current/domains', { params: { page, size } })
  },
  get(id: number) {
    return api.get<ApiResponse<Domain>>(`/workspaces/current/domains/${id}`)
  },
  create(domain: string) {
    return api.post<ApiResponse<Domain>>('/workspaces/current/domains', { domain })
  },
  verify(id: number) {
    return api.post<ApiResponse<Domain>>(`/workspaces/current/domains/${id}/verify`)
  },
  delete(id: number) {
    return api.delete(`/workspaces/current/domains/${id}`)
  },
}
