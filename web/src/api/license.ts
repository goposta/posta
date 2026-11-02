import api from './client'
import type { ApiResponse } from './types'

/** none | valid | grace | degraded | binding_mismatch | unknown_signing_key */
export type LicenseState = 'none' | 'valid' | 'grace' | 'degraded' | 'binding_mismatch' | 'unknown_signing_key'

/** Whether Posta Enterprise features are on, for any signed-in user. */
export interface Entitlements {
  edition: 'community' | 'enterprise'
  state: LicenseState
  /** Enterprise features run. */
  licensed: boolean
  /** Their settings may change: false once a license has expired past its grace period. */
  mutable: boolean
}

/** The installed license, as a platform admin sees it. */
export interface LicenseView {
  edition: 'community' | 'enterprise'
  state: LicenseState
  /** The edition this binary was built as; a community build cannot install a license. */
  build: 'community' | 'enterprise'
  license_id?: string
  customer?: string
  trial?: boolean
  /** The Install ID the license is bound to. */
  install_id?: string
  url?: string
  binding_error?: 'install_id' | 'url'
  not_after?: string
  grace_ends?: string
  /** This installation's Install ID, to give when buying a license. */
  instance_install_id: string
  warnings: string[]
}

export const licenseApi = {
  entitlements: () => api.get<ApiResponse<Entitlements>>('/system/license/entitlements'),
  get: () => api.get<ApiResponse<LicenseView>>('/admin/license'),
  install: (token: string) => api.post<ApiResponse<LicenseView>>('/admin/license', { token }),
  remove: () => api.delete<ApiResponse<LicenseView>>('/admin/license'),
}

/** Where to learn about and buy Posta Enterprise. */
export const ENTERPRISE_URL = 'https://goposta.dev/enterprise'
export const SALES_EMAIL = 'sales@goposta.dev'
