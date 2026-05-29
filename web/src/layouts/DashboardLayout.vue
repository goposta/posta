<script setup lang="ts">
import { computed, ref, onMounted, onBeforeUnmount } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useThemeStore } from '../stores/theme'
import { useWorkspaceStore } from '../stores/workspace'
import { infoApi, type AppInfo } from '../api/info'
import EmailVerificationBanner from '../components/EmailVerificationBanner.vue'

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()
const theme = useThemeStore()
const wsStore = useWorkspaceStore()
const sidebarCollapsed = ref(localStorage.getItem('posta_sidebar_collapsed') === 'true')
const mobileOpen = ref(false)
const appInfo = ref<AppInfo | null>(null)
const userMenuOpen = ref(false)
const wsSwitcherOpen = ref(false)
const themeModes = ['light', 'dark', 'system'] as const

function closeUserMenu(e: MouseEvent) {
  const el = document.querySelector('.user-menu')
  if (el && !el.contains(e.target as Node)) {
    userMenuOpen.value = false
  }
}

const toggleSidebar = () => {
  sidebarCollapsed.value = !sidebarCollapsed.value
  localStorage.setItem('posta_sidebar_collapsed', String(sidebarCollapsed.value))
}

onMounted(async () => {
  document.addEventListener('click', closeUserMenu)
  try {
    const res = await infoApi.get()
    appInfo.value = res.data.data
  } catch {
    // Version display is non-critical
  }

  wsStore.fetchWorkspaces()
})
onBeforeUnmount(() => {
  document.removeEventListener('click', closeUserMenu)
})

const user = computed(() => auth.user)

const navItems = [
  { name: 'Dashboard', path: '/', icon: 'grid' },
  { name: 'Analytics', path: '/analytics', icon: 'bar-chart' },
  { name: 'Emails', path: '/emails', icon: 'mail' },
  { name: 'Inbound', path: '/inbound-emails', icon: 'inbox' },
  { name: 'Templates', path: '/templates', icon: 'file-text' },
  { name: 'Languages', path: '/languages', icon: 'type' },
  { name: 'Stylesheets', path: '/stylesheets', icon: 'edit-3' },

  { name: 'Webhooks', path: '/webhooks', icon: 'link' },
  { name: 'Deliveries', path: '/webhook-deliveries', icon: 'activity' },
  { name: 'Contacts', path: '/contacts', icon: 'users' },
  { name: 'Subscribers', path: '/subscribers', icon: 'users' },
  { name: 'Subscriber Lists', path: '/subscriber-lists', icon: 'list' },
  { name: 'Campaigns', path: '/campaigns', icon: 'send' },
  { name: 'Unsubscribe Lists', path: '/unsubscribe-lists', icon: 'mail-x' },
  { name: 'Bounces', path: '/bounces', icon: 'alert-triangle' },
  { name: 'API Keys', path: '/api-keys', icon: 'key' },
  { name: 'Audit Log', path: '/audit-log', icon: 'activity' },
  { name: 'Workspaces', path: '/workspaces', icon: 'briefcase' },
  { name: 'Domains', path: '/domains', icon: 'globe' },
  { name: 'SMTP Servers', path: '/smtp-servers', icon: 'server' },
  { name: 'Settings', path: '/settings', icon: 'settings' },
]

const adminItems = [
  { name: 'Users', path: '/admin/users', icon: 'users' },
  { name: 'Plans', path: '/admin/plans', icon: 'layers' },
  { name: 'Shared Servers', path: '/admin/servers', icon: 'server' },
  { name: 'Jobs', path: '/admin/jobs', icon: 'clock' },
  { name: 'Metrics', path: '/admin/metrics', icon: 'bar-chart' },
  { name: 'Events', path: '/admin/events', icon: 'activity' },
  { name: 'OAuth', path: '/admin/oauth', icon: 'key' },
  { name: 'Settings', path: '/admin/settings', icon: 'settings' },
  { name: 'About', path: '/about', icon: 'info' },
]

function isActive(path: string): boolean {
  if (path === '/') return route.path === '/'
  return route.path === path || route.path.startsWith(path + '/')
}

function navigate(path: string) {
  router.push(path)
}

function switchContext(wsId: number | null) {
  wsStore.setWorkspace(wsId)
  wsSwitcherOpen.value = false
  router.push('/')
}

function logout() {
  auth.logout()
  router.push('/login')
}

