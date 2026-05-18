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
import type { FeatureCollection, FireRecord, AircraftState, ShipState, CameraInfo, BBox } from '../services/api'

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

const emit = defineEmits<{
  'bbox-change': [viewport: BBox]
}>()

// ── Refs ───────────────────────────────────────────────────────────────────────
const mapEl       = ref<HTMLDivElement | null>(null)
const mapInstance = shallowRef<L.Map | null>(null)
const tileLayer   = shallowRef<L.TileLayer | null>(null)
const geoLayer    = shallowRef<L.GeoJSON | null>(null)
const fireLayer   = shallowRef<L.LayerGroup | null>(null)
const cameraLayer = shallowRef<L.LayerGroup | null>(null)
const currentZoom = ref(2)

const featureCount = computed(() => props.data?.features.length ?? 0)

// ── Aircraft: DivIcon ✈ registry + RAF smooth interpolation ───────────────────
// Shown only at zoom >= 4 (prevents 8k DOM nodes at global view)
type AircraftEntry = {
  marker:    L.Marker
  fromLat:   number; fromLon:   number
  toLat:     number; toLon:     number
  startTime: number
}
const aircraftRegistry = new Map<string, AircraftEntry>()
let rafId: number | null = null
const INTERP_MS = 14_000

function makePlaneIcon(heading: number, onGround: boolean): L.DivIcon {
  const color  = onGround ? '#9ca3af' : '#60a5fa'
  const shadow = onGround ? 'none'    : '0 0 5px rgba(96,165,250,0.7)'
  return L.divIcon({
    html: `<div style="transform:rotate(${heading}deg);color:${color};font-size:14px;line-height:1;text-shadow:${shadow};user-select:none;width:14px;height:14px;display:flex;align-items:center;justify-content:center">✈</div>`,
    className: '',
    iconSize:    [14, 14],
    iconAnchor:  [7, 7],
    popupAnchor: [0, -10],
  })
}

function tickAircraft(now: number) {
  for (const entry of aircraftRegistry.values()) {
    const t = Math.min(1, (now - entry.startTime) / INTERP_MS)
    const lat = entry.fromLat + (entry.toLat - entry.fromLat) * t
    const lon = entry.fromLon + (entry.toLon - entry.fromLon) * t
    entry.marker.setLatLng([lat, lon])
  }
  rafId = requestAnimationFrame(tickAircraft)
}

// ── Ships: canvas circleMarker registry + RAF ──────────────────────────────────
type ShipEntry = {
  marker:    L.CircleMarker
  fromLat:   number; fromLon:   number
  toLat:     number; toLon:     number
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

// ── Formatters ─────────────────────────────────────────────────────────────────
function fmtAlt(m: number): string {
  return m ? `${Math.round(m).toLocaleString()} m (${Math.round(m * 3.28084).toLocaleString()} ft)` : 'n/a'
}
function fmtSpeed(ms: number): string {
  return ms ? `${Math.round(ms)} m/s (${Math.round(ms * 1.944)} kts)` : 'n/a'
}
function fmtKnots(kts: number): string {
  return kts ? `${kts.toFixed(1)} kts` : 'n/a'
}

// ── Zoom-adaptive FIRMS downsampling ──────────────────────────────────────────
// Grid-cell spatial hash: at low zoom, keep one representative point per cell.
// cellDeg at z2 ≈ 3.2°, at z6 ≈ 0.2°, at z7+ full resolution.
function downsampleFire(records: FireRecord[], zoom: number): FireRecord[] {
  if (zoom >= 7) return records
  const cellDeg = Math.pow(2, 7 - zoom) * 0.05
  const grid = new Map<string, FireRecord>()
  for (const r of records) {
    const key = `${Math.floor(r.latitude / cellDeg)},${Math.floor(r.longitude / cellDeg)}`
    if (!grid.has(key)) grid.set(key, r)
  }
  return Array.from(grid.values())
}

function renderFireLayer() {
  const map = mapInstance.value
  if (!map) return
  fireLayer.value?.remove()
  fireLayer.value = null
  if (!props.fireData?.length) return

  const sampled = downsampleFire(props.fireData, currentZoom.value)
  const group = L.layerGroup()
  for (const r of sampled) {
    L.circleMarker([r.latitude, r.longitude], {
      radius: 4, color: '#f97316', fillColor: '#f97316', fillOpacity: 0.7, weight: 0.5,
    })
      .bindPopup(
        `<b>🔥 Fire</b><br>Brightness: ${r.brightness.toFixed(1)} K<br>` +
        `Confidence: ${r.confidence}<br>Date: ${r.acq_date}`,
      )
      .addTo(group)
  }
  if (props.showFire) group.addTo(map)
  fireLayer.value = group
}

// ── Viewport bbox emit ─────────────────────────────────────────────────────────
let moveDebounce: ReturnType<typeof setTimeout> | null = null

function emitCurrentBounds(map: L.Map) {
  const b = map.getBounds()
  const zoom = map.getZoom()
  emit('bbox-change', {
    minLat: b.getSouth(), minLon: b.getWest(),
    maxLat: b.getNorth(), maxLon: b.getEast(),
    zoom,
  })
}

// ── Aircraft zoom visibility ────────────────────────────────────────────────────
// At zoom < 4 the entire globe is visible — hiding aircraft avoids 8k DOM nodes.
function syncAircraftVisibility(map: L.Map) {
  const show = currentZoom.value >= 4 && props.showAircraft
  for (const { marker } of aircraftRegistry.values()) {
    if (show && !map.hasLayer(marker)) marker.addTo(map)
    else if (!show && map.hasLayer(marker)) marker.remove()
  }
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

  // Debounced bbox emit on pan
  map.on('moveend', () => {
    if (moveDebounce) clearTimeout(moveDebounce)
    moveDebounce = setTimeout(() => emitCurrentBounds(map), 400)
  })

  // On zoom: update current zoom, re-downsample fire, sync aircraft visibility
  map.on('zoomend', () => {
    currentZoom.value = map.getZoom()
    renderFireLayer()
    syncAircraftVisibility(map)
  })

  // Emit initial bounds after tiles settle
  setTimeout(() => emitCurrentBounds(map), 300)

  rafId = requestAnimationFrame(tickAircraft)
})

