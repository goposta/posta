// API Response types
export interface ApiResponse<T> {
  success: boolean
  data: T
  error?: ApiError
}

export interface ApiError {
  code: string
  message: string
  error: string
}

export interface Pageable {
  current_page: number
  size: number
  total_pages: number
  total_elements: number
  empty: boolean
}

export interface PaginatedResponse<T> {
  success: boolean
  data: T[]
  pageable: Pageable
}

// Models
export interface User {
  id: number
  name: string
  email: string
  role: 'admin' | 'user'
  active: boolean
  two_factor_enabled: boolean
  created_at: string
  last_login_at: string | null
}

export interface AuthResponse {
  token: string
  user: User
}

export interface Email {
  id: number
  uuid: string
  user_id: number
  api_key_id: number
  smtp_hostname: string | null
  sender: string
  recipients: string[]
  subject: string
  html_body: string
  text_body: string
  attachments_json: string
  headers_json: string
  list_unsubscribe_url: string
  list_unsubscribe_post: boolean
  status: 'pending' | 'queued' | 'processing' | 'sent' | 'failed' | 'suppressed' | 'scheduled'
  error_message: string
  retry_count: number
  created_at: string
  sent_at: string | null
  scheduled_at: string | null
}

export interface ApiKey {
  id: number
  user_id: number
  name: string
  key_prefix: string
  created_at: string
  expires_at: string | null
  last_used_at: string | null
  revoked: boolean
  allowed_ips: string[] | null
}

export interface Contact {
  id: number
  user_id: number
  email: string
  name: string
  sent_count: number
  fail_count: number
  suppressed: boolean
  last_sent_at: string | null
  created_at: string
}

export interface ApiKeyCreateResponse {
  key: string
  id: number
  name: string
  prefix: string
  expires_at: string | null
  message: string
}

export interface Template {
  id: number
  user_id: number
  name: string
  default_language: string
  active_version_id?: number | null
  description: string
  active_version?: TemplateVersion | null
  sample_data: string
  created_at: string
  updated_at?: string
}

export interface TemplateInput {
  name: string
  sample_data?: string
  default_language?: string
  description?: string
}

export interface TemplateExportLocalization {
  language: string
  subject_template: string
  html_template: string
  text_template: string
}

export interface TemplateExportVersion {
  version: number
  sample_data: string
  is_active: boolean
  localizations: TemplateExportLocalization[]
}

export interface TemplateExport {
  posta_version?: string
  exported_at?: string
  name: string
  description: string
  default_language: string
  sample_data: string
  versions: TemplateExportVersion[]
}

export interface TemplatePreview {
  subject: string
  html: string
  text: string
}

export interface SendTestInput {
  to: string[]
  from?: string
  language?: string
  template_data?: Record<string, any>
}

export interface SendTestResponse {
  id: string
  status: string
}

export interface TemplateVersion {
  id: number
  template_id: number
  version: number
  stylesheet_id?: number | null
  stylesheet?: StyleSheet | null
  localizations?: TemplateLocalization[] | null
  sample_data: string
  created_at: string
}

export interface TemplateVersionInput {
  stylesheet_id?: number | null
  sample_data?: string
}

export interface TemplateLocalization {
  id: number
  version_id: number
  language: string
  subject_template: string
  html_template: string
  text_template: string
  created_at: string
  updated_at?: string
}

export interface TemplateLocalizationInput {
  language: string
  subject_template: string
  html_template: string
  text_template: string
}

export interface Language {
  id: number
  user_id: number
  code: string
  name: string
  created_at: string
}

export interface LanguageInput {
  code: string
  name: string
}

export interface StyleSheet {
  id: number
  user_id: number
  name: string
  css: string
  created_at: string
  updated_at?: string
}

export interface StyleSheetInput {
  name: string
  css: string
}

export type ServerStatus = 'enabled' | 'disabled' | 'invalid'

