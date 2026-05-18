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
import type { FeatureCollection, FireRecord, AircraftState, ShipState, CameraInfo } from '../services/api'

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
  shipData:     ShipState[]
  cameraData:   CameraInfo[]
  isLoading:    boolean
  mapStyle:     MapStyle
  showAircraft: boolean
  showShips:    boolean
  showCameras:  boolean
  showFire:     boolean
}>()

const mapEl       = ref<HTMLDivElement | null>(null)
const mapInstance = shallowRef<L.Map | null>(null)
const tileLayer   = shallowRef<L.TileLayer | null>(null)
const geoLayer    = shallowRef<L.GeoJSON | null>(null)
const fireLayer   = shallowRef<L.LayerGroup | null>(null)
const cameraLayer = shallowRef<L.LayerGroup | null>(null)

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
const INTERP_MS = 14_000 // interpolate over slightly less than 15s poll interval

function tickAircraft(now: number) {
  for (const entry of aircraftRegistry.values()) {
    const t = Math.min(1, (now - entry.startTime) / INTERP_MS)
    const lat = entry.fromLat + (entry.toLat - entry.fromLat) * t
    const lon = entry.fromLon + (entry.toLon - entry.fromLon) * t
    entry.marker.setLatLng([lat, lon])
  }
  rafId = requestAnimationFrame(tickAircraft)
}

// ── Ship: same canvas registry pattern ────────────────────────────────────────
type ShipEntry = {
  marker:    L.CircleMarker
  fromLat:   number
  fromLon:   number
  toLat:     number
  toLon:     number
  startTime: number
}
const shipRegistry = new Map<number, ShipEntry>()

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
function fmtKnots(kts: number): string {
  if (!kts) return 'n/a'
  return `${kts.toFixed(1)} kts`
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

  rafId = requestAnimationFrame(tickAircraft)
})

onUnmounted(() => {
  if (rafId !== null) cancelAnimationFrame(rafId)
  // Clean up all aircraft markers
  for (const { marker } of aircraftRegistry.values()) marker.remove()
  aircraftRegistry.clear()
  for (const { marker } of shipRegistry.values()) marker.remove()
  shipRegistry.clear()
  mapInstance.value?.remove()
  mapInstance.value = null
})

// ── Tile layer swap on style change ────────────────────────────────────────────
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

// ── FIRMS fire dots ────────────────────────────────────────────────────────────
watch(() => props.fireData, (records) => {
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
  if (props.showFire) group.addTo(map)
  fireLayer.value = group
})

// ── Aircraft: canvas circleMarker registry with RAF interpolation ──────────────
watch(() => props.aircraftData, (states) => {
  const map = mapInstance.value
  if (!map) return

  const seen = new Set<string>()
  for (const s of states) {
    seen.add(s.icao24)
    const existing = aircraftRegistry.get(s.icao24)
    if (existing) {
      const cur = existing.marker.getLatLng()
      existing.fromLat = cur.lat
      existing.fromLon = cur.lng
      existing.toLat   = s.lat
      existing.toLon   = s.lon
      existing.startTime = performance.now()
      const airColor = s.on_ground ? '#6b7280' : '#60a5fa'
      existing.marker.setStyle({ color: airColor, fillColor: airColor })
    } else {
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
      )
      if (props.showAircraft) marker.addTo(map)
      aircraftRegistry.set(s.icao24, {
        marker, fromLat: s.lat, fromLon: s.lon,
        toLat: s.lat, toLon: s.lon, startTime: performance.now(),
      })
    }
  }
  // Remove stale aircraft
  for (const [id, entry] of aircraftRegistry) {
    if (!seen.has(id)) {
      entry.marker.remove()
      aircraftRegistry.delete(id)
    }
  }
})

// ── Ships: same canvas circleMarker registry ───────────────────────────────────
watch(() => props.shipData, (states) => {
  const map = mapInstance.value
  if (!map) return

  const seen = new Set<number>()
  for (const s of states) {
    seen.add(s.mmsi)
    const existing = shipRegistry.get(s.mmsi)
    if (existing) {
      const cur = existing.marker.getLatLng()
      existing.fromLat   = cur.lat
      existing.fromLon   = cur.lng
      existing.toLat     = s.lat
      existing.toLon     = s.lon
      existing.startTime = performance.now()
    } else {
      const marker = L.circleMarker([s.lat, s.lon], {
        radius: 5, color: '#10b981', fillColor: '#10b981', fillOpacity: 0.85, weight: 1,
      })
      const name = s.name || String(s.mmsi)
      marker.bindPopup(
        `<div style="font-size:12px;min-width:160px">
          <b style="font-size:13px">${name}</b><br>
          <span style="color:#888">MMSI: ${s.mmsi}</span>
          <hr style="margin:4px 0;border-color:#333">
          <b>Speed:</b> ${fmtKnots(s.speed)}<br>
          <b>Heading:</b> ${Math.round(s.heading)}°
        </div>`,
        { maxWidth: 200 },
      )
      if (props.showShips) marker.addTo(map)
      shipRegistry.set(s.mmsi, {
        marker, fromLat: s.lat, fromLon: s.lon,
        toLat: s.lat, toLon: s.lon, startTime: performance.now(),
      })
    }
  }
  // Remove stale ships
  for (const [id, entry] of shipRegistry) {
    if (!seen.has(id)) {
      entry.marker.remove()
      shipRegistry.delete(id)
    }
  }
})

// ── Traffic cameras ─────────────────────────────────────────────────────────────
watch(() => props.cameraData, (cameras) => {
  const map = mapInstance.value
  if (!map) return
  cameraLayer.value?.remove()
  cameraLayer.value = null
  if (!cameras?.length) return

  const group = L.layerGroup()
  for (const c of cameras) {
    L.circleMarker([c.lat, c.lon], {
      radius: 6, color: '#f59e0b', fillColor: '#f59e0b', fillOpacity: 0.9, weight: 1.5,
    })
      .bindPopup(
        `<div style="font-size:12px">
          <b>${c.name}</b> <span style="color:#888">(${c.city})</span><br>
          <img src="${c.snapshot_url}" style="width:280px;max-height:200px;object-fit:cover;border-radius:4px;margin-top:6px;display:block" loading="lazy" onerror="this.style.display='none'">
        </div>`,
        { maxWidth: 300 },
      )
      .addTo(group)
  }
  if (props.showCameras) group.addTo(map)
  cameraLayer.value = group
})

// ── Layer visibility toggles ───────────────────────────────────────────────────
watch(() => props.showAircraft, (v) => {
  const map = mapInstance.value
  if (!map) return
  for (const { marker } of aircraftRegistry.values()) {
    v ? marker.addTo(map) : marker.remove()
  }
})

watch(() => props.showShips, (v) => {
  const map = mapInstance.value
  if (!map) return
  for (const { marker } of shipRegistry.values()) {
    v ? marker.addTo(map) : marker.remove()
  }
})

watch(() => props.showCameras, (v) => {
  const map = mapInstance.value
  if (!map || !cameraLayer.value) return
  v ? cameraLayer.value.addTo(map) : cameraLayer.value.remove()
})

watch(() => props.showFire, (v) => {
  const map = mapInstance.value
  if (!map || !fireLayer.value) return
  v ? fireLayer.value.addTo(map) : fireLayer.value.remove()
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
