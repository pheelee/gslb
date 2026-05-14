<template>
  <div class="config-form-page">
    <header class="page-header">
      <h1>{{ isEdit ? 'Edit Configuration' : 'New Configuration' }}</h1>
      <router-link to="/configs" class="btn btn-secondary">← Back to List</router-link>
    </header>

    <form @submit.prevent="saveConfig" class="form">
      <div class="form-section">
        <h2>Basic Information</h2>
        
        <div class="form-group" :class="{ 'has-error': errors.name }">
          <label for="name">Name *</label>
          <input 
            id="name"
            v-model="form.name" 
            type="text" 
            required
            placeholder="e.g., Production API"
            :class="{ 'input-error': errors.name }"
          />
          <span v-if="errors.name" class="error-message">{{ errors.name }}</span>
        </div>

        <div class="form-group" :class="{ 'has-error': errors.dns_name }">
          <label for="dns_name">DNS Name *</label>
          <input 
            id="dns_name"
            v-model="form.dns_name" 
            type="text" 
            required
            placeholder="e.g., api.example.com"
            :class="{ 'input-error': errors.dns_name }"
          />
          <span v-if="errors.dns_name" class="error-message">{{ errors.dns_name }}</span>
          <span v-else class="help">The DNS record that will be updated with healthy backends</span>
        </div>

        <div class="form-row">
          <div class="form-group" :class="{ 'has-error': errors.dns_ttl }">
            <label for="dns_ttl">DNS TTL (seconds) *</label>
            <input 
              id="dns_ttl"
              v-model.number="form.dns_ttl" 
              type="number" 
              required
              min="1"
              max="86400"
              :class="{ 'input-error': errors.dns_ttl }"
            />
            <span v-if="errors.dns_ttl" class="error-message">{{ errors.dns_ttl }}</span>
            <span v-else class="help">Time-to-live for DNS records (default: 30)</span>
          </div>

          <div class="form-group">
            <label for="lb_method">Load Balancing Method *</label>
            <select id="lb_method" v-model="form.lb_method" required>
              <option value="round_robin">Round Robin</option>
              <option value="weighted">Weighted Round Robin</option>
            </select>
          </div>
        </div>
      </div>

      <div class="form-section">
        <h2>Health Check Configuration</h2>
        
        <div class="form-row">
          <div class="form-group">
            <label for="hc_type">Check Type</label>
            <select id="hc_type" v-model="healthCheck.type">
              <option value="icmp">ICMP Ping</option>
              <option value="tcp">TCP Connect</option>
            </select>
          </div>

          <div class="form-group">
            <label for="hc_interval">Interval (seconds)</label>
            <input 
              id="hc_interval"
              v-model.number="healthCheck.interval_seconds" 
              type="number"
              min="1"
            />
          </div>

          <div class="form-group">
            <label for="hc_timeout">Timeout (seconds)</label>
            <input 
              id="hc_timeout"
              v-model.number="healthCheck.timeout_seconds" 
              type="number"
              min="1"
            />
          </div>
        </div>

      </div>

      <div class="form-section">
        <h2>DNS Provider Configuration</h2>
        <DNSProviderForm
          v-model="dnsProvider"
          ref="dnsProviderForm"
        />
      </div>

      <div v-if="authStore.oidcEnabled && availableRoles.length > 0" class="form-section">
        <h2>Access Control</h2>
        <div class="form-group">
          <label>Roles with Access</label>
          <div class="roles-grid">
            <label
              v-for="role in availableRoles"
              :key="role.id"
              class="role-option"
              :class="{ selected: selectedRoles.includes(role.name) }"
            >
              <input
                type="checkbox"
                :value="role.name"
                v-model="selectedRoles"
                class="role-checkbox"
              />
              <span class="role-name">{{ role.name }}</span>
              <span v-if="role.description" class="role-description">{{ role.description }}</span>
            </label>
          </div>
          <span class="help">Roles that can view and manage this configuration. Leave empty to allow all authenticated users.</span>
        </div>
      </div>

      <div class="form-actions">
        <button type="submit" class="btn btn-primary" :disabled="saving">
          {{ saving ? 'Saving...' : (isEdit ? 'Update Configuration' : 'Create Configuration') }}
        </button>
        <router-link to="/configs" class="btn btn-secondary">Cancel</router-link>
      </div>
    </form>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { createConfig, updateConfig, getConfig, getHealthCheck, getDNSProvider, createHealthCheck, createDNSProvider, getRoles, getConfigRoles, updateConfigRoles } from '@/api/client'
