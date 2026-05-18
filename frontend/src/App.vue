<template>
  <div class="layout">
    <LeftPanel @query="handleQuery" />
    <MainGlobe :data="geoData" />
    <RightPanel :events="telemetryEvents" @refresh="loadSituational" />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import LeftPanel from './components/LeftPanel.vue'
import MainGlobe from './components/MainGlobe.vue'
import RightPanel from './components/RightPanel.vue'
import {
  getSpatialData,
  getSituationalData,
  openEventStream,
  type FeatureCollection,
  type FireRecord,
  type TelemetryEvent,
} from './services/api'

interface QueryParams {
  tags: Array<{ key: string; value: string }>
  bbox: { minLat: number; minLon: number; maxLat: number; maxLon: number }
}

const MAX_EVENTS = 200

const geoData = ref<FeatureCollection | null>(null)
const telemetryEvents = ref<Array<FireRecord | TelemetryEvent>>([])

// SSE cleanup function stored so we can call it on unmount.
let closeStream: (() => void) | null = null

async function loadSituational() {
  try {
    telemetryEvents.value = await getSituationalData()
  } catch (err) {
    console.error('[App] failed to load situational data:', err)
  }
}

function onTelemetryEvent(event: TelemetryEvent) {
  // Prepend so newest events appear at the top of the right panel.
  telemetryEvents.value.unshift(event)
  // Cap to prevent unbounded memory growth.
  if (telemetryEvents.value.length > MAX_EVENTS) {
    telemetryEvents.value.length = MAX_EVENTS
  }
}

onMounted(() => {
  loadSituational()
  closeStream = openEventStream(onTelemetryEvent, (err) => {
    console.warn('[App] SSE stream error:', err)
  })
})

onUnmounted(() => {
  closeStream?.()
})

async function handleQuery(params: QueryParams) {
  try {
    const tagsStr = params.tags
      .filter(t => t.key && t.value)
      .map(t => `${t.key}=${t.value}`)
      .join(',')
    const bboxStr = `${params.bbox.minLat},${params.bbox.minLon},${params.bbox.maxLat},${params.bbox.maxLon}`
    geoData.value = await getSpatialData(tagsStr, bboxStr)
  } catch (err) {
    console.error('[App] spatial query failed:', err)
  }
}
</script>

<style scoped>
.layout {
  display: grid;
  grid-template-columns: 250px 1fr 300px;
  width: 100%;
  height: 100%;
  background-color: #0f1419;
  overflow: hidden;
}
</style>
