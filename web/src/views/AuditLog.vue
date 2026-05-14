<template>
  <div class="audit-log-page">
    <header class="page-header">
      <div>
        <h1>Audit Log</h1>
        <p class="subtitle">Track all user actions and system changes</p>
      </div>
      <div class="header-stats">
        <div class="stat-item">
          <span class="stat-value">{{ totalCount }}</span>
          <span class="stat-label">{{ hasActiveFilter ? 'Matching' : 'Total' }} Entries</span>
        </div>
      </div>
    </header>

    <!-- Filter bar -->
    <div class="filter-bar">
      <div class="filter-row">
        <div class="filter-group">
          <label class="filter-label">Entity Type</label>
          <select v-model="filter.entity_type" class="filter-select" @change="applyFilter">
            <option value="">All</option>
            <option value="config">config</option>
            <option value="backend">backend</option>
            <option value="health_check">health_check</option>
            <option value="dns_provider">dns_provider</option>
            <option value="user">user</option>
          </select>
        </div>
        <div class="filter-group">
          <label class="filter-label">Action</label>
          <select v-model="filter.action" class="filter-select" @change="applyFilter">
            <option value="">All</option>
            <option value="config:create">config:create</option>
            <option value="config:update">config:update</option>
            <option value="config:delete">config:delete</option>
            <option value="config:roles:update">config:roles:update</option>
            <option value="backend:create">backend:create</option>
            <option value="backend:update">backend:update</option>
            <option value="backend:delete">backend:delete</option>
            <option value="health_check:create">health_check:create</option>
            <option value="health_check:update">health_check:update</option>
            <option value="health_check:delete">health_check:delete</option>
            <option value="dns_provider:create">dns_provider:create</option>
            <option value="dns_provider:update">dns_provider:update</option>
            <option value="dns_provider:delete">dns_provider:delete</option>
            <option value="auth:login">auth:login</option>
          </select>
        </div>
        <div class="filter-group">
          <label class="filter-label">From</label>
          <input type="datetime-local" v-model="filterFromLocal" class="filter-input" @change="applyFilter" />
        </div>
        <div class="filter-group">
          <label class="filter-label">To</label>
          <input type="datetime-local" v-model="filterToLocal" class="filter-input" @change="applyFilter" />
        </div>
        <button v-if="hasActiveFilter" class="btn btn-ghost btn-sm" @click="clearFilter">
          Clear filters
        </button>
      </div>
    </div>

    <div class="audit-log-container">
      <div v-if="loading && logs.length === 0" class="skeleton-container">
        <div v-for="n in 5" :key="n" class="skeleton-row">
          <div class="skeleton-cell" style="width: 80px;"></div>
          <div class="skeleton-cell" style="width: 120px;"></div>
          <div class="skeleton-cell" style="width: 200px;"></div>
          <div class="skeleton-cell" style="width: 100px;"></div>
          <div class="skeleton-cell" style="width: 150px;"></div>
        </div>
      </div>

      <div v-else-if="logs.length === 0" class="empty-state">
        <div class="empty-icon">📋</div>
        <h3>No audit log entries</h3>
        <p>{{ hasActiveFilter ? 'No entries match the current filters.' : 'Audit logs will appear here when users perform actions.' }}</p>
      </div>

      <div v-else class="audit-table-wrapper">
        <table class="audit-table">
          <thead>
            <tr>
              <th class="col-expand"></th>
              <th>Time</th>
              <th>Action</th>
              <th>Entity</th>
              <th>Configuration</th>
              <th>Changes</th>
              <th>IP Address</th>
            </tr>
          </thead>
          <tbody>
            <template v-for="log in logs" :key="log.id">
              <tr
                class="audit-row"
                :class="{ 'row-expanded': expandedRows.has(log.id), 'row-expandable': hasChanges(log) }"
                @click="hasChanges(log) && toggleRow(log.id)"
              >
                <td class="col-expand">
                  <span v-if="hasChanges(log)" class="expand-icon" :class="{ rotated: expandedRows.has(log.id) }">
                    ▶
                  </span>
                </td>
                <td class="time-cell">{{ formatDateTime(log.created_at) }}</td>
                <td>
                  <span class="action-badge" :class="getActionClass(log.action)">
                    {{ formatAction(log.action) }}
                  </span>
                </td>
                <td>
                  <span class="entity-badge" :class="getEntityClass(log.entity_type)">
                    {{ log.entity_type }}
                  </span>
                </td>
                <td class="name-cell">
                  <router-link
                    v-if="resolveConfigId(log)"
                    :to="`/configs/${resolveConfigId(log)}`"
                    class="config-link"
                    @click.stop
                  >{{ resolveConfigName(log) }}</router-link>
                  <span v-else>{{ log.entity_name || log.entity_id }}</span>
                </td>
                <td class="changes-cell">
                  <span v-if="hasChanges(log)" class="changes-badge">
                    {{ getChanges(log).length }} field{{ getChanges(log).length !== 1 ? 's' : '' }}
                  </span>
                  <span v-else class="no-changes">—</span>
                </td>
                <td class="ip-cell">{{ log.ip_address || '-' }}</td>
              </tr>
              <tr v-if="expandedRows.has(log.id) && hasChanges(log)" class="diff-row">
                <td colspan="7" class="diff-cell">
                  <table class="diff-table">
                    <thead>
                      <tr>
                        <th>Field</th>
                        <th>Old Value</th>
                        <th>New Value</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr v-for="change in getChanges(log)" :key="change.field">
                        <td class="diff-field">{{ change.field }}</td>
                        <td class="diff-old">
                          <span v-if="change.old" class="diff-value old-value">{{ change.old }}</span>
                          <span v-else class="diff-empty">—</span>
                        </td>
                        <td class="diff-new">
                          <span v-if="change.new" class="diff-value new-value">{{ change.new }}</span>
                          <span v-else class="diff-empty">—</span>
                        </td>
                      </tr>
                    </tbody>
                  </table>
                </td>
              </tr>
            </template>
          </tbody>
        </table>

        <!-- Pagination -->
        <div class="pagination" v-if="totalPages > 1 || currentPage > 1">
          <button class="btn btn-secondary btn-sm" :disabled="currentPage <= 1" @click="goToPage(currentPage - 1)">
            ‹ Prev
          </button>
          <span class="page-info">
            Page {{ currentPage }} of {{ totalPages }}
          </span>
          <button class="btn btn-secondary btn-sm" :disabled="currentPage >= totalPages" @click="goToPage(currentPage + 1)">
            Next ›
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { getAuditLogs, getAuditLogCount, getConfigs, type AuditLogFilter } from '@/api/client'
import type { AuditLog, ChangeEntry, Config } from '@/types/api'

