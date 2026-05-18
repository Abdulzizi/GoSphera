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
import type { FeatureCollection, FireRecord, AircraftState, BBox } from '../services/api'

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

const emit = defineEmits<{
  'bbox-change': [bbox: BBox]
}>()

// ── Refs ───────────────────────────────────────────────────────────────────────
const mapEl       = ref<HTMLDivElement | null>(null)
const mapInstance = shallowRef<L.Map | null>(null)
const tileLayer   = shallowRef<L.TileLayer | null>(null)
const geoLayer    = shallowRef<L.GeoJSON | null>(null)
const fireLayer   = shallowRef<L.LayerGroup | null>(null)

const featureCount = computed(() => props.data?.features.length ?? 0)

// ── Aircraft: canvas circleMarker registry + RAF smooth interpolation ──────────
type AircraftEntry = {
  marker:    L.CircleMarker
  fromLat:   number
  fromLon:   number
  toLat:     number
  toLon:     number
  startTime: number
}
const aircraftRegistry = new Map<string, AircraftEntry>()
let rafId: number | null = null
const INTERP_MS = 14_000 // slightly less than 15s poll interval

function tickAircraft(now: number) {
  for (const entry of aircraftRegistry.values()) {
    const t = Math.min(1, (now - entry.startTime) / INTERP_MS)
    const lat = entry.fromLat + (entry.toLat - entry.fromLat) * t
    const lon = entry.fromLon + (entry.toLon - entry.fromLon) * t
    entry.marker.setLatLng([lat, lon])
  }
  rafId = requestAnimationFrame(tickAircraft)
}

