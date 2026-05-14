<template>
  <div class="config-list-page">
    <header class="page-header">
      <h1>Configurations</h1>
      <router-link
        v-if="canEdit"
        to="/configs/new"
        class="btn btn-primary"
      >+ New Configuration</router-link>
    </header>

    <div class="filters">
      <input 
        v-model="searchQuery" 
        type="text" 
        placeholder="Search configurations..." 
        class="search-input"
      />
    </div>

    <div v-if="loading && configs.length === 0" class="configs-table skeleton-table">
      <table>
        <thead>
          <tr>
            <th>Name</th>
            <th>DNS Name</th>
            <th>Load Balancing</th>
            <th>Backends</th>
            <th>Status</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="n in 3" :key="n">
            <td><span class="skeleton-text" style="width: 120px;"></span></td>
            <td><span class="skeleton-text" style="width: 180px;"></span></td>
            <td><span class="skeleton-text" style="width: 80px;"></span></td>
            <td><span class="skeleton-text" style="width: 40px;"></span></td>
            <td><span class="skeleton-text" style="width: 60px;"></span></td>
            <td><span class="skeleton-text" style="width: 80px;"></span></td>
          </tr>
        </tbody>
      </table>
    </div>
    <div v-else-if="filteredConfigs.length === 0" class="empty">
      <div class="empty-content">
        <template v-if="debouncedSearchQuery">
          <div class="empty-icon">🔍</div>
          <h3>No matching configurations</h3>
          <p>Try adjusting your search terms</p>
          <button class="btn btn-secondary" @click="searchQuery = ''">Clear Search</button>
        </template>
        <template v-else>
          <div class="empty-icon">🌐</div>
          <h3>No configurations yet</h3>
          <p>Create your first load balancer to get started</p>
          <router-link to="/configs/new" class="btn btn-primary">Create Configuration</router-link>
        </template>
      </div>
    </div>
    <div v-else class="configs-table">
      <table>
        <thead>
          <tr>
            <th>Name</th>
            <th>DNS Name</th>
            <th>Load Balancing</th>
            <th>Backends</th>
            <th>Status</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="config in filteredConfigs" :key="config.id">
            <td class="name-cell">
              <strong>{{ config.name }}</strong>
            </td>
            <td class="dns-cell">{{ config.dns_name }}</td>
            <td>{{ config.lb_method }}</td>
            <td>{{ getBackendCount(config.id) }}</td>
            <td>
              <span class="status-badge" :class="getStatusClass(config)">
                {{ getStatusText(config) }}
              </span>
              <span
                v-if="config.last_reconcile_error"
                class="reconcile-error-badge"
                :title="config.last_reconcile_error"
              >DNS error</span>
            </td>
            <td class="actions-cell">
              <router-link :to="`/configs/${config.id}`" class="btn btn-sm btn-secondary">View</router-link>
              <router-link
                v-if="canEdit"
                :to="`/configs/${config.id}/edit`"
                class="btn btn-sm btn-secondary"
              >Edit</router-link>
              <button
                v-if="canDelete"
                class="btn btn-sm btn-danger"
                @click="confirmDelete(config)"
              >Delete</button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { getConfigs, deleteConfig, getBackends } from '@/api/client'
import { globalModal } from '@/composables/useModal'
import { toast } from '@/composables/useToast'
import { useAuthStore } from '@/stores/auth'
import type { Config, Backend } from '@/types/api'

const authStore = useAuthStore()
const canEdit = computed(() =>
  !authStore.oidcEnabled || authStore.isAdmin || authStore.hasRole('operator')
)
const canDelete = computed(() =>
  !authStore.oidcEnabled || authStore.isAdmin || authStore.hasRole('operator')
)

const configs = ref<Config[]>([])
const backends = ref<Map<string, Backend[]>>(new Map())
const loading = ref(true)
const searchQuery = ref('')
const debouncedSearchQuery = ref('')

const { showModal, showAlert } = globalModal

// Debounce search input
let searchTimeout: ReturnType<typeof setTimeout> | null = null
watch(searchQuery, (newValue) => {
  if (searchTimeout) {
    clearTimeout(searchTimeout)
  }
  searchTimeout = setTimeout(() => {
    debouncedSearchQuery.value = newValue
  }, 300)
})

const filteredConfigs = computed(() => {
  if (!debouncedSearchQuery.value) return configs.value
  const query = debouncedSearchQuery.value.toLowerCase()
  return configs.value.filter(c =>
    c.name.toLowerCase().includes(query) ||
    c.dns_name.toLowerCase().includes(query)
  )
})

function getBackendCount(configId: string): number {
  return backends.value.get(configId)?.length || 0
}

function getStatusClass(config: Config): string {
  const list = backends.value.get(config.id) || []
  if (list.length === 0) return 'status-empty'
  return 'status-active'
}