function getIcon(name: string): string {
  const icons: Record<string, string> = {
    'grid': '<svg width="18" height="18" viewBox="0 0 18 18" fill="none"><rect x="1.5" y="1.5" width="6" height="6" rx="1.5" stroke="currentColor" stroke-width="1.5"/><rect x="10.5" y="1.5" width="6" height="6" rx="1.5" stroke="currentColor" stroke-width="1.5"/><rect x="1.5" y="10.5" width="6" height="6" rx="1.5" stroke="currentColor" stroke-width="1.5"/><rect x="10.5" y="10.5" width="6" height="6" rx="1.5" stroke="currentColor" stroke-width="1.5"/></svg>',
    'mail': '<svg width="18" height="18" viewBox="0 0 18 18" fill="none"><rect x="2" y="3.5" width="14" height="11" rx="2" stroke="currentColor" stroke-width="1.5"/><path d="M2 5.5l7 5 7-5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
    'inbox': '<svg width="18" height="18" viewBox="0 0 18 18" fill="none"><path d="M16.5 10.5H13.5l-1.5 2.25h-6L4.5 10.5H1.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/><path d="M4.1 3.2L1.5 10.5v4.5A1.5 1.5 0 003 16.5h12a1.5 1.5 0 001.5-1.5v-4.5l-2.6-7.3A1.5 1.5 0 0012.48 2.25H5.52a1.5 1.5 0 00-1.42 1.05z" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
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
    'send': '<svg width="18" height="18" viewBox="0 0 18 18" fill="none"><path d="M16.5 1.5L8.25 9.75M16.5 1.5l-5.25 15-3-6.75L1.5 6.75l15-5.25z" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
    'briefcase': '<svg width="18" height="18" viewBox="0 0 18 18" fill="none"><rect x="2" y="6" width="14" height="10" rx="1.5" stroke="currentColor" stroke-width="1.5"/><path d="M12 6V4.5A1.5 1.5 0 0010.5 3h-3A1.5 1.5 0 006 4.5V6" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
    'settings': '<svg width="18" height="18" viewBox="0 0 18 18" fill="none"><circle cx="9" cy="9" r="2.25" stroke="currentColor" stroke-width="1.5"/><path d="M14.7 11.1a1.2 1.2 0 00.24 1.32l.04.04a1.46 1.46 0 11-2.06 2.06l-.04-.04a1.2 1.2 0 00-1.32-.24 1.2 1.2 0 00-.73 1.1v.12a1.46 1.46 0 01-2.91 0v-.06a1.2 1.2 0 00-.79-1.1 1.2 1.2 0 00-1.32.24l-.04.04a1.46 1.46 0 11-2.06-2.06l.04-.04a1.2 1.2 0 00.24-1.32 1.2 1.2 0 00-1.1-.73h-.12a1.46 1.46 0 010-2.91h.06a1.2 1.2 0 001.1-.79 1.2 1.2 0 00-.24-1.32l-.04-.04a1.46 1.46 0 112.06-2.06l.04.04a1.2 1.2 0 001.32.24h.06a1.2 1.2 0 00.73-1.1v-.12a1.46 1.46 0 012.91 0v.06a1.2 1.2 0 00.73 1.1 1.2 1.2 0 001.32-.24l.04-.04a1.46 1.46 0 112.06 2.06l-.04.04a1.2 1.2 0 00-.24 1.32v.06a1.2 1.2 0 001.1.73h.12a1.46 1.46 0 010 2.91h-.06a1.2 1.2 0 00-1.1.73z" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
    'layers': '<svg width="18" height="18" viewBox="0 0 18 18" fill="none"><path d="M9 1.5L1.5 6 9 10.5 16.5 6 9 1.5z" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/><path d="M1.5 12L9 16.5 16.5 12" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/><path d="M1.5 9L9 13.5 16.5 9" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
    'info': '<svg width="18" height="18" viewBox="0 0 18 18" fill="none"><circle cx="9" cy="9" r="7" stroke="currentColor" stroke-width="1.5"/><path d="M9 12.75V9M9 5.25h.007" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>',
    'mail-x': '<svg width="18" height="18" viewBox="0 0 18 18" fill="none"><path d="M16 8.5V4a2 2 0 00-2-2H4a2 2 0 00-2 2v8a2 2 0 002 2h6" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/><path d="M2 4.5l7 5 7-5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/><path d="M12.5 12.5l3.5 3.5M16 12.5l-3.5 3.5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>',
  }
  return icons[name] || ''
}
</script>

