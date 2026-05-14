<template>
  <div class="user-menu">
    <!-- OIDC disabled — no auth UI needed -->
    <template v-if="!authStore.oidcEnabled" />

    <!-- Not yet fetched -->
    <template v-else-if="authStore.status === 'unknown'" />

    <!-- Not logged in -->
    <button
      v-else-if="!authStore.isAuthenticated"
      class="btn btn-primary btn-sm"
      @click="authStore.login()"
    >
      Login
    </button>

    <!-- Logged in -->
    <div v-else class="user-menu-wrapper" ref="wrapperRef">
      <button
        class="user-avatar"
        :title="authStore.user?.email"
        :aria-expanded="open"
        aria-haspopup="true"
        @click="open = !open"
      >
        {{ initials }}
      </button>

      <div v-if="open" class="dropdown">
        <div class="dropdown-header">
          <span class="dropdown-name">{{ authStore.user?.name || authStore.user?.email }}</span>
          <span class="dropdown-roles">{{ authStore.userRoles.join(', ') }}</span>
        </div>
        <div class="dropdown-divider" />
        <button class="dropdown-item" @click="logout">
          <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
            <polyline points="16 17 21 12 16 7" />
            <line x1="21" y1="12" x2="9" y2="12" />
          </svg>
          Logout
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()
const open = ref(false)
const wrapperRef = ref<HTMLElement | null>(null)

const initials = computed(() => {
  const name = authStore.user?.name || authStore.user?.email || ''
  return name
    .split(/[\s@]/)
    .filter(Boolean)
    .slice(0, 2)
    .map(p => p[0].toUpperCase())
    .join('')
})

function logout() {
  open.value = false
  authStore.logout()
}

function handleClickOutside(event: MouseEvent) {
  if (wrapperRef.value && !wrapperRef.value.contains(event.target as Node)) {
    open.value = false
  }
}

onMounted(() => document.addEventListener('mousedown', handleClickOutside))
onUnmounted(() => document.removeEventListener('mousedown', handleClickOutside))
</script>

<style scoped>
.user-menu {
  display: flex;
  align-items: center;
}

.user-menu-wrapper {
  position: relative;
}

.user-avatar {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: var(--color-primary);
  color: var(--color-bg);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 0.75rem;
  font-weight: 700;
  flex-shrink: 0;
  cursor: pointer;
  border: 2px solid transparent;
  transition: border-color 0.15s, opacity 0.15s;
  padding: 0;
}

.user-avatar:hover {
  border-color: var(--color-primary);
  opacity: 0.9;
}

.dropdown {
  position: absolute;
  top: calc(100% + 0.5rem);
  right: 0;
  min-width: 180px;
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.18);
  z-index: 200;
  overflow: hidden;
}

.dropdown-header {
  display: flex;
  flex-direction: column;
  padding: 0.75rem 1rem;
  gap: 0.15rem;
}

.dropdown-name {
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--color-text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.dropdown-roles {
  font-size: 0.75rem;
  color: var(--color-text-secondary);
  text-transform: capitalize;
}

.dropdown-divider {
  height: 1px;
  background: var(--color-border);
  margin: 0;
}

.dropdown-item {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  width: 100%;
  padding: 0.625rem 1rem;
  background: transparent;
  border: none;
  color: var(--color-text-secondary);
  font-size: 0.875rem;
  cursor: pointer;
  transition: background 0.15s, color 0.15s;
  text-align: left;
}

.dropdown-item:hover {
  background: var(--color-bg-secondary);
  color: var(--color-text);
}

.btn-primary {
  background: var(--color-primary);
  color: var(--color-bg);
  border: none;
  border-radius: var(--radius-md);
  cursor: pointer;
  font-weight: 500;
}

.btn-sm {
  padding: 0.375rem 0.75rem;
  font-size: 0.875rem;
}
</style>
