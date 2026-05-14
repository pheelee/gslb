<template>
  <div class="admin-users-page">
    <header class="page-header">
      <h1>User Management</h1>
      <span class="badge-admin">Admin</span>
    </header>

    <div v-if="loading" class="loading">Loading users…</div>

    <div v-else-if="error" class="error-banner">{{ error }}</div>

    <div v-else class="users-table-wrapper">
      <table class="users-table">
        <thead>
          <tr>
            <th>User</th>
            <th>Subject</th>
            <th>Last Login</th>
            <th>Status</th>
            <th>Roles</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="user in users" :key="user.id" :class="{ inactive: !user.is_active }">
            <td>
              <div class="user-cell">
                <div class="avatar">{{ initials(user) }}</div>
                <div>
                  <div class="user-name">{{ user.name || '—' }}</div>
                  <div class="user-email">{{ user.email }}</div>
                </div>
              </div>
            </td>
            <td class="mono text-secondary">{{ user.subject }}</td>
            <td class="text-secondary">{{ formatDate(user.last_login_at) }}</td>
            <td>
              <span class="badge" :class="user.is_active ? 'badge-success' : 'badge-danger'">
                {{ user.is_active ? 'Active' : 'Disabled' }}
              </span>
            </td>
            <td>
              <div class="role-tags">
                <span v-for="role in user.roles" :key="role.id" class="role-tag">
                  {{ role.name }}
                </span>
                <span v-if="user.roles.length === 0" class="text-secondary">—</span>
              </div>
            </td>
            <td>
              <div class="action-buttons">
                <button
                  v-for="role in availableRoles.filter(r => !userHasRole(user, r.id))"
                  :key="role.id"
                  class="btn btn-xs btn-secondary"
                  :disabled="saving === user.id"
                  @click="addRole(user, role.id)"
                  :title="`Grant ${role.name}`"
                >
                  + {{ role.name }}
                </button>
                <button
                  v-for="role in user.roles"
                  :key="'rm-' + role.id"
                  class="btn btn-xs btn-danger"
                  :disabled="saving === user.id"
                  @click="revokeRole(user, role.id)"
                  :title="`Revoke ${role.name}`"
                >
                  − {{ role.name }}
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>

      <p v-if="users.length === 0" class="empty-state">No users have logged in yet.</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { getUsers, getRoles, assignUserRole, removeUserRole } from '@/api/client'
import { toast } from '@/composables/useToast'
import type { User, Role } from '@/types/api'

const users = ref<User[]>([])
const availableRoles = ref<Role[]>([])
const loading = ref(true)
const error = ref('')
const saving = ref<string | null>(null)

function initials(user: User): string {
  const name = user.name || user.email || ''
  return name
    .split(/[\s@]/)
    .filter(Boolean)
    .slice(0, 2)
    .map(p => p[0].toUpperCase())
    .join('')
}

function formatDate(iso: string): string {
  if (!iso || iso.startsWith('0001')) return '—'
  return new Date(iso).toLocaleString()
}

function userHasRole(user: User, roleId: string): boolean {
  return user.roles.some(r => r.id === roleId)
}

async function addRole(user: User, roleId: string): Promise<void> {
  saving.value = user.id
  try {
    await assignUserRole(user.id, roleId)
    await reload()
    toast.success('Role assigned')
  } catch (e: unknown) {
    toast.error(e instanceof Error ? e.message : 'Failed to assign role')
  } finally {
    saving.value = null
  }
}

async function revokeRole(user: User, roleId: string): Promise<void> {
  saving.value = user.id
  try {
    await removeUserRole(user.id, roleId)
    await reload()
    toast.success('Role revoked')
  } catch (e: unknown) {
    toast.error(e instanceof Error ? e.message : 'Failed to revoke role')
  } finally {
    saving.value = null
  }
}

async function reload(): Promise<void> {
  users.value = await getUsers()
}

onMounted(async () => {
  try {
    ;[users.value, availableRoles.value] = await Promise.all([getUsers(), getRoles()])
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : 'Failed to load users'
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
.admin-users-page {
  max-width: 1100px;
  margin: 0 auto;
}

.page-header {
  display: flex;
  align-items: center;
  gap: 1rem;
  margin-bottom: 1.5rem;
}

.page-header h1 {
  margin: 0;
}

.badge-admin {
  background: rgba(243, 139, 168, 0.15);
  color: var(--ctp-red, #f38ba8);
  border: 1px solid rgba(243, 139, 168, 0.3);
  padding: 0.2rem 0.6rem;
  border-radius: 999px;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.loading,
.error-banner {
  padding: 2rem;
  text-align: center;
  color: var(--color-text-secondary);
}

.error-banner {
  color: var(--ctp-red, #f38ba8);
}

.users-table-wrapper {
  background: var(--color-surface);
  border-radius: var(--radius-lg, 8px);
  overflow: auto;
}

.users-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.875rem;
}

.users-table th {
  text-align: left;
  padding: 0.75rem 1rem;
  border-bottom: 1px solid var(--color-border);
  color: var(--color-text-secondary);
  font-weight: 600;
  font-size: 0.8125rem;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  white-space: nowrap;
}

.users-table td {
  padding: 0.875rem 1rem;
  border-bottom: 1px solid var(--color-border);
  vertical-align: middle;
}

.users-table tr:last-child td {
  border-bottom: none;
}

.users-table tr.inactive {
  opacity: 0.55;
}

.user-cell {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.avatar {
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
}

.user-name {
  font-weight: 500;
  color: var(--color-text);
}

.user-email {
  font-size: 0.8125rem;
  color: var(--color-text-secondary);
}

.mono {
  font-family: monospace;
  font-size: 0.8125rem;
}

.text-secondary {
  color: var(--color-text-secondary);
}

.badge {
  display: inline-block;
  padding: 0.2rem 0.5rem;
  border-radius: 999px;
  font-size: 0.75rem;
  font-weight: 600;
}

.badge-success {
  background: rgba(166, 227, 161, 0.15);
  color: var(--ctp-green, #a6e3a1);
}

.badge-danger {
  background: rgba(243, 139, 168, 0.15);
  color: var(--ctp-red, #f38ba8);
}

.role-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 0.375rem;
}

.role-tag {
  background: rgba(137, 180, 250, 0.15);
  color: var(--color-primary);
  border: 1px solid rgba(137, 180, 250, 0.3);
  padding: 0.15rem 0.5rem;
  border-radius: 999px;
  font-size: 0.75rem;
  font-weight: 500;
  text-transform: capitalize;
}

.action-buttons {
  display: flex;
  flex-wrap: wrap;
  gap: 0.375rem;
}

.btn-xs {
  padding: 0.2rem 0.5rem;
  font-size: 0.75rem;
  border-radius: var(--radius-sm, 4px);
  cursor: pointer;
  border: 1px solid transparent;
  font-weight: 500;
  transition: all 0.15s;
}

.btn-xs:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-xs.btn-secondary {
  background: var(--color-bg-secondary);
  border-color: var(--color-border);
  color: var(--color-text);
}

.btn-xs.btn-secondary:hover:not(:disabled) {
  border-color: var(--color-primary);
  color: var(--color-primary);
}

.btn-xs.btn-danger {
  background: rgba(243, 139, 168, 0.1);
  border-color: rgba(243, 139, 168, 0.3);
  color: var(--ctp-red, #f38ba8);
}

.btn-xs.btn-danger:hover:not(:disabled) {
  background: rgba(243, 139, 168, 0.2);
}

.empty-state {
  text-align: center;
  padding: 2rem;
  color: var(--color-text-secondary);
}
</style>
