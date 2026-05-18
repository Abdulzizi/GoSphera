<template>
  <div class="app-shell">
    <TopNav
      :firms-count="firmsTotal"
      :firms-fetched="firmsData.length"
      :sse-connected="sseConnected"
    />
    <div class="main-grid">
      <LeftPanel
        :is-loading="isLoading"
        @query="handleQuery"
      />
      <MainGlobe
        :data="geoData"
        :fire-data="firmsData"
        :is-loading="isLoading"
      />
      <RightPanel
        :fire-records="firmsData"
        :live-events="liveEvents"
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
const firmsData = ref<FireRecord[]>([])
const firmsTotal = ref(0)
const liveEvents = ref<TelemetryEvent[]>([])
const sseConnected = ref(false)
const isLoading = ref(false)

let closeStream: (() => void) | null = null

async function loadSituational() {
  try {
    const { records, total } = await getSituationalData()
    firmsData.value = records
    firmsTotal.value = total
  } catch (err) {
    console.error('[App] failed to load situational data:', err)
  }
}

function onTelemetryEvent(event: TelemetryEvent) {
  liveEvents.value.unshift(event)
  if (liveEvents.value.length > MAX_EVENTS) {
    liveEvents.value.length = MAX_EVENTS
  }
}

onMounted(() => {
  loadSituational()
  closeStream = openEventStream(
    onTelemetryEvent,
    () => { sseConnected.value = false },
    () => { sseConnected.value = true },
  )
})

onUnmounted(() => {
  closeStream?.()
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
