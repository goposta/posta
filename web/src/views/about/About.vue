<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { infoApi, type AppInfo } from '../../api/info'

const loading = ref(true)
const appInfo = ref<AppInfo | null>(null)

const features = [
  {
    title: 'Email Delivery',
    icon: 'mail',
    items: [
      'REST API for transactional, batch, and templated emails',
      'Attachments, custom headers, and unsubscribe support',
      'Scheduled sending and preview mode',
      'Async processing with Redis and Asynq',
      'Automatic retries and priority queues',
    ],
  },
  {
    title: 'Templates',
    icon: 'file-text',
    items: [
      'Versioned and multi-language templates',
      'Variable substitution and stylesheet inlining',
      'Import/export and preview support',
    ],
  },
  {
    title: 'SMTP & Domains',
    icon: 'server',
    items: [
      'Multiple SMTP providers with TLS support',
      'Shared SMTP pools for teams',
      'Domain verification (SPF, DKIM, DMARC)',
      'Verified sender enforcement',
    ],
  },
  {
    title: 'Security',
    icon: 'key',
    items: [
      'API keys with expiration, hashing, and IP allowlisting',
      'JWT authentication and RBAC',
      'Two-factor authentication (TOTP)',
      'OAuth / SSO login (Google, Keycloak, authentik, and more)',
      'Rate limiting and session management',
    ],
  },
  {
    title: 'Contacts & Suppression',
    icon: 'users',
    items: [
      'Contact tracking and segmentation',
      'Bounce and complaint handling',
      'Automatic suppression lists',
    ],
  },
  {
    title: 'Workspaces',
    icon: 'briefcase',
    items: [
      'Multi-tenant architecture with isolated workspaces',
      'Role-based access control',
      'Shared resources and scoped API keys',
    ],
  },
  {
    title: 'Webhooks & Events',
    icon: 'link',
    items: [
      'Event-driven architecture with webhook delivery',
      'Retry strategies and delivery tracking',
      'Audit logs and real-time event streaming',
    ],
  },
  {
    title: 'Analytics & Monitoring',
    icon: 'bar-chart',
    items: [
      'Email delivery metrics and trends',
      'Prometheus integration',
      'Health endpoints and daily reports',
    ],
  },
]

const links = [
  { title: 'Website', url: 'https://goposta.dev/', icon: 'globe' },
  { title: 'Documentation', url: 'https://docs.goposta.dev/', icon: 'book-open' },
  { title: 'GitHub', url: 'https://github.com/goposta/posta', icon: 'github' },
  { title: 'Go SDK', url: 'https://github.com/goposta/posta-go', icon: 'code' },
  { title: 'PHP SDK', url: 'https://github.com/goposta/posta-php', icon: 'code' },
  { title: 'Java SDK', url: 'https://github.com/goposta/posta-java', icon: 'code' },
]


onMounted(async () => {
  try {
    const res = await infoApi.get()
    appInfo.value = res.data.data
  } catch {
    // Non-critical
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div>
    <div class="page-header">
      <h1>About Posta</h1>
    </div>

    <div v-if="loading" class="loading-page">
      <div class="spinner"></div>
    </div>

    <template v-else>
      <!-- Hero -->
      <div class="card about-hero">
        <div class="card-body">
          <div class="hero-content">
            <img src="/logo.png" alt="Posta" class="hero-logo" />
            <div>
              <h2 class="hero-title">Posta</h2>
              <p class="hero-description">
                Self-hosted email delivery platform for developers and teams.
                A developer-first, fully self-hostable alternative to services like SendGrid or Mailgun.
              </p>
              <div class="hero-meta">
                <span v-if="appInfo" class="badge badge-info">
                  v{{ appInfo.version }}
                  <template v-if="appInfo.commit_id"> ({{ appInfo.commit_id.slice(0, 7) }})</template>
                </span>
                <span class="badge badge-secondary">Apache License 2.0</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Features -->
      <div class="about-section">
        <h2 class="section-title">Core Features</h2>
        <div class="features-grid">
          <div v-for="feature in features" :key="feature.title" class="card feature-card">
            <div class="card-body">
              <h3 class="feature-title">{{ feature.title }}</h3>
              <ul class="feature-list">
                <li v-for="item in feature.items" :key="item">{{ item }}</li>
              </ul>
            </div>
          </div>
        </div>
      </div>

      <!-- Links -->
      <div class="about-section">
        <h2 class="section-title">Resources & SDKs</h2>
        <div class="card">
          <div class="card-body">
            <div class="links-grid">
              <a
                v-for="link in links"
                :key="link.url"
                :href="link.url"
                target="_blank"
                rel="noopener noreferrer"
                class="about-link"
              >
                <span class="about-link-title">{{ link.title }}</span>
                <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
                  <path d="M6 3h7v7M13 3L3 13" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
              </a>
            </div>
          </div>
        </div>
      </div>

      <!-- Footer -->
      <div class="about-footer">
        <p>&copy; {{ new Date().getFullYear() }} Jonas Kaninda and contributors</p>
      </div>
    </template>
  </div>
</template>

<style scoped>
.about-hero {
  margin-bottom: 24px;
}

.hero-content {
  display: flex;
  align-items: flex-start;
  gap: 20px;
}

.hero-logo {
  width: 72px;
  height: 72px;
  object-fit: contain;
  flex-shrink: 0;
}

.hero-title {
  font-size: 24px;
  font-weight: 700;
  margin: 0 0 8px;
  color: var(--text-primary);
}

.hero-description {
  color: var(--text-secondary);
  line-height: 1.6;
  margin: 0 0 12px;
  max-width: 600px;
}

.hero-meta {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.about-section {
  margin-bottom: 24px;
}

.section-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 12px;
}

.features-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 16px;
}

.feature-card {
  margin: 0;
}

.feature-title {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 10px;
}

.feature-list {
  margin: 0;
  padding: 0 0 0 18px;
  list-style: disc;
}

.feature-list li {
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.7;
}

.tech-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 12px;
}

.tech-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 12px;
  border-radius: var(--radius-sm, 6px);
  background: var(--bg-secondary);
}

.tech-category {
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--text-muted);
}

.tech-name {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-primary);
}

.links-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 10px;
}

.about-link {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 14px;
  border-radius: var(--radius-sm, 6px);
  background: var(--bg-secondary);
  color: var(--text-primary);
  text-decoration: none;
  transition: all var(--transition, 150ms ease);
  font-size: 14px;
  font-weight: 500;
}

.about-link:hover {
  background: var(--bg-tertiary);
  color: var(--primary-600, #9333ea);
}

.about-link svg {
  color: var(--text-muted);
  flex-shrink: 0;
}

.about-link:hover svg {
  color: var(--primary-600, #9333ea);
}

.about-footer {
  text-align: center;
  padding: 24px 0;
  color: var(--text-muted);
  font-size: 13px;
}
</style>
