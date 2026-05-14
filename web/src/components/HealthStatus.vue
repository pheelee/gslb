<template>
  <span class="health-status" :class="statusClass">
    {{ displayText }}
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  status: string
}>()

const statusClass = computed(() => {
  switch (props.status) {
    case 'healthy':
      return 'status-healthy'
    case 'unhealthy':
      return 'status-unhealthy'
    default:
      return 'status-unknown'
  }
})

const displayText = computed(() => {
  switch (props.status) {
    case 'healthy':
      return '● Healthy'
    case 'unhealthy':
      return '● Unhealthy'
    default:
      return '● Unknown'
  }
})
</script>

<style scoped>
.health-status {
  display: inline-block;
  padding: 0.25rem 0.75rem;
  border-radius: 12px;
  font-size: 0.85rem;
  font-weight: 500;
}

.status-healthy {
  background: rgba(166, 227, 161, 0.15);
  color: var(--ctp-green);
  border: 1px solid rgba(166, 227, 161, 0.3);
}

.status-unhealthy {
  background: rgba(243, 139, 168, 0.15);
  color: var(--ctp-red);
  border: 1px solid rgba(243, 139, 168, 0.3);
}

.status-unknown {
  background: rgba(249, 226, 175, 0.15);
  color: var(--ctp-yellow);
  border: 1px solid rgba(249, 226, 175, 0.3);
}
</style>