onUnmounted(() => {
  if (moveDebounce) clearTimeout(moveDebounce)
  if (rafId !== null) cancelAnimationFrame(rafId)
  for (const { marker } of aircraftRegistry.values()) marker.remove()
  aircraftRegistry.clear()
  for (const { marker } of shipRegistry.values()) marker.remove()
  shipRegistry.clear()
  mapInstance.value?.remove()
  mapInstance.value = null
})

// ── Tile swap ──────────────────────────────────────────────────────────────────
watch(() => props.mapStyle, (style) => {
  const map = mapInstance.value; if (!map) return
  tileLayer.value?.remove()
  tileLayer.value = L.tileLayer(tileDefs[style].url, tileDefs[style].options).addTo(map)
})

// ── GeoJSON spatial query ──────────────────────────────────────────────────────
watch(() => props.data, (fc) => {
  const map = mapInstance.value; if (!map) return
  geoLayer.value?.remove(); geoLayer.value = null
  if (!fc || fc.features.length === 0) return
  const layer = L.geoJSON(fc as unknown as GeoJSON.FeatureCollection, {
    style: { color: '#3a8fd4', weight: 1.5, fillOpacity: 0.25, fillColor: '#3a8fd4' },
    pointToLayer: (_f, latlng) =>
      L.circleMarker(latlng, { radius: 6, color: '#3a8fd4', fillColor: '#3a8fd4', fillOpacity: 0.85, weight: 1 }),
    onEachFeature: (feature, layer) => {
      const p = feature.properties ?? {}
      const rows = Object.entries(p)
        .filter(([k]) => !k.startsWith('osm_'))
        .map(([k, v]) => `<tr><td><b>${k}</b></td><td>${v}</td></tr>`).join('')
      if (rows) layer.bindPopup(`<table style="font-size:12px;border-collapse:collapse">${rows}</table>`)
    },
  }).addTo(map)
  geoLayer.value = layer
  try { map.fitBounds(layer.getBounds(), { maxZoom: 14, padding: [20, 20] }) } catch {}
})

// ── FIRMS fire dots ────────────────────────────────────────────────────────────
watch(() => props.fireData, () => renderFireLayer())