// ── Tile definitions ───────────────────────────────────────────────────────────
const tileDefs: Record<MapStyle, { url: string; options: L.TileLayerOptions }> = {
  dark: {
    url: 'https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png',
    options: {
      attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> &copy; <a href="https://carto.com/">CARTO</a>',
      subdomains: 'abcd', maxZoom: 19,
    },
  },
  light: {
    url: 'https://{s}.basemaps.cartocdn.com/light_all/{z}/{x}/{y}{r}.png',
    options: {
      attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> &copy; <a href="https://carto.com/">CARTO</a>',
      subdomains: 'abcd', maxZoom: 19,
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

function fmtAlt(m: number): string {
  if (!m) return 'n/a'
  return `${Math.round(m).toLocaleString()} m (${Math.round(m * 3.28084).toLocaleString()} ft)`
}
function fmtSpeed(ms: number): string {
  if (!ms) return 'n/a'
  return `${Math.round(ms)} m/s (${Math.round(ms * 1.944)} kts)`
}

// ── Zoom-adaptive FIRMS downsampling (no external library) ─────────────────────
// At low zoom, merge nearby fire dots into a single representative point using a
// spatial hash grid.  Grid cell size shrinks as zoom increases → full detail at z7+.
function downsampleFire(records: FireRecord[], zoom: number): FireRecord[] {
  if (zoom >= 7) return records
  // cellDeg: ~3.2° at z2 → ~0.1° at z6
  const cellDeg = Math.pow(2, 7 - zoom) * 0.05
  const grid = new Map<string, FireRecord>()
  for (const r of records) {
    const key = `${Math.floor(r.latitude / cellDeg)},${Math.floor(r.longitude / cellDeg)}`
    if (!grid.has(key)) grid.set(key, r)
  }
  return Array.from(grid.values())
}

// Extracted fire render — called on data change AND on zoomend
function renderFireLayer() {
  const map = mapInstance.value
  if (!map) return
  fireLayer.value?.remove()
  fireLayer.value = null
  if (!props.fireData?.length) return

  const zoom = map.getZoom()
  const sampled = downsampleFire(props.fireData, zoom)

  const group = L.layerGroup()
  for (const r of sampled) {
    L.circleMarker([r.latitude, r.longitude], {
      radius: 4, color: '#f97316', fillColor: '#f97316', fillOpacity: 0.7, weight: 0.5,
    })
      .bindPopup(
        `<b>Fire</b><br>Brightness: ${r.brightness.toFixed(1)} K<br>` +
        `Confidence: ${r.confidence}<br>Date: ${r.acq_date}`,
      )
      .addTo(group)
  }
  group.addTo(map)
  fireLayer.value = group
}

// ── Viewport bbox emit ─────────────────────────────────────────────────────────
let moveDebounce: ReturnType<typeof setTimeout> | null = null

function emitCurrentBounds(map: L.Map) {
  const b = map.getBounds()
  emit('bbox-change', {
    minLat: b.getSouth(),
    minLon: b.getWest(),
    maxLat: b.getNorth(),
    maxLon: b.getEast(),
  })
}

// ── Lifecycle ──────────────────────────────────────────────────────────────────
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

  // Emit viewport bbox after pan/zoom — debounced to avoid rapid-fire fetches
  map.on('moveend', () => {
    if (moveDebounce) clearTimeout(moveDebounce)
    moveDebounce = setTimeout(() => emitCurrentBounds(map), 400)
  })

  // Re-render fire layer after zoom so downsampling reflects new zoom level
  map.on('zoomend', () => renderFireLayer())

  // Emit initial bounds after tiles settle (~300ms)
  setTimeout(() => emitCurrentBounds(map), 300)

  // Start RAF animation loop for smooth aircraft interpolation
  rafId = requestAnimationFrame(tickAircraft)
})

onUnmounted(() => {
  if (moveDebounce) clearTimeout(moveDebounce)
  if (rafId !== null) cancelAnimationFrame(rafId)
  for (const { marker } of aircraftRegistry.values()) marker.remove()
  aircraftRegistry.clear()
  mapInstance.value?.remove()
  mapInstance.value = null
})

// ── Tile swap on style change ──────────────────────────────────────────────────
watch(() => props.mapStyle, (style) => {
  const map = mapInstance.value
  if (!map) return
  tileLayer.value?.remove()
  const def = tileDefs[style]
  tileLayer.value = L.tileLayer(def.url, def.options).addTo(map)
})

// ── GeoJSON spatial query layer ────────────────────────────────────────────────
watch(() => props.data, (fc) => {
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
})

// ── FIRMS fire dots — zoom-adaptive, re-renders on data change ─────────────────
watch(() => props.fireData, () => renderFireLayer())

// ── Aircraft: canvas circleMarker registry with RAF interpolation ──────────────
watch(() => props.aircraftData, (states) => {
  const map = mapInstance.value
  if (!map) return

  const seen = new Set<string>()
  for (const s of states) {
    seen.add(s.icao24)
    const existing = aircraftRegistry.get(s.icao24)
    if (existing) {
      // Reuse marker — update interpolation target from current position
      const cur = existing.marker.getLatLng()
      existing.fromLat   = cur.lat
      existing.fromLon   = cur.lng
      existing.toLat     = s.lat
      existing.toLon     = s.lon
      existing.startTime = performance.now()
      const airColor = s.on_ground ? '#6b7280' : '#60a5fa'
      existing.marker.setStyle({ color: airColor, fillColor: airColor })
    } else {
      // New aircraft — create canvas circleMarker and add to map
      const airColor = s.on_ground ? '#6b7280' : '#60a5fa'
      const marker = L.circleMarker([s.lat, s.lon], {
        radius: 4, color: airColor, fillColor: airColor, fillOpacity: 0.85, weight: 1,
      })
      const callsign = s.callsign || s.icao24
      marker.bindPopup(
        `<div style="font-size:12px;min-width:160px">
          <b style="font-size:13px">${callsign}</b><br>
          <span style="color:#888">${s.icao24} · ${s.country}</span>
          <hr style="margin:4px 0;border-color:#333">
          <b>Alt:</b> ${fmtAlt(s.altitude)}<br>
          <b>Speed:</b> ${fmtSpeed(s.velocity)}<br>
          <b>Heading:</b> ${Math.round(s.heading)}°<br>
          <b>Status:</b> ${s.on_ground ? '🟡 On ground' : '🔵 Airborne'}
        </div>`,
        { maxWidth: 220 },
      ).addTo(map)
      aircraftRegistry.set(s.icao24, {
        marker, fromLat: s.lat, fromLon: s.lon,
        toLat: s.lat, toLon: s.lon, startTime: performance.now(),
      })
    }
  }

  // Remove aircraft that disappeared from the feed
  for (const [id, entry] of aircraftRegistry) {
    if (!seen.has(id)) {
      entry.marker.remove()
      aircraftRegistry.delete(id)
    }
  }
})
</script>

<style scoped>
.globe-wrap {
  position: relative;
  height: 100%;
  min-height: 0;
  background-color: #0e0e10;
  overflow: hidden;
}

.globe-wrap.style-light     { background-color: #f0ede8; }
.globe-wrap.style-satellite { background-color: #1a1a1a; }

.map-container {
  width: 100%;
  height: 100%;
}

.style-dark :deep(.leaflet-container)     { background: #0e0e10; }
.style-light :deep(.leaflet-container)    { background: #f0ede8; }
.style-satellite :deep(.leaflet-container){ background: #1a1a1a; }

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
