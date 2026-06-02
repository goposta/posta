import type {ReactNode} from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';
import CodeBlock from '@theme/CodeBlock';

import styles from './index.module.css';

const GITHUB_URL = 'https://github.com/goposta/posta';

const SEND_EXAMPLE = `curl -X POST https://posta.example.com/api/v1/emails/send \\
  -H "Authorization: Bearer YOUR_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '{
    "from": "Acme <hello@example.com>",
    "to": ["jonas@example.com", "bob@example.com"],
    "subject": "Hello from Posta",
    "html": "<h1>Hello!</h1>"
  }'`;

const SEND_RESPONSE = `{
  "id": "0ae4b04e-5c64-4b2f-bad6-460f8d5d98b3",
  "status": "queued"
}`;

function HomepageHeader() {
  return (
    <header className={styles.hero}>
      <div className={styles.heroGlow} aria-hidden="true" />
      <div className={clsx('container', styles.heroInner)}>
        <span className={styles.badge}>
          <span className={styles.badgeDot} aria-hidden="true" />
          Open Source &amp; Self-Hosted
        </span>

        <Heading as="h1" className={styles.heroTitle}>
          Self-Hosted{' '}
          <span className={styles.gradientText}>Email Delivery</span> Platform
        </Heading>

        <p className={styles.heroSubtitle}>
          Send and receive emails through a simple HTTP API. Posta handles SMTP
          delivery, inbound mail, templates, campaigns, tracking, webhooks, and
          security — on your own infrastructure.
        </p>

        <div className={styles.buttons}>
          <Link
            className="button button--primary button--lg"
            to="/docs/getting-started/introduction">
            Get Started
          </Link>
          <Link
            className="button button--secondary button--lg"
            to="/docs/getting-started/quickstart">
            Quick Start
          </Link>
          <Link
            className={clsx('button button--secondary button--outline button--lg', styles.githubButton)}
            href={GITHUB_URL}>
            <svg className={styles.githubIcon} width="20" height="20" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
              <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z" />
            </svg>
            GitHub
          </Link>
        </div>

        <div className={styles.stats}>
          {[
            {value: 'API-First', label: 'Architecture'},
            {value: '3 SDKs', label: 'Go · PHP · Java'},
            {value: 'In & Outbound', label: 'SMTP Delivery'},
            {value: 'GDPR', label: 'Compliant'},
          ].map((stat) => (
            <div key={stat.value} className={styles.statItem}>
              <div className={styles.statValue}>{stat.value}</div>
              <div className={styles.statLabel}>{stat.label}</div>
            </div>
          ))}
        </div>
      </div>
    </header>
  );
}

function QuickStart() {
  return (
    <section className={styles.section}>
      <div className="container">
        <div className={styles.sectionHeader}>
          <p className={styles.eyebrow}>Simple API</p>
          <Heading as="h2" className={styles.sectionTitle}>
            Send your first email in seconds
          </Heading>
          <p className={styles.sectionLead}>
            A single HTTP request is all you need. Posta handles queuing, SMTP
            delivery, retries, and tracking automatically.
          </p>
        </div>

        <div className={styles.codeColumns}>
          <CodeBlock language="bash" title="Send an email">
            {SEND_EXAMPLE}
          </CodeBlock>
          <CodeBlock language="json" title="Response">
            {SEND_RESPONSE}
          </CodeBlock>
        </div>

        <div className={styles.codeCta}>
          <Link to="/docs/getting-started/quickstart">
            Read the full quickstart guide →
          </Link>
        </div>
      </div>
    </section>
  );
}

type FeatureItem = {
  title: string;
  description: string;
  link: string;
};

