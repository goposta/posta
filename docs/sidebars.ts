import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  docsSidebar: [
    {
      type: 'category',
      label: 'Getting Started',
      collapsed: false,
      items: [
        'getting-started/introduction',
        'getting-started/installation',
        'getting-started/configuration',
        'getting-started/quickstart',
      ],
    },
    {
      type: 'category',
      label: 'Email Sending',
      items: [
        'email-sending/single-email',
        'email-sending/template-email',
        'email-sending/batch-email',
        'email-sending/scheduled-email',
        'email-sending/attachments',
        'email-sending/email-status',
      ],
    },
    {
      type: 'category',
      label: 'Templates',
      items: [
        'templates/overview',
        'templates/creating-templates',
        'templates/versioning',
        'templates/localization',
        'templates/stylesheets',
        'templates/preview-and-test',
        'templates/import-export',
      ],
    },
    {
      type: 'category',
      label: 'Subscribers',
      items: [
        'subscribers/subscriber-management',
        'subscribers/bulk-import',
        'subscribers/subscriber-lists',
      ],
    },
    {
      type: 'category',
      label: 'Campaigns',
      items: [
        'campaigns/overview',
        'campaigns/sending',
        'campaigns/ab-testing',
      ],
    },
    {
      type: 'category',
      label: 'SMTP & Domains',
      items: [
        'smtp-domains/smtp-servers',
        'smtp-domains/domain-verification',
        'smtp-domains/shared-smtp-pool',
      ],
    },
    {
      type: 'category',
      label: 'Security',
      items: [
        'security/authentication',
        'security/api-keys',
        'security/two-factor-auth',
        'security/rate-limiting',
        'security/sessions',
      ],
    },
    {
      type: 'category',
      label: 'Contacts & Suppression',
      items: [
        'contacts/contact-management',
        'contacts/contact-lists',
        'contacts/bounce-handling',
        'contacts/suppression-list',
      ],
    },
    {
      type: 'category',
      label: 'Webhooks & Events',
      items: [
        'webhooks/overview',
        'webhooks/event-types',
        'webhooks/delivery-tracking',
        'webhooks/audit-log',
      ],
    },
    {
      type: 'category',
      label: 'Analytics & Monitoring',
      items: [
        'analytics/dashboard',
        'analytics/email-analytics',
        'analytics/prometheus-metrics',
        'analytics/health-checks',
      ],
    },
    {
      type: 'category',
      label: 'Workspaces',
      items: [
        'workspaces/overview',
      ],
    },
    {
      type: 'category',
      label: 'Admin Panel',
      items: [
        'admin/user-management',
        'admin/platform-settings',
        'admin/platform-metrics',
        'admin/shared-servers',
        'admin/scheduled-jobs',
        'admin/oauth-providers',
        'admin/plans',
      ],
    },
    {
      type: 'category',
      label: 'GDPR & Data',
      items: [
        'gdpr/data-export-import',
        'gdpr/data-deletion',
      ],
    },
    {
      type: 'category',
      label: 'SDKs',
      items: [
        'sdks/overview',
        'sdks/go',
        'sdks/php',
        'sdks/java',
      ],
    },
    {
      type: 'link',
      label: 'API Reference',
      href: 'https://app.goposta.dev/docs',
    },
  ],
};

export default sidebars;
