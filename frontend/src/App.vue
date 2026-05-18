<template>
  <div class="app-shell">
    <TopNav
      :firms-count="firmsTotal"
      :firms-fetched="firmsData.length"
      :sse-connected="sseConnected"
      :aircraft-count="aircraftData.length"
      :map-style="mapStyle"
      @style-change="mapStyle = $event"
    />
    <div class="main-grid">
      <LeftPanel
        :is-loading="isLoading"
        @query="handleQuery"
      />
      <MainGlobe
        :data="geoData"
        :fire-data="firmsData"
        :aircraft-data="aircraftData"
        :is-loading="isLoading"
        :map-style="mapStyle"
        @bbox-change="onBboxChange"
      />
      <RightPanel
        :fire-records="firmsData"
        :live-events="liveEvents"
        :aircraft="aircraftData"
        @refresh="loadSituational"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import TopNav from './components/TopNav.vue'
import LeftPanel from './components/LeftPanel.vue'
import MainGlobe from './components/MainGlobe.vue'
import RightPanel from './components/RightPanel.vue'
import {
  getSpatialData,
  getSituationalData,
  getAircraftData,
  openEventStream,
  type BBox,
  type FeatureCollection,
  type FireRecord,
  type TelemetryEvent,
  type AircraftState,
} from './services/api'

interface QueryParams {
  tags: Array<{ key: string; value: string }>
  bbox: { minLat: number; minLon: number; maxLat: number; maxLon: number }
}

const MAX_EVENTS = 200

const geoData       = ref<FeatureCollection | null>(null)
const firmsData     = ref<FireRecord[]>([])
const firmsTotal    = ref(0)
const liveEvents    = ref<TelemetryEvent[]>([])
const aircraftData  = ref<AircraftState[]>([])
const sseConnected  = ref(false)
const isLoading     = ref(false)
const mapStyle      = ref<'dark' | 'light' | 'satellite'>('dark')
const mapBbox       = ref<BBox | null>(null)

let closeStream:   (() => void) | null = null
let aircraftTimer: ReturnType<typeof setInterval> | null = null

async function loadSituational() {
  try {
    const { records, total } = await getSituationalData(mapBbox.value ?? undefined)
    firmsData.value = records
    firmsTotal.value = total
  } catch (err) {
    console.error('[App] failed to load situational data:', err)
  }
}

async function loadAircraft() {
  try {
    aircraftData.value = await getAircraftData(mapBbox.value ?? undefined)
  } catch (err) {
    console.error('[App] failed to load aircraft data:', err)
  }
}

/** Called by MainGlobe on moveend (debounced 400ms) and once on initial load. */
function onBboxChange(bbox: BBox) {
  mapBbox.value = bbox
  // Reload both layers with the new viewport bbox
  loadAircraft()
  loadSituational()
}

function onTelemetryEvent(event: TelemetryEvent) {
  liveEvents.value.unshift(event)
  if (liveEvents.value.length > MAX_EVENTS) {
    liveEvents.value.length = MAX_EVENTS
  }
}

onMounted(() => {
  // Initial loads run without bbox (bbox=null → backend caps at 5000)
  // The map emits bbox ~300ms after mount, triggering a filtered reload
  loadSituational()
  loadAircraft()

  // Continue polling every 15s for new data within the current viewport
  aircraftTimer = setInterval(loadAircraft, 15_000)

  closeStream = openEventStream(
    onTelemetryEvent,
    () => { sseConnected.value = false },
    () => { sseConnected.value = true },
  )
})

onUnmounted(() => {
  closeStream?.()
  if (aircraftTimer) clearInterval(aircraftTimer)
})

async function handleQuery(params: QueryParams) {
  const tagsStr = params.tags
    .filter(t => t.key && t.value)
    .map(t => `${t.key}=${t.value}`)
    .join(',')
  const bboxStr = `${params.bbox.minLat},${params.bbox.minLon},${params.bbox.maxLat},${params.bbox.maxLon}`

  isLoading.value = true
  try {
    geoData.value = await getSpatialData(tagsStr, bboxStr)
  } catch (err) {
    console.error('[App] spatial query failed:', err)
  } finally {
    isLoading.value = false
  }
}
</script>

<style scoped>
.app-shell {
  display: grid;
  grid-template-rows: 48px 1fr;
  height: 100vh;
  overflow: hidden;
  background-color: #0f1419;
}

.main-grid {
  display: grid;
  grid-template-columns: 280px 1fr 320px;
  min-height: 0;
  overflow: hidden;
}
</style>
