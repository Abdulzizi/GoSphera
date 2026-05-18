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

    <!-- Camera fullscreen live feed modal -->
    <Teleport to="body">
      <div v-if="fullscreenCam" class="cam-fs-backdrop" @click.self="closeCamFs">
        <div class="cam-fs-modal">
          <div class="cam-fs-header">
            <span>📷 <b>{{ fullscreenCam.name }}</b>
              <span class="cam-fs-city">({{ fullscreenCam.city }})</span>
              <span v-if="fullscreenCam.streamType === 'hls'" class="cam-fs-badge cam-fs-badge--hls">HLS</span>
              <span v-else-if="fullscreenCam.streamType === 'mjpeg'" class="cam-fs-badge cam-fs-badge--mjpeg">MJPEG</span>
            </span>
            <div class="cam-fs-actions">
              <span class="cam-fs-live">🔴 Live</span>
              <button class="cam-fs-close" @click="closeCamFs">✕</button>
            </div>
          </div>

          <!-- HLS: HTML5 video element with hls.js -->
          <video
            v-if="fullscreenCam.streamType === 'hls'"
            ref="camVideoEl"
            class="cam-fs-video"
            autoplay muted playsinline controls
            @error="($event.target as HTMLVideoElement).style.opacity = '0.2'"
          />

          <!-- MJPEG: browser-native multipart stream via <img> -->
          <img
            v-else-if="fullscreenCam.streamType === 'mjpeg'"
            ref="camMjpegEl"
            :src="fullscreenCam.streamUrl || fullscreenCam.url"
            class="cam-fs-img"
            alt="MJPEG live feed"
            @error="($event.target as HTMLImageElement).style.opacity = '0.2'"
          >

          <!-- Snapshot: polled JPEG -->
          <img
            v-else
            :src="fullscreenSrc"
            class="cam-fs-img"
            alt="Live camera snapshot"
            @error="($event.target as HTMLImageElement).style.opacity = '0.2'"
          >

          <div class="cam-fs-footer">
            <span v-if="fullscreenCam.streamType === 'hls'">HLS live stream</span>
            <span v-else-if="fullscreenCam.streamType === 'mjpeg'">MJPEG live feed</span>
            <span v-else>Snapshot · auto-refreshes every 5 s</span>
            &nbsp;·&nbsp;{{ fullscreenCam.city }}
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, shallowRef, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import 'leaflet.markercluster'
import 'leaflet.markercluster/dist/MarkerCluster.css'
import Hls from 'hls.js'
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
const mapEl         = ref<HTMLDivElement | null>(null)
const mapInstance   = shallowRef<L.Map | null>(null)
const tileLayer     = shallowRef<L.TileLayer | null>(null)
const geoLayer      = shallowRef<L.GeoJSON | null>(null)
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const fireCluster   = shallowRef<any>(null)   // L.MarkerClusterGroup
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const shipCluster   = shallowRef<any>(null)   // L.MarkerClusterGroup
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const cameraCluster = shallowRef<any>(null)   // L.MarkerClusterGroup
const currentZoom   = ref(2)
const featureCount  = computed(() => props.data?.features.length ?? 0)

// ── Camera fullscreen overlay ─────────────────────────────────────────────────
type FsCam = { id: string; name: string; city: string; url: string; streamUrl?: string; streamType: string }
const fullscreenCam = ref<FsCam | null>(null)
const fullscreenSrc = ref('')
const camVideoEl   = ref<HTMLVideoElement | null>(null)
const camMjpegEl   = ref<HTMLImageElement | null>(null)
let fullscreenTimer: ReturnType<typeof setInterval> | null = null
let hlsInstance:    Hls | null = null

