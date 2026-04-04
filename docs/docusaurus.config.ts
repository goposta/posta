import {themes as prismThemes} from 'prism-react-renderer';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const config: Config = {
  title: 'Posta',
  tagline: 'Self-hosted email delivery platform for developers',
  favicon: 'img/favicon.ico',

  future: {
    v4: true,
  },

  url: 'https://docs.posta.dev',
  baseUrl: '/',

  organizationName: 'goposta',
  projectName: 'posta',

  onBrokenLinks: 'warn',

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  presets: [
    [
      'classic',
      {
        docs: {
          sidebarPath: './sidebars.ts',
          editUrl: 'https://github.com/goposta/posta/tree/main/docs/',
          routeBasePath: 'docs',
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  themeConfig: {
    image: 'img/posta-social-card.png',
    colorMode: {
      defaultMode: 'dark',
      respectPrefersColorScheme: true,
    },
    navbar: {
      title: 'Posta',
      logo: {
        alt: 'Posta Logo',
        src: 'img/logo.png',
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'docsSidebar',
          position: 'left',
          label: 'Documentation',
        },
        {
          href: 'https://app.goposta.dev/docs',
          label: 'API Reference',
          position: 'left',
        },
        {
          to: '/docs/sdks/overview',
          label: 'SDKs',
          position: 'left',
        },
        {
          href: 'https://github.com/goposta/posta',
          label: 'GitHub',
          position: 'right',
        },
      ],
    },
    footer: {
      style: 'dark',
      links: [
        {
          title: 'Documentation',
          items: [
            {label: 'Getting Started', to: '/docs/getting-started/introduction'},
            {label: 'API Reference', href: 'https://app.goposta.dev/docs'},
            {label: 'SDKs', to: '/docs/sdks/overview'},
          ],
        },
        {
          title: 'Features',
          items: [
            {label: 'Email Sending', to: '/docs/email-sending/single-email'},
            {label: 'Templates', to: '/docs/templates/overview'},
            {label: 'Webhooks', to: '/docs/webhooks/overview'},
          ],
        },
        {
          title: 'More',
          items: [
            {
              label: 'GitHub',
              href: 'https://github.com/goposta/posta',
            },
            {
              label: 'Website',
              href: 'https://posta.dev',
            },
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} Jonas Kaninda. Licensed under Apache 2.0.`,
    },
    prism: {
      theme: prismThemes.github,
      darkTheme: prismThemes.dracula,
      additionalLanguages: ['bash', 'json', 'go', 'php', 'java', 'rust', 'yaml', 'toml'],
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
