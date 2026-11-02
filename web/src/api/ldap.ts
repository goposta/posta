import api from './client'
import type { ApiResponse } from './types'

export type LDAPTLSMode = 'none' | 'starttls' | 'ldaps'
export type MappableRole = 'admin' | 'editor' | 'viewer'

export interface LDAPGroupMapping {
  id: number
  ldap_config_id: number
  group_dn: string
  platform_admin: boolean
  workspace_id: number | null
  workspace_role: MappableRole | ''
  created_at: string
}

export interface LDAPDirectory {
  id: number
  name: string
  display_name: string
  host: string
  port: number
  tls_mode: LDAPTLSMode
  ca_cert_pem: string
  insecure_skip_tls: boolean
  timeout_seconds: number
  bind_dn: string
  /** Whether a service password is stored; the password itself is never returned. */
  bind_password_set: boolean
  user_base_dn: string
  user_filter: string
  attr_email: string
  attr_name: string
  group_base_dn: string
  group_filter: string
  member_attr: string
  nested_groups: boolean
  enabled: boolean
  mappings?: LDAPGroupMapping[]
  created_at: string
}

export interface LDAPDirectoryInput {
  display_name: string
  host: string
  port?: number
  tls_mode: LDAPTLSMode
  ca_cert_pem?: string
  insecure_skip_tls?: boolean
  timeout_seconds?: number
  bind_dn?: string
  /** Empty on update keeps the stored password. */
  bind_password?: string
  user_base_dn: string
  user_filter?: string
  attr_email?: string
  attr_name?: string
  group_base_dn?: string
  group_filter?: string
  member_attr?: string
  nested_groups?: boolean
  enabled?: boolean
}

export interface LDAPMappingInput {
  group_dn: string
  platform_admin: boolean
  workspace_id?: number
  workspace_role?: MappableRole
}

export interface LDAPTestResult {
  ok: boolean
  message?: string
  error?: string
}

export const ldapApi = {
  list() {
    return api.get<ApiResponse<LDAPDirectory[]>>('/admin/ldap')
  },
  create(input: LDAPDirectoryInput) {
    return api.post<ApiResponse<LDAPDirectory>>('/admin/ldap', input)
  },
  update(id: number, input: LDAPDirectoryInput) {
    return api.put<ApiResponse<LDAPDirectory>>(`/admin/ldap/${id}`, input)
  },
  remove(id: number) {
    return api.delete(`/admin/ldap/${id}`)
  },
  test(id: number) {
    return api.post<ApiResponse<LDAPTestResult>>(`/admin/ldap/${id}/test`)
  },
  addMapping(id: number, input: LDAPMappingInput) {
    return api.post<ApiResponse<LDAPGroupMapping>>(`/admin/ldap/${id}/mappings`, input)
  },
  removeMapping(id: number, mappingId: number) {
    return api.delete(`/admin/ldap/${id}/mappings/${mappingId}`)
  },
}
