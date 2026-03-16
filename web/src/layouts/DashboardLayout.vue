<script setup lang="ts">
import { computed, ref, onMounted, onBeforeUnmount } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useThemeStore } from '../stores/theme'
import { infoApi, type AppInfo } from '../api/info'

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()
const theme = useThemeStore()
const sidebarCollapsed = ref(false)
const appInfo = ref<AppInfo | null>(null)
const userMenuOpen = ref(false)
const themeModes = ['light', 'dark', 'system'] as const

function closeUserMenu(e: MouseEvent) {
  const el = document.querySelector('.user-menu')
  if (el && !el.contains(e.target as Node)) {
    userMenuOpen.value = false
  }
}

const toggleSidebar = () => {
  sidebarCollapsed.value = !sidebarCollapsed.value
  localStorage.setItem('sidebarCollapsed', sidebarCollapsed.value.toString())
}

onMounted(() => {
  document.addEventListener('click', closeUserMenu)
  const stored = localStorage.getItem('sidebarCollapsed')
  if (stored !== null) {
    sidebarCollapsed.value = stored === 'true'
  }
})
onBeforeUnmount(() => {
  document.removeEventListener('click', closeUserMenu)
})

onMounted(async () => {
  try {
    const res = await infoApi.get()
    appInfo.value = res.data.data
  } catch {
    // Version display is non-critical
  }
})

const user = computed(() => auth.user)

const navItems = [
  { name: 'Dashboard', path: '/', icon: 'grid' },
  { name: 'Analytics', path: '/analytics', icon: 'bar-chart' },
  { name: 'Emails', path: '/emails', icon: 'mail' },
  { name: 'Templates', path: '/templates', icon: 'file-text' },
  { name: 'Languages', path: '/languages', icon: 'type' },
  { name: 'Stylesheets', path: '/stylesheets', icon: 'edit-3' },
{ name: 'SMTP Servers', path: '/smtp-servers', icon: 'server' },
  { name: 'Domains', path: '/domains', icon: 'globe' },
  { name: 'Webhooks', path: '/webhooks', icon: 'link' },
  { name: 'Deliveries', path: '/webhook-deliveries', icon: 'activity' },
  { name: 'Contacts', path: '/contacts', icon: 'users' },
  { name: 'Lists', path: '/contact-lists', icon: 'list' },
  { name: 'Bounces', path: '/bounces', icon: 'alert-triangle' },
  { name: 'API Keys', path: '/api-keys', icon: 'key' },
  { name: 'Audit Log', path: '/audit-log', icon: 'activity' },
  { name: 'Settings', path: '/settings', icon: 'settings' },
]

const adminItems = [
  { name: 'Users', path: '/admin/users', icon: 'users' },
  { name: 'Shared Servers', path: '/admin/servers', icon: 'server' },
  { name: 'Jobs', path: '/admin/jobs', icon: 'clock' },
  { name: 'Metrics', path: '/admin/metrics', icon: 'bar-chart' },
  { name: 'Events', path: '/admin/events', icon: 'activity' },
  { name: 'Settings', path: '/admin/settings', icon: 'settings' },
]

function isActive(path: string): boolean {
  if (path === '/') return route.path === '/'
  return route.path.startsWith(path)
}

function navigate(path: string) {
  router.push(path)
}

function logout() {
  auth.logout()
  router.push('/login')
}
</script>

