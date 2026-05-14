<template>
  <div ref="containerRef" class="backend-history">
    <div v-if="loading" class="history-loading">Loading history…</div>
    <div v-else-if="fetchError" class="history-error">History unavailable</div>
    <template v-else>
      <!-- Uptime summary -->
      <div class="uptime-row">
        <span class="uptime-label">Uptime:</span>
        <span class="uptime-item">
          <span class="uptime-period">24h</span>
          <span class="uptime-value" :class="uptimeClass(history!.uptime['24h'])">
            {{ formatUptime(history!.uptime['24h']) }}
          </span>
        </span>
        <span class="uptime-sep">/</span>
        <span class="uptime-item">
          <span class="uptime-period">7d</span>
          <span class="uptime-value" :class="uptimeClass(history!.uptime['7d'])">
            {{ formatUptime(history!.uptime['7d']) }}
          </span>
        </span>
        <span class="uptime-sep">/</span>
        <span class="uptime-item">
          <span class="uptime-period">30d</span>
          <span class="uptime-value" :class="uptimeClass(history!.uptime['30d'])">
            {{ formatUptime(history!.uptime['30d']) }}
          </span>
        </span>
      </div>

      <!-- Health sparkline -->
      <div class="chart-label">Health (3h)</div>
      <div
        class="sparkline-wrap"
        @mousemove="onMouseMove"
        @mouseleave="hideTooltip"
      >
        <svg
          v-if="slots.length > 0"
          :viewBox="`0 0 ${slots.length} 1`"
          preserveAspectRatio="none"
          class="sparkline"
          aria-label="Health sparkline"
        >
          <rect
            v-for="(s, i) in slots"
            :key="i"
            :x="i"
            y="0"
            width="0.9"
            height="1"
            :fill="healthColor(s)"
          />
        </svg>
        <span v-else class="no-data">No data</span>
      </div>

      <!-- Selection timeline -->
      <div class="chart-label">Selected for DNS (3h)</div>
      <div
        class="timeline-wrap"
        @mousemove="onMouseMove"
        @mouseleave="hideTooltip"
      >
        <svg
          v-if="slots.length > 0"
          :viewBox="`0 0 ${slots.length} 1`"
          preserveAspectRatio="none"
          class="selection-timeline"
          aria-label="DNS selection timeline"
        >
          <rect
            v-for="(s, i) in slots"
            :key="i"
            :x="i"
            y="0"
            width="0.9"
            height="1"
            :fill="selectionColor(s)"
          />
        </svg>
        <span v-else class="no-data">No data</span>
      </div>

      <!-- Tooltip (hidden on touch devices via CSS) -->
      <div
        v-show="tooltip.visible"
        class="bar-tooltip"
        :style="{ left: tooltip.x + 'px', top: tooltip.y + 'px' }"
      >
        {{ tooltip.text }}
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { getBackendHistory } from '@/api/client'
import type { BackendHistoryResponse, HealthHistoryBucket } from '@/types/api'

interface Props {
  backendId: string
}
const props = defineProps<Props>()

// Module-level session cache keyed by backend ID.
const cache = new Map<string, BackendHistoryResponse>()

const containerRef = ref<HTMLElement | null>(null)
const loading = ref(true)
const fetchError = ref(false)
const history = ref<BackendHistoryResponse | null>(null)

const tooltip = ref({ visible: false, x: 0, y: 0, text: '' })

const BUCKET_MS = 5 * 60 * 1000

interface Slot {
  bucket: HealthHistoryBucket | null
  startMs: number
}

// Build the full 3h grid of 5-minute slots, filling in gaps with null.
// The grid ends at the last *completed* bucket (nowFloor - BUCKET_MS) so the
// rightmost bar always corresponds to a bucket that the aggregator has had a
// chance to flush, rather than the current still-accumulating window.
const slots = computed((): Slot[] => {
  if (!history.value) return []

  const now = Date.now()
  const since = now - 3 * 60 * 60 * 1000
  const sinceFloor = Math.floor(since / BUCKET_MS) * BUCKET_MS
  // Stop one bucket before the current (incomplete) window.
  const lastCompleted = Math.floor(now / BUCKET_MS) * BUCKET_MS - BUCKET_MS

  // Build lookup by bucket_start epoch ms.
  const byTime = new Map<number, HealthHistoryBucket>()
  for (const b of history.value.buckets) {
    byTime.set(new Date(b.bucket_start).getTime(), b)
  }

  const result: Slot[] = []
  for (let t = sinceFloor; t <= lastCompleted; t += BUCKET_MS) {
    result.push({ bucket: byTime.get(t) ?? null, startMs: t })
  }
  return result
})

