import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const routes = [
  {
    path: '/login',
    name: 'login',
    component: () => import('../views/auth/Login.vue'),
    meta: { guest: true },
  },
  {
    path: '/register',
    name: 'register',
    component: () => import('../views/auth/Register.vue'),
    meta: { guest: true },
  },
  {
    path: '/auth/oauth/callback',
    name: 'oauth-callback',
    component: () => import('../views/auth/OAuthCallback.vue'),
    meta: { guest: true },
  },
  {
    path: '/',
    component: () => import('../layouts/DashboardLayout.vue'),
    meta: { auth: true },
    children: [
      { path: '', name: 'dashboard', component: () => import('../views/dashboard/Dashboard.vue') },
      { path: 'api-keys', name: 'api-keys', component: () => import('../views/apikeys/ApiKeys.vue') },
      { path: 'emails', name: 'emails', component: () => import('../views/emails/Emails.vue') },
      { path: 'emails/preview', name: 'email-preview', component: () => import('../views/emails/EmailPreview.vue') },
      { path: 'emails/:id', name: 'email-detail', component: () => import('../views/emails/EmailDetail.vue') },
      { path: 'templates', name: 'templates', component: () => import('../views/templates/Templates.vue') },
      { path: 'templates/:id/preview', name: 'template-preview', component: () => import('../views/templates/TemplatePreview.vue') },
      { path: 'templates/:id/versions', name: 'template-detail', component: () => import('../views/templates/TemplateDetail.vue') },
      { path: 'templates/:id/versions/:versionId/localizations/:localizationId/edit', name: 'template-editor', component: () => import('../views/templates/TemplateEditor.vue') },
      { path: 'templates/:id/versions/:versionId/localizations/:localizationId/builder', name: 'template-builder', component: () => import('../views/templates/EmailBuilder.vue') },
      { path: 'languages', name: 'languages', component: () => import('../views/languages/Languages.vue') },
      { path: 'stylesheets', name: 'stylesheets', component: () => import('../views/stylesheets/Stylesheets.vue') },
      { path: 'smtp-servers', name: 'smtp-servers', component: () => import('../views/smtp/SmtpServers.vue') },
      { path: 'smtp-servers/:id', name: 'smtp-server-detail', component: () => import('../views/smtp/SmtpServerDetail.vue') },
      { path: 'domains', name: 'domains', component: () => import('../views/domains/Domains.vue') },
      { path: 'webhooks', name: 'webhooks', component: () => import('../views/webhooks/Webhooks.vue') },
      { path: 'webhook-deliveries', name: 'webhook-deliveries', component: () => import('../views/webhooks/WebhookDeliveries.vue') },
      { path: 'bounces', name: 'bounces', component: () => import('../views/bounces/Bounces.vue') },
      { path: 'contacts', name: 'contacts', component: () => import('../views/contacts/Contacts.vue') },
      { path: 'contacts/:id', name: 'contact-detail', component: () => import('../views/contacts/ContactDetail.vue') },
      { path: 'subscribers', name: 'subscribers', component: () => import('../views/subscribers/Subscribers.vue') },
      { path: 'subscribers/:id', name: 'subscriber-detail', component: () => import('../views/subscribers/SubscriberDetail.vue') },
      { path: 'subscriber-lists', name: 'subscriber-lists-page', component: () => import('../views/subscriber-lists/SubscriberLists.vue') },
      { path: 'subscriber-lists/:id', name: 'subscriber-list-detail', component: () => import('../views/subscriber-lists/SubscriberListDetail.vue') },
      { path: 'campaigns', name: 'campaigns', component: () => import('../views/campaigns/Campaigns.vue') },
      { path: 'campaigns/:id', name: 'campaign-detail', component: () => import('../views/campaigns/CampaignDetail.vue') },
      { path: 'analytics', name: 'analytics', component: () => import('../views/analytics/Analytics.vue') },
      { path: 'audit-log', name: 'audit-log', component: () => import('../views/audit/AuditLog.vue') },
      { path: 'settings', name: 'settings', component: () => import('../views/settings/Settings.vue') },
      { path: 'workspaces', name: 'workspaces', component: () => import('../views/workspaces/Workspaces.vue') },
      { path: 'workspaces/:id', name: 'workspace-detail', component: () => import('../views/workspaces/WorkspaceSettings.vue') },
      { path: 'about', name: 'about', component: () => import('../views/about/About.vue') },
      { path: 'profile', name: 'profile', component: () => import('../views/auth/Profile.vue') },
      { path: 'change-password', redirect: '/profile' },
      // Admin
      { path: 'admin/users', name: 'admin-users', component: () => import('../views/admin/Users.vue'), meta: { admin: true } },
      { path: 'admin/users/:id', name: 'admin-user-detail', component: () => import('../views/admin/UserDetail.vue'), meta: { admin: true } },
      { path: 'admin/metrics', name: 'admin-metrics', component: () => import('../views/admin/Metrics.vue'), meta: { admin: true } },
      { path: 'admin/events', name: 'admin-events', component: () => import('../views/admin/Events.vue'), meta: { admin: true } },
      { path: 'admin/plans', name: 'admin-plans', component: () => import('../views/admin/Plans.vue'), meta: { admin: true } },
      { path: 'admin/plans/:id', name: 'admin-plan-detail', component: () => import('../views/admin/PlanDetail.vue'), meta: { admin: true } },
      { path: 'admin/servers', name: 'admin-servers', component: () => import('../views/admin/Servers.vue'), meta: { admin: true } },
      { path: 'admin/servers/:id', name: 'admin-server-detail', component: () => import('../views/admin/ServerDetail.vue'), meta: { admin: true } },
      { path: 'admin/jobs', name: 'admin-jobs', component: () => import('../views/admin/Jobs.vue'), meta: { admin: true } },
      { path: 'admin/oauth', name: 'admin-oauth', component: () => import('../views/admin/OAuthProviders.vue'), meta: { admin: true } },
      { path: 'admin/settings', name: 'admin-settings', component: () => import('../views/admin/Settings.vue'), meta: { admin: true } },
    ],
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

export default router