function getStatusText(config: Config): string {
  const list = backends.value.get(config.id) || []
  if (list.length === 0) return 'No Backends'
  return 'Active'
}

async function loadData() {
  try {
    loading.value = true
    configs.value = await getConfigs()
    
    for (const config of configs.value) {
      const list = await getBackends(config.id)
      backends.value.set(config.id, list)
    }
  } catch (error) {
    console.error('Failed to load configs:', error)
    await showAlert({
      title: 'Error',
      message: 'Failed to load configurations. Please try again.',
      variant: 'danger'
    })
  } finally {
    loading.value = false
  }
}

async function confirmDelete(config: Config) {
  const confirmed = await showModal({
    title: 'Delete Configuration',
    message: `Are you sure you want to delete "${config.name}"? This will also delete all associated backends.`,
    confirmText: 'Delete',
    cancelText: 'Cancel',
    variant: 'danger'
  })

  if (!confirmed) return

  try {
    await deleteConfig(config.id)
    configs.value = configs.value.filter(c => c.id !== config.id)
    backends.value.delete(config.id)
    toast.success('Configuration deleted successfully')
  } catch (error) {
    console.error('Failed to delete config:', error)
    await showAlert({
      title: 'Error',
      message: 'Failed to delete configuration. Please try again.',
      variant: 'danger'
    })
  }
}

onMounted(loadData)
</script>

<style scoped>
.config-list-page {
  max-width: 1200px;
  margin: 0 auto;
  padding: 2rem;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 2rem;
}

.page-header h1 {
  margin: 0;
  color: var(--color-text);
}

.filters {
  margin-bottom: 1.5rem;
}

.search-input {
  width: 100%;
  max-width: 400px;
  padding: 0.75rem;
  border: 1px solid var(--color-border);
  border-radius: 6px;
  font-size: 1rem;
  background: var(--color-surface);
  color: var(--color-text);
}

.loading {
  text-align: center;
  padding: 4rem;
  color: var(--color-text-secondary);
  background: var(--color-surface);
  border-radius: 8px;
}

.empty {
  text-align: center;
  padding: 4rem 2rem;
  color: var(--color-text-secondary);
  background: var(--color-surface);
  border-radius: 8px;
}

.empty-content {
  max-width: 400px;
  margin: 0 auto;
}

.empty-icon {
  font-size: 3rem;
  margin-bottom: 1rem;
  opacity: 0.7;
}

.empty h3 {
  color: var(--color-text);
  margin: 0 0 0.5rem 0;
  font-size: 1.25rem;
}

.empty p {
  margin: 0 0 1.5rem 0;
}

.empty .btn {
  display: inline-flex;
}

.configs-table {
  background: var(--color-surface);
  border-radius: 8px;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
  overflow: hidden;
}

table {
  width: 100%;
  border-collapse: collapse;
}

th,
td {
  padding: 1rem;
  text-align: left;
  border-bottom: 1px solid var(--color-border);
}

th {
  background: var(--color-bg-secondary);
  font-weight: 600;
  color: var(--color-text-secondary);
}

.name-cell strong {
  color: var(--color-text);
}

.dns-cell {
  font-family: var(--font-mono);
  color: var(--color-primary);
}

.status-badge {
  display: inline-block;
  padding: 0.25rem 0.75rem;
  border-radius: 12px;
  font-size: 0.85rem;
  font-weight: 500;
}

.status-active {
  background: var(--ctp-green);
  color: var(--color-bg);
}

.status-empty {
  background: var(--ctp-yellow);
  color: var(--color-bg);
}

.reconcile-error-badge {
  display: inline-block;
  margin-left: 0.5rem;
  padding: 0.2rem 0.5rem;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 600;
  background: rgba(243, 139, 168, 0.15);
  color: var(--ctp-red);
  border: 1px solid rgba(243, 139, 168, 0.3);
  cursor: help;
  vertical-align: middle;
}

.actions-cell {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
}

@media (max-width: 640px) {
  .config-list-page {
    padding: 1rem;
  }

  .page-header {
    flex-direction: column;
    gap: 1rem;
  }

  .configs-table {
    overflow-x: auto;
  }

  table {
    min-width: 600px;
  }

  .actions-cell {
    flex-direction: column;
    gap: 0.375rem;
  }
}

.skeleton-text {
  background: linear-gradient(90deg, var(--ctp-surface1) 25%, var(--ctp-surface2) 50%, var(--ctp-surface1) 75%);
  background-size: 200% 100%;
  animation: skeleton-loading 1.5s infinite;
  color: transparent;
  border-radius: 4px;
  display: inline-block;
  height: 1em;
}

.skeleton-table {
  opacity: 0.7;
}

@keyframes skeleton-loading {
  0% {
    background-position: 200% 0;
  }
  100% {
    background-position: -200% 0;
  }
}
</style>