import { globalModal } from '@/composables/useModal'
import { toast } from '@/composables/useToast'
import DNSProviderForm from '@/components/DNSProviderForm.vue'
import { useAuthStore } from '@/stores/auth'
import type { Config, HealthCheck, DNSProviderConfig, Role } from '@/types/api'

const authStore = useAuthStore()
const availableRoles = ref<Role[]>([])
const selectedRoles = ref<string[]>([])

const route = useRoute()
const router = useRouter()
const { showAlert } = globalModal

const isEdit = ref(false)
const saving = ref(false)

// Snapshots of the originally loaded values used to detect changes on save.
const originalConfig = ref<Partial<Config> | null>(null)
const originalHealthCheck = ref<Partial<HealthCheck> | null>(null)
const originalDnsProvider = ref<{ providerType: string; configJson: string } | null>(null)
const originalRoles = ref<string[]>([])

interface ValidationErrors {
  name?: string
  dns_name?: string
  dns_ttl?: string
  config_json?: string
}

const errors = ref<ValidationErrors>({})

const form = ref<Partial<Config>>({
  name: '',
  dns_name: '',
  dns_ttl: 30,
  lb_method: 'round_robin'
})

const healthCheck = ref<Partial<HealthCheck>>({
  type: 'tcp',
  interval_seconds: 10,
  timeout_seconds: 5,
  threshold_healthy: 2,
  threshold_unhealthy: 3,
})

const dnsProvider = ref({
  providerType: '',
  configJson: '{}'
})

const dnsProviderForm = ref<InstanceType<typeof DNSProviderForm> | null>(null)

function validateForm(): boolean {
  errors.value = {}
  let isValid = true

  // Validate name
  if (!form.value.name?.trim()) {
    errors.value.name = 'Name is required'
    isValid = false
  } else if (form.value.name.length < 2) {
    errors.value.name = 'Name must be at least 2 characters'
    isValid = false
  }

  // Validate DNS name
  if (!form.value.dns_name?.trim()) {
    errors.value.dns_name = 'DNS name is required'
    isValid = false
  } else {
    // Basic DNS validation
    const dnsRegex = /^[a-zA-Z0-9][a-zA-Z0-9-]{0,61}[a-zA-Z0-9](?:\.[a-zA-Z0-9][a-zA-Z0-9-]{0,61}[a-zA-Z0-9])*$/
    if (!dnsRegex.test(form.value.dns_name)) {
      errors.value.dns_name = 'Please enter a valid DNS name'
      isValid = false
    }
  }

  // Validate TTL
  if (!form.value.dns_ttl || form.value.dns_ttl < 1) {
    errors.value.dns_ttl = 'TTL must be at least 1 second'
    isValid = false
  }

  // Validate DNS provider
  if (dnsProviderForm.value && !dnsProviderForm.value.validate()) {
    isValid = false
  }

  return isValid
}