// ── Aircraft: ✈ DivIcon registry, zoom-gated, RAF-interpolated ────────────────
watch(() => props.aircraftData, (states) => {
  const map = mapInstance.value; if (!map) return
  const showNow = currentZoom.value >= 4 && props.showAircraft

  const seen = new Set<string>()
  for (const s of states) {
    seen.add(s.icao24)
    const existing = aircraftRegistry.get(s.icao24)
    if (existing) {
      const cur = existing.marker.getLatLng()
      existing.fromLat = cur.lat; existing.fromLon = cur.lng
      existing.toLat   = s.lat;   existing.toLon   = s.lon
      existing.startTime = performance.now()
      existing.marker.setIcon(makePlaneIcon(s.heading, s.on_ground))
    } else {
      const marker = L.marker([s.lat, s.lon], { icon: makePlaneIcon(s.heading, s.on_ground) })
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
      if (showNow) marker.addTo(map)
      aircraftRegistry.set(s.icao24, {
        marker, fromLat: s.lat, fromLon: s.lon,
        toLat: s.lat, toLon: s.lon, startTime: performance.now(),
      })
    }
  }
  // Remove aircraft no longer in feed
  for (const [id, entry] of aircraftRegistry) {
    if (!seen.has(id)) { entry.marker.remove(); aircraftRegistry.delete(id) }
  }
})

// ── Ships: canvas circleMarker registry ────────────────────────────────────────
watch(() => props.shipData, (states) => {
  const map = mapInstance.value; if (!map) return

  const seen = new Set<number>()
  for (const s of states) {
    seen.add(s.mmsi)
    const existing = shipRegistry.get(s.mmsi)
    if (existing) {
      const cur = existing.marker.getLatLng()
      existing.fromLat = cur.lat; existing.fromLon = cur.lng
      existing.toLat   = s.lat;   existing.toLon   = s.lon
      existing.startTime = performance.now()
    } else {
      const marker = L.circleMarker([s.lat, s.lon], {
        radius: 5, color: '#10b981', fillColor: '#10b981', fillOpacity: 0.85, weight: 1,
      })
      const name = s.name || String(s.mmsi)
      marker.bindPopup(
        `<div style="font-size:12px;min-width:160px">
          <b style="font-size:13px">🚢 ${name}</b><br>
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
  for (const [id, entry] of shipRegistry) {
    if (!seen.has(id)) { entry.marker.remove(); shipRegistry.delete(id) }
  }
})

// ── CCTV cameras ───────────────────────────────────────────────────────────────
watch(() => props.cameraData, (cameras) => {
  const map = mapInstance.value; if (!map) return
  cameraLayer.value?.remove(); cameraLayer.value = null
  if (!cameras?.length) return
  const group = L.layerGroup()
  for (const c of cameras) {
    L.circleMarker([c.lat, c.lon], {
      radius: 6, color: '#f59e0b', fillColor: '#f59e0b', fillOpacity: 0.9, weight: 1.5,
    })
      .bindPopup(
        `<div style="font-size:12px">
          <b>📷 ${c.name}</b> <span style="color:#888">(${c.city})</span><br>
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
watch(() => props.showAircraft, () => {
  const map = mapInstance.value; if (!map) return
  syncAircraftVisibility(map)
})

watch(() => props.showShips, (v) => {
  const map = mapInstance.value; if (!map) return
  for (const { marker } of shipRegistry.values()) {
    v ? marker.addTo(map) : marker.remove()
  }
})

watch(() => props.showCameras, (v) => {
  const map = mapInstance.value; if (!map || !cameraLayer.value) return
  v ? cameraLayer.value.addTo(map) : cameraLayer.value.remove()
})

watch(() => props.showFire, (v) => {
  const map = mapInstance.value; if (!map || !fireLayer.value) return
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

.map-container { width: 100%; height: 100%; }

.style-dark :deep(.leaflet-container)      { background: #0e0e10; }
.style-light :deep(.leaflet-container)     { background: #f0ede8; }
.style-satellite :deep(.leaflet-container) { background: #1a1a1a; }
.style-light :deep(.leaflet-popup-content-wrapper) { background: #fff; color: #111; }

.loading-overlay {
  position: absolute; inset: 0;
  display: flex; align-items: center; justify-content: center; gap: 10px;
  background: rgba(13,17,23,0.6);
  z-index: 1001; pointer-events: none;
}

@keyframes spin { to { transform: rotate(360deg); } }
.spinner {
  width: 22px; height: 22px;
  border: 3px solid #2a3a4a; border-top-color: #3a8fd4;
  border-radius: 50%; animation: spin 0.7s linear infinite;
}
.loading-text { font-size: 0.88rem; color: #8ab0c8; }

.overlay {
  position: absolute; top: 12px; left: 12px;
  background: rgba(13,17,23,0.88); border: 1px solid #2a3040;
  border-radius: 4px; padding: 6px 12px;
  pointer-events: none; z-index: 1000;
}
.hint { color: #5a6a7a; font-size: 0.82rem; }
.info { color: #3a8fd4; font-size: 0.82rem; font-weight: 600; }
</style>