const FEATURES: FeatureItem[] = [
  {
    title: 'Email Delivery',
    description: 'REST API for transactional, batch, and templated sends with attachments, scheduling, retries, and priority queues.',
    link: '/docs/email-sending/single-email',
  },
  {
    title: 'Inbound Email',
    description: 'Built-in SMTP receiver with TLS plus HMAC webhook ingest, attachment storage, forwarding, and a real-time SSE stream.',
    link: '/docs/inbound/overview',
  },
  {
    title: 'Templates & Localization',
    description: 'Version-controlled templates with multi-language fallback, variable substitution, and CSS inlining.',
    link: '/docs/templates/overview',
  },
  {
    title: 'Campaigns',
    description: 'Bulk sending with subscriber targeting, timezone-aware scheduling, A/B testing, and send-rate throttling.',
    link: '/docs/campaigns/overview',
  },
  {
    title: 'SMTP & Domains',
    description: 'Configure multiple SMTP providers, share pools across teams, and verify domains with SPF, DKIM, and DMARC.',
    link: '/docs/smtp-domains/smtp-servers',
  },
  {
    title: 'Security',
    description: 'API keys with IP allowlists, JWT auth, two-factor authentication, OAuth/SSO, and rate limiting.',
    link: '/docs/security/authentication',
  },
  {
    title: 'Contacts & Subscribers',
    description: 'Auto-tracked contacts, static and dynamic lists, bulk CSV/JSON import, and automatic suppression handling.',
    link: '/docs/subscribers/subscriber-management',
  },
  {
    title: 'Tracking & Analytics',
    description: 'Pixel-based open and click tracking, per-email engagement metrics, Prometheus integration, and dashboards.',
    link: '/docs/tracking/overview',
  },
  {
    title: 'Webhooks & Events',
    description: 'Event-driven delivery with HMAC signatures, retry strategies, delivery tracking, and audit logs.',
    link: '/docs/webhooks/overview',
  },
  {
    title: 'Workspaces & RBAC',
    description: 'Multi-tenant isolated workspaces with role-based access, member invitations, and workspace-scoped API keys.',
    link: '/docs/workspaces/overview',
  },
  {
    title: 'Official SDKs',
    description: 'Client libraries for Go, PHP, and Java with typed errors and full API coverage.',
    link: '/docs/sdks/overview',
  },
  {
    title: 'GDPR & Compliance',
    description: 'Per-contact data export, import, and deletion tools to keep your sending compliant.',
    link: '/docs/gdpr/data-export-import',
  },
];

function Feature({title, description, link}: FeatureItem) {
  return (
    <Link to={link} className={styles.card}>
      <Heading as="h3" className={styles.cardTitle}>{title}</Heading>
      <p className={styles.cardDescription}>{description}</p>
      <span className={styles.cardArrow} aria-hidden="true">→</span>
    </Link>
  );
}

function Features() {
  return (
    <section className={clsx(styles.section, styles.sectionMuted)}>
      <div className="container">
        <div className={styles.sectionHeader}>
          <p className={styles.eyebrow}>Features</p>
          <Heading as="h2" className={styles.sectionTitle}>
            Everything you need to deliver email
          </Heading>
          <p className={styles.sectionLead}>
            A complete email infrastructure platform — outbound and inbound
            delivery, templates, campaigns, contacts, tracking, webhooks,
            workspaces, security, and a built-in dashboard.
          </p>
        </div>

        <div className={styles.featureGrid}>
          {FEATURES.map((feature) => (
            <Feature key={feature.title} {...feature} />
          ))}
        </div>
      </div>
    </section>
  );
}

function CallToAction() {
  return (
    <section className={styles.section}>
      <div className="container">
        <div className={styles.cta}>
          <Heading as="h2" className={styles.ctaTitle}>
            Ready to run your own email platform?
          </Heading>
          <p className={styles.ctaLead}>
            Deploy Posta with Docker in minutes and send your first email today.
          </p>
          <div className={styles.buttons}>
            <Link
              className="button button--primary button--lg"
              to="/docs/getting-started/installation">
              Install Posta
            </Link>
            <Link
              className="button button--secondary button--lg"
              to="/docs/getting-started/quickstart">
              Quick Start
            </Link>
          </div>
        </div>
      </div>
    </section>
  );
}

export default function Home(): ReactNode {
  const {siteConfig} = useDocusaurusContext();
  return (
    <Layout
      title="Documentation"
      description={`${siteConfig.title} — ${siteConfig.tagline}`}>
      <HomepageHeader />
      <main>
        <QuickStart />
        <Features />
        <CallToAction />
      </main>
    </Layout>
  );
}