<template>
  <div class="layout" :class="{ collapsed: sidebarCollapsed }">
    <aside class="sidebar">
      <div class="sidebar-top">
        <div class="sidebar-brand">
          <div class="logo" @click="navigate('/')">
            <img src="/logo.png" alt="Posta" class="logo-img" />
            <span v-if="!sidebarCollapsed" class="logo-text">Posta</span>
          </div>
          <button class="collapse-btn" @click="toggleSidebar" :title="sidebarCollapsed ? 'Expand' : 'Collapse'">
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none"><path :d="sidebarCollapsed ? 'M6 3l5 5-5 5' : 'M10 3L5 8l5 5'" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>
          </button>
        </div>
        </div>
      <div class="sidebar-middle">
        <nav class="nav">
          <a
            v-for="item in navItems"
            :key="item.path"
            class="nav-item"
            :class="{ active: isActive(item.path) }"
            :title="sidebarCollapsed ? item.name : ''"
            @click="navigate(item.path)"
          >
            <span class="nav-icon" v-html="getIcon(item.icon)"></span>
            <span v-if="!sidebarCollapsed" class="nav-label">{{ item.name }}</span>
          </a>

          <template v-if="user?.role === 'admin'">
            <div v-if="!sidebarCollapsed" class="nav-section">Admin</div>
            <div v-else class="nav-divider"></div>
            <a
              v-for="item in adminItems"
              :key="item.path"
              class="nav-item"
              :class="{ active: isActive(item.path) }"
              :title="sidebarCollapsed ? item.name : ''"
              @click="navigate(item.path)"
            >
              <span class="nav-icon" v-html="getIcon(item.icon)"></span>
              <span v-if="!sidebarCollapsed" class="nav-label">{{ item.name }}</span>
            </a>
          </template>
        </nav>
      </div>

      <div class="sidebar-bottom">
        <template v-if="appInfo?.openapi_docs">
          <div v-if="!sidebarCollapsed" class="nav-section">Docs</div>
          <div v-else class="nav-divider"></div>
          <a class="nav-item" href="/docs" target="_blank" rel="noopener noreferrer" :title="sidebarCollapsed ? 'Swagger UI' : ''">
            <span class="nav-icon" v-html="getIcon('book-open')"></span>
            <span v-if="!sidebarCollapsed" class="nav-label">Swagger UI</span>
          </a>
          <a class="nav-item" href="/redoc" target="_blank" rel="noopener noreferrer" :title="sidebarCollapsed ? 'Redoc' : ''">
            <span class="nav-icon" v-html="getIcon('file-text')"></span>
            <span v-if="!sidebarCollapsed" class="nav-label">Redoc</span>
          </a>
        </template>

        <div v-if="appInfo && !sidebarCollapsed" class="sidebar-app-info">
          <span class="sidebar-app-version">v{{ appInfo.version }}</span>
        </div>
      </div>
    </aside>

    <main class="main-content">
      <header class="topbar">
        <div></div>
        <div class="topbar-right">
          <div class="user-menu" @click="userMenuOpen = !userMenuOpen">
            <div class="user-avatar">{{ user?.name?.charAt(0)?.toUpperCase() || '?' }}</div>
            <div class="user-menu-info">
              <div class="user-name">{{ user?.name || 'User' }}</div>
              <div class="user-email">{{ user?.email || '' }}</div>
            </div>
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none"><path d="M4 6l4 4 4-4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>

            <div v-if="userMenuOpen" class="user-dropdown">
              <a class="dropdown-item" @click.stop="navigate('/profile'); userMenuOpen = false">
                <svg width="15" height="15" viewBox="0 0 16 16" fill="none"><path d="M12.67 14v-1.33A2.67 2.67 0 0010 10H6a2.67 2.67 0 00-2.67 2.67V14" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/><circle cx="8" cy="5.33" r="2.67" stroke="currentColor" stroke-width="1.5"/></svg>
                My Profile
              </a>
              <div class="dropdown-divider"></div>
              <div class="dropdown-theme">
                <div class="dropdown-theme-label">
                  <svg width="15" height="15" viewBox="0 0 16 16" fill="none"><circle cx="8" cy="8" r="3" stroke="currentColor" stroke-width="1.5"/><path d="M8 1v2M8 13v2M1 8h2M13 8h2M3.05 3.05l1.41 1.41M11.54 11.54l1.41 1.41M3.05 12.95l1.41-1.41M11.54 4.46l1.41-1.41" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>
                  Theme
                </div>
                <div class="dropdown-theme-switcher">
                  <button
                    v-for="m in themeModes"
                    :key="m"
                    :class="['dropdown-theme-btn', { active: theme.mode === m }]"
                    :title="m.charAt(0).toUpperCase() + m.slice(1)"
                    @click.stop="theme.setMode(m)"
                  >
                    <svg v-if="m === 'light'" width="14" height="14" viewBox="0 0 16 16" fill="none"><circle cx="8" cy="8" r="3" stroke="currentColor" stroke-width="1.5"/><path d="M8 1v2M8 13v2M1 8h2M13 8h2M3.05 3.05l1.41 1.41M11.54 11.54l1.41 1.41M3.05 12.95l1.41-1.41M11.54 4.46l1.41-1.41" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>
                    <svg v-else-if="m === 'dark'" width="14" height="14" viewBox="0 0 16 16" fill="none"><path d="M14 9.5A6.5 6.5 0 016.5 2 6.5 6.5 0 1014 9.5z" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>
                    <svg v-else width="14" height="14" viewBox="0 0 16 16" fill="none"><rect x="2" y="3" width="12" height="10" rx="1.5" stroke="currentColor" stroke-width="1.5"/><path d="M2 5.5h12" stroke="currentColor" stroke-width="1.5"/></svg>
                  </button>
                </div>
              </div>
              <div class="dropdown-divider"></div>
              <a class="dropdown-item dropdown-item-danger" @click.stop="logout">
                <svg width="15" height="15" viewBox="0 0 16 16" fill="none"><path d="M6 14H3.33A1.33 1.33 0 012 12.67V3.33A1.33 1.33 0 013.33 2H6M10.67 11.33L14 8l-3.33-3.33M14 8H6" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>
                Logout
              </a>
            </div>
          </div>
        </div>
      </header>
      <div class="main-body">
        <router-view />
      </div>
      <footer class="main-footer">
        <div class="footer-left">
          <span>&copy; {{ new Date().getFullYear() }} Jonas Kaninda</span>
          <span v-if="appInfo" class="footer-version" :title="appInfo.commit_id ? `Commit: ${appInfo.commit_id}` : ''">
            v{{ appInfo.version }}<template v-if="appInfo.commit_id"> ({{ appInfo.commit_id.slice(0, 7) }})</template>
          </span>
        </div>
        <div class="footer-right">
          <a href="https://github.com/jkaninda/posta" target="_blank" rel="noopener noreferrer" class="footer-link">
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none"><path d="M8 1C4.13 1 1 4.13 1 8a7 7 0 004.79 6.65c.35.06.48-.15.48-.34 0-.17-.01-.71-.01-1.29-1.76.33-2.2-.43-2.34-.82-.08-.2-.42-.82-.71-.99-.24-.13-.59-.46-.01-.47.55-.01.94.51 1.07.71.63 1.05 1.63.76 2.03.57.06-.45.24-.76.44-.93-1.55-.17-3.18-.78-3.18-3.46 0-.76.27-1.39.71-1.88-.07-.17-.31-.89.07-1.85 0 0 .58-.19 1.9.71a6.5 6.5 0 013.46 0c1.32-.9 1.9-.71 1.9-.71.38.96.14 1.68.07 1.85.44.49.71 1.11.71 1.88 0 2.69-1.64 3.29-3.19 3.46.25.22.47.64.47 1.29 0 .93-.01 1.68-.01 1.91 0 .19.13.41.48.34A7 7 0 0015 8c0-3.87-3.13-7-7-7z" fill="currentColor"/></svg>
            GitHub
          </a>
        </div>
      </footer>
    </main>
  </div>
