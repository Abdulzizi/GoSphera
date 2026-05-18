<template>
  <div class="app-shell">
    <TopNav
      :firms-count="firmsTotal"
      :firms-fetched="firmsData.length"
      :sse-connected="sseConnected"
      :aircraft-count="aircraftData.length"
      :ship-count="shipData.length"
      :map-style="mapStyle"
      :show-aircraft="showAircraft"
      :show-ships="showShips"
      :show-cameras="showCameras"
      :show-fire="showFire"
      @style-change="mapStyle = $event"
      @toggle-layer="onToggleLayer"
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
        :ship-data="shipData"
        :camera-data="cameraData"
        :is-loading="isLoading"
        :map-style="mapStyle"
        :show-aircraft="showAircraft"
        :show-ships="showShips"
        :show-cameras="showCameras"
        :show-fire="showFire"
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
  getShipData,
  getCameraData,
  openEventStream,
  type BBox,
  type FeatureCollection,
  type FireRecord,
  type TelemetryEvent,
  type AircraftState,
  type ShipState,
  type CameraInfo,
} from './services/api'

interface QueryParams {
  tags: Array<{ key: string; value: string }>
  bbox: { minLat: number; minLon: number; maxLat: number; maxLon: number }
}

const MAX_EVENTS = 200

const geoData      = ref<FeatureCollection | null>(null)
const firmsData    = ref<FireRecord[]>([])
const firmsTotal   = ref(0)
const liveEvents   = ref<TelemetryEvent[]>([])
const aircraftData = ref<AircraftState[]>([])
const shipData     = ref<ShipState[]>([])
const cameraData   = ref<CameraInfo[]>([])
const sseConnected = ref(false)
const isLoading    = ref(false)
const mapStyle     = ref<'dark' | 'light' | 'satellite'>('dark')
const mapBbox      = ref<BBox | null>(null)

// Layer visibility — all on by default
const showAircraft = ref(true)
const showShips    = ref(true)
const showCameras  = ref(true)
const showFire     = ref(true)

let closeStream:   (() => void) | null = null
let aircraftTimer: ReturnType<typeof setInterval> | null = null
let shipTimer:     ReturnType<typeof setInterval> | null = null

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

async function loadShips() {
  try {
    shipData.value = await getShipData()
  } catch (err) {
    console.error('[App] failed to load ship data:', err)
  }
}

async function loadCameras() {
  try {
    cameraData.value = await getCameraData()
  } catch (err) {
    console.error('[App] failed to load camera data:', err)
  }
}

/**
 * Called by MainGlobe on moveend (debounced 400ms) and once on initial load.
 * Only passes bbox to the API when zoom >= 4 — at global zoom the bbox covers
 * the whole world so the filter returns all 200k+ records unchanged.
 * At zoom < 4 we let the backend use its 5000-record default cap instead.
 */
function onBboxChange(viewport: BBox) {
  mapBbox.value = viewport.zoom >= 4 ? viewport : null
  loadAircraft()
  loadSituational()
}

function onToggleLayer(layer: string) {
  if (layer === 'aircraft') showAircraft.value = !showAircraft.value
  else if (layer === 'ships')   showShips.value    = !showShips.value
  else if (layer === 'cameras') showCameras.value  = !showCameras.value
  else if (layer === 'fire')    showFire.value     = !showFire.value
}

function onTelemetryEvent(event: TelemetryEvent) {
  liveEvents.value.unshift(event)
  if (liveEvents.value.length > MAX_EVENTS) {
    liveEvents.value.length = MAX_EVENTS
  }
}

onMounted(() => {
  // Initial loads without bbox (bbox=null → backend caps at 5000)
  // Map emits bbox ~300ms after mount, triggering a viewport-filtered reload
  loadSituational()
  loadAircraft()
  loadShips()
  loadCameras()  // static list, no polling needed

  // Poll aircraft every 15s for new positions within current viewport
  aircraftTimer = setInterval(loadAircraft, 15_000)
  // Poll ships every 15s (AISStream updates continuously on backend)
  shipTimer = setInterval(loadShips, 15_000)

  closeStream = openEventStream(
    onTelemetryEvent,
    () => { sseConnected.value = false },
    () => { sseConnected.value = true },
  )
})

onUnmounted(() => {
  closeStream?.()
  if (aircraftTimer) clearInterval(aircraftTimer)
  if (shipTimer)     clearInterval(shipTimer)
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
