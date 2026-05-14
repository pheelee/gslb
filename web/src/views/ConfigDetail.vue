<template>
  <div class="config-detail-page">
    <header class="page-header">
      <div>
        <h1 :class="{ 'skeleton-text': loading && !config }">{{ config?.name || 'Loading...' }}</h1>
        <p class="dns-name" :class="{ 'skeleton-text': loading && !config }">{{ config?.dns_name || '...' }}</p>
      </div>
      <div class="header-actions">
        <router-link
          v-if="canEdit"
          :to="`/configs/${configId}/edit`"
          class="btn btn-secondary"
        >Edit</router-link>
        <router-link to="/configs" class="btn btn-secondary">← Back</router-link>
      </div>
    </header>

    <div v-if="!loading && !config" class="error">Configuration not found</div>

    <div v-if="config?.last_reconcile_error" class="reconcile-error-banner">
      <span class="reconcile-error-icon">⚠</span>
      <div class="reconcile-error-body">
        <strong>DNS reconciliation failed</strong>
        <span class="reconcile-error-msg">{{ config.last_reconcile_error }}</span>
      </div>
    </div>

    <div class="detail-content">
      <div class="detail-grid">
        <div v-if="authStore.oidcEnabled && configRoles.length > 0" class="detail-card roles-card">
          <h2>Access Control</h2>
          <div class="role-tags">
            <span v-for="role in configRoles" :key="role.id" class="role-tag">
              {{ role.name }}
            </span>
          </div>
          <p class="roles-help">Roles permitted to access this configuration.</p>
        </div>

        <div class="detail-card">
          <h2>Configuration Details</h2>
          <div class="detail-row">
            <span class="label">DNS Name:</span>
            <span class="value" :class="{ 'skeleton-text': loading && !config }">{{ config?.dns_name || '...' }}</span>
          </div>
          <div class="detail-row">
            <span class="label">DNS TTL:</span>
            <span class="value" :class="{ 'skeleton-text': loading && !config }">{{ config ? config.dns_ttl + ' seconds' : '...' }}</span>
          </div>
          <div class="detail-row">
            <span class="label">Load Balancing:</span>
            <span class="value" :class="{ 'skeleton-text': loading && !config }">{{ config?.lb_method || '...' }}</span>
          </div>
          <div class="detail-row">
            <span class="label">Created:</span>
            <span class="value" :class="{ 'skeleton-text': loading && !config }">{{ config ? formatDate(config.created_at) : '...' }}</span>
          </div>
        </div>

        <div class="detail-card">
          <h2>Health Check</h2>
          <div v-if="healthCheck" class="health-check-info">
            <div class="detail-row">
              <span class="label">Type:</span>
              <span class="value">{{ healthCheck.type }}</span>
            </div>
            <div class="detail-row">
              <span class="label">Interval:</span>
              <span class="value">{{ healthCheck.interval_seconds }}s</span>
            </div>
            <div class="detail-row">
              <span class="label">Timeout:</span>
              <span class="value">{{ healthCheck.timeout_seconds }}s</span>
            </div>
          </div>
          <div v-else-if="!loading" class="empty-section">
            No health check configured
            <router-link :to="`/configs/${configId}/edit`">Configure</router-link>
          </div>
          <div v-else class="detail-row">
            <span class="label">Status:</span>
            <span class="value skeleton-text">Loading...</span>
          </div>
        </div>
      </div>

      <div class="backends-section">
        <div class="section-header">
          <h2>Backends ({{ backends.length }})</h2>
          <button class="btn btn-primary" @click="showAddBackend = true">+ Add Backend</button>
        </div>

        <div v-if="backends.length === 0 && !loading" class="empty-section">
          No backends configured yet.
          <a href="#" @click.prevent="showAddBackend = true">Add your first backend</a>
        </div>

        <div v-else class="backends-list">
          <div
            v-for="(backend, index) in backends"
            :key="backend.id"
            class="backend-item"
            :class="{ disabled: !backend.enabled, dragging: draggedIndex === index }"
            @dragover.prevent="dragOver(index)"
            @drop="drop(index)"
          >
            <div
              class="drag-handle"
              title="Drag to reorder"
              draggable="true"
              @dragstart="dragStart(index, $event)"
              @dragend="dragEnd"
              @touchstart.passive="touchStart(index, $event)"
              @touchmove="touchMove($event)"
              @touchend="touchEnd"
            >
              <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor">
                <circle cx="4" cy="4" r="1.5"/>
                <circle cx="8" cy="4" r="1.5"/>
                <circle cx="12" cy="4" r="1.5"/>
                <circle cx="4" cy="8" r="1.5"/>
                <circle cx="8" cy="8" r="1.5"/>
                <circle cx="12" cy="8" r="1.5"/>
                <circle cx="4" cy="12" r="1.5"/>
                <circle cx="8" cy="12" r="1.5"/>
                <circle cx="12" cy="12" r="1.5"/>
              </svg>
            </div>
            <div class="backend-info">
              <div class="backend-address">
                {{ backend.ip }}{{ backend.port ? ':' + backend.port : '' }}
              </div>
              <div class="backend-meta">
                <span class="order-badge">#{{ index + 1 }}</span>
                <span v-if="!backend.enabled" class="disabled-badge">Disabled</span>
              </div>
              <BackendHistory :backend-id="backend.id" />
            </div>
            <div class="backend-health">
              <HealthStatus :status="getHealthStatus(backend.id)" />
            </div>
            <div class="backend-actions">
              <button
                class="btn btn-sm"
                :class="backend.enabled ? 'btn-warning' : 'btn-success'"
                @click="toggleBackend(backend)"
                :disabled="togglingBackend === backend.id"
              >
                {{ togglingBackend === backend.id ? '...' : (backend.enabled ? 'Disable' : 'Enable') }}
              </button>
              <button 
                class="btn btn-sm btn-danger" 
                @click="deleteBackend(backend)"
                :disabled="deletingBackend === backend.id"
              >
                {{ deletingBackend === backend.id ? '...' : 'Delete' }}
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Add Backend Modal -->
    <div v-if="showAddBackend" class="modal-overlay" @click.self="showAddBackend = false">
      <div class="modal">
        <h2>Add Backend</h2>
        <form @submit.prevent="addBackend">
          <div class="form-group">
            <label>IP Address *</label>
            <input v-model="newBackend.ip" type="text" required placeholder="192.168.1.1" />
          </div>
          <div class="form-group">
            <label>Port</label>
            <input v-model.number="newBackend.port" type="number" placeholder="80" />
          </div>
          <div class="form-group">
            <label>Weight</label>
            <input v-model.number="newBackend.weight" type="number" min="1" value="1" />
          </div>
          <div class="form-actions">
            <button type="submit" class="btn btn-primary" :disabled="addingBackend">
              {{ addingBackend ? 'Adding...' : 'Add Backend' }}
            </button>
            <button type="button" class="btn btn-secondary" @click="showAddBackend = false">Cancel</button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute } from 'vue-router'
