import { ref } from 'vue'
import type { Ref } from 'vue'
import * as api from '@/api/client'

export interface UseApiState<T> {
  data: Ref<T | null>
  loading: Ref<boolean>
  error: Ref<Error | null>
}

export function useApi<T>(
  apiCall: () => Promise<T>
): UseApiState<T> & { execute: () => Promise<void> } {
  const data = ref<T | null>(null) as Ref<T | null>
  const loading = ref(false)
  const error = ref<Error | null>(null)

  async function execute(): Promise<void> {
    loading.value = true
    error.value = null
    try {
      data.value = await apiCall()
    } catch (err) {
      error.value = err instanceof Error ? err : new Error(String(err))
    } finally {
      loading.value = false
    }
  }

  return {
    data,
    loading,
    error,
    execute,
  }
}

export function useConfigApi() {
  return {
    getConfigs: () => api.getConfigs(),
    getConfig: (id: string) => api.getConfig(id),
    createConfig: (config: Parameters<typeof api.createConfig>[0]) => api.createConfig(config),
    updateConfig: (id: string, config: Parameters<typeof api.updateConfig>[1]) => api.updateConfig(id, config),
    deleteConfig: (id: string) => api.deleteConfig(id),
  }
}

export function useBackendApi() {
  return {
    getBackends: (configId: string) => api.getBackends(configId),
    createBackend: (configId: string, backend: Parameters<typeof api.createBackend>[1]) => api.createBackend(configId, backend),
    updateBackend: (id: string, backend: Parameters<typeof api.updateBackend>[1]) => api.updateBackend(id, backend),
    deleteBackend: (id: string) => api.deleteBackend(id),
  }
}

export function useHealthApi() {
  return {
    getHealthCheck: (configId: string) => api.getHealthCheck(configId),
    createHealthCheck: (configId: string, hc: Parameters<typeof api.createHealthCheck>[1]) => api.createHealthCheck(configId, hc),
    getBackendHealth: (backendId: string) => api.getBackendHealth(backendId),
  }
}

export function useDNSProviderApi() {
  return {
    getDNSProvider: (configId: string) => api.getDNSProvider(configId),
    createDNSProvider: (configId: string, provider: Parameters<typeof api.createDNSProvider>[1]) => api.createDNSProvider(configId, provider),
  }
}