const PAGE_SIZE = 50

const logs = ref<AuditLog[]>([])
const totalCount = ref(0)
const loading = ref(true)
const currentPage = ref(1)
const expandedRows = ref<Set<string>>(new Set())
const configMap = ref<Map<string, Config>>(new Map())

const filter = ref<AuditLogFilter>({
  entity_type: '',
  action: '',
})
const filterFromLocal = ref('')
const filterToLocal = ref('')

const totalPages = computed(() => Math.max(1, Math.ceil(totalCount.value / PAGE_SIZE)))

const hasActiveFilter = computed(() =>
  !!(filter.value.entity_type || filter.value.action || filterFromLocal.value || filterToLocal.value)
)

function buildFilter(): AuditLogFilter {
  const f: AuditLogFilter = {}
  if (filter.value.entity_type) f.entity_type = filter.value.entity_type
  if (filter.value.action) f.action = filter.value.action
  if (filterFromLocal.value) f.from = new Date(filterFromLocal.value).toISOString()
  if (filterToLocal.value) {
    // Set to end of the selected minute
    const d = new Date(filterToLocal.value)
    d.setSeconds(59, 999)
    f.to = d.toISOString()
  }
  return f
}

function formatDateTime(dateStr: string): string {
  const date = new Date(dateStr)
  return date.toLocaleString()
}

function formatAction(action: string): string {
  return action.replace(':', ' ').toUpperCase()
}

function getActionClass(action: string): string {
  if (action.includes(':create')) return 'action-create'
  if (action.includes(':update')) return 'action-update'
  if (action.includes(':delete')) return 'action-delete'
  return 'action-default'
}

function getEntityClass(entityType: string): string {
  const classes: Record<string, string> = {
    config: 'entity-config',
    backend: 'entity-backend',
    health_check: 'entity-health',
    dns_provider: 'entity-dns',
  }
  return classes[entityType] || 'entity-default'
}

function getChanges(log: AuditLog): ChangeEntry[] {
  if (!log.details || log.details === '{}') return []
  try {
    const parsed = JSON.parse(log.details)
    return parsed.changes ?? []
  } catch {
    return []
  }
}

function hasChanges(log: AuditLog): boolean {
  return getChanges(log).length > 0
}

function toggleRow(id: string) {
  if (expandedRows.value.has(id)) {
    expandedRows.value.delete(id)
  } else {
    expandedRows.value.add(id)
  }
}

function resolveConfigId(log: AuditLog): string {
  if (log.entity_type === 'config') return log.entity_id
  return log.config_id || ''
}