import { getConfig, getBackends, getBackendHealth, getHealthCheck, createBackend, updateBackend, deleteBackend as apiDeleteBackend, getConfigRoles } from '@/api/client'
import HealthStatus from '@/components/HealthStatus.vue'
import BackendHistory from '@/components/BackendHistory.vue'
import { globalModal } from '@/composables/useModal'
import { toast } from '@/composables/useToast'
import { useAuthStore } from '@/stores/auth'
import type { Config, Backend, HealthState, Role } from '@/types/api'

const authStore = useAuthStore()
const configRoles = ref<Role[]>([])
const canEdit = computed(() =>
  !authStore.oidcEnabled || authStore.isAdmin || authStore.hasRole('operator')
)

const { showModal, showAlert } = globalModal

const route = useRoute()
const configId = route.params.id as string

const config = ref<Config | null>(null)
const backends = ref<Backend[]>([])
const healthStates = ref<Map<string, HealthState>>(new Map())
const healthCheck = ref<any>(null)
const loading = ref(true)
const showAddBackend = ref(false)
const addingBackend = ref(false)
const togglingBackend = ref<string | null>(null)
const deletingBackend = ref<string | null>(null)

const newBackend = ref({
  ip: '',
  port: undefined as number | undefined,
  weight: 1,
  enabled: true
})

// Drag and drop state
const draggedIndex = ref<number | null>(null)
const draggedBackend = ref<Backend | null>(null)
const touchStartY = ref<number>(0)
const touchCurrentIndex = ref<number | null>(null)

function getHealthStatus(backendId: string): string {
  return healthStates.value.get(backendId)?.status || 'unknown'
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleString()
}

