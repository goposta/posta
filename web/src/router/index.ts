import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const APP_NAME = 'Posta'
const DEFAULT_TITLE = 'Posta — Self-Hosted Email Delivery & Inbound Platform'

const routes = [
  {
    path: '/login',
    name: 'login',
    component: () => import('../views/auth/Login.vue'),
    meta: { guest: true, title: 'Sign in' },
  },
  {
    path: '/register',
    name: 'register',
    component: () => import('../views/auth/Register.vue'),
    meta: { guest: true, title: 'Create an account' },
  },
  {
    path: '/auth/oauth/callback',
    name: 'oauth-callback',
    component: () => import('../views/auth/OAuthCallback.vue'),
    meta: { guest: true, title: 'Signing you in…' },
  },
  {
    path: '/invitations',
    name: 'invitation-accept',
    component: () => import('../views/invitations/InvitationAccept.vue'),
    meta: { title: 'Workspace Invitation' },
  },
  {
    path: '/auth/verify-email',
    name: 'verify-email',
    component: () => import('../views/auth/VerifyEmail.vue'),
    meta: { title: 'Verify email' },
  },
  {
    path: '/',
    component: () => import('../layouts/DashboardLayout.vue'),
    meta: { auth: true },
    children: [
      { path: '', name: 'dashboard', component: () => import('../views/dashboard/Dashboard.vue'), meta: { title: 'Dashboard' } },
      { path: 'api-keys', name: 'api-keys', component: () => import('../views/apikeys/ApiKeys.vue'), meta: { title: 'API Keys' } },
      { path: 'emails', name: 'emails', component: () => import('../views/emails/Emails.vue'), meta: { title: 'Emails' } },
      { path: 'emails/:id', name: 'email-detail', component: () => import('../views/emails/EmailDetail.vue'), meta: { title: 'Email' } },
      { path: 'inbound-emails', name: 'inbound-emails', component: () => import('../views/inbound/InboundEmails.vue'), meta: { title: 'Inbound Emails' } },
      { path: 'inbound-emails/:id', name: 'inbound-email-detail', component: () => import('../views/inbound/InboundEmailDetail.vue'), meta: { title: 'Inbound Email' } },
      { path: 'templates', name: 'templates', component: () => import('../views/templates/Templates.vue'), meta: { title: 'Templates' } },
      { path: 'templates/preview', name: 'template-preview-general', component: () => import('../views/emails/EmailPreview.vue'), meta: { title: 'Template Preview' } },
      { path: 'templates/:id/preview', name: 'template-preview', component: () => import('../views/templates/TemplatePreview.vue'), meta: { title: 'Template Preview' } },
      { path: 'templates/:id/versions', name: 'template-detail', component: () => import('../views/templates/TemplateDetail.vue'), meta: { title: 'Template Versions' } },
      { path: 'templates/:id/versions/:versionId/localizations/:localizationId/edit', name: 'template-editor', component: () => import('../views/templates/TemplateEditor.vue'), meta: { title: 'Template Editor' } },
      { path: 'templates/:id/versions/:versionId/localizations/:localizationId/builder', name: 'template-builder', component: () => import('../views/templates/EmailBuilder.vue'), meta: { title: 'Email Builder' } },
      { path: 'languages', name: 'languages', component: () => import('../views/languages/Languages.vue'), meta: { title: 'Languages' } },
      { path: 'stylesheets', name: 'stylesheets', component: () => import('../views/stylesheets/Stylesheets.vue'), meta: { title: 'Stylesheets' } },
      { path: 'smtp-servers', name: 'smtp-servers', component: () => import('../views/smtp/SmtpServers.vue'), meta: { title: 'SMTP Servers' } },
      { path: 'smtp-servers/:id', name: 'smtp-server-detail', component: () => import('../views/smtp/SmtpServerDetail.vue'), meta: { title: 'SMTP Server' } },
      { path: 'domains', name: 'domains', component: () => import('../views/domains/Domains.vue'), meta: { title: 'Domains' } },
      { path: 'webhooks', name: 'webhooks', component: () => import('../views/webhooks/Webhooks.vue'), meta: { title: 'Webhooks' } },
      { path: 'webhook-deliveries', name: 'webhook-deliveries', component: () => import('../views/webhooks/WebhookDeliveries.vue'), meta: { title: 'Webhook Deliveries' } },
      { path: 'bounces', name: 'bounces', component: () => import('../views/bounces/Bounces.vue'), meta: { title: 'Bounces' } },
      { path: 'contacts', name: 'contacts', component: () => import('../views/contacts/Contacts.vue'), meta: { title: 'Contacts' } },
      { path: 'contacts/:id', name: 'contact-detail', component: () => import('../views/contacts/ContactDetail.vue'), meta: { title: 'Contact' } },
      { path: 'subscribers', name: 'subscribers', component: () => import('../views/subscribers/Subscribers.vue'), meta: { title: 'Subscribers' } },
      { path: 'subscribers/:id', name: 'subscriber-detail', component: () => import('../views/subscribers/SubscriberDetail.vue'), meta: { title: 'Subscriber' } },
      { path: 'subscriber-lists', name: 'subscriber-lists-page', component: () => import('../views/subscriber-lists/SubscriberLists.vue'), meta: { title: 'Subscriber Lists' } },
      { path: 'subscriber-lists/:id', name: 'subscriber-list-detail', component: () => import('../views/subscriber-lists/SubscriberListDetail.vue'), meta: { title: 'Subscriber List' } },
      { path: 'campaigns', name: 'campaigns', component: () => import('../views/campaigns/Campaigns.vue'), meta: { title: 'Campaigns' } },
      { path: 'campaigns/:id', name: 'campaign-detail', component: () => import('../views/campaigns/CampaignDetail.vue'), meta: { title: 'Campaign' } },
      { path: 'analytics', name: 'analytics', component: () => import('../views/analytics/Analytics.vue'), meta: { title: 'Analytics' } },
      { path: 'audit-log', name: 'audit-log', component: () => import('../views/audit/AuditLog.vue'), meta: { title: 'Audit Log' } },
      { path: 'settings', name: 'settings', component: () => import('../views/settings/Settings.vue'), meta: { title: 'Settings' } },
      { path: 'workspaces', name: 'workspaces', component: () => import('../views/workspaces/Workspaces.vue'), meta: { title: 'Workspaces' } },
      { path: 'workspaces/:id', name: 'workspace-detail', component: () => import('../views/workspaces/WorkspaceSettings.vue'), meta: { title: 'Workspace Settings' } },
      { path: 'about', name: 'about', component: () => import('../views/about/About.vue'), meta: { title: 'About' } },
      { path: 'profile', name: 'profile', component: () => import('../views/auth/Profile.vue'), meta: { title: 'Profile' } },
      { path: 'change-password', redirect: '/profile' },
      // Admin
      { path: 'admin/users', name: 'admin-users', component: () => import('../views/admin/Users.vue'), meta: { admin: true, title: 'Admin · Users' } },
      { path: 'admin/users/:id', name: 'admin-user-detail', component: () => import('../views/admin/UserDetail.vue'), meta: { admin: true, title: 'Admin · User' } },
      { path: 'admin/metrics', name: 'admin-metrics', component: () => import('../views/admin/Metrics.vue'), meta: { admin: true, title: 'Admin · Metrics' } },
      { path: 'admin/events', name: 'admin-events', component: () => import('../views/admin/Events.vue'), meta: { admin: true, title: 'Admin · Events' } },
      { path: 'admin/plans', name: 'admin-plans', component: () => import('../views/admin/Plans.vue'), meta: { admin: true, title: 'Admin · Plans' } },
      { path: 'admin/plans/:id', name: 'admin-plan-detail', component: () => import('../views/admin/PlanDetail.vue'), meta: { admin: true, title: 'Admin · Plan' } },
      { path: 'admin/servers', name: 'admin-servers', component: () => import('../views/admin/Servers.vue'), meta: { admin: true, title: 'Admin · Servers' } },
      { path: 'admin/servers/:id', name: 'admin-server-detail', component: () => import('../views/admin/ServerDetail.vue'), meta: { admin: true, title: 'Admin · Server' } },
      { path: 'admin/jobs', name: 'admin-jobs', component: () => import('../views/admin/Jobs.vue'), meta: { admin: true, title: 'Admin · Jobs' } },
      { path: 'admin/oauth', name: 'admin-oauth', component: () => import('../views/admin/OAuthProviders.vue'), meta: { admin: true, title: 'Admin · OAuth Providers' } },
      { path: 'admin/settings', name: 'admin-settings', component: () => import('../views/admin/Settings.vue'), meta: { admin: true, title: 'Admin · Settings' } },
    ],
  },
  {
    path: '/:pathMatch(.*)*',
    name: 'not-found',
    component: () => import('../views/NotFound.vue'),
    meta: { title: 'Page not found' },
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to) => {
  const auth = useAuthStore()

  if (to.meta.auth && !auth.isAuthenticated) {
    return { name: 'login' }
  }
  if (to.meta.guest && auth.isAuthenticated) {
    return { name: 'dashboard' }
  }
  if (to.meta.admin && !auth.isAdmin) {
    return { name: 'dashboard' }
  }
})

router.afterEach((to) => {
  const pageTitle = to.meta.title as string | undefined
  document.title = pageTitle ? `${pageTitle} · ${APP_NAME}` : DEFAULT_TITLE
})

export default router
