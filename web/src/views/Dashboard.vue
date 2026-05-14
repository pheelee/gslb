<template>
  <div class="dashboard">
    <main class="main">
      <div class="stats-grid">
        <div class="stat-card">
          <div class="stat-icon configs-icon">
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path>
              <polyline points="14 2 14 8 20 8"></polyline>
              <line x1="16" y1="13" x2="8" y2="13"></line>
              <line x1="16" y1="17" x2="8" y2="17"></line>
              <polyline points="10 9 9 9 8 9"></polyline>
            </svg>
          </div>
          <div class="stat-value">{{ configs.length }}</div>
          <div class="stat-label">Configurations</div>
        </div>
        <div class="stat-card">
          <div class="stat-icon backends-icon">
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <rect x="2" y="2" width="20" height="8" rx="2" ry="2"></rect>
              <rect x="2" y="14" width="20" height="8" rx="2" ry="2"></rect>
              <line x1="6" y1="6" x2="6.01" y2="6"></line>
              <line x1="6" y1="18" x2="6.01" y2="18"></line>
            </svg>
          </div>
          <div class="stat-value">{{ totalBackends }}</div>
          <div class="stat-label">Total Backends</div>
        </div>
        <div class="stat-card">
          <div class="stat-icon healthy-icon">
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"></path>
              <polyline points="22 4 12 14.01 9 11.01"></polyline>
            </svg>
          </div>
          <div class="stat-value">{{ healthyBackends }}</div>
          <div class="stat-label">Healthy Backends</div>
        </div>
        <div class="stat-card">
          <div class="stat-icon unhealthy-icon">
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="12" cy="12" r="10"></circle>
              <line x1="15" y1="9" x2="9" y2="15"></line>
              <line x1="9" y1="9" x2="15" y2="15"></line>
            </svg>
          </div>
          <div class="stat-value">{{ unhealthyBackends }}</div>
          <div class="stat-label">Unhealthy Backends</div>
        </div>
      </div>

      <div class="actions">
        <router-link to="/configs/new" class="btn btn-primary">+ New Configuration</router-link>
      </div>

      <div class="recent-section">
        <h2>Recent Configurations</h2>
        <div v-if="loading && configs.length === 0" class="config-list">
          <div v-for="n in 3" :key="n" class="config-item skeleton-item">
            <div class="config-info">
              <h3 class="skeleton-text" style="width: 150px;">Loading...</h3>
              <p class="dns-name skeleton-text" style="width: 200px;">...</p>
              <p class="method skeleton-text" style="width: 100px;">...</p>
            </div>
          </div>
        </div>
        <div v-else-if="configs.length === 0" class="empty">
          <div class="empty-icon">📋</div>
          <h3>No configurations yet</h3>
          <p>Get started by creating your first load balancer configuration</p>
          <router-link to="/configs/new" class="btn btn-primary">Create Configuration</router-link>
        </div>
        <div v-else class="config-list">
          <div v-for="config in configs.slice(0, 5)" :key="config.id" class="config-item">
            <div class="config-info">
              <h3>{{ config.name }}</h3>
              <p class="dns-name">{{ config.dns_name }}</p>
              <p class="method">Method: {{ config.lb_method }}</p>
            </div>
            <div class="config-actions">
              <router-link :to="`/configs/${config.id}`" class="btn btn-secondary">View</router-link>
              <router-link :to="`/configs/${config.id}/edit`" class="btn btn-secondary">Edit</router-link>
            </div>
          </div>
        </div>
      </div>
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { getConfigs, getBackends, getBackendHealth } from '@/api/client'
import type { Config, Backend, HealthState } from '@/types/api'

const configs = ref<Config[]>([])
const backends = ref<Map<string, Backend[]>>(new Map())
const healthStates = ref<Map<string, HealthState>>(new Map())
const loading = ref(true)

const totalBackends = computed(() => {
  let count = 0
  backends.value.forEach((list) => count += list.length)
  return count
})

const healthyBackends = computed(() => {
  let count = 0
  healthStates.value.forEach((state) => {
    if (state.status === 'healthy') count++
  })
  return count
})

const unhealthyBackends = computed(() => {
  let count = 0
  healthStates.value.forEach((state) => {
    if (state.status === 'unhealthy') count++
  })
  return count
})

