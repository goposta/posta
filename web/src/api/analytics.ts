import api from './client'
import type { ApiResponse, AnalyticsResponse, DashboardAnalyticsResponse, ProviderBreakdownResponse } from './types'

export const analyticsApi = {
  user(from?: string, to?: string, status?: string) {
    const params: Record<string, string> = {}
    if (from) params.from = from
    if (to) params.to = to
    if (status) params.status = status
    return api.get<ApiResponse<AnalyticsResponse>>('/workspaces/current/analytics', { params })
  },
  admin(from?: string, to?: string, status?: string) {
    const params: Record<string, string> = {}
    if (from) params.from = from
    if (to) params.to = to
    if (status) params.status = status
    return api.get<ApiResponse<AnalyticsResponse>>('/admin/analytics', { params })
  },
  dashboardAnalytics(from?: string, to?: string) {
    const params: Record<string, string> = {}
    if (from) params.from = from
    if (to) params.to = to
    return api.get<ApiResponse<DashboardAnalyticsResponse>>('/workspaces/current/analytics/dashboard', { params })
  },
  adminDashboardAnalytics(from?: string, to?: string) {
    const params: Record<string, string> = {}
    if (from) params.from = from
    if (to) params.to = to
    return api.get<ApiResponse<DashboardAnalyticsResponse>>('/admin/analytics/dashboard', { params })
  },
  providerBreakdown(from?: string, to?: string) {
    const params: Record<string, string> = {}
    if (from) params.from = from
    if (to) params.to = to
    return api.get<ApiResponse<ProviderBreakdownResponse>>('/workspaces/current/analytics/providers', { params })
  },
  adminProviderBreakdown(from?: string, to?: string) {
    const params: Record<string, string> = {}
    if (from) params.from = from
    if (to) params.to = to
    return api.get<ApiResponse<ProviderBreakdownResponse>>('/admin/analytics/providers', { params })
  },
}