</template>

<script lang="ts">
function getIcon(name: string): string {
  const icons: Record<string, string> = {
    'grid': '<svg width="18" height="18" viewBox="0 0 18 18" fill="none"><rect x="1.5" y="1.5" width="6" height="6" rx="1.5" stroke="currentColor" stroke-width="1.5"/><rect x="10.5" y="1.5" width="6" height="6" rx="1.5" stroke="currentColor" stroke-width="1.5"/><rect x="1.5" y="10.5" width="6" height="6" rx="1.5" stroke="currentColor" stroke-width="1.5"/><rect x="10.5" y="10.5" width="6" height="6" rx="1.5" stroke="currentColor" stroke-width="1.5"/></svg>',
    'mail': '<svg width="18" height="18" viewBox="0 0 18 18" fill="none"><rect x="2" y="3.5" width="14" height="11" rx="2" stroke="currentColor" stroke-width="1.5"/><path d="M2 5.5l7 5 7-5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
    'key': '<svg width="18" height="18" viewBox="0 0 18 18" fill="none"><path d="M15.5 2.5l-2 2m1 1l-2 2-3-3 2-2m-3.18 3.18a4 4 0 10-5.64 5.64 4 4 0 005.64-5.64z" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
    'file-text': '<svg width="18" height="18" viewBox="0 0 18 18" fill="none"><path d="M10.5 1.5H4.5a1.5 1.5 0 00-1.5 1.5v12a1.5 1.5 0 001.5 1.5h9a1.5 1.5 0 001.5-1.5V6l-4.5-4.5z" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/><path d="M10.5 1.5V6H15M12 9.75H6M12 12.75H6M7.5 6.75H6" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
    'server': '<svg width="18" height="18" viewBox="0 0 18 18" fill="none"><rect x="2" y="2" width="14" height="5" rx="1.5" stroke="currentColor" stroke-width="1.5"/><rect x="2" y="11" width="14" height="5" rx="1.5" stroke="currentColor" stroke-width="1.5"/><circle cx="5" cy="4.5" r="0.75" fill="currentColor"/><circle cx="5" cy="13.5" r="0.75" fill="currentColor"/></svg>',
    'globe': '<svg width="18" height="18" viewBox="0 0 18 18" fill="none"><circle cx="9" cy="9" r="7" stroke="currentColor" stroke-width="1.5"/><path d="M2 9h14M9 2a11.05 11.05 0 013 7 11.05 11.05 0 01-3 7 11.05 11.05 0 01-3-7 11.05 11.05 0 013-7z" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
    'link': '<svg width="18" height="18" viewBox="0 0 18 18" fill="none"><path d="M7.5 10.5a3.75 3.75 0 005.3.45l2.25-2.25a3.75 3.75 0 00-5.3-5.3l-1.29 1.28" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/><path d="M10.5 7.5a3.75 3.75 0 00-5.3-.45L2.96 9.3a3.75 3.75 0 005.3 5.3l1.28-1.28" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
    'alert-triangle': '<svg width="18" height="18" viewBox="0 0 18 18" fill="none"><path d="M7.86 2.87L1.21 14.25a1.31 1.31 0 001.14 1.97h13.3a1.31 1.31 0 001.14-1.97L10.14 2.87a1.31 1.31 0 00-2.28 0z" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/><path d="M9 6.75v3M9 12.75h.007" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>',
    'users': '<svg width="18" height="18" viewBox="0 0 18 18" fill="none"><path d="M12.75 15.75v-1.5a3 3 0 00-3-3h-6a3 3 0 00-3 3v1.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/><circle cx="6.75" cy="5.25" r="3" stroke="currentColor" stroke-width="1.5"/><path d="M17.25 15.75v-1.5a3 3 0 00-2.25-2.9M12 2.33a3 3 0 010 5.84" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
    'bar-chart': '<svg width="18" height="18" viewBox="0 0 18 18" fill="none"><path d="M13.5 15V7.5M9 15V3M4.5 15v-4.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
    'type': '<svg width="18" height="18" viewBox="0 0 18 18" fill="none"><path d="M3 3h12M9 3v12M5.25 15h7.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
    'clock': '<svg width="18" height="18" viewBox="0 0 18 18" fill="none"><circle cx="9" cy="9" r="7" stroke="currentColor" stroke-width="1.5"/><path d="M9 4.5V9l3 1.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
    'activity': '<svg width="18" height="18" viewBox="0 0 18 18" fill="none"><path d="M16.5 9h-3l-2.25 6.75L6.75 2.25 4.5 9h-3" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
    'book-open': '<svg width="18" height="18" viewBox="0 0 18 18" fill="none"><path d="M1.5 2.25h5.25a3 3 0 013 3v10.5a2.25 2.25 0 00-2.25-2.25H1.5V2.25zM16.5 2.25h-5.25a3 3 0 00-3 3v10.5a2.25 2.25 0 012.25-2.25h6V2.25z" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
    'list': '<svg width="18" height="18" viewBox="0 0 18 18" fill="none"><path d="M6.75 4.5h9M6.75 9h9M6.75 13.5h9M2.25 4.5h.007M2.25 9h.007M2.25 13.5h.007" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
    'edit-3': '<svg width="18" height="18" viewBox="0 0 18 18" fill="none"><path d="M12 2.25l3.75 3.75L6 15.75H2.25V12L12 2.25z" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
    'settings': '<svg width="18" height="18" viewBox="0 0 18 18" fill="none"><circle cx="9" cy="9" r="2.25" stroke="currentColor" stroke-width="1.5"/><path d="M14.7 11.1a1.2 1.2 0 00.24 1.32l.04.04a1.46 1.46 0 11-2.06 2.06l-.04-.04a1.2 1.2 0 00-1.32-.24 1.2 1.2 0 00-.73 1.1v.12a1.46 1.46 0 01-2.91 0v-.06a1.2 1.2 0 00-.79-1.1 1.2 1.2 0 00-1.32.24l-.04.04a1.46 1.46 0 11-2.06-2.06l.04-.04a1.2 1.2 0 00.24-1.32 1.2 1.2 0 00-1.1-.73h-.12a1.46 1.46 0 010-2.91h.06a1.2 1.2 0 001.1-.79 1.2 1.2 0 00-.24-1.32l-.04-.04a1.46 1.46 0 112.06-2.06l.04.04a1.2 1.2 0 001.32.24h.06a1.2 1.2 0 00.73-1.1v-.12a1.46 1.46 0 012.91 0v.06a1.2 1.2 0 00.73 1.1 1.2 1.2 0 001.32-.24l.04-.04a1.46 1.46 0 112.06 2.06l-.04.04a1.2 1.2 0 00-.24 1.32v.06a1.2 1.2 0 001.1.73h.12a1.46 1.46 0 010 2.91h-.06a1.2 1.2 0 00-1.1.73z" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
  }
  return icons[name] || ''
}
export default { methods: { getIcon } }
</script>