async function loadConfig() {
  const id = route.params.id as string
  if (!id) return

  isEdit.value = true
  try {
    const config = await getConfig(id)
    form.value = { ...config }
    originalConfig.value = { ...config }

    // Load health check
    try {
      const hc = await getHealthCheck(id)
      healthCheck.value = { ...hc }
      originalHealthCheck.value = { ...hc }
    } catch (e) {
      console.log('No health check found')
    }

    // Load DNS provider
    try {
      const provider = await getDNSProvider(id)
      const loaded = {
        providerType: provider.provider_type,
        configJson: provider.config_json
      }
      dnsProvider.value = { ...loaded }
      originalDnsProvider.value = { ...loaded }
    } catch (e) {
      console.log('No DNS provider found')
    }

    // Load existing role assignments for edit
    if (authStore.oidcEnabled) {
      try {
        const existing = await getConfigRoles(id)
        selectedRoles.value = existing.map(r => r.name)
        originalRoles.value = existing.map(r => r.name)
      } catch { /* ignore */ }
    }
  } catch (error) {
    console.error('Failed to load config:', error)
    await showAlert({
      title: 'Error',
      message: 'Failed to load configuration. Please try again.',
      variant: 'danger'
    })
    router.push('/configs')
  }
}

function configChanged(): boolean {
  if (!originalConfig.value) return true
  const o = originalConfig.value
  const f = form.value
  return f.name !== o.name || f.dns_name !== o.dns_name ||
         f.dns_ttl !== o.dns_ttl || f.lb_method !== o.lb_method
}

function healthCheckChanged(): boolean {
  if (!healthCheck.value.type) return false
  if (!originalHealthCheck.value) return true
  const o = originalHealthCheck.value
  const h = healthCheck.value
  return h.type !== o.type ||
         h.interval_seconds !== o.interval_seconds ||
         h.timeout_seconds !== o.timeout_seconds ||
         h.threshold_healthy !== o.threshold_healthy ||
         h.threshold_unhealthy !== o.threshold_unhealthy
}

function dnsProviderChanged(): boolean {
  if (!dnsProvider.value.providerType) return false
  if (!originalDnsProvider.value) return true
  if (dnsProvider.value.providerType !== originalDnsProvider.value.providerType) return true
  // Normalize both JSON strings before comparing to ignore key-order differences.
  try {
    const curr = JSON.stringify(JSON.parse(dnsProvider.value.configJson))
    const orig = JSON.stringify(JSON.parse(originalDnsProvider.value.configJson))
    return curr !== orig
  } catch {
    return dnsProvider.value.configJson !== originalDnsProvider.value.configJson
  }
}

function rolesChanged(): boolean {
  const curr = [...selectedRoles.value].sort()
  const orig = [...originalRoles.value].sort()
  return curr.length !== orig.length || curr.some((r, i) => r !== orig[i])
}

async function saveConfig() {
  if (!validateForm()) {
    toast.error('Please fix the validation errors')
    return
  }

  saving.value = true
  try {
    let configId: string

    if (isEdit.value) {
      configId = route.params.id as string

      if (configChanged()) {
        await updateConfig(configId, form.value)
      }

      if (healthCheckChanged()) {
        await createHealthCheck(configId, healthCheck.value as Omit<HealthCheck, 'id' | 'config_id'>)
      }

      if (dnsProviderChanged()) {
        await createDNSProvider(configId, {
          provider_type: dnsProvider.value.providerType,
          config_json: dnsProvider.value.configJson,
        } as Omit<DNSProviderConfig, 'id' | 'config_id'>)
      }

      if (authStore.oidcEnabled && availableRoles.value.length > 0 && rolesChanged()) {
        await updateConfigRoles(configId, selectedRoles.value)
      }

      toast.success('Configuration updated successfully')
    } else {
      const payload: Record<string, unknown> = { ...form.value }
      if (authStore.oidcEnabled && selectedRoles.value.length > 0) {
        payload.roles = selectedRoles.value
      }
      const created = await createConfig(payload as Omit<Config, 'id' | 'created_at' | 'updated_at'>)
      configId = created.id

      if (healthCheck.value.type) {
        await createHealthCheck(configId, healthCheck.value as Omit<HealthCheck, 'id' | 'config_id'>)
      }

      if (dnsProvider.value.providerType) {
        await createDNSProvider(configId, {
          provider_type: dnsProvider.value.providerType,
          config_json: dnsProvider.value.configJson,
        } as Omit<DNSProviderConfig, 'id' | 'config_id'>)
      }
      toast.success('Configuration created successfully')
    }

    router.push(`/configs/${configId}`)
  } catch (error) {
    console.error('Failed to save config:', error)
    await showAlert({
      title: 'Error',
      message: 'Failed to save configuration. Please try again.',
      variant: 'danger',
    })
  } finally {
    saving.value = false
  }
}

