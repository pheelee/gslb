import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { User, Role } from '@/types/api'

export type AuthStatus = 'unknown' | 'authenticated' | 'unauthenticated' | 'disabled'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<User | null>(null)
  const status = ref<AuthStatus>('unknown')

  const isAuthenticated = computed(() => status.value === 'authenticated')
  // When OIDC is disabled the backend has no auth — treat as open access
  const oidcEnabled = computed(() => status.value !== 'disabled')
  const isAdmin = computed(() => user.value?.roles.some(r => r.name === 'admin') ?? false)
  const userRoles = computed<string[]>(() => user.value?.roles.map(r => r.name) ?? [])

  async function fetchUser(): Promise<void> {
    try {
      const response = await fetch('/api/v1/auth/user')
      if (response.ok) {
        const body = await response.json()
        user.value = body.data as User
        status.value = 'authenticated'
      } else if (response.status === 401) {
        user.value = null
        status.value = 'unauthenticated'
      } else if (response.status === 404 || response.status === 501) {
        // Route not registered — OIDC is disabled on this instance
        user.value = null
        status.value = 'disabled'
      } else {
        user.value = null
        status.value = 'unauthenticated'
      }
    } catch {
      user.value = null
      status.value = 'unauthenticated'
    }
  }

  function login(): void {
    window.location.href = '/api/v1/auth/login'
  }

  async function logout(): Promise<void> {
    try {
      await fetch('/api/v1/auth/logout', { method: 'POST' })
    } finally {
      user.value = null
      status.value = 'unauthenticated'
      window.location.href = '/'
    }
  }

  function hasRole(role: string): boolean {
    return userRoles.value.includes(role)
  }

  function canAccessConfig(configRoles: Role[]): boolean {
    if (isAdmin.value) return true
    return configRoles.some(r => userRoles.value.includes(r.name))
  }

  return {
    user,
    status,
    isAuthenticated,
    oidcEnabled,
    isAdmin,
    userRoles,
    fetchUser,
    login,
    logout,
    hasRole,
    canAccessConfig,
  }
})
