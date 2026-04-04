import type {ReactNode} from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';

import styles from './index.module.css';

function HomepageHeader() {
  const {siteConfig} = useDocusaurusContext();
  return (
    <header className={clsx('hero hero--primary', styles.heroBanner)}>
      <div className="container">
        <Heading as="h1" className="hero__title">
          {siteConfig.title}
        </Heading>
        <p className="hero__subtitle">{siteConfig.tagline}</p>
        <div className={styles.buttons}>
          <Link
            className="button button--secondary button--lg"
            to="/docs/getting-started/introduction">
            Get Started
          </Link>
          <Link
            className="button button--secondary button--outline button--lg"
            style={{marginLeft: '1rem'}}
            href="https://app.goposta.dev/docs">
            API Reference
          </Link>
        </div>
      </div>
    </header>
  );
}

type FeatureItem = {
  title: string;
  description: ReactNode;
  link: string;
};

const FeatureList: FeatureItem[] = [
  {
    title: 'Email Delivery',
    description: 'Send single, template, and batch emails via REST API with attachments, scheduling, and delivery tracking.',
    link: '/docs/email-sending/single-email',
  },
  {
    title: 'Templates & Localization',
    description: 'Version-controlled templates with multi-language support, variable substitution, and CSS inlining.',
    link: '/docs/templates/overview',
  },
  {
    title: 'SMTP & Domain Management',
    description: 'Configure multiple SMTP servers and verify domains with SPF, DKIM, and DMARC.',
    link: '/docs/smtp-domains/smtp-servers',
  },
  {
    title: 'Security',
    description: 'API keys with IP allowlists, JWT auth, two-factor authentication, and rate limiting.',
    link: '/docs/security/authentication',
  },
  {
    title: 'Webhooks & Events',
    description: 'Real-time notifications on email events with HMAC signature verification and retry logic.',
    link: '/docs/webhooks/overview',
  },
  {
    title: 'Official SDKs',
    description: 'Client libraries for Go, PHP, and Java with typed errors and full API coverage.',
    link: '/docs/sdks/overview',
  },
];

function Feature({title, description, link}: FeatureItem) {
  return (
    <div className={clsx('col col--4')} style={{marginBottom: '2rem'}}>
      <div className="padding-horiz--md">
        <Heading as="h3">
          <Link to={link}>{title}</Link>
        </Heading>
        <p>{description}</p>
      </div>
    </div>
  );
}

export default function Home(): ReactNode {
  const {siteConfig} = useDocusaurusContext();
  return (
    <Layout
      title="Documentation"
      description="Posta - Self-hosted email delivery platform for developers">
      <HomepageHeader />
      <main>
        <section style={{padding: '2rem 0'}}>
          <div className="container">
            <div className="row">
              {FeatureList.map((props, idx) => (
                <Feature key={idx} {...props} />
              ))}
            </div>
          </div>
        </section>
      </main>
    </Layout>
  );
}