onMounted(async () => {
  if (authStore.oidcEnabled) {
    try {
      availableRoles.value = await getRoles()
      if (!route.params.id) {
        // New config: default to current user's roles
        selectedRoles.value = [...authStore.userRoles]
      }
    } catch {
      // Roles unavailable — skip access control section
    }
  }
  await loadConfig()
})
</script>

<style scoped>
.config-form-page {
  max-width: 800px;
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

.form {
  background: var(--color-surface);
  padding: 2rem;
  border-radius: 8px;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.form-section {
  margin-bottom: 2rem;
  padding-bottom: 2rem;
  border-bottom: 1px solid var(--color-border);
}

.form-section:last-of-type {
  border-bottom: none;
}

.form-section h2 {
  margin-top: 0;
  margin-bottom: 1.5rem;
  color: var(--color-text);
  font-size: 1.25rem;
}

.form-group {
  margin-bottom: 1.5rem;
}

.form-row {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 1rem;
}

label {
  display: block;
  margin-bottom: 0.5rem;
  font-weight: 500;
  color: var(--color-text);
}

input,
select,
textarea {
  width: 100%;
  padding: 0.75rem;
  border: 1px solid var(--color-border);
  border-radius: 6px;
  font-size: 1rem;
  transition: border-color 0.2s;
  background: var(--color-bg);
  color: var(--color-text);
}

input:focus,
select:focus,
textarea:focus {
  outline: none;
  border-color: var(--color-primary);
}

.help {
  display: block;
  margin-top: 0.5rem;
  font-size: 0.875rem;
  color: var(--color-text-secondary);
}

.form-actions {
  display: flex;
  gap: 1rem;
  margin-top: 2rem;
  padding-top: 2rem;
  border-top: 1px solid var(--color-border);
  flex-wrap: wrap;
}

@media (max-width: 640px) {
  .config-form-page {
    padding: 1rem;
  }

  .page-header {
    flex-direction: column;
    gap: 1rem;
  }

  .form {
    padding: 1.5rem;
  }

  .form-row {
    grid-template-columns: 1fr;
  }

  .form-actions {
    flex-direction: column;
    gap: 0.75rem;
  }

  .form-actions .btn {
    width: 100%;
  }
}

.roles-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 0.75rem;
  margin-bottom: 0.5rem;
}

.role-option {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  padding: 0.875rem 1rem;
  border: 1px solid var(--color-border);
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: all 0.2s;
  background: var(--color-bg);
}

.role-option:hover {
  border-color: var(--color-primary);
  background: rgba(137, 180, 250, 0.05);
}

.role-option.selected {
  border-color: var(--color-primary);
  background: rgba(137, 180, 250, 0.1);
}

.role-checkbox {
  width: auto;
  margin-bottom: 0.25rem;
}

.role-name {
  font-weight: 600;
  font-size: 0.875rem;
  color: var(--color-text);
  text-transform: capitalize;
}

.role-description {
  font-size: 0.8125rem;
  color: var(--color-text-secondary);
  line-height: 1.3;
}

/* Form Validation Styles */
.form-group.has-error {
  margin-bottom: 1rem;
}

.input-error {
  border-color: var(--ctp-red) !important;
  box-shadow: 0 0 0 3px rgba(243, 139, 168, 0.15) !important;
}

.input-error:focus {
  border-color: var(--ctp-red) !important;
  box-shadow: 0 0 0 3px rgba(243, 139, 168, 0.2) !important;
}

.error-message {
  display: block;
  margin-top: 0.375rem;
  font-size: 0.8125rem;
  color: var(--ctp-red);
  font-weight: 500;
}
</style>
