<template>
  <div class="globe-wrap" :class="`style-${mapStyle}`">
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
import type { FeatureCollection, FireRecord, AircraftState } from '../services/api'

import markerIcon2x from 'leaflet/dist/images/marker-icon-2x.png'
import markerIcon   from 'leaflet/dist/images/marker-icon.png'
import markerShadow from 'leaflet/dist/images/marker-shadow.png'
delete (L.Icon.Default.prototype as unknown as Record<string, unknown>)._getIconUrl
L.Icon.Default.mergeOptions({ iconRetinaUrl: markerIcon2x, iconUrl: markerIcon, shadowUrl: markerShadow })

type MapStyle = 'dark' | 'light' | 'satellite'

const props = defineProps<{
  data:         FeatureCollection | null
  fireData:     FireRecord[]
  aircraftData: AircraftState[]
  isLoading:    boolean
  mapStyle:     MapStyle
}>()

const mapEl       = ref<HTMLDivElement | null>(null)
const mapInstance = shallowRef<L.Map | null>(null)
const tileLayer   = shallowRef<L.TileLayer | null>(null)
const geoLayer    = shallowRef<L.GeoJSON | null>(null)
const fireLayer   = shallowRef<L.LayerGroup | null>(null)
const aircraftLayer = shallowRef<L.LayerGroup | null>(null)

const featureCount = computed(() => props.data?.features.length ?? 0)

const tileDefs: Record<MapStyle, { url: string; options: L.TileLayerOptions }> = {
  dark: {
    url: 'https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png',
    options: {
      attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> &copy; <a href="https://carto.com/">CARTO</a>',
      subdomains: 'abcd',
      maxZoom: 19,
    },
  },
  light: {
    url: 'https://{s}.basemaps.cartocdn.com/light_all/{z}/{x}/{y}{r}.png',
    options: {
      attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> &copy; <a href="https://carto.com/">CARTO</a>',
      subdomains: 'abcd',
      maxZoom: 19,
    },
  },
  satellite: {
    url: 'https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}',
    options: {
      attribution: 'Tiles &copy; Esri &mdash; Source: Esri, Maxar, Earthstar Geographics',
      maxZoom: 18,
    },
  },
}

function makePlaneIcon(heading: number, onGround: boolean): L.DivIcon {
  const color  = onGround ? '#6b7280' : '#60a5fa'
  const shadow = onGround ? 'none'    : '0 0 4px rgba(96,165,250,0.6)'
  return L.divIcon({
    html: `<div style="transform:rotate(${heading}deg);color:${color};font-size:15px;line-height:1;text-shadow:${shadow};user-select:none">✈</div>`,
    className: '',
    iconSize:   [16, 16],
    iconAnchor: [8, 8],
    popupAnchor:[0, -12],
  })
}

function fmtAlt(m: number): string {
  if (!m) return 'n/a'
  return `${Math.round(m).toLocaleString()} m (${Math.round(m * 3.28084).toLocaleString()} ft)`
}
function fmtSpeed(ms: number): string {
  if (!ms) return 'n/a'
  return `${Math.round(ms)} m/s (${Math.round(ms * 1.944)} kts)`
}

onMounted(async () => {
  await nextTick()
  if (!mapEl.value) return

  const map = L.map(mapEl.value, {
    center: [20, 0],
    zoom: 2,
    minZoom: 2,
    preferCanvas: true,
    zoomControl: false,
    maxBounds: [[-90, -Infinity], [90, Infinity]],
    maxBoundsViscosity: 1.0,
  })

  const def = tileDefs[props.mapStyle]
  tileLayer.value = L.tileLayer(def.url, def.options).addTo(map)

  L.control.zoom({ position: 'bottomright' }).addTo(map)
  map.invalidateSize(true)
  mapInstance.value = map
})

onUnmounted(() => {
  mapInstance.value?.remove()
  mapInstance.value = null
})

// Swap tile layer when map style changes
watch(
  () => props.mapStyle,
  (style) => {
    const map = mapInstance.value
    if (!map) return
    tileLayer.value?.remove()
    const def = tileDefs[style]
    tileLayer.value = L.tileLayer(def.url, def.options).addTo(map)
  },
)

