<template>
  <div class="layout">
    <LeftPanel @query="handleQuery" />
    <MainGlobe :data="geoData" />
    <RightPanel :events="telemetryEvents" @refresh="loadSituational" />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import LeftPanel from './components/LeftPanel.vue'
import MainGlobe from './components/MainGlobe.vue'
import RightPanel from './components/RightPanel.vue'
import { getSpatialData, getSituationalData, type FeatureCollection, type FireRecord } from './services/api'

interface QueryParams {
  tags: Array<{ key: string; value: string }>
  bbox: { minLat: number; minLon: number; maxLat: number; maxLon: number }
}

const geoData = ref<FeatureCollection | null>(null)
const telemetryEvents = ref<FireRecord[]>([])

async function loadSituational() {
  try {
    telemetryEvents.value = await getSituationalData()
  } catch (err) {
    console.error('[App] failed to load situational data:', err)
  }
}

onMounted(loadSituational)

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
