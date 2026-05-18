<template>
  <div class="globe-wrap">
    <div ref="mapEl" class="map-container" />
    <div class="overlay">
      <span v-if="!data" class="hint">Select tags &amp; bbox, then click Query</span>
      <span v-else class="info">{{ featureCount }} features loaded</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, shallowRef, computed, onMounted, onUnmounted, watch } from 'vue'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import type { FeatureCollection } from '../services/api'

// Fix Leaflet's default icon paths broken by Vite's asset pipeline.
import markerIcon2x from 'leaflet/dist/images/marker-icon-2x.png'
import markerIcon from 'leaflet/dist/images/marker-icon.png'
import markerShadow from 'leaflet/dist/images/marker-shadow.png'
delete (L.Icon.Default.prototype as unknown as Record<string, unknown>)._getIconUrl
L.Icon.Default.mergeOptions({
  iconRetinaUrl: markerIcon2x,
  iconUrl: markerIcon,
  shadowUrl: markerShadow,
})

const props = defineProps<{ data: FeatureCollection | null }>()

const mapEl = ref<HTMLDivElement | null>(null)

// shallowRef prevents Vue deep-proxying Leaflet internals —
// keeps all rendering off the reactivity graph.
const mapInstance = shallowRef<L.Map | null>(null)
const geoLayer = shallowRef<L.GeoJSON | null>(null)

const featureCount = computed(() => props.data?.features.length ?? 0)

onMounted(() => {
  if (!mapEl.value) return

  const map = L.map(mapEl.value, {
    center: [20, 0],
    zoom: 2,
    preferCanvas: true, // faster for large point datasets
    zoomControl: true,
  })

  L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
    attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
    maxZoom: 19,
  }).addTo(map)

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

    // Remove previous layer.
    if (geoLayer.value) {
      geoLayer.value.remove()
      geoLayer.value = null
    }

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
          radius: 5,
          color: '#3a8fd4',
          fillColor: '#3a8fd4',
          fillOpacity: 0.85,
          weight: 1,
        }),
      onEachFeature: (feature, layer) => {
        const props = feature.properties ?? {}
        const rows = Object.entries(props)
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
      // getBounds() throws if layer has no valid bounds (e.g. empty)
    }
  },
)
</script>

<style scoped>
.globe-wrap {
  position: relative;
  height: 100%;
  background-color: #0d1117;
  overflow: hidden;
}

.map-container {
  width: 100%;
  height: 100%;
}

/* Dark tile layer tint via CSS filter */
.map-container :deep(.leaflet-tile) {
  filter: brightness(0.75) invert(1) hue-rotate(180deg);
}

.overlay {
  position: absolute;
  top: 12px;
  left: 50px; /* clear Leaflet zoom controls */
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