<style scoped>
.layout { display: flex; min-height: 100vh; }

.sidebar {
  width: 240px;
  min-width: 240px;
  background: var(--bg-sidebar);
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  position: fixed;
  top: 0;
  left: 0;
  bottom: 0;
  z-index: 100;
  transition: width var(--transition-slow), min-width var(--transition-slow);
}

.collapsed .sidebar { width: 64px; min-width: 64px; }
.collapsed .main-content { margin-left: 64px; }

.sidebar-top { display: flex; overflow-x: auto; }
.sidebar-middle { flex: 1; overflow-y: auto; overflow-x: hidden; margin-top: 10px; padding-top: 5px;}

.sidebar-brand {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 14px 12px;
}

.logo {
  display: flex;
  align-items: center;
  gap: 10px;
  cursor: pointer;
}

.logo-img {
  width: 36px;
  height: 36px;
  object-fit: contain;
}

.logo-text {
  font-size: 22px;
  font-weight: 800;
  color: var(--sidebar-text-active);
  letter-spacing: -0.5px;
}

.collapse-btn {
  background: transparent;
  border: none;
  color: var(--sidebar-text);
  cursor: pointer;
  padding: 4px;
  border-radius: var(--radius-sm);
  transition: all var(--transition);
  display: flex;
  align-items: center;
  position: absolute;
  top: 20px;
  right: 8px;
}
.collapse-btn:hover { color: var(--sidebar-text-active); background: var(--sidebar-hover); }
.collapsed .collapse-btn {
  position: absolute;
  top: 20px;
  right: -5px;
  z-index: 9999;
}