<template>
  <div class="layout" :class="{ 'sidebar-collapsed': sidebarCollapsed }">
    <aside class="sidebar">
        <div class="sidebar-header">
            <img src="/logo.png" alt="Posta" class="sidebar-logo" @click="navigate('/')" />
            <span class="sidebar-brand-text" @click="navigate('/')">Posta</span>
         
          <button class="sidebar-collapse-btn" @click="toggleSidebar"
            :title="sidebarCollapsed ? 'Expand sidebar' : 'Collapse sidebar'">
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
              <path :d="sidebarCollapsed ? 'M6 3l5 5-5 5' : 'M10 3L5 8l5 5'" stroke="currentColor" stroke-width="1.5"
                stroke-linecap="round" stroke-linejoin="round" />
            </svg>
          </button>

        </div>
        <!-- Workspace Switcher -->
        <div v-if="wsStore.workspaces.length > 0" class="ws-switcher">
          <div class="ws-switcher-toggle" @click="wsSwitcherOpen = !wsSwitcherOpen">
            <div class="ws-switcher-current">
              <div class="ws-avatar">{{ wsStore.currentWorkspace?.name?.charAt(0)?.toUpperCase() || 'P' }}</div>
              <span v-if="!sidebarCollapsed" class="ws-switcher-name">{{ wsStore.contextLabel }}</span>
            </div>
            <svg v-if="!sidebarCollapsed" width="14" height="14" viewBox="0 0 16 16" fill="none">
              <path d="M4 6l4 4 4-4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"
                stroke-linejoin="round" />
            </svg>
          </div>
          <div v-if="wsSwitcherOpen" class="ws-switcher-dropdown">
            <div class="ws-switcher-option" :class="{ active: wsStore.isPersonal }" @click="switchContext(null)">
              <div class="ws-avatar-sm">P</div>
              <span>Personal</span>
            </div>
            <div v-for="ws in wsStore.workspaces" :key="ws.id" class="ws-switcher-option"
              :class="{ active: wsStore.currentWorkspaceId === ws.id }" @click="switchContext(ws.id)">
              <div class="ws-avatar-sm">{{ ws.name.charAt(0).toUpperCase() }}</div>
              <span>{{ ws.name }}</span>
              <span class="ws-role-badge">{{ ws.role }}</span>
            </div>
          </div>
        </div>

        <nav class="sidebar-nav">
          <div class="nav-section">
            <router-link v-for="item in navItems" :key="item.path" class="nav-item"
              :class="{ active: isActive(item.path) }" :title="sidebarCollapsed ? item.name : ''" :to="item.path">
              <span class="nav-icon" v-html="getIcon(item.icon)"></span>
              <span v-if="!sidebarCollapsed" class="nav-label">{{ item.name }}</span>
            </router-link>

          </div>
          <div v-if="auth.isAdmin" class="nav-section">
            <div class="nav-section-title">Admin</div>
            <router-link v-for="item in adminItems" :key="item.path" class="nav-item"
              :class="{ active: isActive(item.path) }" :title="sidebarCollapsed ? item.name : ''" :to="item.path">
              <span class="nav-icon" v-html="getIcon(item.icon)"></span>
              <span v-if="!sidebarCollapsed" class="nav-label">{{ item.name }}</span>
            </router-link>
          </div>
        </nav>

      <div class="sidebar-footer">
        <div class="nav-section">
          <template v-if="appInfo?.openapi_docs">
            <div class="nav-section-title">Docs</div>
            <a class="nav-item" href="/docs" target="_blank" rel="noopener noreferrer"
              :title="sidebarCollapsed ? 'Scalar UI' : ''">
              <span class="nav-icon" v-html="getIcon('book-open')"></span>
              <span class="nav-label">Scalar UI</span>
            </a>
            <a class="nav-item" href="/swagger" target="_blank" rel="noopener noreferrer"
              :title="sidebarCollapsed ? 'Swagger UI' : ''">
              <span class="nav-icon" v-html="getIcon('file-text')"></span>
              <span class="nav-label">Swagger UI</span>
            </a>
          </template>

          <div v-if="appInfo && !sidebarCollapsed" class="sidebar-version">
            <span class="nav-label">v{{ appInfo.version }}</span>
          </div>
        </div>
      </div>
    </aside>

    <div class="main-wrapper">
      <header class="topbar">
        <div class="topbar-left">
          <!-- Mobile menu button -->
          <button class="mobile-menu-btn" @click="mobileOpen = true">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
              stroke-linecap="round" stroke-linejoin="round">
              <line x1="3" y1="12" x2="21" y2="12" />
              <line x1="3" y1="6" x2="21" y2="6" />
              <line x1="3" y1="18" x2="21" y2="18" />
            </svg>
          </button>
        </div>
        <div class="topbar-right">
          <div class="user-menu" @click="userMenuOpen = !userMenuOpen">
            <div class="user-avatar">{{ user?.name?.charAt(0)?.toUpperCase() || '?' }}</div>
            <div class="user-menu-info">
              <div class="user-name">{{ user?.name || 'User' }}</div>
              <div class="user-email">{{ user?.email || '' }}</div>
            </div>
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
              <path d="M4 6l4 4 4-4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"
                stroke-linejoin="round" />
            </svg>

            <div v-if="userMenuOpen" class="user-dropdown">
              <div class="user-dropdown-header">
                <span class="user-avatar user-avatar-lg">{{ user?.name?.charAt(0)?.toUpperCase() || '?' }}</span>
                <div class="user-dropdown-info">
                  <div class="user-dropdown-name">{{ user?.name || 'User' }}</div>
                  <div class="user-dropdown-email">{{ user?.email || '' }}</div>
                </div>
              </div>
              <div class="user-dropdown-divider" />

              <a class="user-dropdown-item" @click.stop="navigate('/profile'); userMenuOpen = false">
                <svg width="15" height="15" viewBox="0 0 16 16" fill="none">
                  <path d="M12.67 14v-1.33A2.67 2.67 0 0010 10H6a2.67 2.67 0 00-2.67 2.67V14" stroke="currentColor"
                    stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" />
                  <circle cx="8" cy="5.33" r="2.67" stroke="currentColor" stroke-width="1.5" />
                </svg>
                My Profile
              </a>
              <div class="user-dropdown-divider"></div>
              <div class="user-dropdown-theme">
                <div class="user-dropdown-theme-label">
                  <svg width="15" height="15" viewBox="0 0 16 16" fill="none">
                    <circle cx="8" cy="8" r="3" stroke="currentColor" stroke-width="1.5" />
                    <path
                      d="M8 1v2M8 13v2M1 8h2M13 8h2M3.05 3.05l1.41 1.41M11.54 11.54l1.41 1.41M3.05 12.95l1.41-1.41M11.54 4.46l1.41-1.41"
                      stroke="currentColor" stroke-width="1.5" stroke-linecap="round" />
                  </svg>
                  Theme
                </div>
                <div class="theme-switcher">
                  <button v-for="m in themeModes" :key="m" :class="['theme-btn', { active: theme.mode === m }]"
                    :title="m.charAt(0).toUpperCase() + m.slice(1)" @click.stop="theme.setMode(m)">
                    <svg v-if="m === 'light'" width="14" height="14" viewBox="0 0 16 16" fill="none">
                      <circle cx="8" cy="8" r="3" stroke="currentColor" stroke-width="1.5" />
                      <path
                        d="M8 1v2M8 13v2M1 8h2M13 8h2M3.05 3.05l1.41 1.41M11.54 11.54l1.41 1.41M3.05 12.95l1.41-1.41M11.54 4.46l1.41-1.41"
                        stroke="currentColor" stroke-width="1.5" stroke-linecap="round" />
                    </svg>
                    <svg v-else-if="m === 'dark'" width="14" height="14" viewBox="0 0 16 16" fill="none">
                      <path d="M14 9.5A6.5 6.5 0 016.5 2 6.5 6.5 0 1014 9.5z" stroke="currentColor" stroke-width="1.5"
                        stroke-linecap="round" stroke-linejoin="round" />
                    </svg>
                    <svg v-else width="14" height="14" viewBox="0 0 16 16" fill="none">
                      <rect x="2" y="3" width="12" height="10" rx="1.5" stroke="currentColor" stroke-width="1.5" />
                      <path d="M2 5.5h12" stroke="currentColor" stroke-width="1.5" />
                    </svg>
                  </button>
                </div>
              </div>
              <div class="user-dropdown-divider"></div>
              <a class="user-dropdown-item user-dropdown-logout" @click.stop="logout">
                <svg width="15" height="15" viewBox="0 0 16 16" fill="none">
                  <path
                    d="M6 14H3.33A1.33 1.33 0 012 12.67V3.33A1.33 1.33 0 013.33 2H6M10.67 11.33L14 8l-3.33-3.33M14 8H6"
                    stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" />
                </svg>
                Logout
              </a>
            </div>
          </div>
        </div>
      </header>
      <main class="main-content">
        <EmailVerificationBanner />
        <router-view />
      </main>
      <footer class="main-footer">
        <div class="footer-left">
          <span>&copy; {{ new Date().getFullYear() }} Jonas Kaninda</span>
          <span v-if="appInfo" class="footer-version" :title="appInfo.commit_id ? `Commit: ${appInfo.commit_id}` : ''">
            v{{ appInfo.version }}<template v-if="appInfo.commit_id"> ({{ appInfo.commit_id.slice(0, 7) }})</template>
          </span>
        </div>
        <div class="footer-right">
          <a href="https://github.com/goposta/posta" target="_blank" rel="noopener noreferrer" class="footer-link">
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
              <path
                d="M8 1C4.13 1 1 4.13 1 8a7 7 0 004.79 6.65c.35.06.48-.15.48-.34 0-.17-.01-.71-.01-1.29-1.76.33-2.2-.43-2.34-.82-.08-.2-.42-.82-.71-.99-.24-.13-.59-.46-.01-.47.55-.01.94.51 1.07.71.63 1.05 1.63.76 2.03.57.06-.45.24-.76.44-.93-1.55-.17-3.18-.78-3.18-3.46 0-.76.27-1.39.71-1.88-.07-.17-.31-.89.07-1.85 0 0 .58-.19 1.9.71a6.5 6.5 0 013.46 0c1.32-.9 1.9-.71 1.9-.71.38.96.14 1.68.07 1.85.44.49.71 1.11.71 1.88 0 2.69-1.64 3.29-3.19 3.46.25.22.47.64.47 1.29 0 .93-.01 1.68-.01 1.91 0 .19.13.41.48.34A7 7 0 0015 8c0-3.87-3.13-7-7-7z"
                fill="currentColor" />
            </svg>
            GitHub
          </a>
        </div>
      </footer>
    </div>
    <!-- Mobile sidebar overlay -->
    <Transition name="overlay-fade">
      <div v-if="mobileOpen" class="sidebar-overlay" @click="mobileOpen = false" />
    </Transition>

    <!-- Mobile sidebar -->
    <Transition name="sidebar-slide">
      <aside v-if="mobileOpen" class="sidebar sidebar-mobile">
        <div class="sidebar-header">
          <img src="/logo.png" alt="Posta" class="sidebar-logo" />
          <span class="sidebar-brand-text">Posta </span>
          <button class="sidebar-collapse-btn" @click="mobileOpen = false">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
              stroke-linecap="round" stroke-linejoin="round">
              <line x1="18" y1="6" x2="6" y2="18" />
              <line x1="6" y1="6" x2="18" y2="18" />
            </svg>
          </button>
        </div>

        <!-- ws Switcher (mobile) -->
        <div v-if="wsStore.workspaces.length > 0" class="ws-switcher">
          <div class="ws-switcher-toggle" @click="wsSwitcherOpen = !wsSwitcherOpen">
            <div style="display: flex; align-items: center; gap: 8px; overflow: hidden;">
              <span class="ws-avatar">
                {{ wsStore.currentWorkspace?.name?.charAt(0)?.toUpperCase() || 'P' }}
              </span>
              <span class="ws-name">{{ wsStore.contextLabel }}</span>
            </div>
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"
              stroke-linecap="round" stroke-linejoin="round">
              <polyline points="6 9 12 15 18 9" />
            </svg>
          </div>

          <Transition name="dropdown">
            <div v-if="wsSwitcherOpen" class="ws-dropdown">
              <div class="ws-option" :class="{ active: wsStore.isPersonal }"
                @click="switchContext(null); mobileOpen = false">
                <span class="ws-avatar">P</span>
                <span>Personal</span>
              </div>

              <div v-for="ws in wsStore.workspaces" :key="ws.id" class="ws-option"
                :class="{ active: wsStore.currentWorkspaceId === ws.id }"
                @click="switchContext(ws.id); mobileOpen = false">
                <span class="ws-avatar">{{ ws.name?.charAt(0)?.toUpperCase() }}</span>
                <div style="display: flex; flex-direction: column;">
                  <span>{{ ws.name }}</span>
                  <small v-if="ws.role" style="font-size: 0.75rem; opacity: 0.7;">{{ ws.role }}</small>
                </div>
              </div>
            </div>
          </Transition>
        </div>

        <nav class="sidebar-nav">
          <div class="nav-section">
            <router-link v-for="item in navItems" :key="item.path" class="nav-item"
              :class="{ active: isActive(item.path) }" @click="mobileOpen = false" :to="item.path">
              <span class="nav-icon" v-html="getIcon(item.icon)"></span>
              <span v-if="!sidebarCollapsed" class="nav-label">{{ item.name }}</span>
            </router-link>
          </div>

          <!-- Admin section -->
          <div v-if="auth.isAdmin" class="nav-section">
            <div class="nav-section-title">Admin</div>
            <router-link v-for="item in adminItems" :key="item.path" class="nav-item"
              :class="{ active: isActive(item.path) }" @click="mobileOpen = false" :to="item.path">
              <span class="nav-icon" v-html="getIcon(item.icon)"></span>
              <span v-if="!sidebarCollapsed" class="nav-label">{{ item.name }}</span>
            </router-link>
          </div>

        </nav>
      </aside>
    </Transition>
  </div>