function healthColor(s: Slot): string {
  if (!s.bucket) return 'var(--ctp-surface1, #45475a)'
  if (s.bucket.failure_count > 0) return 'var(--ctp-red, #f38ba8)'
  if (s.bucket.success_count > 0) return 'var(--ctp-green, #a6e3a1)'
  return 'var(--ctp-surface1, #45475a)'
}

function selectionColor(s: Slot): string {
  if (s.bucket?.selected === 1) return 'var(--ctp-blue, #89b4fa)'
  return 'var(--ctp-surface1, #45475a)'
}

function formatUptime(pct: number): string {
  return pct.toFixed(1) + '%'
}

function uptimeClass(pct: number): string {
  if (pct >= 99) return 'uptime-good'
  if (pct >= 95) return 'uptime-warn'
  return 'uptime-bad'
}

function formatBucketEnd(endMs: number): string {
  return new Date(endMs).toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function onMouseMove(e: MouseEvent) {
  const wrap = e.currentTarget as HTMLElement
  const container = containerRef.value
  if (!wrap || !container || slots.value.length === 0) return

  const wrapRect = wrap.getBoundingClientRect()
  const containerRect = container.getBoundingClientRect()

  const relX = (e.clientX - wrapRect.left) / wrapRect.width
  const index = Math.min(Math.floor(relX * slots.value.length), slots.value.length - 1)
  if (index < 0) return

  const slot = slots.value[index]
  const endMs = slot.startMs + BUCKET_MS

  // Position tooltip above the cursor, relative to the component root.
  const x = e.clientX - containerRect.left
  const y = e.clientY - containerRect.top - 36

  tooltip.value = { visible: true, x, y, text: formatBucketEnd(endMs) }
}

function hideTooltip() {
  tooltip.value.visible = false
}

onMounted(async () => {
  const cached = cache.get(props.backendId)
  if (cached) {
    history.value = cached
    loading.value = false
    return
  }
  try {
    const data = await getBackendHistory(props.backendId, '3h')
    cache.set(props.backendId, data)
    history.value = data
  } catch {
    fetchError.value = true
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
.backend-history {
  position: relative;
  margin-top: 0.75rem;
  padding-top: 0.75rem;
  border-top: 1px solid var(--color-border);
}

.history-loading,
.history-error,
.no-data {
  font-size: 0.8rem;
  color: var(--color-text-secondary);
}

.uptime-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.35rem;
  margin-bottom: 0.5rem;
  font-size: 0.8rem;
}

.uptime-label {
  color: var(--color-text-secondary);
  margin-right: 0.15rem;
}

.uptime-item {
  display: inline-flex;
  align-items: center;
  gap: 0.2rem;
}

.uptime-period {
  color: var(--color-text-secondary);
}

.uptime-sep {
  color: var(--color-border);
}

.uptime-good { color: var(--ctp-green, #a6e3a1); font-weight: 600; }
.uptime-warn { color: var(--ctp-yellow, #f9e2af); font-weight: 600; }
.uptime-bad  { color: var(--ctp-red, #f38ba8); font-weight: 600; }

.chart-label {
  font-size: 0.7rem;
  color: var(--color-text-secondary);
  margin-bottom: 0.2rem;
}

.sparkline-wrap,
.timeline-wrap {
  width: 100%;
  margin-bottom: 0.4rem;
}

.sparkline {
  display: block;
  width: 100%;
  height: 30px;
}

.selection-timeline {
  display: block;
  width: 100%;
  height: 10px;
}

/* Tooltip — only rendered on devices that support hover (non-touch) */
.bar-tooltip {
  display: none;
}

@media (hover: hover) and (pointer: fine) {
  .bar-tooltip {
    display: block;
    position: absolute;
    pointer-events: none;
    white-space: nowrap;
    font-size: 0.75rem;
    padding: 0.25rem 0.5rem;
    border-radius: 4px;
    background: var(--color-surface, #1e1e2e);
    color: var(--color-text, #cdd6f4);
    border: 1px solid var(--color-border, #313244);
    box-shadow: 0 2px 6px rgba(0, 0, 0, 0.3);
    z-index: 10;
    transform: translateX(-50%);
  }
}
</style>