.nav { display: flex; flex-direction: column; padding: 0 8px; gap: 2px; }

.nav-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 9px 12px;
  border-radius: var(--radius);
  color: var(--sidebar-text);
  font-size: 13.5px;
  font-weight: 500;
  cursor: pointer;
  transition: all var(--transition);
  text-decoration: none;
  white-space: nowrap;
  overflow: hidden;
}
.nav-item:hover { color: var(--sidebar-text-active); background: var(--sidebar-hover); }
.nav-item.active { color: var(--sidebar-text-active); background: var(--primary-700); }

.collapsed .nav-item { justify-content: center; padding: 10px; }

.nav-icon { display: flex; align-items: center; justify-content: center; flex-shrink: 0; width: 20px; height: 20px; }

.nav-section {
  padding: 20px 12px 6px;
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.1em;
  color: var(--sidebar-text);
  opacity: 0.5;
}

.nav-divider {
  height: 1px;
  background: var(--sidebar-border);
  margin: 12px 12px 8px;
}

.sidebar-bottom {
  padding: 12px 8px 14px;
  border-top: 1px solid var(--sidebar-border);
}


.main-content {
  flex: 1;
  margin-left: 240px;
  min-height: 100vh;
  background: var(--bg-secondary);
  display: flex;
  flex-direction: column;
  transition: margin-left var(--transition-slow), background var(--transition-slow);
}