function resolveConfigName(log: AuditLog): string {
  const cfgId = resolveConfigId(log)
  if (!cfgId) return log.entity_name || log.entity_id
  const cfg = configMap.value.get(cfgId)
  return cfg ? cfg.name : (log.entity_name || cfgId)
}

async function loadPage(page: number) {
  loading.value = true
  expandedRows.value.clear()
  try {
    const f = buildFilter()
    const offset = (page - 1) * PAGE_SIZE
    const [logsData, count] = await Promise.all([
      getAuditLogs(PAGE_SIZE, offset, f),
      getAuditLogCount(f),
    ])
    logs.value = logsData
    totalCount.value = count
    currentPage.value = page
  } catch (error) {
    console.error('Failed to load audit logs:', error)
  } finally {
    loading.value = false
  }
}

function goToPage(page: number) {
  if (page < 1 || page > totalPages.value) return
  loadPage(page)
}

function applyFilter() {
  loadPage(1)
}

function clearFilter() {
  filter.value = { entity_type: '', action: '' }
  filterFromLocal.value = ''
  filterToLocal.value = ''
  loadPage(1)
}

onMounted(async () => {
  try {
    const configs = await getConfigs()
    const map = new Map<string, Config>()
    for (const cfg of configs) map.set(cfg.id, cfg)
    configMap.value = map
  } catch {
    // non-fatal
  }
  loadPage(1)
})
</script>

<style scoped>
.audit-log-page {
  max-width: 1200px;
  margin: 0 auto;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 1.5rem;
}

.page-header h1 {
  margin: 0 0 0.25rem 0;
  color: var(--color-text);
}

.subtitle {
  color: var(--color-text-secondary);
  margin: 0;
}

.header-stats {
  background: var(--color-surface);
  padding: 1rem 1.5rem;
  border-radius: 8px;
  border: 1px solid var(--color-border);
}

.stat-item {
  text-align: center;
}

.stat-value {
  display: block;
  font-size: 1.5rem;
  font-weight: bold;
  color: var(--color-primary);
}

.stat-label {
  font-size: 0.75rem;
  color: var(--color-text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

/* Filter bar */
.filter-bar {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  padding: 1rem 1.25rem;
  margin-bottom: 1.25rem;
}

.filter-row {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
  align-items: flex-end;
}

.filter-group {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.filter-label {
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--color-text-secondary);
}

.filter-select,
.filter-input {
  height: 2.25rem;
  padding: 0 0.625rem;
  border: 1px solid var(--color-border);
  border-radius: 6px;
  background: var(--color-bg-secondary);
  color: var(--color-text);
  font-size: 0.875rem;
  min-width: 140px;
}

.filter-select:focus,
.filter-input:focus {
  outline: none;
  border-color: var(--color-primary);
}

.btn-ghost {
  background: transparent;
  border: 1px solid var(--color-border);
  color: var(--color-text-secondary);
  padding: 0 0.75rem;
  height: 2.25rem;
  border-radius: 6px;
  cursor: pointer;
  font-size: 0.875rem;
  align-self: flex-end;
}

.btn-ghost:hover {
  background: var(--color-bg-secondary);
  color: var(--color-text);
}

/* Table */
.audit-log-container {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: 12px;
  overflow: hidden;
}

.skeleton-container {
  padding: 1rem;
}

.skeleton-row {
  display: flex;
  gap: 1rem;
  padding: 1rem;
  border-bottom: 1px solid var(--color-border);
}

.skeleton-cell {
  height: 1rem;
  background: linear-gradient(90deg, var(--ctp-surface1) 25%, var(--ctp-surface2) 50%, var(--ctp-surface1) 75%);
  background-size: 200% 100%;
  animation: skeleton-loading 1.5s infinite;
  border-radius: 4px;
}

@keyframes skeleton-loading {
  0% { background-position: 200% 0; }
  100% { background-position: -200% 0; }
}

.empty-state {
  text-align: center;
  padding: 4rem 2rem;
  color: var(--color-text-secondary);
}

.empty-icon {
  font-size: 3rem;
  margin-bottom: 1rem;
  opacity: 0.7;
}

.empty-state h3 {
  color: var(--color-text);
  margin: 0 0 0.5rem 0;
}

.empty-state p {
  margin: 0;
}

.audit-table-wrapper {
  overflow-x: auto;
}

.audit-table {
  width: 100%;
  border-collapse: collapse;
}

.audit-table th {
  background: var(--color-bg-secondary);
  padding: 1rem;
  text-align: left;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--color-text-secondary);
  border-bottom: 1px solid var(--color-border);
}

.audit-table td {
  padding: 1rem;
  border-bottom: 1px solid var(--color-border);
}

.col-expand {
  width: 1.5rem;
  padding: 1rem 0.5rem 1rem 1rem !important;
}

.audit-row:hover {
  background: var(--color-bg-secondary);
}

.row-expandable {
  cursor: pointer;
}

.row-expanded {
  background: var(--color-bg-secondary);
}

.expand-icon {
  display: inline-block;
  font-size: 0.625rem;
  color: var(--color-text-secondary);
  transition: transform 0.15s ease;
  user-select: none;
}

.expand-icon.rotated {
  transform: rotate(90deg);
}

.time-cell {
  font-family: var(--font-mono);
  font-size: 0.875rem;
  color: var(--color-text-secondary);
  white-space: nowrap;
}

.name-cell {
  font-family: var(--font-mono);
  color: var(--color-text);
}

.config-link {
  color: var(--color-primary);
  text-decoration: none;
  font-family: var(--font-mono);
}

.config-link:hover {
  text-decoration: underline;
}

.ip-cell {
  font-family: var(--font-mono);
  font-size: 0.875rem;
  color: var(--color-text-secondary);
}

.changes-cell {
  white-space: nowrap;
}

.changes-badge {
  display: inline-block;
  padding: 0.2rem 0.6rem;
  border-radius: 9999px;
  font-size: 0.75rem;
  font-weight: 500;
  background: rgba(137, 180, 250, 0.12);
  color: var(--ctp-blue);
}

.no-changes {
  color: var(--color-text-secondary);
  font-size: 0.875rem;
}

.action-badge {
  display: inline-block;
  padding: 0.25rem 0.75rem;
  border-radius: 9999px;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.action-create {
  background: rgba(166, 227, 161, 0.15);
  color: var(--ctp-green);
}

.action-update {
  background: rgba(249, 226, 175, 0.15);
  color: var(--ctp-yellow);
}

.action-delete {
  background: rgba(243, 139, 168, 0.15);
  color: var(--ctp-red);
}

.action-default {
  background: rgba(137, 180, 250, 0.15);
  color: var(--ctp-blue);
}

.entity-badge {
  display: inline-block;
  padding: 0.25rem 0.5rem;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 500;
  text-transform: capitalize;
  background: var(--color-bg-secondary);
  color: var(--color-text-secondary);
}

/* Diff expansion row */
.diff-row td {
  padding: 0;
  border-bottom: 1px solid var(--color-border);
  background: var(--color-bg-secondary);
}

.diff-cell {
  padding: 0.75rem 1rem 0.75rem 3.5rem !important;
}

.diff-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.8125rem;
}

.diff-table th {
  padding: 0.375rem 0.75rem;
  text-align: left;
  font-size: 0.6875rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--color-text-secondary);
  background: transparent;
  border-bottom: 1px solid var(--color-border);
}