</template>


<style scoped>
.layout {
  display: flex;
  min-height: 100vh;
  background: var(--bg-secondary);
}

/* ─── Sidebar ─── */
.sidebar {
  position: fixed;
  top: 0;
  left: 0;
  bottom: 0;
  width: 240px;
  background: var(--bg-sidebar);
  display: flex;
  flex-direction: column;
  z-index: 40;
  transition: width var(--transition-slow);
}

.sidebar-collapsed .sidebar:not(.sidebar-mobile) {
  width: 64px;
}

.sidebar-header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 16px 14px 12px;
  flex-shrink: 0;
  position: relative;
}

.sidebar-logo {
  width: 40px;
  height: 40px;
  flex-shrink: 0;
  border-radius: 6px;
  cursor: pointer;
}

.sidebar-brand-text-dot {
  color: var(--primary-500);
  margin-left: 1px;
}
.sidebar-brand-text {
  font-size: 15px;
  font-weight: 700;
  color: #ffffff;
  white-space: nowrap;
  overflow: hidden;
  opacity: 1;
  transition: opacity var(--transition-slow);
  cursor: pointer;
}

.sidebar-collapsed .sidebar:not(.sidebar-mobile) .sidebar-brand-text {
  opacity: 0;
  width: 0;
}

.sidebar-collapse-btn {
  background: none;
  border: none;
  color: var(--sidebar-text);
  cursor: pointer;
  padding: 4px;
  border-radius: var(--radius-sm);
  display: flex;
  align-items: center;
  justify-content: center;
  transition: color var(--transition), background var(--transition);
  flex-shrink: 0;
  position: absolute;
  top: 18px;
  right: 10px;
}