async function loadData(isBackground = false) {
  try {
    if (!isBackground) loading.value = true
    config.value = await getConfig(configId)

    if (authStore.oidcEnabled) {
      try { configRoles.value = await getConfigRoles(configId) } catch { /* ignore */ }
    }

    const loadedBackends = await getBackends(configId)
    // Sort backends by weight
    backends.value = loadedBackends.sort((a, b) => (a.weight || 0) - (b.weight || 0))

    // Load health check config
    try {
      healthCheck.value = await getHealthCheck(configId)
    } catch (e) {
      console.log('No health check configured')
      healthCheck.value = null
    }

    // Load health states
    for (const backend of backends.value) {
      try {
        const health = await getBackendHealth(backend.id)
        healthStates.value.set(backend.id, health)
      } catch (e) {
        console.error(`Failed to load health for ${backend.id}:`, e)
      }
    }
  } catch (error) {
    console.error('Failed to load config:', error)
  } finally {
    loading.value = false
  }
}

async function addBackend() {
  addingBackend.value = true
  try {
    await createBackend(configId, newBackend.value)
    showAddBackend.value = false
    newBackend.value = { ip: '', port: undefined, weight: 1, enabled: true }
    await loadData()
    toast.success('Backend added successfully')
  } catch (error) {
    console.error('Failed to add backend:', error)
    await showAlert({
      title: 'Error',
      message: 'Failed to add backend. Please try again.',
      variant: 'danger'
    })
  } finally {
    addingBackend.value = false
  }
}

async function toggleBackend(backend: Backend) {
  togglingBackend.value = backend.id
  try {
    await updateBackend(backend.id, { enabled: !backend.enabled })
    backend.enabled = !backend.enabled
    toast.success(backend.enabled ? 'Backend enabled' : 'Backend disabled')
  } catch (error) {
    console.error('Failed to toggle backend:', error)
    await showAlert({
      title: 'Error',
      message: 'Failed to update backend status. Please try again.',
      variant: 'danger'
    })
    // Revert the local state change on error
    await loadData()
  } finally {
    togglingBackend.value = null
  }
}

async function deleteBackend(backend: Backend) {
  const confirmed = await showModal({
    title: 'Delete Backend',
    message: `Are you sure you want to delete backend ${backend.ip}?`,
    confirmText: 'Delete',
    cancelText: 'Cancel',
    variant: 'danger'
  })

  if (!confirmed) return

  deletingBackend.value = backend.id
  try {
    await apiDeleteBackend(backend.id)
    backends.value = backends.value.filter(b => b.id !== backend.id)
    toast.success('Backend deleted successfully')
  } catch (error) {
    console.error('Failed to delete backend:', error)
    await showAlert({
      title: 'Error',
      message: 'Failed to delete backend. Please try again.',
      variant: 'danger'
    })
  } finally {
    deletingBackend.value = null
  }
}

// Drag and drop handlers
function dragStart(index: number, event: DragEvent) {
  draggedIndex.value = index
  draggedBackend.value = backends.value[index]
  // Use the whole backend-item row as the drag image so it's clear what's moving
  const row = (event.target as HTMLElement).closest('.backend-item') as HTMLElement
  if (row && event.dataTransfer) {
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setDragImage(row, 0, 0)
  }
}

function dragOver(index: number) {
  if (draggedIndex.value === null || draggedIndex.value === index) return

  // Reorder the array
  const item = backends.value[draggedIndex.value]
  backends.value.splice(draggedIndex.value, 1)
  backends.value.splice(index, 0, item)

  // Update the dragged index
  draggedIndex.value = index
}

async function drop(_index: number) {
  if (draggedIndex.value === null) return

  // Save the new order to backend
  try {
    await saveBackendOrder()
    toast.success('Backend order updated')
  } catch (error) {
    console.error('Failed to save backend order:', error)
    await showAlert({
      title: 'Error',
      message: 'Failed to save backend order. Please try again.',
      variant: 'danger'
    })
    // Reload to restore original order
    await loadData()
  }
}

function dragEnd() {
  draggedIndex.value = null
  draggedBackend.value = null
}

// Touch event handlers for mobile drag and drop
function touchStart(index: number, event: TouchEvent) {
  draggedIndex.value = index
  draggedBackend.value = backends.value[index]
  touchStartY.value = event.touches[0].clientY
  touchCurrentIndex.value = index
}

