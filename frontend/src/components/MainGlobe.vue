<template>
  <div class="globe-wrap">
    <div ref="mapEl" class="map-container" />

    <div v-if="isLoading" class="loading-overlay">
      <span class="spinner" />
      <span class="loading-text">Querying…</span>
    </div>

    <div class="overlay">
      <span v-if="!data && !isLoading" class="hint">Select tags &amp; bbox, then click Query</span>
      <span v-else-if="data" class="info">{{ featureCount }} features loaded</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, shallowRef, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import type { FeatureCollection, FireRecord } from '../services/api'

import markerIcon2x from 'leaflet/dist/images/marker-icon-2x.png'
import markerIcon from 'leaflet/dist/images/marker-icon.png'
import markerShadow from 'leaflet/dist/images/marker-shadow.png'
delete (L.Icon.Default.prototype as unknown as Record<string, unknown>)._getIconUrl
L.Icon.Default.mergeOptions({
  iconRetinaUrl: markerIcon2x,
  iconUrl: markerIcon,
  shadowUrl: markerShadow,
})

const props = defineProps<{
  data: FeatureCollection | null
  fireData: FireRecord[]
  isLoading: boolean
}>()

const mapEl = ref<HTMLDivElement | null>(null)
const mapInstance = shallowRef<L.Map | null>(null)
const geoLayer = shallowRef<L.GeoJSON | null>(null)
const fireLayer = shallowRef<L.LayerGroup | null>(null)

const featureCount = computed(() => props.data?.features.length ?? 0)

onMounted(async () => {
  await nextTick()
  if (!mapEl.value) return

  const map = L.map(mapEl.value, {
    center: [20, 0],
    zoom: 2,
    preferCanvas: true,
    zoomControl: false,
  })

  L.tileLayer('https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png', {
    attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors &copy; <a href="https://carto.com/">CARTO</a>',
    subdomains: 'abcd',
    maxZoom: 19,
  }).addTo(map)

  L.control.zoom({ position: 'bottomright' }).addTo(map)

  map.invalidateSize(true)
  mapInstance.value = map
})

onUnmounted(() => {
  mapInstance.value?.remove()
  mapInstance.value = null
})

watch(
  () => props.data,
  (fc) => {
    const map = mapInstance.value
    if (!map) return

    geoLayer.value?.remove()
    geoLayer.value = null

    if (!fc || fc.features.length === 0) return

    const layer = L.geoJSON(fc as unknown as GeoJSON.FeatureCollection, {
      style: {
        color: '#3a8fd4',
        weight: 1.5,
        fillOpacity: 0.25,
        fillColor: '#3a8fd4',
      },
      pointToLayer: (_feature, latlng) =>
        L.circleMarker(latlng, {
          radius: 6,
          color: '#3a8fd4',
          fillColor: '#3a8fd4',
          fillOpacity: 0.85,
          weight: 1,
        }),
      onEachFeature: (feature, layer) => {
        const p = feature.properties ?? {}
        const rows = Object.entries(p)
          .filter(([k]) => !k.startsWith('osm_'))
          .map(([k, v]) => `<tr><td><b>${k}</b></td><td>${v}</td></tr>`)
          .join('')
        if (rows) {
          layer.bindPopup(`<table style="font-size:12px;border-collapse:collapse">${rows}</table>`)
        }
      },
    }).addTo(map)

    geoLayer.value = layer

    try {
      map.fitBounds(layer.getBounds(), { maxZoom: 14, padding: [20, 20] })
    } catch {
      // empty bounds — safe to ignore
    }
  },
)

watch(
  () => props.fireData,
  (records) => {
    const map = mapInstance.value
    if (!map) return

    fireLayer.value?.remove()
    fireLayer.value = null

    if (!records || records.length === 0) return

    const group = L.layerGroup()
    for (const r of records) {
      L.circleMarker([r.latitude, r.longitude], {
        radius: 4,
        color: '#f97316',
        fillColor: '#f97316',
        fillOpacity: 0.7,
        weight: 0.5,
      })
        .bindPopup(
          `<b>Fire</b><br>Brightness: ${r.brightness.toFixed(1)} K<br>Confidence: ${r.confidence}<br>Date: ${r.acq_date}`,
        )
        .addTo(group)
    }
    group.addTo(map)
    fireLayer.value = group
  },
)
</script>

<style scoped>
.globe-wrap {
  position: relative;
  height: 100%;
  min-height: 0;
  background-color: #0d1117;
  overflow: hidden;
}

.map-container {
  width: 100%;
  height: 100%;
}

.loading-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  background: rgba(13, 17, 23, 0.6);
  z-index: 1001;
  pointer-events: none;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.spinner {
  width: 22px;
  height: 22px;
  border: 3px solid #2a3a4a;
  border-top-color: #3a8fd4;
  border-radius: 50%;
  animation: spin 0.7s linear infinite;
}

.loading-text {
  font-size: 0.88rem;
  color: #8ab0c8;
}

.overlay {
  position: absolute;
  top: 12px;
  left: 12px;
  background: rgba(13, 17, 23, 0.88);
  border: 1px solid #2a3040;
  border-radius: 4px;
  padding: 6px 12px;
  pointer-events: none;
  z-index: 1000;
}

.hint {
  color: #5a6a7a;
  font-size: 0.82rem;
}

.info {
  color: #3a8fd4;
  font-size: 0.82rem;
  font-weight: 600;
}
</style>