.sidebar-collapse-btn:hover {
  color: #ffffff;
  background: var(--sidebar-hover);
}

.sidebar-collapsed .sidebar:not(.sidebar-mobile) .sidebar-collapse-btn {
  /* left: 50%; */
  transform: translateX(-50%);
  right: -15px;
  top: 20px;
  z-index: 999;
}

/* ─── Navigation ─── */
.sidebar-nav {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  padding: 8px;
}

.nav-section {
  margin-bottom: 8px;
}

.nav-section-title {
  font-size: 11px;
  font-weight: 600;
  color: var(--sidebar-text);
  text-transform: uppercase;
  letter-spacing: 0.06em;
  padding: 12px 12px 6px;
  white-space: nowrap;
  overflow: hidden;
  opacity: 1;
  transition: opacity var(--transition-slow);
}

.sidebar-collapsed .sidebar:not(.sidebar-mobile) .nav-section-title {
  opacity: 0;
  height: 0;
  padding: 0;
  margin: 0;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 9px 12px;
  border-radius: var(--radius);
  color: var(--sidebar-text);
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: background var(--transition), color var(--transition);
  text-decoration: none;
  white-space: nowrap;
  overflow: hidden;
}

.nav-item:hover {
  background: var(--sidebar-hover);
  color: var(--sidebar-text-active);
}

