import { ref, onMounted } from 'vue'

export type AccentColor = 
  | 'blue'
  | 'lavender'
  | 'sapphire'
  | 'sky'
  | 'teal'
  | 'green'
  | 'yellow'
  | 'peach'
  | 'maroon'
  | 'red'
  | 'mauve'
  | 'pink'
  | 'flamingo'
  | 'rosewater'

const accentColorMap: Record<AccentColor, string> = {
  blue: '#89b4fa',
  lavender: '#b4befe',
  sapphire: '#74c7ec',
  sky: '#89dceb',
  teal: '#94e2d5',
  green: '#a6e3a1',
  yellow: '#f9e2af',
  peach: '#fab387',
  maroon: '#eba0ac',
  red: '#f38ba8',
  mauve: '#cba6f7',
  pink: '#f5c2e7',
  flamingo: '#f2cdcd',
  rosewater: '#f5e0dc',
}

const STORAGE_KEY = 'gslb-accent-color'

const currentAccent = ref<AccentColor>('blue')

export function useTheme() {
  const setAccentColor = (color: AccentColor) => {
    currentAccent.value = color
    applyAccentColor(color)
    localStorage.setItem(STORAGE_KEY, color)
  }

  const applyAccentColor = (color: AccentColor) => {
    const root = document.documentElement
    const hexColor = accentColorMap[color]
    
    // Set the primary accent color
    root.style.setProperty('--color-primary', hexColor)
    
    // Calculate hover color (slightly lighter)
    root.style.setProperty('--color-primary-hover', lightenColor(hexColor, 10))
  }

  const lightenColor = (hex: string, percent: number): string => {
    const num = parseInt(hex.replace('#', ''), 16)
    const amt = Math.round(2.55 * percent)
    const R = (num >> 16) + amt
    const G = (num >> 8 & 0x00FF) + amt
    const B = (num & 0x0000FF) + amt
    return '#' + (0x1000000 + (R < 255 ? R < 1 ? 0 : R : 255) * 0x10000 +
      (G < 255 ? G < 1 ? 0 : G : 255) * 0x100 +
      (B < 255 ? B < 1 ? 0 : B : 255))
      .toString(16).slice(1)
  }

  const accentColors: { value: AccentColor; label: string }[] = [
    { value: 'blue', label: 'Blue' },
    { value: 'lavender', label: 'Lavender' },
    { value: 'sapphire', label: 'Sapphire' },
    { value: 'sky', label: 'Sky' },
    { value: 'teal', label: 'Teal' },
    { value: 'green', label: 'Green' },
    { value: 'yellow', label: 'Yellow' },
    { value: 'peach', label: 'Peach' },
    { value: 'maroon', label: 'Maroon' },
    { value: 'red', label: 'Red' },
    { value: 'mauve', label: 'Mauve' },
    { value: 'pink', label: 'Pink' },
    { value: 'flamingo', label: 'Flamingo' },
    { value: 'rosewater', label: 'Rosewater' },
  ]

  // Load saved theme on mount
  onMounted(() => {
    const saved = localStorage.getItem(STORAGE_KEY) as AccentColor
    if (saved && accentColorMap[saved]) {
      setAccentColor(saved)
    }
  })

  return {
    currentAccent,
    accentColors,
    setAccentColor,
    accentColorMap,
  }
}
