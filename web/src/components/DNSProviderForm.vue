<template>
  <div class="dns-provider-form">
    <div class="form-group">
      <label for="provider_type">DNS Provider *</label>
      <select
        id="provider_type"
        v-model="providerType"
        @change="onProviderChange"
      >
        <option value="">Select a provider</option>
        <option value="cloudflare">Cloudflare</option>
        <option value="mock">Mock (Testing)</option>
      </select>
      <span class="help">Select your DNS provider</span>
    </div>

    <div v-if="providerType === 'cloudflare'" class="provider-fields">
      <div class="form-group" :class="{ 'has-error': errors.apiToken }">
        <label for="cf_api_token">API Token *</label>
        <input
          id="cf_api_token"
          v-model="cloudflareConfig.apiToken"
          type="password"
          placeholder="Enter your Cloudflare API token"
          :class="{ 'input-error': errors.apiToken }"
        />
        <span v-if="errors.apiToken" class="error-message">{{ errors.apiToken }}</span>
        <span v-else class="help">
          Create a token at Cloudflare dashboard → My Profile → API Tokens
        </span>
      </div>

      <div class="form-group" :class="{ 'has-error': errors.zoneId }">
        <label for="cf_zone_id">Zone ID *</label>
        <input
          id="cf_zone_id"
          v-model="cloudflareConfig.zoneId"
          type="text"
          placeholder="e.g., 1a2b3c4d5e6f..."
          :class="{ 'input-error': errors.zoneId }"
        />
        <span v-if="errors.zoneId" class="error-message">{{ errors.zoneId }}</span>
        <span v-else class="help">
          Found in Cloudflare dashboard → your domain → Overview (right sidebar)
        </span>
      </div>

      <div class="info-box">
        <h4>🔒 Security Note</h4>
        <p>Your API token is stored encrypted and is only used to update DNS records. 
           The token should have <strong>Zone:Edit</strong> permissions for your domain.</p>
      </div>
    </div>

    <div v-else-if="providerType === 'mock'" class="provider-fields">
      <div class="info-box warning">
        <h4>⚠️ Test Mode</h4>
        <p>The Mock provider simulates DNS operations without making real changes. 
           Use this for testing GSLB functionality without affecting live DNS.</p>
      </div>
    </div>

    <div v-else class="provider-placeholder">
      <p class="placeholder-text">Select a DNS provider above to configure it</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, toRef } from 'vue'

interface CloudflareConfig {
  apiToken: string
  zoneId: string
}

interface Props {
  modelValue: {
    providerType: string
    configJson: string
  }
}

const props = defineProps<Props>()
const emit = defineEmits<{
  'update:modelValue': [value: { providerType: string; configJson: string }]
}>()

const providerType = ref(props.modelValue.providerType || '')
const cloudflareConfig = ref<CloudflareConfig>({ apiToken: '', zoneId: '' })
const errors = ref<{ apiToken?: string; zoneId?: string }>({})

function applyModelValue(val: Props['modelValue']) {
  if (!val) return
  // Only assign when the value actually changes to avoid triggering the
  // [providerType, cloudflareConfig] watcher and creating a reactive loop.
  const newType = val.providerType || ''
  if (newType !== providerType.value) {
    providerType.value = newType
  }
  if (val.providerType === 'cloudflare' && val.configJson) {
    try {
      const parsed = JSON.parse(val.configJson)
      const newToken = parsed.api_token || ''
      const newZone = parsed.zone_id || ''
      if (newToken !== cloudflareConfig.value.apiToken || newZone !== cloudflareConfig.value.zoneId) {
        cloudflareConfig.value = { apiToken: newToken, zoneId: newZone }
      }
    } catch {
      // Invalid JSON, leave fields empty
    }
  }
}

// Apply initial value (covers the case where parent passes data synchronously)
applyModelValue(props.modelValue)

// Re-apply whenever the parent updates the prop (e.g. after async loadConfig()).
// The change-guard inside applyModelValue prevents a circular update loop.
watch(toRef(props, 'modelValue'), applyModelValue, { deep: true })

const isValid = computed(() => {
  if (!providerType.value) return false
  
  if (providerType.value === 'cloudflare') {
    return cloudflareConfig.value.apiToken.length > 0 && 
           cloudflareConfig.value.zoneId.length > 0
  }
  
  return true
})

function validate(): boolean {
  errors.value = {}
  
  if (providerType.value === 'cloudflare') {
    if (!cloudflareConfig.value.apiToken) {
      errors.value.apiToken = 'API Token is required'
    }
    if (!cloudflareConfig.value.zoneId) {
      errors.value.zoneId = 'Zone ID is required'
    }
  }
  
  return Object.keys(errors.value).length === 0
}

function onProviderChange() {
  // Reset config when provider changes
  cloudflareConfig.value = { apiToken: '', zoneId: '' }
  updateValue()
}

function updateValue() {
  let configJson = '{}'
  
  if (providerType.value === 'cloudflare') {
    configJson = JSON.stringify({
      api_token: cloudflareConfig.value.apiToken,
      zone_id: cloudflareConfig.value.zoneId
    })
  }
  
  emit('update:modelValue', {
    providerType: providerType.value,
    configJson
  })
}

// Watch for changes and update
watch([providerType, cloudflareConfig], () => {
  if (validate()) {
    updateValue()
  }
}, { deep: true })

// Expose validate method for parent
defineExpose({ validate, isValid })
</script>

<style scoped>
.dns-provider-form {
  margin-top: 1rem;
}

.provider-fields {
  margin-top: 1.5rem;
  padding: 1.5rem;
  background: var(--color-bg-secondary);
  border-radius: 8px;
  border: 1px solid var(--color-border);
}

.form-group {
  margin-bottom: 1.5rem;
}

.form-group:last-child {
  margin-bottom: 0;
}

label {
  display: block;
  margin-bottom: 0.5rem;
  color: var(--color-text);
  font-weight: 500;
}

input, select {
  width: 100%;
  padding: 0.75rem;
  border: 1px solid var(--color-border);
  border-radius: 6px;
  background: var(--color-surface);
  color: var(--color-text);
  font-size: 0.875rem;
  transition: border-color 0.2s;
}

input:focus, select:focus {
  outline: none;
  border-color: var(--color-primary);
}

input[type="password"] {
  font-family: monospace;
}

.help {
  display: block;
  margin-top: 0.375rem;
  font-size: 0.8125rem;
  color: var(--color-text-secondary);
}

.error-message {
  display: block;
  margin-top: 0.375rem;
  font-size: 0.8125rem;
  color: var(--ctp-red);
}

.input-error {
  border-color: var(--ctp-red) !important;
}

.info-box {
  margin-top: 1.5rem;
  padding: 1rem;
  background: rgba(137, 180, 250, 0.1);
  border-left: 4px solid var(--ctp-blue);
  border-radius: 4px;
}

.info-box.warning {
  background: rgba(249, 226, 175, 0.1);
  border-left-color: var(--ctp-yellow);
}

.info-box h4 {
  margin: 0 0 0.5rem 0;
  color: var(--color-text);
  font-size: 0.875rem;
}

.info-box p {
  margin: 0;
  font-size: 0.8125rem;
  color: var(--color-text-secondary);
  line-height: 1.5;
}

.provider-placeholder {
  margin-top: 1.5rem;
  padding: 2rem;
  text-align: center;
  background: var(--color-bg-secondary);
  border-radius: 8px;
  border: 1px dashed var(--color-border);
}

.placeholder-text {
  color: var(--color-text-secondary);
  margin: 0;
}
</style>
