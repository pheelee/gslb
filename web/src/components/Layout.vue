<template>
  <div class="layout">
    <header class="layout-header">
      <div class="header-brand">
        <router-link to="/" class="brand-link">
          <span class="brand-icon">🌍</span>
          <span class="brand-text">GSLB Manager</span>
        </router-link>
      </div>
      
      <nav class="header-nav">
        <router-link 
          to="/" 
          class="nav-link" 
          :class="{ active: $route.path === '/' }"
        >
          <svg class="nav-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <rect x="3" y="3" width="7" height="7"></rect>
            <rect x="14" y="3" width="7" height="7"></rect>
            <rect x="14" y="14" width="7" height="7"></rect>
            <rect x="3" y="14" width="7" height="7"></rect>
          </svg>
          Dashboard
        </router-link>
        <router-link 
          to="/configs" 
          class="nav-link" 
          :class="{ active: $route.path.startsWith('/config') }"
        >
          <svg class="nav-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path>
            <polyline points="14 2 14 8 20 8"></polyline>
          </svg>
          Configurations
        </router-link>
        <router-link
          to="/audit-log"
          class="nav-link"
          :class="{ active: $route.path === '/audit-log' }"
        >
          <svg class="nav-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path>
            <polyline points="14 2 14 8 20 8"></polyline>
            <line x1="16" y1="13" x2="8" y2="13"></line>
            <line x1="16" y1="17" x2="8" y2="17"></line>
            <polyline points="10 9 9 9 8 9"></polyline>
          </svg>
          Audit Log
        </router-link>
        <router-link
          v-if="authStore.isAdmin"
          to="/admin/users"
          class="nav-link"
          :class="{ active: $route.path.startsWith('/admin') }"
        >
          <svg class="nav-icon" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"></path>
            <circle cx="9" cy="7" r="4"></circle>
            <path d="M23 21v-2a4 4 0 0 0-3-3.87"></path>
            <path d="M16 3.13a4 4 0 0 1 0 7.75"></path>
          </svg>
          Users
        </router-link>
      </nav>

      <div class="header-actions">
        <ThemeSelector />
        <UserMenu />
      </div>
    </header>
    
    <main class="layout-main">
      <router-view />
    </main>
  </div>
</template>

<script setup lang="ts">
import { RouterView } from 'vue-router'
import ThemeSelector from './ThemeSelector.vue'
import UserMenu from './UserMenu.vue'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()
</script>

<style scoped>
.layout {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background: var(--color-bg);
}

.layout-header {
  background: var(--color-bg-secondary);
  border-bottom: 1px solid var(--color-border);
  padding: 0 2rem;
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 64px;
  position: sticky;
  top: 0;
  z-index: 100;
}

.header-brand {
  flex-shrink: 0;
}

.brand-link {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  text-decoration: none;
  color: var(--color-text);
  font-weight: 600;
  font-size: 1.125rem;
  transition: opacity 0.2s;
}

.brand-link:hover {
  opacity: 0.8;
}

.brand-icon {
  font-size: 1.25rem;
}

.brand-text {
  background: linear-gradient(135deg, var(--color-primary), var(--ctp-mauve));
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.header-nav {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin: 0 2rem;
}

.nav-link {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  color: var(--color-text-secondary);
  text-decoration: none;
  padding: 0.5rem 1rem;
  border-radius: var(--radius-md);
  transition: all 0.2s;
  font-weight: 500;
  font-size: 0.875rem;
}

.nav-link:hover {
  color: var(--color-text);
  background: var(--color-surface);
}

.nav-link.active {
  color: var(--color-primary);
  background: rgba(137, 180, 250, 0.1);
}

.nav-icon {
  opacity: 0.7;
}

.nav-link.active .nav-icon {
  opacity: 1;
}

.header-actions {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.layout-main {
  flex: 1;
  padding: 2rem;
  max-width: 1400px;
  width: 100%;
  margin: 0 auto;
}

@media (max-width: 768px) {
  .layout-header {
    padding: 0 1rem;
    height: auto;
    min-height: 56px;
    flex-wrap: wrap;
    gap: 0.5rem;
    padding-top: 0.5rem;
    padding-bottom: 0.5rem;
  }

  .header-nav {
    margin: 0;
    order: 3;
    width: 100%;
    justify-content: center;
  }

  .brand-text {
    font-size: 1rem;
  }

  .layout-main {
    padding: 1rem;
  }
}

@media (max-width: 480px) {
  .nav-link {
    padding: 0.5rem;
    font-size: 0.8125rem;
  }

  .nav-link span:not(.nav-icon) {
    display: none;
  }
}
</style>
