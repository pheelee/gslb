import axios from 'axios'
import type {
  Config,
  Backend,
  HealthCheck,
  HealthState,
  DNSProviderConfig,
  AuditLog,
  Role,
  User,
  BackendHistoryResponse,
} from '@/types/api'

interface ApiResponse<T> {
  data?: T
  error?: string
}

const client = axios.create({
  baseURL: '/api/v1',
  headers: {
    'Content-Type': 'application/json',
  },
})

client.interceptors.response.use(
  (response) => {
    const apiResponse = response.data as ApiResponse<unknown>
    if (apiResponse.error) {
      return Promise.reject(new Error(apiResponse.error))
    }
    response.data = apiResponse.data
    return response
  },
  (error) => {
    const status = error.response?.status
    if (status === 401) {
      // Session expired or missing — redirect to OIDC login
      window.location.href = '/api/v1/auth/login'
      return new Promise(() => {}) // never resolves; page is navigating away
    }
    const message = error.response?.data?.error || error.message || 'Unknown error'
    return Promise.reject(new Error(message))
  }
)

// Configs
export async function getConfigs(): Promise<Config[]> {
  const response = await client.get<Config[]>('/configs')
  return response.data || []
}

export async function getConfig(id: string): Promise<Config> {
  const response = await client.get<Config>(`/configs/${id}`)
  return response.data
}

export async function createConfig(
  config: Omit<Config, 'id' | 'created_at' | 'updated_at'>
): Promise<Config> {
  const response = await client.post<Config>('/configs', config)
  return response.data
}

export async function updateConfig(
  id: string,
  config: Partial<Config>
): Promise<Config> {
  const response = await client.put<Config>(`/configs/${id}`, config)
  return response.data
}

export async function deleteConfig(id: string): Promise<void> {
  await client.delete(`/configs/${id}`)
}

// Backends
export async function getBackends(configId: string): Promise<Backend[]> {
  const response = await client.get<Backend[]>(`/configs/${configId}/backends`)
  return response.data || []
}

export async function createBackend(
  configId: string,
  backend: Omit<Backend, 'id' | 'config_id'>
): Promise<Backend> {
  const response = await client.post<Backend>(
    `/configs/${configId}/backends`,
    backend
  )
  return response.data
}

export async function updateBackend(
  id: string,
  backend: Partial<Backend>
): Promise<Backend> {
  const response = await client.put<Backend>(`/backends/${id}`, backend)
  return response.data
}

export async function deleteBackend(id: string): Promise<void> {
  await client.delete(`/backends/${id}`)
}

// Health Checks
export async function getHealthCheck(configId: string): Promise<HealthCheck> {
  const response = await client.get<HealthCheck>(
    `/configs/${configId}/health-check`
  )
  return response.data
}

export async function createHealthCheck(
  configId: string,
  hc: Omit<HealthCheck, 'id' | 'config_id'>
): Promise<HealthCheck> {
  const response = await client.post<HealthCheck>(
    `/configs/${configId}/health-check`,
    hc
  )
  return response.data
}

export async function getBackendHealth(backendId: string): Promise<HealthState> {
  const response = await client.get<HealthState>(
    `/backends/${backendId}/health`
  )
  return response.data
}

// Backend History
export async function getBackendHistory(
  backendId: string,
  range: '3h' | '24h' | '7d' | '30d' = '3h'
): Promise<BackendHistoryResponse> {
  const response = await client.get<BackendHistoryResponse>(
    `/backends/${backendId}/history?range=${range}`
  )
  return response.data
}

// DNS Providers
export async function getDNSProvider(
  configId: string
): Promise<DNSProviderConfig> {
  const response = await client.get<DNSProviderConfig>(
    `/configs/${configId}/dns-provider`
  )
  return response.data
}

export async function createDNSProvider(
  configId: string,
  provider: Omit<DNSProviderConfig, 'id' | 'config_id'>
): Promise<DNSProviderConfig> {
  const response = await client.post<DNSProviderConfig>(
    `/configs/${configId}/dns-provider`,
    provider
  )
  return response.data
}

// Audit Logs
export interface AuditLogFilter {
  entity_type?: string
  action?: string
  from?: string // RFC3339
  to?: string   // RFC3339
}

function buildAuditParams(filter: AuditLogFilter, extra: Record<string, string | number> = {}): string {
  const params = new URLSearchParams()
  if (filter.entity_type) params.set('entity_type', filter.entity_type)
  if (filter.action) params.set('action', filter.action)
  if (filter.from) params.set('from', filter.from)
  if (filter.to) params.set('to', filter.to)
  for (const [k, v] of Object.entries(extra)) params.set(k, String(v))
  const qs = params.toString()
  return qs ? `?${qs}` : ''
}

export async function getAuditLogs(limit = 100, offset = 0, filter: AuditLogFilter = {}): Promise<AuditLog[]> {
  const qs = buildAuditParams(filter, { limit, offset })
  const response = await client.get<AuditLog[]>(`/audit-logs${qs}`)
  return response.data || []
}

export async function getAuditLogCount(filter: AuditLogFilter = {}): Promise<number> {
  const qs = buildAuditParams(filter)
  const response = await client.get<{ count: number }>(`/audit-logs/count${qs}`)
  return response.data?.count || 0
}

export async function getAuditLog(id: string): Promise<AuditLog> {
  const response = await client.get<AuditLog>(`/audit-logs/${id}`)
  return response.data
}

export async function getAuditLogsForEntity(
  entityType: string,
  entityId: string,
  limit = 50
): Promise<AuditLog[]> {
  const response = await client.get<AuditLog[]>(
    `/audit-logs/entity/${entityType}/${entityId}?limit=${limit}`
  )
  return response.data || []
}

// Config Roles
export async function getConfigRoles(configId: string): Promise<Role[]> {
  const response = await client.get<Role[]>(`/configs/${configId}/roles`)
  return response.data || []
}

export async function updateConfigRoles(configId: string, roles: string[]): Promise<void> {
  await client.put(`/configs/${configId}/roles`, { roles })
}

// Auth / Roles
export async function getRoles(): Promise<Role[]> {
  const response = await client.get<Role[]>('/roles')
  return response.data || []
}

export async function getUsers(): Promise<User[]> {
  const response = await client.get<User[]>('/admin/users')
  return response.data || []
}

export async function assignUserRole(userId: string, roleId: string): Promise<void> {
  await client.post(`/admin/users/${userId}/roles`, { role_id: roleId })
}

export async function removeUserRole(userId: string, roleId: string): Promise<void> {
  await client.delete(`/admin/users/${userId}/roles/${roleId}`)
}

export default client