.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 32px;
  background: var(--bg-primary);
  border-bottom: 1px solid var(--border-primary);
  position: sticky;
  top: 0;
  z-index: 50;
}

.topbar-right {
  display: flex;
  align-items: center;
  gap: 12px;
}


.user-menu {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 12px;
  border-radius: var(--radius);
  cursor: pointer;
  transition: all var(--transition);
  position: relative;
  user-select: none;
  color: var(--text-secondary);
}
.user-menu:hover { background: var(--bg-secondary); }

.user-avatar {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: var(--primary-600);
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 13px;
  font-weight: 600;
  flex-shrink: 0;
}

.user-menu-info { overflow: hidden; }
.user-name { font-size: 13px; font-weight: 600; color: var(--text-primary); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.user-email { font-size: 11px; color: var(--text-muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }

.user-dropdown {
  position: absolute;
  top: calc(100% + 6px);
  right: 0;
  min-width: 200px;
  background: var(--bg-primary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius);
  box-shadow: var(--shadow-lg);
  padding: 4px;
  z-index: 200;
}

.dropdown-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  font-size: 13px;
  font-weight: 500;
  color: var(--text-secondary);
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: all var(--transition);
  text-decoration: none;
}
.dropdown-item:hover { background: var(--bg-secondary); color: var(--text-primary); }
.dropdown-item-danger:hover { background: var(--danger-50, #fef2f2); color: var(--danger-600, #dc2626); }

.dropdown-divider {
  height: 1px;
  background: var(--border-primary);
  margin: 4px 0;
}

.dropdown-theme {
  padding: 6px 12px;
}

.dropdown-theme-label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  font-weight: 500;
  color: var(--text-secondary);
  margin-bottom: 8px;
}

.dropdown-theme-switcher {
  display: flex;
  background: var(--bg-secondary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius);
  padding: 2px;
  gap: 2px;
}

.dropdown-theme-btn {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 5px;
  border: none;
  border-radius: calc(var(--radius) - 2px);
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  transition: all var(--transition);
}

.dropdown-theme-btn:hover {
  color: var(--text-primary);
}

.dropdown-theme-btn.active {
  background: var(--bg-primary);
  color: var(--text-primary);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.06);
}

.main-body {
  flex: 1;
  padding: 32px;
}

.main-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 32px;
  border-top: 1px solid var(--border-primary);
  font-size: 12px;
  color: var(--text-muted);
}

.footer-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.footer-version {
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  opacity: 0.7;
}

.footer-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.footer-link {
  display: flex;
  align-items: center;
  gap: 5px;
  color: var(--text-muted);
  text-decoration: none;
  transition: color var(--transition);
}
.footer-link:hover { color: var(--text-primary); }

.sidebar-app-info {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px 2px;
  gap: 6px;
}

.sidebar-app-name {
  font-size: 12px;
  font-weight: 600;
  color: var(--sidebar-text-active);
  opacity: 0.75;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.sidebar-app-version {
  font-size: 11px;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  color: var(--sidebar-text);
  opacity: 0.5;
  flex-shrink: 0;
}
</style>
