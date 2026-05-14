<template>
  <div class="theme-selector" ref="selectorRef">
    <button 
      class="theme-toggle"
      @click.stop="isOpen = !isOpen"
      :title="'Current accent: ' + currentAccent"
    >
      <span class="color-preview" :style="{ backgroundColor: accentColorMap[currentAccent] }"></span>
      <span class="theme-label">Theme</span>
    </button>
    
    <Transition name="dropdown">
      <div v-if="isOpen" class="theme-dropdown">
        <div class="theme-header">
          <span>Choose Accent Color</span>
        </div>
        <div class="color-grid">
          <button
            v-for="color in accentColors"
            :key="color.value"
            class="color-option"
            :class="{ active: currentAccent === color.value }"
            :style="{ backgroundColor: accentColorMap[color.value] }"
            :title="color.label"
            @click.stop="selectColor(color.value)"
          >
            <span v-if="currentAccent === color.value" class="checkmark">✓</span>
          </button>
        </div>
      </div>
    </Transition>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useTheme } from '@/composables/useTheme'

const { currentAccent, accentColors, setAccentColor, accentColorMap } = useTheme()
const isOpen = ref(false)
const selectorRef = ref<HTMLElement>()

const selectColor = (color: string) => {
  console.log('Selecting color:', color)
  setAccentColor(color as any)
  isOpen.value = false
}

// Close dropdown when clicking outside
const handleClickOutside = (event: MouseEvent) => {
  if (selectorRef.value && !selectorRef.value.contains(event.target as Node)) {
    isOpen.value = false
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>

<style scoped>
.theme-selector {
  position: relative;
}

.theme-toggle {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 0.75rem;
  background: var(--ctp-surface0);
  border: 1px solid var(--ctp-surface1);
  border-radius: 6px;
  color: var(--ctp-text);
  cursor: pointer;
  transition: all 0.2s ease;
}

.theme-toggle:hover {
  background: var(--ctp-surface1);
  border-color: var(--ctp-surface2);
}

.color-preview {
  width: 16px;
  height: 16px;
  border-radius: 50%;
  border: 2px solid var(--ctp-surface2);
}

.theme-label {
  font-size: 0.8125rem;
  font-weight: 500;
}

.theme-dropdown {
  position: absolute;
  top: calc(100% + 0.5rem);
  right: 0;
  background: var(--ctp-surface0);
  border: 1px solid var(--ctp-surface1);
  border-radius: 8px;
  padding: 0.75rem;
  min-width: 200px;
  box-shadow: 0 10px 25px rgba(0, 0, 0, 0.3);
  z-index: 1000;
}

.theme-header {
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--ctp-subtext0);
  margin-bottom: 0.75rem;
  padding-bottom: 0.5rem;
  border-bottom: 1px solid var(--ctp-surface1);
}

.color-grid {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  gap: 0.5rem;
}

.color-option {
  width: 24px;
  height: 24px;
  border-radius: 50%;
  border: 2px solid transparent;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s ease;
  padding: 0;
}

.color-option:hover {
  transform: scale(1.15);
  border-color: var(--ctp-text);
}

.color-option.active {
  border-color: var(--ctp-text);
  box-shadow: 0 0 0 2px var(--ctp-surface0), 0 0 0 4px var(--ctp-text);
}

.checkmark {
  color: var(--ctp-crust);
  font-size: 0.75rem;
  font-weight: bold;
  text-shadow: 0 1px 2px rgba(0, 0, 0, 0.3);
}

.dropdown-enter-active,
.dropdown-leave-active {
  transition: all 0.2s ease;
}

.dropdown-enter-from,
.dropdown-leave-to {
  opacity: 0;
  transform: translateY(-10px);
}

@media (max-width: 640px) {
  .theme-dropdown {
    right: -1rem;
    min-width: 180px;
  }

  .color-grid {
    grid-template-columns: repeat(5, 1fr);
  }

  .theme-label {
    display: none;
  }
}
</style>
