export interface Role {
  id: string
  name: string
  description: string
  created_at: string
}

export interface User {
  id: string
  subject: string
  email: string
  name: string
  created_at: string
  last_login_at: string
  is_active: boolean
  roles: Role[]
}

export type HealthStatus = 'healthy' | 'unhealthy' | 'unknown'
export type LBMethod = 'round_robin' | 'weighted'
export type HealthCheckType = 'icmp' | 'tcp'

export interface Config {
  id: string
  name: string
  dns_name: string
  dns_ttl: number
  lb_method: LBMethod
  created_at: string
  updated_at: string
  last_reconcile_error?: string
}

export interface Backend {
  id: string
  config_id: string
  ip: string
  port?: number
  weight: number
  enabled: boolean
}

export interface HealthCheck {
  id: string
  config_id: string
  type: HealthCheckType
  interval_seconds: number
  timeout_seconds: number
  threshold_healthy: number
  threshold_unhealthy: number
}

export interface HealthState {
  backend_id: string
  status: HealthStatus
  consecutive_successes: number
  consecutive_failures: number
  last_check_at?: string
  last_healthy_at?: string
  last_error?: string
}

export interface DNSProviderConfig {
  id: string
  config_id: string
  provider_type: 'cloudflare' | 'mock'
  config_json: string
}

export interface HealthHistoryBucket {
  id?: number
  backend_id: string
  bucket_start: string
  success_count: number
  failure_count: number
  selected: number // 1 = selected for DNS, 0 = not selected
}

export interface BackendHistoryResponse {
  buckets: HealthHistoryBucket[]
  uptime: {
    '24h': number
    '7d': number
    '30d': number
  }
}

export interface ChangeEntry {
  field: string
  old: string
  new: string
}

export interface AuditLogDetails {
  changes?: ChangeEntry[]
}

export interface AuditLog {
  id: string
  action: string
  entity_type: string
  entity_id: string
  entity_name: string
  config_id: string
  details: string
  user_id: string
  ip_address: string
  user_agent: string
  created_at: string
}