.nav-item.active {
  background: var(--sidebar-hover);
  color: var(--sidebar-text-active);
}

.nav-item.active::before {
  content: '';
  position: absolute;
  left: 0;
  top: 50%;
  transform: translateY(-50%);
  width: 3px;
  height: 20px;
  background: var(--primary-500);
  border-radius: 0 3px 3px 0;
}

.nav-icon {
  flex-shrink: 0;
  width: 18px;
  height: 18px;
}

.nav-label {
  overflow: hidden;
  opacity: 1;
  transition: opacity var(--transition-slow);
}

.sidebar-collapsed .sidebar:not(.sidebar-mobile) .nav-label {
  opacity: 0;
  width: 0;
}

.sidebar-collapsed .sidebar:not(.sidebar-mobile) .nav-item {
  justify-content: center;
  padding: 9px;
}

/* ─── Sidebar footer ─── */
.sidebar-footer {
  border-top: 1px solid var(--sidebar-border);
  padding: 8px;
  flex-shrink: 0;
}

.sidebar-version {
  padding: 8px 12px;
  font-size: 12px;
  color: var(--sidebar-text);
  opacity: 0.6;
  white-space: nowrap;
  overflow: hidden;
}

.sidebar-collapsed .sidebar:not(.sidebar-mobile) .sidebar-version {
  opacity: 0;
}

.sidebar-collapsed .sidebar:not(.sidebar-mobile) .sidebar-footer .nav-item {
  justify-content: center;
  padding: 9px;
}

/* ─── Main wrapper ─── */
.main-wrapper {
  flex: 1;
  margin-left: 240px;
  display: flex;
  flex-direction: column;
  min-height: 100vh;
  transition: margin-left var(--transition-slow);
}

.sidebar-collapsed .main-wrapper {
  margin-left: 64px;
}

/* ─── Top bar ─── */
.topbar {
  position: sticky;
  top: 0;
  z-index: 30;
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 56px;
  padding: 0 24px;
  background: var(--bg-primary);
  border-bottom: 1px solid var(--border-primary);
  transition: background var(--transition-slow), border-color var(--transition-slow);
}

.topbar-left {
  display: flex;
  align-items: center;
}

.mobile-menu-btn {
  display: none;
  align-items: center;
  justify-content: center;
  background: none;
  border: none;
  color: var(--text-tertiary);
  cursor: pointer;
  padding: 6px;
  border-radius: var(--radius-sm);
  transition: color var(--transition), background var(--transition);
}

.mobile-menu-btn:hover {
  color: var(--text-primary);
  background: var(--bg-hover);
}

.topbar-right {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-left: auto;
}

/* Workspace Switcher */
.ws-switcher {
  padding: 0 8px 8px;
  position: relative;
}