function openCamFs(cam: FsCam) {
  // Tear down any previous session first
  _teardownFs()
  fullscreenCam.value = cam

  if (cam.streamType === 'hls') {
    // HLS: always route through the Go relay proxy — it fetches the source .m3u8,
    // rewrites all segment URLs to /api/stream/{id}/seg, and adds CORS headers.
    // This makes ANY HLS source play in the browser regardless of the source's CORS policy.
    nextTick(() => {
      const video = camVideoEl.value
      if (!video) return
      const proxySrc = `/api/stream/${cam.id}/playlist.m3u8`
      if (Hls.isSupported()) {
        hlsInstance = new Hls({
          enableWorker: true,
          lowLatencyMode: false,
          debug: false,
          manifestLoadingMaxRetry: 4,
          levelLoadingMaxRetry: 4,
          fragLoadingMaxRetry: 6,
        })
        hlsInstance.loadSource(proxySrc)
        hlsInstance.attachMedia(video)
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        hlsInstance.on(Hls.Events.MANIFEST_PARSED, () => { video.play().catch(() => {}) })
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        hlsInstance.on(Hls.Events.ERROR, (_e: any, data: any) => {
          if (data.fatal) console.warn('[HLS] fatal error — stream may be offline:', data.type, data.details)
        })
      } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
        // Safari has native HLS — use the proxy for CORS consistency
        video.src = proxySrc
        video.play().catch(() => {})
      }
    })
  } else if (cam.streamType === 'mjpeg') {
    // MJPEG: browser streams via <img> natively — src set via template binding
    // Nothing extra needed here; teardown sets src='' to close the connection
  } else {
    // Snapshot: poll every 5 s
    fullscreenSrc.value = `${cam.url}?t=${Date.now()}`
    fullscreenTimer = setInterval(() => {
      fullscreenSrc.value = `${cam.url}?t=${Date.now()}`
    }, 5_000)
  }
}

/** Destroy all active stream resources without clearing fullscreenCam. */
function _teardownFs() {
  if (fullscreenTimer !== null) { clearInterval(fullscreenTimer); fullscreenTimer = null }
  if (hlsInstance) { hlsInstance.destroy(); hlsInstance = null }
  const video = camVideoEl.value
  if (video) { video.pause(); video.removeAttribute('src'); video.load() }
  const mjpeg = camMjpegEl.value
  if (mjpeg) { mjpeg.src = '' }
  fullscreenSrc.value = ''
}

function closeCamFs() {
  _teardownFs()
  fullscreenCam.value = null
}

// ── Aircraft: DivIcon ✈ + RAF interpolation ───────────────────────────────────
type AircraftEntry = {
  marker: L.Marker
  fromLat: number; fromLon: number
  toLat:   number; toLon:   number
  startTime: number
}
const aircraftRegistry = new Map<string, AircraftEntry>()
let rafId: number | null = null
const INTERP_MS = 14_000

function makePlaneIcon(heading: number, onGround: boolean): L.DivIcon {
  const color  = onGround ? '#9ca3af' : '#60a5fa'
  const shadow = onGround ? 'none' : '0 0 5px rgba(96,165,250,0.7)'
  return L.divIcon({
    html: `<div style="transform:rotate(${heading}deg);color:${color};font-size:14px;line-height:1;text-shadow:${shadow};user-select:none;width:14px;height:14px;display:flex;align-items:center;justify-content:center">✈</div>`,
    className: '',
    iconSize: [14, 14], iconAnchor: [7, 7], popupAnchor: [0, -10],
  })
}

// ── Ships: vessel-shaped DivIcon + markerClusterGroup ────────────────────────
// Ships move at 5–10 m/s — no RAF interpolation needed (sub-pixel per frame).
// Positions are snapped directly on each 15s poll.
type ShipEntry = { marker: L.Marker }
const shipRegistry = new Map<number, ShipEntry>()