async function loadData() {
  try {
    loading.value = true
    configs.value = await getConfigs()
    
    // Load backends for each config
    for (const config of configs.value) {
      const configBackends = await getBackends(config.id)
      backends.value.set(config.id, configBackends)
      
      // Load health for each backend
      for (const backend of configBackends) {
        try {
          const health = await getBackendHealth(backend.id)
          healthStates.value.set(backend.id, health)
        } catch (e) {
          console.error(`Failed to load health for backend ${backend.id}:`, e)
        }
      }
    }
  } catch (error) {
    console.error('Failed to load dashboard data:', error)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadData()
  // Auto-refresh data every 10 seconds
  const interval = setInterval(loadData, 10000)
  
  // Cleanup on unmount
  onUnmounted(() => {
    clearInterval(interval)
  })
})
</script>

<style scoped>
.main {
  max-width: 1200px;
  margin: 0 auto;
  padding: 2rem;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 1rem;
  margin-bottom: 2rem;
}

.stat-card {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  padding: 1.5rem;
  border-radius: 12px;
  text-align: center;
  transition: all 0.2s;
  position: relative;
}

.stat-card:hover {
  border-color: var(--color-primary);
  box-shadow: 0 4px 12px rgba(137, 180, 250, 0.1);
}

.stat-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 48px;
  height: 48px;
  border-radius: 12px;
  margin: 0 auto 1rem;
}

.configs-icon {
  background: rgba(137, 180, 250, 0.15);
  color: var(--ctp-blue);
}

.backends-icon {
  background: rgba(203, 166, 247, 0.15);
  color: var(--ctp-mauve);
}

.healthy-icon {
  background: rgba(166, 227, 161, 0.15);
  color: var(--ctp-green);
}

.unhealthy-icon {
  background: rgba(243, 139, 168, 0.15);
  color: var(--ctp-red);
}

.stat-value {
  font-size: 2.5rem;
  font-weight: bold;
  color: var(--color-text);
}

.stat-label {
  color: var(--color-text-secondary);
  margin-top: 0.5rem;
}

.actions {
  margin-bottom: 2rem;
}

.recent-section {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  padding: 1.5rem;
  border-radius: 12px;
}

.recent-section h2 {
  margin-top: 0;
  margin-bottom: 1rem;
  color: var(--color-text);
}

.loading {
  text-align: center;
  padding: 3rem;
  color: var(--color-text-secondary);
}

.empty {
  text-align: center;
  padding: 3rem;
  color: var(--color-text-secondary);
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
  max-width: 300px;
  margin-left: auto;
  margin-right: auto;
}

.empty .btn {
  display: inline-flex;
}

.config-list {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.config-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem;
  background: var(--color-bg);
  border: 1px solid var(--color-border);
  border-radius: 8px;
  transition: all 0.2s;
}

.config-item:hover {
  border-color: var(--color-primary);
  box-shadow: 0 2px 8px rgba(137, 180, 250, 0.1);
}

.config-info h3 {
  margin: 0 0 0.25rem 0;
  color: var(--color-text);
}

.config-info p {
  margin: 0.25rem 0;
  color: var(--color-text-secondary);
  font-size: 0.9rem;
}

.config-info .dns-name {
  font-family: var(--font-mono);
  color: var(--color-primary);
}

.config-actions {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
}

@media (max-width: 640px) {
  .header {
    flex-direction: column;
    gap: 1rem;
    text-align: center;
  }

  .nav {
    width: 100%;
    justify-content: center;
  }

  .main {
    padding: 1rem;
  }

  .stats-grid {
    grid-template-columns: repeat(2, 1fr);
  }

  .config-item {
    flex-direction: column;
    gap: 1rem;
    text-align: center;
  }

  .config-actions {
    width: 100%;
    justify-content: center;
  }
}

.skeleton-text {
  background: linear-gradient(90deg, var(--ctp-surface1) 25%, var(--ctp-surface2) 50%, var(--ctp-surface1) 75%);
  background-size: 200% 100%;
  animation: skeleton-loading 1.5s infinite;
  color: transparent;
  border-radius: 4px;
  display: inline-block;
}

.skeleton-item {
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