.diff-table td {
  padding: 0.375rem 0.75rem;
  border-bottom: 1px solid color-mix(in srgb, var(--color-border) 50%, transparent);
  vertical-align: top;
}

.diff-table tr:last-child td {
  border-bottom: none;
}

.diff-field {
  font-family: var(--font-mono);
  font-weight: 500;
  color: var(--color-text);
  white-space: nowrap;
  width: 30%;
}

.diff-value {
  font-family: var(--font-mono);
  display: inline-block;
  padding: 0.125rem 0.375rem;
  border-radius: 4px;
  word-break: break-all;
}

.old-value {
  background: rgba(243, 139, 168, 0.12);
  color: var(--ctp-red);
  text-decoration: line-through;
  text-decoration-color: color-mix(in srgb, var(--ctp-red) 50%, transparent);
}

.new-value {
  background: rgba(166, 227, 161, 0.12);
  color: var(--ctp-green);
}

.diff-empty {
  color: var(--color-text-secondary);
  font-style: italic;
}

/* Pagination */
.pagination {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 1rem;
  padding: 1rem;
  border-top: 1px solid var(--color-border);
}

.page-info {
  font-size: 0.875rem;
  color: var(--color-text-secondary);
}

.btn {
  cursor: pointer;
  border-radius: 6px;
  font-size: 0.875rem;
  font-weight: 500;
  transition: background 0.15s, opacity 0.15s;
}

.btn-secondary {
  background: var(--color-bg-secondary);
  border: 1px solid var(--color-border);
  color: var(--color-text);
  padding: 0.375rem 0.875rem;
}

.btn-secondary:hover:not(:disabled) {
  background: var(--color-border);
}

.btn-secondary:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.btn-sm {
  font-size: 0.8125rem;
  padding: 0.3rem 0.75rem;
}

@media (max-width: 640px) {
  .page-header {
    flex-direction: column;
    gap: 1rem;
  }

  .audit-table {
    min-width: 600px;
  }

  .filter-row {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>