function makeShipIcon(heading: number): L.DivIcon {
  // Boat silhouette: narrow bow pointing north, rotated by heading
  const svg = `<svg viewBox="0 0 12 20" width="12" height="20" xmlns="http://www.w3.org/2000/svg">
    <polygon points="6,0 12,20 6,14 0,20" fill="#10b981" stroke="#047857" stroke-width="0.8"/>
  </svg>`
  return L.divIcon({
    html: `<div style="transform:rotate(${heading}deg);line-height:0;filter:drop-shadow(0 0 3px rgba(16,185,129,0.6))">${svg}</div>`,
    className: '',
    iconSize: [12, 20], iconAnchor: [6, 10], popupAnchor: [0, -10],
  })
}

// ── Cluster count icons ────────────────────────────────────────────────────────
function hexToRgb(hex: string): string {
  return `${parseInt(hex.slice(1,3),16)},${parseInt(hex.slice(3,5),16)},${parseInt(hex.slice(5,7),16)}`
}

function makeCountCluster(count: number, hex: string): L.DivIcon {
  const rgb = hexToRgb(hex)
  return L.divIcon({
    html: `<div style="background:rgba(${rgb},0.15);border:1.5px solid ${hex};border-radius:4px;padding:2px 8px;color:${hex};font-size:11px;font-weight:700;font-family:ui-monospace,monospace;white-space:nowrap;box-shadow:0 0 8px rgba(${rgb},0.35)">[${count}]</div>`,
    className: '',
    iconSize: L.point(52, 22, true),
    iconAnchor: [26, 11],
  })
}

// ── RAF tick: aircraft only (ships are too slow to need per-frame lerp) ───────
function tickAircraft(now: number) {
  for (const e of aircraftRegistry.values()) {
    const t = Math.min(1, (now - e.startTime) / INTERP_MS)
    e.marker.setLatLng([e.fromLat + (e.toLat - e.fromLat) * t, e.fromLon + (e.toLon - e.fromLon) * t])
  }
  rafId = requestAnimationFrame(tickAircraft)
}