function touchMove(event: TouchEvent) {
  if (draggedIndex.value === null) return
  
  event.preventDefault()
  const touch = event.touches[0]
  const element = document.elementFromPoint(touch.clientX, touch.clientY)
  const backendItem = element?.closest('.backend-item')
  
  if (backendItem) {
    const index = Array.from(backendItem.parentElement?.children || []).indexOf(backendItem)
    if (index !== -1 && index !== touchCurrentIndex.value) {
      // Reorder the array
      const item = backends.value[draggedIndex.value]
      backends.value.splice(draggedIndex.value, 1)
      backends.value.splice(index, 0, item)
      
      // Update indices
      draggedIndex.value = index
      touchCurrentIndex.value = index
    }
  }
}

async function touchEnd() {
  if (draggedIndex.value === null) return
  
  try {
    await saveBackendOrder()
    toast.success('Backend order updated')
  } catch (error) {
    console.error('Failed to save backend order:', error)
    await showAlert({
      title: 'Error',
      message: 'Failed to save backend order. Please try again.',
      variant: 'danger'
    })
    await loadData()
  } finally {
    draggedIndex.value = null
    draggedBackend.value = null
    touchCurrentIndex.value = null
  }
}

async function saveBackendOrder() {
  // Update each backend with its new weight (position + 1)
  const updates = backends.value.map((backend, index) => ({
    id: backend.id,
    weight: index + 1
  }))

  // Update each backend
  for (const update of updates) {
    await updateBackend(update.id, { weight: update.weight })
  }
}

onMounted(() => {
  loadData()
  // Auto-refresh data every 5 seconds
  const interval = setInterval(() => loadData(true), 5000)
  
  // Cleanup on unmount
  onUnmounted(() => {
    clearInterval(interval)
  })
})
</script>

<style scoped>
.config-detail-page {
  max-width: 1200px;
  margin: 0 auto;
  padding: 2rem;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 2rem;
}

.page-header h1 {
  margin: 0;
  color: var(--color-text);
}

.dns-name {
  color: var(--color-primary);
  font-family: var(--font-mono);
  margin: 0.5rem 0 0 0;
}

.header-actions {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.loading,
.error {
  text-align: center;
  padding: 4rem;
  background: var(--color-surface);
  border-radius: 8px;
}

.detail-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(400px, 1fr));
  gap: 1.5rem;
  margin-bottom: 2rem;
}

.roles-card .role-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin-bottom: 0.75rem;
}

.role-tag {
  background: rgba(137, 180, 250, 0.15);
  color: var(--color-primary);
  border: 1px solid rgba(137, 180, 250, 0.3);
  padding: 0.2rem 0.6rem;
  border-radius: 999px;
  font-size: 0.8125rem;
  font-weight: 500;
  text-transform: capitalize;
}

.roles-help {
  font-size: 0.8125rem;
  color: var(--color-text-secondary);
  margin: 0;
}

