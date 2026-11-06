<script setup lang="ts">
// Form styling stays in each page: scoped CSS applies to slot content in the parent's scope, not this component's.
import { useThemeStore } from '../../stores/theme'

withDefaults(
  defineProps<{
    title: string
    subtitle?: string
    // Secondary pages drop the brand panel: the hero is an entry-point flourish that makes mid-flow steps feel heavier.
    hero?: boolean
  }>(),
  { hero: true }
)

const theme = useThemeStore()
</script>

<template>
  <div class="auth" :class="{ 'auth-no-hero': !hero }">
    <!-- Decorative and duplicated by the form column's heading, so it is hidden from assistive tech. -->
    <aside v-if="hero" class="auth-hero" aria-hidden="true">
      <div class="auth-hero-inner">
        <div class="auth-hero-wordmark">Posta<span class="wm-dot">.</span></div>

        <div class="auth-hero-body">
          <h2 class="auth-hero-title">Email infrastructure,<br />self-hosted.</h2>
          <p class="auth-hero-lead">
            Send transactional and marketing email, receive and parse inbound
            messages — all through a single HTTP API.
          </p>
          <ul class="auth-hero-features">
            <li><span class="mdi mdi-email-fast-outline"></span> Transactional &amp; marketing email</li>
            <li><span class="mdi mdi-file-document-multiple-outline"></span> Templates &amp; localization</li>
            <li><span class="mdi mdi-bullhorn-outline"></span> Campaigns &amp; subscriber lists</li>
            <li><span class="mdi mdi-webhook"></span> Inbound parsing &amp; webhooks</li>
            <li><span class="mdi mdi-chart-areaspline"></span> Delivery analytics &amp; tracking</li>
            <li><span class="mdi mdi-account-group-outline"></span> Workspaces &amp; RBAC</li>
          </ul>
        </div>

        <p class="auth-hero-foot">Open-source · Self-hosted email platform</p>
      </div>
    </aside>

    <main class="auth-main">
      <div class="auth-card">
        <div class="auth-header">
          <div class="auth-wordmark" aria-label="Posta">Posta<span class="auth-wordmark-dot">.</span></div>
          <h1 class="auth-title">{{ title }}</h1>
          <p v-if="subtitle" class="auth-subtitle">{{ subtitle }}</p>
        </div>

        <slot />
      </div>
    </main>

    <button
      class="theme-btn"
      :title="theme.isDark ? 'Light mode' : 'Dark mode'"
      :aria-label="theme.isDark ? 'Switch to light mode' : 'Switch to dark mode'"
      @click="theme.toggle()"
    >
      <svg v-if="theme.isDark" width="18" height="18" viewBox="0 0 16 16" fill="none"><circle cx="8" cy="8" r="3" stroke="currentColor" stroke-width="1.5"/><path d="M8 1v2M8 13v2M1 8h2M13 8h2M3.05 3.05l1.41 1.41M11.54 11.54l1.41 1.41M3.05 12.95l1.41-1.41M11.54 4.46l1.41-1.41" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>
      <svg v-else width="18" height="18" viewBox="0 0 16 16" fill="none"><path d="M14 9.5A6.5 6.5 0 016.5 2 6.5 6.5 0 1014 9.5z" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>
    </button>
  </div>
</template>

<style scoped>
.auth {
  min-height: 100vh;
  display: grid;
  grid-template-columns: 1.05fr 1fr;
  background: var(--bg-primary);
  position: relative;
}

.auth-hero {
  position: relative;
  overflow: hidden;
  display: flex;
  color: #fff;
  background:
    radial-gradient(120% 80% at 100% 0%, rgba(255, 255, 255, 0.16), transparent 55%),
    radial-gradient(90% 70% at 0% 100%, rgba(13, 20, 36, 0.5), transparent 60%),
    linear-gradient(150deg, var(--primary-600) 0%, var(--primary-800) 70%, #2a0f4d 100%);
}
.auth-hero::after {
  content: 'P';
  position: absolute;
  right: -4%;
  bottom: -22%;
  font-size: 34rem;
  font-weight: 800;
  line-height: 1;
  color: #fff;
  opacity: 0.05;
  pointer-events: none;
  user-select: none;
}
.auth-hero-inner {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  width: 100%;
  max-width: 460px;
  margin: auto;
  padding: 56px 52px;
}
.auth-hero-wordmark {
  align-self: flex-start;
  margin-bottom: auto;
  font-size: 1.6rem;
  font-weight: 800;
  letter-spacing: -0.02em;
  color: #fff;
}
.auth-hero-wordmark .wm-dot { color: var(--primary-400); }

.auth-hero-body { margin: 48px 0; }
.auth-hero-title {
  font-size: clamp(1.9rem, 2.6vw, 2.6rem);
  font-weight: 800;
  line-height: 1.1;
  letter-spacing: -0.02em;
  margin: 0 0 16px;
}
.auth-hero-lead {
  font-size: 15px;
  line-height: 1.6;
  color: rgba(255, 255, 255, 0.82);
  max-width: 42ch;
  margin: 0 0 28px;
}
.auth-hero-features {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.auth-hero-features li {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 14px;
  color: rgba(255, 255, 255, 0.92);
}
.auth-hero-features .mdi {
  font-size: 20px;
  flex-shrink: 0;
  width: 34px;
  height: 34px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius);
  background: rgba(255, 255, 255, 0.12);
}
.auth-hero-foot {
  margin: 0;
  font-size: 12.5px;
  color: rgba(255, 255, 255, 0.6);
}

.auth-main {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px 24px;
}
.auth-card {
  width: 100%;
  max-width: 380px;
}
.auth-header { text-align: center; margin-bottom: 24px; }
.auth-wordmark {
  font-size: 32px;
  font-weight: 800;
  letter-spacing: -1px;
  color: var(--text-primary);
  margin-bottom: 20px;
  line-height: 1;
}
.auth-wordmark-dot {
  color: var(--primary-500);
  margin-left: 1px;
}
.auth-title {
  font-size: 22px;
  font-weight: 600;
  color: var(--text-primary);
  letter-spacing: -0.2px;
  margin: 0 0 8px;
}
.auth-subtitle {
  font-size: 14px;
  color: var(--text-muted);
  margin: 0;
}

.theme-btn {
  position: fixed;
  top: 20px;
  right: 20px;
  background: var(--bg-primary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius);
  padding: 10px;
  cursor: pointer;
  color: var(--text-tertiary);
  display: flex;
  align-items: center;
  transition: all var(--transition);
  box-shadow: var(--shadow-sm);
}
.theme-btn:hover { color: var(--text-primary); border-color: var(--border-input); }

.auth-no-hero { grid-template-columns: 1fr; }

@media (max-width: 900px) {
  .auth { grid-template-columns: 1fr; }
  .auth-hero { display: none; }
}
</style>