// ── Tile definitions ───────────────────────────────────────────────────────────
const tileDefs: Record<MapStyle, { url: string; options: L.TileLayerOptions }> = {
  dark: {
    url: 'https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png',
    options: { attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> &copy; <a href="https://carto.com/">CARTO</a>', subdomains: 'abcd', maxZoom: 19 },
  },
  light: {
    url: 'https://{s}.basemaps.cartocdn.com/light_all/{z}/{x}/{y}{r}.png',
    options: { attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> &copy; <a href="https://carto.com/">CARTO</a>', subdomains: 'abcd', maxZoom: 19 },
  },
  satellite: {
    url: 'https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}',
    options: { attribution: 'Tiles &copy; Esri &mdash; Source: Esri, Maxar, Earthstar Geographics', maxZoom: 18 },
  },
}

// ── Formatters ─────────────────────────────────────────────────────────────────
function fmtAlt(m: number)   { return m ? `${Math.round(m).toLocaleString()} m (${Math.round(m*3.28084).toLocaleString()} ft)` : 'n/a' }
function fmtSpeed(ms: number){ return ms ? `${Math.round(ms)} m/s (${Math.round(ms*1.944)} kts)` : 'n/a' }
function fmtKnots(k: number) { return k ? `${k.toFixed(1)} kts` : 'n/a' }

// ── Fire: dot icon for use inside markerClusterGroup ─────────────────────────
const fireDotIcon = L.divIcon({
  html: `<div style="width:7px;height:7px;border-radius:50%;background:#f97316;box-shadow:0 0 4px rgba(249,115,22,0.8)"></div>`,
  className: '',
  iconSize: [7, 7], iconAnchor: [3, 3], popupAnchor: [0, -6],
})

// Spatial-hash downsampling keeps one point per cell — prevents dumping
// 5000 markers into the cluster at global zoom (cluster would just show [5000]
// everywhere). At z<6 we thin first so regional clusters show meaningful counts.
function downsampleFire(records: FireRecord[], zoom: number): FireRecord[] {
  if (zoom >= 6) return records
  const cellDeg = Math.pow(2, 6 - zoom) * 0.1   // ~3.2° at z2, ~0.4° at z5
  const grid = new Map<string, FireRecord>()
  for (const r of records) {
    const key = `${Math.floor(r.latitude / cellDeg)},${Math.floor(r.longitude / cellDeg)}`
    if (!grid.has(key)) grid.set(key, r)
  }
  return Array.from(grid.values())
}

function renderFireLayer() {
  const cluster = fireCluster.value; if (!cluster) return
  cluster.clearLayers()
  if (!props.fireData?.length) return
  const sampled = downsampleFire(props.fireData, currentZoom.value)
  for (const r of sampled) {
    L.marker([r.latitude, r.longitude], { icon: fireDotIcon })
      .bindPopup(`<b>🔥 Fire</b><br>Brightness: ${r.brightness.toFixed(1)} K<br>Confidence: ${r.confidence}<br>Date: ${r.acq_date}`)
      .addTo(cluster)
  }
}

// ── Viewport bbox emit ─────────────────────────────────────────────────────────
let moveDebounce: ReturnType<typeof setTimeout> | null = null

function emitCurrentBounds(map: L.Map) {
  const b = map.getBounds()
  emit('bbox-change', {
    minLat: b.getSouth(), minLon: b.getWest(),
    maxLat: b.getNorth(), maxLon: b.getEast(),
    zoom: map.getZoom(),
  })
}

// ── Aircraft zoom visibility ────────────────────────────────────────────────────
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
    center: [20, 0], zoom: 2, minZoom: 2,
    preferCanvas: true, zoomControl: false,
    maxBounds: [[-90, -Infinity], [90, Infinity]], maxBoundsViscosity: 1.0,
  })
  const def = tileDefs[props.mapStyle]
  tileLayer.value = L.tileLayer(def.url, def.options).addTo(map)
  L.control.zoom({ position: 'bottomright' }).addTo(map)
  map.invalidateSize(true)
  mapInstance.value = map

  // Ship cluster group — teal [N] cluster icons
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const sClu = (L as any).markerClusterGroup({
    maxClusterRadius: 60,
    disableClusteringAtZoom: 11,
    spiderfyOnMaxZoom: true,
    showCoverageOnHover: false,
    iconCreateFunction: (cluster: { getChildCount(): number }) =>
      makeCountCluster(cluster.getChildCount(), '#10b981'),
  })
  if (props.showShips) sClu.addTo(map)
  shipCluster.value = sClu

  // Camera cluster group — amber [N] cluster icons
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const cClu = (L as any).markerClusterGroup({
    maxClusterRadius: 60,
    disableClusteringAtZoom: 13,
    spiderfyOnMaxZoom: true,
    showCoverageOnHover: false,
    iconCreateFunction: (cluster: { getChildCount(): number }) =>
      makeCountCluster(cluster.getChildCount(), '#f59e0b'),
  })
  if (props.showCameras) cClu.addTo(map)
  cameraCluster.value = cClu

  // Fire cluster group — orange [N] cluster icons; individual dots at zoom ≥ 8
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const fClu = (L as any).markerClusterGroup({
    maxClusterRadius: 50,
    disableClusteringAtZoom: 8,
    spiderfyOnMaxZoom: false,
    showCoverageOnHover: false,
    iconCreateFunction: (cluster: { getChildCount(): number }) =>
      makeCountCluster(cluster.getChildCount(), '#f97316'),
  })
  if (props.showFire) fClu.addTo(map)
  fireCluster.value = fClu

  map.on('moveend', () => {
    if (moveDebounce) clearTimeout(moveDebounce)
    moveDebounce = setTimeout(() => emitCurrentBounds(map), 400)
  })

  map.on('zoomend', () => {
    currentZoom.value = map.getZoom()
    renderFireLayer()
    syncAircraftVisibility(map)
  })

  setTimeout(() => emitCurrentBounds(map), 300)
  rafId = requestAnimationFrame(tickAircraft)
})