.detail-card {
  background: var(--color-surface);
  padding: 1.5rem;
  border-radius: 8px;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.detail-card h2 {
  margin-top: 0;
  margin-bottom: 1rem;
  color: var(--color-text);
  font-size: 1.1rem;
}

.detail-row {
  display: flex;
  justify-content: space-between;
  padding: 0.75rem 0;
  border-bottom: 1px solid var(--color-border);
}

.detail-row:last-child {
  border-bottom: none;
}

.label {
  color: var(--color-text-secondary);
  font-weight: 500;
}

.value {
  color: var(--color-text);
  font-family: var(--font-mono);
}

.empty-section {
  color: var(--color-text-secondary);
  padding: 2rem 0;
}

.empty-section a {
  color: var(--color-primary);
}

.backends-section {
  background: var(--color-surface);
  padding: 1.5rem;
  border-radius: 8px;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1.5rem;
}

.section-header h2 {
  margin: 0;
  color: var(--color-text);
}

.backends-list {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.backend-item {
  display: flex;
  align-items: center;
  padding: 1rem;
  border: 1px solid var(--color-border);
  border-radius: 6px;
  gap: 1rem;
}

.backend-item.disabled {
  opacity: 0.6;
  background: var(--color-bg-secondary);
}

.backend-info {
  flex: 1;
}

.backend-address {
  font-family: var(--font-mono);
  font-size: 1.1rem;
  color: var(--color-text);
}

.backend-meta {
  margin-top: 0.25rem;
  display: flex;
  gap: 0.5rem;
}

.weight {
  background: var(--color-primary);
  color: var(--color-bg);
  padding: 0.25rem 0.5rem;
  border-radius: 4px;
  font-size: 0.85rem;
}

.disabled-badge {
  background: var(--ctp-red);
  color: var(--color-bg);
  padding: 0.25rem 0.5rem;
  border-radius: 4px;
  font-size: 0.85rem;
}

.backend-health {
  width: 100px;
}

.backend-actions {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.drag-handle {
  cursor: grab;
  /* Generous padding gives a large touch target without growing the icon visually */
  padding: 0.75rem 0.625rem;
  margin: -0.75rem -0.25rem;
  color: var(--color-text-secondary);
  opacity: 0.5;
  transition: opacity 0.2s, color 0.2s;
  display: flex;
  align-items: center;
  flex-shrink: 0;
  touch-action: none; /* prevent scroll interfering with touch-drag */
  user-select: none;
  -webkit-user-select: none;
}

.drag-handle:hover {
  opacity: 1;
  color: var(--color-primary);
}

.drag-handle:active {
  cursor: grabbing;
  opacity: 1;
}

.backend-item.dragging {
  opacity: 0.7;
  background: var(--color-bg-secondary);
  border: 2px dashed var(--color-primary);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
}

.btn {
  display: inline-block;
  padding: 0.75rem 1.5rem;
  border-radius: 6px;
  text-decoration: none;
  font-weight: 500;
  cursor: pointer;
  border: none;
  transition: all 0.2s;
}

.btn-primary {
  background: var(--color-primary);
  color: var(--color-bg);
}

.btn-secondary {
  background: var(--color-bg-secondary);
  color: var(--color-text);
}

.btn-success {
  background: var(--ctp-green);
  color: var(--color-bg);
}

.btn-warning {
  background: var(--ctp-yellow);
  color: var(--color-bg);
}

.btn-danger {
  background: var(--ctp-red);
  color: var(--color-bg);
}

.btn-sm {
  padding: 0.4rem 0.8rem;
  font-size: 0.875rem;
}

.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal {
  background: var(--color-surface);
  padding: 2rem;
  border-radius: 8px;
  width: 100%;
  max-width: 400px;
}

.modal h2 {
  margin-top: 0;
  margin-bottom: 1.5rem;
  color: var(--color-text);
}

.form-group {
  margin-bottom: 1rem;
}

.form-group label {
  display: block;
  margin-bottom: 0.5rem;
  font-weight: 500;
  color: var(--color-text);
}

.form-group input {
  width: 100%;
  padding: 0.75rem;
  border: 1px solid var(--color-border);
  border-radius: 6px;
  background: var(--color-bg);
  color: var(--color-text);
}

.form-actions {
  display: flex;
  gap: 1rem;
  margin-top: 1.5rem;
  flex-wrap: wrap;
}

@media (max-width: 640px) {
  .page-header {
    flex-direction: column;
    gap: 1rem;
  }

  .header-actions {
    width: 100%;
    justify-content: flex-start;
  }

  .detail-grid {
    grid-template-columns: 1fr;
  }

  .backend-item {
    flex-direction: column;
    align-items: flex-start;
    gap: 0.75rem;
  }

  .backend-health {
    width: auto;
  }

  .backend-actions {
    width: 100%;
    justify-content: flex-start;
  }
}

.skeleton-text {
  background: linear-gradient(90deg, var(--ctp-surface1) 25%, var(--ctp-surface2) 50%, var(--ctp-surface1) 75%);
  background-size: 200% 100%;
  animation: skeleton-loading 1.5s infinite;
  color: transparent;
  border-radius: 4px;
  display: inline-block;
  min-width: 100px;
}

@keyframes skeleton-loading {
  0% {
    background-position: 200% 0;
  }
  100% {
    background-position: -200% 0;
  }
}

.reconcile-error-banner {
  display: flex;
  align-items: flex-start;
  gap: 0.75rem;
  padding: 0.875rem 1.25rem;
  margin-bottom: 1.5rem;
  border-radius: 8px;
  background: rgba(243, 139, 168, 0.1);
  border: 1px solid rgba(243, 139, 168, 0.35);
}

.reconcile-error-icon {
  font-size: 1.1rem;
  color: var(--ctp-red);
  flex-shrink: 0;
  margin-top: 0.05rem;
}

.reconcile-error-body {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
}

.reconcile-error-body strong {
  color: var(--ctp-red);
  font-size: 0.9rem;
}

.reconcile-error-msg {
  font-family: var(--font-mono);
  font-size: 0.8125rem;
  color: var(--color-text-secondary);
  word-break: break-word;
}
</style>