// GeoJSON spatial query layer
watch(
  () => props.data,
  (fc) => {
    const map = mapInstance.value
    if (!map) return
    geoLayer.value?.remove()
    geoLayer.value = null
    if (!fc || fc.features.length === 0) return

    const layer = L.geoJSON(fc as unknown as GeoJSON.FeatureCollection, {
      style: { color: '#3a8fd4', weight: 1.5, fillOpacity: 0.25, fillColor: '#3a8fd4' },
      pointToLayer: (_f, latlng) =>
        L.circleMarker(latlng, { radius: 6, color: '#3a8fd4', fillColor: '#3a8fd4', fillOpacity: 0.85, weight: 1 }),
      onEachFeature: (feature, layer) => {
        const p = feature.properties ?? {}
        const rows = Object.entries(p)
          .filter(([k]) => !k.startsWith('osm_'))
          .map(([k, v]) => `<tr><td><b>${k}</b></td><td>${v}</td></tr>`)
          .join('')
        if (rows) layer.bindPopup(`<table style="font-size:12px;border-collapse:collapse">${rows}</table>`)
      },
    }).addTo(map)
    geoLayer.value = layer
    try { map.fitBounds(layer.getBounds(), { maxZoom: 14, padding: [20, 20] }) } catch {}
  },
)

// FIRMS fire dots layer
watch(
  () => props.fireData,
  (records) => {
    const map = mapInstance.value
    if (!map) return
    fireLayer.value?.remove()
    fireLayer.value = null
    if (!records?.length) return

    const group = L.layerGroup()
    for (const r of records) {
      L.circleMarker([r.latitude, r.longitude], {
        radius: 4, color: '#f97316', fillColor: '#f97316', fillOpacity: 0.7, weight: 0.5,
      })
        .bindPopup(`<b>Fire</b><br>Brightness: ${r.brightness.toFixed(1)} K<br>Confidence: ${r.confidence}<br>Date: ${r.acq_date}`)
        .addTo(group)
    }
    group.addTo(map)
    fireLayer.value = group
  },
)

// Live aircraft layer
watch(
  () => props.aircraftData,
  (states) => {
    const map = mapInstance.value
    if (!map) return
    aircraftLayer.value?.remove()
    aircraftLayer.value = null
    if (!states?.length) return

    const group = L.layerGroup()
    for (const s of states) {
      const callsign = s.callsign || s.icao24
      L.marker([s.lat, s.lon], { icon: makePlaneIcon(s.heading, s.on_ground) })
        .bindPopup(
          `<div style="font-size:12px;min-width:160px">
            <b style="font-size:13px">${callsign}</b><br>
            <span style="color:#888">${s.icao24} · ${s.country}</span><hr style="margin:4px 0;border-color:#333">
            <b>Alt:</b> ${fmtAlt(s.altitude)}<br>
            <b>Speed:</b> ${fmtSpeed(s.velocity)}<br>
            <b>Heading:</b> ${Math.round(s.heading)}°<br>
            <b>Status:</b> ${s.on_ground ? '🟡 On ground' : '🔵 Airborne'}
          </div>`,
          { maxWidth: 220 },
        )
        .addTo(group)
    }
    group.addTo(map)
    aircraftLayer.value = group
  },
)
</script>

<style scoped>
.globe-wrap {
  position: relative;
  height: 100%;
  min-height: 0;
  background-color: #0e0e10;
  overflow: hidden;
}

/* Background color matches each tile style to avoid white flashes */
.globe-wrap.style-light     { background-color: #f0ede8; }
.globe-wrap.style-satellite { background-color: #1a1a1a; }

.map-container {
  width: 100%;
  height: 100%;
}

/* CartoDB dark background */
.style-dark :deep(.leaflet-container)     { background: #0e0e10; }
.style-light :deep(.leaflet-container)    { background: #f0ede8; }
.style-satellite :deep(.leaflet-container){ background: #1a1a1a; }

/* Light-mode popup text */
.style-light :deep(.leaflet-popup-content-wrapper) {
  background: #fff;
  color: #111;
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

@keyframes spin { to { transform: rotate(360deg); } }

.spinner {
  width: 22px;
  height: 22px;
  border: 3px solid #2a3a4a;
  border-top-color: #3a8fd4;
  border-radius: 50%;
  animation: spin 0.7s linear infinite;
}

.loading-text { font-size: 0.88rem; color: #8ab0c8; }

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

.hint { color: #5a6a7a; font-size: 0.82rem; }
.info { color: #3a8fd4; font-size: 0.82rem; font-weight: 600; }
</style>