onUnmounted(() => {
  if (moveDebounce) clearTimeout(moveDebounce)
  if (rafId !== null) cancelAnimationFrame(rafId)
  _teardownFs()
  for (const { marker } of aircraftRegistry.values()) marker.remove()
  aircraftRegistry.clear()
  shipRegistry.clear()
  shipCluster.value?.clearLayers()
  cameraCluster.value?.clearLayers()
  fireCluster.value?.clearLayers()
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
  try { map.fitBounds(layer.getBounds(), { maxZoom: 14, padding: [20, 20] }) } catch { /* empty */ }
})

// ── FIRMS fire layer ───────────────────────────────────────────────────────────
watch(() => props.fireData, () => renderFireLayer())

// ── Aircraft ──────────────────────────────────────────────────────────────────
watch(() => props.aircraftData, (states) => {
  const map = mapInstance.value; if (!map) return
  const showNow = currentZoom.value >= 4 && props.showAircraft
  const seen = new Set<string>()
  for (const s of states) {
    seen.add(s.icao24)
    const ex = aircraftRegistry.get(s.icao24)
    if (ex) {
      const cur = ex.marker.getLatLng()
      ex.fromLat = cur.lat; ex.fromLon = cur.lng
      ex.toLat = s.lat; ex.toLon = s.lon
      ex.startTime = performance.now()
      ex.marker.setIcon(makePlaneIcon(s.heading, s.on_ground))
    } else {
      const marker = L.marker([s.lat, s.lon], { icon: makePlaneIcon(s.heading, s.on_ground) })
      marker.bindPopup(
        `<div style="font-size:12px;min-width:160px">
          <b style="font-size:13px">${s.callsign || s.icao24}</b><br>
          <span style="color:#888">${s.icao24} · ${s.country}</span>
          <hr style="margin:4px 0;border-color:#333">
          <b>Alt:</b> ${fmtAlt(s.altitude)}<br>
          <b>Speed:</b> ${fmtSpeed(s.velocity)}<br>
          <b>Heading:</b> ${Math.round(s.heading)}°<br>
          <b>Status:</b> ${s.on_ground ? '🟡 On ground' : '🔵 Airborne'}
        </div>`, { maxWidth: 220 })
      if (showNow) marker.addTo(map)
      aircraftRegistry.set(s.icao24, { marker, fromLat: s.lat, fromLon: s.lon, toLat: s.lat, toLon: s.lon, startTime: performance.now() })
    }
  }
  for (const [id, e] of aircraftRegistry) {
    if (!seen.has(id)) { e.marker.remove(); aircraftRegistry.delete(id) }
  }
})

// ── Ships ─────────────────────────────────────────────────────────────────────
watch(() => props.shipData, (states) => {
  const cluster = shipCluster.value; if (!cluster) return
  const seen = new Set<number>()
  for (const s of states) {
    seen.add(s.mmsi)
    const ex = shipRegistry.get(s.mmsi)
    if (ex) {
      // Snap to new position on poll — ships are too slow to need interpolation
      ex.marker.setLatLng([s.lat, s.lon])
      ex.marker.setIcon(makeShipIcon(s.heading))
    } else {
      const marker = L.marker([s.lat, s.lon], { icon: makeShipIcon(s.heading) })
      marker.bindPopup(
        `<div style="font-size:12px;min-width:160px">
          <b style="font-size:13px">🚢 ${s.name || s.mmsi}</b><br>
          <span style="color:#888">MMSI: ${s.mmsi}</span>
          <hr style="margin:4px 0;border-color:#333">
          <b>Speed:</b> ${fmtKnots(s.speed)}<br>
          <b>Heading:</b> ${Math.round(s.heading)}°
        </div>`, { maxWidth: 200 })
      cluster.addLayer(marker)
      shipRegistry.set(s.mmsi, { marker })
    }
  }
  for (const [id, e] of shipRegistry) {
    if (!seen.has(id)) { cluster.removeLayer(e.marker); shipRegistry.delete(id) }
  }
})

