<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { infoApi, type AppInfo } from '../../api/info'

const loading = ref(true)
const appInfo = ref<AppInfo | null>(null)

const links = [
  { title: 'Website', url: 'https://goposta.dev/', mdi: 'mdi-web' },
  { title: 'Documentation', url: 'https://docs.goposta.dev/', mdi: 'mdi-book-open-page-variant-outline' },
  { title: 'GitHub', url: 'https://github.com/goposta/posta', mdi: 'mdi-github' },
  { title: 'Go SDK', url: 'https://github.com/goposta/posta-go', mdi: 'mdi-code-tags' },
  { title: 'PHP SDK', url: 'https://github.com/goposta/posta-php', mdi: 'mdi-code-tags' },
  { title: 'Java SDK', url: 'https://github.com/goposta/posta-java', mdi: 'mdi-code-tags' },
]

// Capability groups — a high-level tour of what Posta does, both outbound and inbound.
const capabilities = [
  {
    icon: 'mdi-send-outline',
    title: 'Outbound Email',
    items: [
      'REST API for transactional, batch & templated email',
      'Attachments, custom headers & scheduled sending',
      'Async delivery with automatic retries & priority queues',
    ],
  },
  {
    icon: 'mdi-inbox-arrow-down-outline',
    title: 'Inbound Email',
    items: [
      'Built-in SMTP receiver with TLS',
      'Webhook ingest with HMAC verification',
      'Message, header & attachment storage with forwarding',
    ],
  },
  {
    icon: 'mdi-file-document-multiple-outline',
    title: 'Templates & Campaigns',
    items: [
      'Versioned, multi-language templates',
      'Bulk campaigns with scheduling & targeting',
      'A/B testing with per-variant metrics',
    ],
  },
  {
    icon: 'mdi-account-group-outline',
    title: 'Contacts & Subscribers',
    items: [
      'Static & dynamic (segmented) subscriber lists',
      'CSV / JSON import with column mapping',
      'Bounce & complaint suppression, RFC 8058 unsubscribe',
    ],
  },
  {
    icon: 'mdi-shield-lock-outline',
    title: 'Security & Access',
    items: [
      'API keys with hashing, expiry & IP allowlisting',
      'JWT auth, RBAC & two-factor (TOTP)',
      'OAuth / SSO, rate limiting & session management',
    ],
  },
  {
    icon: 'mdi-chart-line',
    title: 'Analytics & Webhooks',
    items: [
      'Open & click tracking with delivery metrics',
      'Event-driven webhooks with retries & tracking',
      'Prometheus metrics and audit logs',
    ],
  },
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
                A self-hosted, developer-first email platform that handles both outbound delivery and
                inbound receiving through a single HTTP API.
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

      <!-- Capabilities -->
      <div class="about-section">
        <h2 class="section-title">Capabilities</h2>
        <div class="features-grid">
          <div v-for="cap in capabilities" :key="cap.title" class="card feature-card">
            <div class="card-body">
              <h3 class="feature-title">
                <span class="mdi feature-icon" :class="cap.icon"></span>
                {{ cap.title }}
              </h3>
              <ul class="feature-list">
                <li v-for="item in cap.items" :key="item">{{ item }}</li>
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
                <span class="about-link-left">
                  <span class="mdi about-link-icon" :class="link.mdi"></span>
                  <span class="about-link-title">{{ link.title }}</span>
                </span>
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
        <p class="about-powered">
          Powered by
          <a href="https://github.com/jkaninda/okapi" target="_blank" rel="noopener noreferrer">
            <span class="mdi mdi-github"></span> Okapi
          </a>
        </p>
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
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 10px;
}

.feature-icon {
  font-size: 18px;
  color: var(--primary-600, #9333ea);
  line-height: 1;
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

.about-link-left {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.about-link-icon {
  font-size: 18px;
  color: var(--text-muted);
  line-height: 1;
  flex-shrink: 0;
  transition: color var(--transition, 150ms ease);
}

.about-link:hover {
  background: var(--bg-tertiary);
  color: var(--primary-600, #9333ea);
}

.about-link:hover .about-link-icon {
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

.about-powered {
  margin-bottom: 4px;
}

.about-powered a {
  color: var(--text-secondary);
  font-weight: 500;
  text-decoration: none;
  transition: color var(--transition, 150ms ease);
}

.about-powered a:hover {
  color: var(--primary-600, #9333ea);
}

.about-powered .mdi {
  font-size: 14px;
  vertical-align: -1px;
}
</style>