.ws-switcher-toggle {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 10px;
  border-radius: var(--radius, 8px);
  cursor: pointer;
  transition: all var(--transition, 150ms ease);
  color: var(--sidebar-text);
  border: 1px solid var(--sidebar-border);
}

.ws-switcher-toggle:hover {
  background: var(--sidebar-hover);
  color: var(--sidebar-text-active);
}

.ws-switcher-current {
  display: flex;
  align-items: center;
  gap: 8px;
  overflow: hidden;
}

.ws-avatar {
  width: 24px;
  height: 24px;
  border-radius: 6px;
  background: var(--primary-600, #9333ea);
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  font-weight: 600;
  flex-shrink: 0;
}

.ws-avatar-sm {
  width: 20px;
  height: 20px;
  border-radius: 5px;
  background: var(--primary-600, #9333ea);
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 10px;
  font-weight: 600;
  flex-shrink: 0;
}

.ws-switcher-name {
  font-size: 13px;
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.ws-switcher.collapsed .ws-switcher-toggle {
  justify-content: center;
  padding: 8px;
  border: none;
}

.ws-switcher-dropdown {
  position: absolute;
  top: calc(100% + 2px);
  left: 8px;
  right: 8px;
  background: var(--bg-primary, #fff);
  border: 1px solid var(--border-primary, #e5e7eb);
  border-radius: var(--radius, 8px);
  box-shadow: var(--shadow-lg, 0 10px 25px rgba(0, 0, 0, 0.08));
  padding: 4px;
  z-index: 200;
  min-width: 200px;
}

.ws-switcher-option {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  font-size: 13px;
  font-weight: 500;
  color: var(--text-secondary, #4b5563);
  border-radius: var(--radius-sm, 6px);
  cursor: pointer;
  transition: all var(--transition, 150ms ease);
}

.ws-switcher-option:hover {
  background: var(--bg-secondary, #f9fafb);
  color: var(--text-primary, #111827);
}

.ws-switcher-option.active {
  background: var(--primary-50, #faf5ff);
  color: var(--primary-700, #7e22ce);
}

.ws-role-badge {
  margin-left: auto;
  font-size: 10px;
  font-weight: 600;
  text-transform: uppercase;
  color: var(--text-muted, #9ca3af);
  letter-spacing: 0.05em;
}

/* ─── User menu ─── */
.user-menu {
  position: relative;
}

.user-menu-trigger {
  display: flex;
  align-items: center;
  gap: 8px;
  background: none;
  border: 1px solid transparent;
  padding: 5px 10px 5px 5px;
  border-radius: var(--radius);
  cursor: pointer;
  color: var(--text-secondary);
  font-family: inherit;
  font-size: 14px;
  transition: background var(--transition), border-color var(--transition);
}

.user-menu-trigger:hover {
  background: var(--bg-hover);
  border-color: var(--border-primary);
}

.user-avatar {
  width: 30px;
  height: 30px;
  border-radius: 50%;
  background: var(--primary-600);
  color: var(--text-on-primary);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 13px;
  font-weight: 600;
  flex-shrink: 0;
}

.user-avatar-lg {
  width: 36px;
  height: 36px;
  font-size: 15px;
}

.user-name {
  font-weight: 500;
  max-width: 140px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* ─── User dropdown ─── */
.user-dropdown {
  position: absolute;
  top: calc(100% + 6px);
  right: 0;
  width: 280px;
  background: var(--bg-primary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-lg);
  z-index: 50;
  overflow: hidden;
}

.user-dropdown-header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px 16px;
}

.user-dropdown-info {
  overflow: hidden;
}

.user-dropdown-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.user-dropdown-email {
  font-size: 12px;
  color: var(--text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.user-dropdown-divider {
  height: 1px;
  background: var(--border-primary);
}

.user-dropdown-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 16px;
  font-size: 14px;
  color: var(--text-secondary);
  cursor: pointer;
  transition: background var(--transition), color var(--transition);
  text-decoration: none;
  border: none;
  background: none;
  width: 100%;
  font-family: inherit;
}

.user-dropdown-item:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.user-dropdown-logout {
  color: var(--danger-600);
}

.user-dropdown-logout:hover {
  background: var(--danger-50);
  color: var(--danger-700);
}

/* ─── Theme switcher ─── */
.user-dropdown-theme {
  padding: 10px 16px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.user-dropdown-theme-label {
  font-size: 13px;
  color: var(--text-muted);
  font-weight: 500;
}

.theme-switcher {
  display: flex;
  background: var(--bg-tertiary);
  border-radius: var(--radius-sm);
  padding: 2px;
  gap: 2px;
}

.theme-btn {
  padding: 4px 10px;
  border: none;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  color: var(--text-tertiary);
  background: transparent;
  font-family: inherit;
  transition: all var(--transition);
}

.theme-btn:hover {
  color: var(--text-primary);
}

.theme-btn.active {
  background: var(--bg-primary);
  color: var(--text-primary);
  box-shadow: var(--shadow-sm);
}

/* ─── Main content ─── */
.main-content {
  flex: 1;
  padding: 28px;
}

/* ─── Footer ─── */
.main-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 28px;
  border-top: 1px solid var(--border-primary);
  font-size: 13px;
  color: var(--text-muted);
  background: var(--bg-primary);
  transition: background var(--transition-slow), border-color var(--transition-slow);
}

.main-footer a {
  color: var(--primary-600);
  text-decoration: none;
}

.main-footer a:hover {
  color: var(--primary-700);
}

.footer-version {
  color: var(--text-muted);
  font-size: 12px;
}

.footer-github {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--text-muted);
  transition: color var(--transition);
}

.footer-github:hover {
  color: var(--text-primary);
}

/* ─── Mobile overlay ─── */
.sidebar-overlay {
  position: fixed;
  inset: 0;
  background: var(--overlay);
  z-index: 35;
  backdrop-filter: blur(4px);
}

.sidebar-mobile {
  z-index: 45;
  width: 240px;
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

.user-menu:hover {
  background: var(--bg-secondary);
}

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

.user-menu-info {
  overflow: hidden;
}

.user-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.user-email {
  font-size: 11px;
  color: var(--text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

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

.dropdown-item:hover {
  background: var(--bg-secondary);
  color: var(--text-primary);
}

.dropdown-item-danger:hover {
  background: var(--danger-50, #fef2f2);
  color: var(--danger-600, #dc2626);
}

.dropdown-divider {
  height: 1px;
  background: var(--border-primary);
  margin: 4px 0;
}

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

.overlay-fade-enter-active {
  transition: opacity 200ms ease;
}

.overlay-fade-leave-active {
  transition: opacity 150ms ease;
}

.overlay-fade-enter-from,
.overlay-fade-leave-to {
  opacity: 0;
}

.sidebar-slide-enter-active {
  transition: transform 200ms ease;
}

.sidebar-slide-leave-active {
  transition: transform 150ms ease;
}

.sidebar-slide-enter-from,
.sidebar-slide-leave-to {
  transform: translateX(-100%);
}

/* ─── Responsive ─── */
@media (max-width: 1024px) {
  .sidebar:not(.sidebar-mobile) {
    display: none;
  }

  .main-wrapper {
    margin-left: 0 !important;
  }

  .mobile-menu-btn {
    display: flex;
  }
}

@media (max-width: 640px) {
  .main-content {
    padding: 20px 16px;
    width: 100%;
    max-width: 100vw;
    box-sizing: border-box;
    min-width: 0;
    overflow-x: hidden;
  }

  .topbar {
    padding: 0 16px;
  }

  .main-footer {
    padding: 14px 16px;
    flex-direction: column;
    gap: 8px;
    text-align: center;
  }

  .user-name,
  .user-email {
    display: none;
  }
}

/* ─── ws Switcher ─── */
.ws-switcher {
  padding: 0 8px 8px;
  position: relative;
}

.ws-switcher-toggle {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 10px;
  border-radius: var(--radius);
  cursor: pointer;
  color: var(--sidebar-text);
  border: 1px solid var(--sidebar-border);
  transition: background var(--transition), color var(--transition);
}

.ws-switcher-toggle:hover {
  background: var(--sidebar-hover);
  color: var(--sidebar-text-active);
}

.ws-avatar {
  width: 24px;
  height: 24px;
  border-radius: 6px;
  background: var(--primary-600);
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  font-weight: 600;
  flex-shrink: 0;
}

.ws-name {
  font-size: 13px;
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.ws-dropdown {
  position: absolute;
  top: 100%;
  left: 8px;
  right: 8px;
  background: var(--bg-primary);
  border: 1px solid var(--border-primary);
  border-radius: var(--radius);
  box-shadow: var(--shadow-lg);
  padding: 4px;
  z-index: 200;
  margin-top: 4px;
}

.ws-option {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border-radius: var(--radius-sm);
  cursor: pointer;
  font-size: 13px;
  color: var(--text-secondary);
  transition: background var(--transition), color var(--transition);
}

.ws-option:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.ws-option.active {
  background: var(--primary-50, rgba(99, 102, 241, 0.1));
  color: var(--primary-600);
  font-weight: 500;
}

.sidebar-collapsed .sidebar:not(.sidebar-mobile) .ws-switcher-toggle {
  justify-content: center;
  padding: 8px;
}

.sidebar-collapsed .sidebar:not(.sidebar-mobile) .ws-switcher-toggle .ws-name,
.sidebar-collapsed .sidebar:not(.sidebar-mobile) .ws-switcher-toggle svg.nav-label {
  opacity: 0;
  width: 0;
  overflow: hidden;
}
</style>