// ── Cameras ───────────────────────────────────────────────────────────────────
// Popup content is generated LAZILY on popupopen — no img tags pre-allocated.
// Markers use an SVG camera-shaped icon instead of a plain dot.
function makeCameraIcon(): L.DivIcon {
  return L.divIcon({
    className: '',
    html: `<svg width="22" height="22" viewBox="0 0 22 22" xmlns="http://www.w3.org/2000/svg">
      <rect x="1" y="6" width="20" height="13" rx="2.5" fill="#f59e0b" stroke="#92400e" stroke-width="1.2"/>
      <circle cx="11" cy="12.5" r="4.2" fill="#0d1117" stroke="#f59e0b" stroke-width="1.2"/>
      <circle cx="11" cy="12.5" r="2.2" fill="#fbbf24" opacity="0.7"/>
      <rect x="7" y="3" width="8" height="4" rx="1.2" fill="#f59e0b" stroke="#92400e" stroke-width="1"/>
      <rect x="16.5" y="8" width="3" height="2" rx="0.6" fill="#fcd34d"/>
    </svg>`,
    iconSize:   [22, 22],
    iconAnchor: [11, 11],
    popupAnchor:[0, -13],
  })
}

watch(() => props.cameraData, (cameras) => {
  const cluster = cameraCluster.value; if (!cluster) return
  cluster.clearLayers()

  const camIcon = makeCameraIcon()

  for (const c of cameras) {
    const marker = L.marker([c.lat, c.lon], { icon: camIcon })
    // Bind an empty popup — content injected lazily on open
    marker.bindPopup('', { maxWidth: 300 })
    let refreshTimer: ReturnType<typeof setInterval> | null = null
    const streamType = c.stream_type ?? 'snapshot'

    marker.on('popupopen', () => {
      const ts = Date.now()
      const streamBadge = streamType === 'hls'
        ? `<span style="background:#1e3a2a;color:#34d399;font-size:9px;padding:2px 5px;border-radius:3px;margin-left:4px">HLS</span>`
        : streamType === 'mjpeg'
        ? `<span style="background:#1e2a3a;color:#60a5fa;font-size:9px;padding:2px 5px;border-radius:3px;margin-left:4px">MJPEG</span>`
        : ''
      marker.getPopup()?.setContent(
        `<div style="font-size:12px;min-width:240px">
          <b>📷 ${c.name}</b>${streamBadge} <span style="color:#888">(${c.city})</span>
          <div style="margin-top:6px;background:#050809;border-radius:4px;overflow:hidden;min-height:60px">
            <img id="cam-prev-${c.id}" src="${c.snapshot_url}?t=${ts}"
              style="width:100%;max-height:160px;object-fit:cover;display:block"
              onerror="this.style.opacity='0.15'">
          </div>
          <button id="cam-fs-btn-${c.id}"
            style="margin-top:6px;width:100%;padding:6px 0;background:#0d1e30;border:1px solid #1a4a6a;border-radius:4px;color:#60a5fa;cursor:pointer;font-size:11px;letter-spacing:0.02em">
            ⛶ &nbsp;Full Screen · ${streamType === 'snapshot' ? 'Live Feed' : 'Live Stream'}
          </button>
        </div>`
      )
      // Refresh snapshot preview every 5 s while popup is open
      if (streamType === 'snapshot') {
        refreshTimer = setInterval(() => {
          const img = document.getElementById(`cam-prev-${c.id}`) as HTMLImageElement | null
          if (img) img.src = `${c.snapshot_url}?t=${Date.now()}`
        }, 5_000)
      }
      // Wire fullscreen button after DOM settles
      setTimeout(() => {
        const btn = document.getElementById(`cam-fs-btn-${c.id}`)
        if (btn) btn.onclick = () => openCamFs({
          id: c.id, name: c.name, city: c.city,
          url: c.snapshot_url,
          streamUrl: c.stream_url,
          streamType,
        })
      }, 80)
    })

    marker.on('popupclose', () => {
      if (refreshTimer) { clearInterval(refreshTimer); refreshTimer = null }
      marker.getPopup()?.setContent('')
    })

    cluster.addLayer(marker)
  }
})