export interface SharedServer {
  id: number
  name: string
  host: string
  port: number
  username: string
  encryption: 'none' | 'starttls' | 'ssl'
  max_retries: number
  status: ServerStatus
  allowed_domains: string[]
  security_mode: 'permissive' | 'strict'
  sent_count: number
  failed_count: number
  validation_error: string
  validated_at: string | null
  created_at: string
  updated_at: string
}

export interface SharedServerInput {
  name: string
  host: string
  port: number
  username?: string
  password?: string
  encryption?: 'none' | 'starttls' | 'ssl'
  max_retries?: number
  status?: ServerStatus
  allowed_domains?: string[]
  security_mode?: 'permissive' | 'strict'
}

export interface SmtpServer {
  id: number
  user_id: number
  host: string
  port: number
  username: string
  password: string
  encryption: 'none' | 'starttls' | 'ssl'
  max_retries: number
  allowed_emails: string[]
  status: ServerStatus
  validation_error: string
  validated_at: string | null
  created_at: string
}

export interface SmtpServerInput {
  host: string
  port: number
  username: string
  password: string
  encryption: 'none' | 'starttls' | 'ssl'
  max_retries?: number
  allowed_emails?: string[]
  status?: ServerStatus
}

export interface Domain {
  id: number
  user_id: number
  domain: string
  ownership_verified: boolean
  spf_verified: boolean
  dkim_verified: boolean
  dmarc_verified: boolean
  verification_token: string
  created_at: string
  dns_records?: DnsRecords
}

export interface DnsRecords {
  verification: DnsRecord
  spf: DnsRecord
  dkim: DnsRecord
  dmarc: DnsRecord
}

export interface DnsRecord {
  type: string
  name: string
  value: string
}

export interface Webhook {
  id: number
  user_id: number
  url: string
  events: string[]
  filters: string[] | null
  secret?: string
  created_at: string
}

export interface WebhookInput {
  url: string
  events: string[]
  filters: string[]
}

export interface Bounce {
  id: number
  user_id: number
  email_id: number
  recipient: string
  type: 'hard' | 'soft' | 'complaint'
  reason: string
  created_at: string
}

export interface Suppression {
  id: number
  user_id: number
  email: string
  reason: string
  created_at: string
}

export interface WebhookDelivery {
  id: number
  webhook_id: number
  user_id: number
  event: string
  status: 'success' | 'failed'
  http_status_code: number
  error_message?: string
  attempt: number
  created_at: string
}

export interface WebhookDeliveryStats {
  total_deliveries: number
  success_deliveries: number
  failed_deliveries: number
  success_rate: number
}

export interface DashboardStats {
  total_emails: number
  queued_emails: number
  processing_emails: number
  sent_emails: number
  failed_emails: number
  suppressed_emails: number
  failure_rate: number
  total_domains: number
  total_smtp_servers: number
  total_api_keys: number
  active_api_keys: number
  total_contacts: number
  total_bounces: number
  total_suppressions: number
  total_webhooks: number
  total_contact_lists: number
  daily_volume: DailyVolume[]
  webhook_deliveries: WebhookDeliveryStats | null
}

export interface DailyVolume {
  date: string
  sent: number
  failed: number
}

export interface AdminMetrics {
  total_users: number
  total_emails: number
  queued_emails: number
  processing_emails: number
  sent_emails: number
  failed_emails: number
  suppressed_emails: number
  failure_rate: number
  total_api_keys: number
  active_api_keys: number
  total_bounces: number
  total_suppressions: number
  active_workers: number
  shared_smtp_servers: number
  total_domains: number
  webhook_deliveries: WebhookDeliveryStats | null
}

export interface WorkerStatus {
  active_workers: number
  workers: WorkerDetail[]
}

export interface WorkerDetail {
  host: string
  pid: number
  queues: Record<string, number>
  type: 'embedded' | 'standalone'
}

export interface Event {
  id: number
  category: 'user' | 'email' | 'system'
  type: string
  actor_id: number | null
  actor_name: string
  client_ip?: string
  message: string
  metadata: string
  created_at: string
}

