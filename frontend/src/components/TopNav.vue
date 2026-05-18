<template>
  <nav class="top-nav">
    <div class="nav-left">
      <svg class="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
        <circle cx="12" cy="12" r="10" />
        <ellipse cx="12" cy="12" rx="4" ry="10" />
        <line x1="2" y1="12" x2="22" y2="12" />
        <line x1="12" y1="2" x2="12" y2="22" />
      </svg>
      <span class="nav-wordmark">GoSphera</span>
    </div>

    <div class="nav-center">
      <span class="stat-chip">
        <span class="dot dot-orange" />
        <template v-if="firmsCount === 0">FIRMS: loading…</template>
        <template v-else-if="firmsFetched < firmsCount">
          FIRMS: {{ firmsFetched.toLocaleString() }} of {{ firmsCount.toLocaleString() }}
        </template>
        <template v-else>FIRMS: {{ firmsCount.toLocaleString() }} records</template>
      </span>

      <span class="stat-chip chip-blue">
        <span class="dot dot-blue" />
        ✈ {{ aircraftCount.toLocaleString() }}
      </span>

      <span class="stat-chip chip-teal">
        <span class="dot dot-teal" />
        🚢 {{ shipCount.toLocaleString() }}
      </span>

      <span :class="['stat-chip', sseConnected ? 'chip-green' : 'chip-red']">
        <span :class="['dot', sseConnected ? 'dot-green dot-pulse' : 'dot-red']" />
        SSE: {{ sseConnected ? 'live' : 'off' }}
      </span>
    </div>

    <div class="nav-right">
      <!-- Layer toggles -->
      <button
        :class="['layer-btn', showAircraft ? 'layer-active-blue' : '']"
        title="Toggle aircraft layer"
        @click="emit('toggle-layer', 'aircraft')"
      >✈ Air</button>

      <button
        :class="['layer-btn', showShips ? 'layer-active-teal' : '']"
        title="Toggle ships layer"
        @click="emit('toggle-layer', 'ships')"
      >🚢 Sea</button>

      <button
        :class="['layer-btn', showCameras ? 'layer-active-amber' : '']"
        title="Toggle cameras layer"
        @click="emit('toggle-layer', 'cameras')"
      >📷 Cam</button>

      <button
        :class="['layer-btn', showFire ? 'layer-active-orange' : '']"
        title="Toggle fire layer"
        @click="emit('toggle-layer', 'fire')"
      >🔥 Fire</button>

      <div class="sep" />

      <!-- Map style toggle -->
      <button class="nav-btn" :title="`Switch map style (current: ${mapStyle})`" @click="cycleStyle">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" width="15" height="15">
          <path v-if="mapStyle === 'dark'" d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z" />
          <template v-else-if="mapStyle === 'light'">
            <circle cx="12" cy="12" r="5" />
            <line x1="12" y1="1" x2="12" y2="3" />
            <line x1="12" y1="21" x2="12" y2="23" />
            <line x1="4.22" y1="4.22" x2="5.64" y2="5.64" />
            <line x1="18.36" y1="18.36" x2="19.78" y2="19.78" />
            <line x1="1" y1="12" x2="3" y2="12" />
            <line x1="21" y1="12" x2="23" y2="12" />
          </template>
          <template v-else>
            <rect x="3" y="3" width="7" height="7" rx="1" />
            <rect x="14" y="3" width="7" height="7" rx="1" />
            <rect x="3" y="14" width="7" height="7" rx="1" />
            <rect x="14" y="14" width="7" height="7" rx="1" />
          </template>
        </svg>
        {{ styleLabel }}
      </button>
    </div>
  </nav>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  firmsCount:   number
  firmsFetched: number
  sseConnected: boolean
  aircraftCount: number
  shipCount:    number
  mapStyle:     'dark' | 'light' | 'satellite'
  showAircraft: boolean
  showShips:    boolean
  showCameras:  boolean
  showFire:     boolean
}>()

const emit = defineEmits<{
  'style-change':  [style: 'dark' | 'light' | 'satellite']
  'toggle-layer':  [layer: string]
}>()

const styles = ['dark', 'light', 'satellite'] as const

const styleLabel = computed(() => {
  const labels = { dark: 'Dark', light: 'Light', satellite: 'Satellite' }
  return labels[props.mapStyle]
})

function cycleStyle() {
  const idx = styles.indexOf(props.mapStyle)
  emit('style-change', styles[(idx + 1) % styles.length])
}
</script>

<style scoped>
.top-nav {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 12px;
  background: #0d1117;
  border-bottom: 1px solid #1e2a35;
  height: 48px;
  flex-shrink: 0;
  z-index: 100;
  gap: 8px;
}

.nav-left {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.nav-icon {
  width: 20px;
  height: 20px;
  color: #3a8fd4;
  flex-shrink: 0;
}

.nav-wordmark {
  font-size: 0.95rem;
  font-weight: 700;
  color: #e0eaf4;
  letter-spacing: 0.04em;
}

.nav-center {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: 1;
  justify-content: center;
  overflow: hidden;
}

.stat-chip {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 3px 9px;
  background: #131920;
  border: 1px solid #1e2a35;
  border-radius: 20px;
  font-size: 0.75rem;
  color: #8ab0c8;
  white-space: nowrap;
  flex-shrink: 0;
}

.chip-blue  { border-color: #1a3a5a; }
.chip-teal  { border-color: #0d3a2a; }
.chip-green { border-color: #1a4a2a; }
.chip-red   { border-color: #4a1a1a; }

.dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  flex-shrink: 0;
}

.dot-orange { background: #f97316; }
.dot-blue   { background: #3a8fd4; }
.dot-teal   { background: #10b981; }
.dot-green  { background: #22c55e; }
.dot-red    { background: #ef4444; }

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}
.dot-pulse { animation: pulse 2s ease-in-out infinite; }

.nav-right {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

.sep {
  width: 1px;
  height: 20px;
  background: #1e2a35;
  margin: 0 4px;
  flex-shrink: 0;
}

/* Layer toggle buttons */
.layer-btn {
  padding: 4px 9px;
  background: #131920;
  border: 1px solid #1e2a35;
  border-radius: 5px;
  color: #5a6a7a;
  font-size: 0.75rem;
  cursor: pointer;
  transition: background 0.12s, color 0.12s, border-color 0.12s;
  white-space: nowrap;
}
.layer-btn:hover { background: #1a2535; color: #8ab0c8; }

.layer-active-blue   { border-color: #2a5a8a; color: #60a5fa; background: #0d1e30; }
.layer-active-teal   { border-color: #0d5a3a; color: #10b981; background: #071e14; }
.layer-active-amber  { border-color: #5a420d; color: #f59e0b; background: #1e1505; }
.layer-active-orange { border-color: #5a2e0d; color: #f97316; background: #1e0e05; }

/* Map style button */
.nav-btn {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 4px 10px;
  background: #131920;
  border: 1px solid #1e2a35;
  border-radius: 5px;
  color: #8ab0c8;
  font-size: 0.75rem;
  cursor: pointer;
  transition: background 0.12s, color 0.12s;
  min-width: 80px;
  white-space: nowrap;
}
.nav-btn:hover { background: #1a2535; color: #c0d8e8; }
</style>