// ── Layer visibility ───────────────────────────────────────────────────────────
watch(() => props.showAircraft, () => {
  const map = mapInstance.value; if (map) syncAircraftVisibility(map)
})
watch(() => props.showShips, (v) => {
  const map = mapInstance.value; if (!map || !shipCluster.value) return
  v ? shipCluster.value.addTo(map) : shipCluster.value.remove()
})
watch(() => props.showCameras, (v) => {
  const map = mapInstance.value; if (!map || !cameraCluster.value) return
  v ? cameraCluster.value.addTo(map) : cameraCluster.value.remove()
})
watch(() => props.showFire, (v) => {
  const map = mapInstance.value; if (!map || !fireCluster.value) return
  v ? fireCluster.value.addTo(map) : fireCluster.value.remove()
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

/* Remove default markercluster styling — we use custom [N] icons */
:deep(.leaflet-cluster-anim .leaflet-marker-icon),
:deep(.leaflet-cluster-anim .leaflet-marker-shadow) { transition: transform 0.3s ease-out, opacity 0.3s ease-out; }

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

/* ── Camera fullscreen modal ─────────────────────────────────────────────── */
.cam-fs-backdrop {
  position: fixed; inset: 0; z-index: 10000;
  background: rgba(0, 0, 0, 0.88);
  display: flex; align-items: center; justify-content: center;
  backdrop-filter: blur(6px);
}

.cam-fs-modal {
  background: #0d1117;
  border: 1px solid #2a3a4a;
  border-radius: 10px;
  overflow: hidden;
  max-width: min(92vw, 960px);
  max-height: 90vh;
  min-width: 360px;
  display: flex; flex-direction: column;
  box-shadow: 0 24px 80px rgba(0, 0, 0, 0.8);
}

.cam-fs-header {
  display: flex; align-items: center; justify-content: space-between;
  padding: 12px 16px;
  background: #080d13;
  border-bottom: 1px solid #1e2a35;
  font-size: 0.9rem; color: #c0d8e8;
  flex-shrink: 0;
}
.cam-fs-city { color: #4a6a7a; margin-left: 4px; }

.cam-fs-actions { display: flex; align-items: center; gap: 12px; }

@keyframes pulse { 0%,100% { opacity:1 } 50% { opacity:0.4 } }
.cam-fs-live {
  font-size: 0.75rem; color: #ef4444;
  animation: pulse 2s ease-in-out infinite;
}

.cam-fs-close {
  background: none; border: none;
  color: #5a6a7a; font-size: 1rem; cursor: pointer;
  padding: 2px 7px; border-radius: 4px;
  transition: color 0.12s, background 0.12s;
}
.cam-fs-close:hover { color: #e0eaf4; background: #1a2535; }

.cam-fs-img,
.cam-fs-video {
  width: 100%;
  max-height: calc(90vh - 100px);
  object-fit: contain;
  background: #050809;
  flex: 1;
  display: block;
}

.cam-fs-badge {
  display: inline-block;
  font-size: 0.68rem;
  font-weight: 700;
  padding: 1px 6px;
  border-radius: 3px;
  margin-left: 6px;
  vertical-align: middle;
  letter-spacing: 0.04em;
}
.cam-fs-badge--hls  { background: #1e3a2a; color: #34d399; }
.cam-fs-badge--mjpeg{ background: #1e2a3a; color: #60a5fa; }

.cam-fs-footer {
  padding: 8px 16px;
  font-size: 0.72rem; color: #2a4a5a;
  text-align: center;
  border-top: 1px solid #1e2a35;
  flex-shrink: 0;
}
</style>