export interface UserDetailMetrics {
  user: User
  total_emails: number
  sent_emails: number
  failed_emails: number
  suppressed_emails: number
  failure_rate: number
  total_api_keys: number
  active_api_keys: number
  total_contacts: number
  total_bounces: number
  total_suppressions: number
  total_domains: number
  total_smtp_servers: number
  webhook_deliveries: WebhookDeliveryStats | null
}

// Analytics
export interface DailyCount {
  date: string
  count: number
}

export interface StatusBreakdown {
  status: string
  count: number
}

export interface AnalyticsResponse {
  daily_counts: DailyCount[]
  status_breakdown: StatusBreakdown[]
}

export interface DeliveryRatePoint {
  date: string
  sent: number
  failed: number
  total: number
  delivery_rate: number
}

export interface BounceRatePoint {
  date: string
  hard: number
  soft: number
  complaint: number
  total: number
}

export interface LatencyPercentiles {
  p50: number
  p75: number
  p90: number
  p99: number
  avg: number
}

export interface DashboardAnalyticsResponse {
  delivery_rate_trends: DeliveryRatePoint[]
  bounce_rate_trends: BounceRatePoint[]
  latency_percentiles: LatencyPercentiles
}

// Contact Lists
export interface ContactList {
  id: number
  user_id: number
  name: string
  description: string
  created_at: string
  updated_at: string
}

export interface ContactListWithCount extends ContactList {
  member_count: number
}

export interface ContactListMember {
  id: number
  list_id: number
  email: string
  name: string
  created_at: string
}

// Settings
export interface AdminSetting {
  id: number
  key: string
  value: string
  type: 'string' | 'int' | 'bool'
  created_at: string
  updated_at: string
}

export interface AdminSettingInput {
  key: string
  value: string
  type: string
}

export interface UserSettings {
  id: number
  user_id: number
  timezone: string
  default_sender_name: string
  default_sender_email: string
  email_notifications: boolean
  notification_email: string
  webhook_retry_count: number
  default_template_id: number | null
  api_key_expiry_days: number
  bounce_auto_suppress: boolean
  daily_report: boolean
  created_at: string
  updated_at: string
}

// Cron Jobs
export interface CronJob {
  name: string
  schedule: string
  running: boolean
  last_run_at: string | null
  last_error?: string
  next_run_at: string | null
}

// 2FA
export interface Setup2FAResponse {
  secret: string
  url: string
}

// User Profile (extended)
export interface UserProfile {
  id: number
  name: string
  email: string
  role: 'admin' | 'user'
  two_factor_enabled: boolean
  require_verified_domain: boolean
  created_at: string
}

// User Data Export/Import
export interface UserDataExport {
  posta_version?: string
  exported_at?: string
  templates: TemplateExport[]
  stylesheets: ExportStyleSheet[]
  languages: ExportLanguage[]
  contacts: ExportContact[]
  contact_lists: ExportContactList[]
  suppressions: ExportSuppression[]
  webhooks: ExportWebhook[]
  settings?: ExportUserSettings
}

export interface ExportStyleSheet {
  name: string
  css: string
}

export interface ExportLanguage {
  code: string
  name: string
}

export interface ExportContact {
  email: string
  name: string
  sent_count: number
  fail_count: number
}

export interface ExportContactList {
  name: string
  description: string
  members: ExportContactMember[]
}

export interface ExportContactMember {
  email: string
  name: string
  data?: string
}

export interface ExportSuppression {
  email: string
  reason: string
}

export interface ExportWebhook {
  url: string
  events: string[]
  filters?: string[]
}

export interface ExportUserSettings {
  timezone: string
  default_sender_name: string
  default_sender_email: string
  email_notifications: boolean
  notification_email: string
  webhook_retry_count: number
  api_key_expiry_days: number
  bounce_auto_suppress: boolean
  daily_report: boolean
}

export interface GDPRDeleteResult {
  deleted: number
  message: string
}
