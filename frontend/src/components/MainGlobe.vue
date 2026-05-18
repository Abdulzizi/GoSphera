<template>
  <div class="globe-wrap">
    <canvas ref="mapCanvas" class="map-canvas" />
    <div class="overlay">
      <span v-if="!data" class="hint">Select tags and bbox, then click Query</span>
      <span v-else class="info">{{ featureCount }} features loaded</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { shallowRef, computed, onMounted, watch } from 'vue'
import type { FeatureCollection } from '../services/api'

const props = defineProps<{ data: FeatureCollection | null }>()

// shallowRef prevents Vue from deep-proxying the canvas element,
// keeping rendering entirely off the reactivity graph.
const mapCanvas = shallowRef<HTMLCanvasElement | null>(null)

const featureCount = computed(() => props.data?.features.length ?? 0)

function drawBackground(ctx: CanvasRenderingContext2D, w: number, h: number) {
  ctx.fillStyle = '#0d1117'
  ctx.fillRect(0, 0, w, h)
  // Subtle grid lines
  ctx.strokeStyle = '#1a2030'
  ctx.lineWidth = 0.5
  for (let x = 0; x < w; x += 40) {
    ctx.beginPath(); ctx.moveTo(x, 0); ctx.lineTo(x, h); ctx.stroke()
  }
  for (let y = 0; y < h; y += 40) {
    ctx.beginPath(); ctx.moveTo(0, y); ctx.lineTo(w, y); ctx.stroke()
  }
}

function lonLatToXY(lon: number, lat: number, w: number, h: number): [number, number] {
  return [
    ((lon + 180) / 360) * w,
    ((90 - lat) / 180) * h,
  ]
}

function renderData(canvas: HTMLCanvasElement) {
  const ctx = canvas.getContext('2d')
  if (!ctx) return
  const { width: w, height: h } = canvas
  drawBackground(ctx, w, h)

  if (!props.data) return

  for (const feature of props.data.features) {
    const g = feature.geometry
    if (g.type === 'Point') {
      const [lon, lat] = g.coordinates as [number, number]
      const [x, y] = lonLatToXY(lon, lat, w, h)
      ctx.fillStyle = '#3a8fd4'
      ctx.beginPath()
      ctx.arc(x, y, 3, 0, Math.PI * 2)
      ctx.fill()
    } else if (g.type === 'LineString') {
      const coords = g.coordinates as [number, number][]
      ctx.strokeStyle = '#5ab4e5'
      ctx.lineWidth = 1.5
      ctx.beginPath()
      coords.forEach(([lon, lat], i) => {
        const [x, y] = lonLatToXY(lon, lat, w, h)
        i === 0 ? ctx.moveTo(x, y) : ctx.lineTo(x, y)
      })
      ctx.stroke()
    } else if (g.type === 'Polygon') {
      const rings = g.coordinates as [number, number][][]
      ctx.strokeStyle = '#8fd43a'
      ctx.fillStyle = 'rgba(143, 212, 58, 0.15)'
      ctx.lineWidth = 1
      for (const ring of rings) {
        ctx.beginPath()
        ring.forEach(([lon, lat], i) => {
          const [x, y] = lonLatToXY(lon, lat, w, h)
          i === 0 ? ctx.moveTo(x, y) : ctx.lineTo(x, y)
        })
        ctx.closePath()
        ctx.fill()
        ctx.stroke()
      }
    }
  }
}

function syncSize() {
  const canvas = mapCanvas.value
  if (!canvas) return
  canvas.width = canvas.offsetWidth
  canvas.height = canvas.offsetHeight
}

onMounted(() => {
  syncSize()
  const canvas = mapCanvas.value
  if (!canvas) return
  const ctx = canvas.getContext('2d')
  if (ctx) drawBackground(ctx, canvas.width, canvas.height)

  window.addEventListener('resize', () => {
    syncSize()
    renderData(canvas)
  })
})

watch(() => props.data, () => {
  if (mapCanvas.value) renderData(mapCanvas.value)
})
</script>

<style scoped>
.globe-wrap {
  position: relative;
  height: 100%;
  background-color: #0d1117;
  overflow: hidden;
}
.map-canvas {
  width: 100%;
  height: 100%;
  display: block;
  cursor: crosshair;
}
.overlay {
  position: absolute;
  top: 12px;
  left: 12px;
  background: rgba(15, 20, 25, 0.85);
  border: 1px solid #2a3040;
  border-radius: 4px;
  padding: 8px 12px;
  pointer-events: none;
}
.hint { color: #5a6a7a; font-size: 0.85rem; }
.info { color: #3a8fd4; font-size: 0.85rem; font-weight: 600; }
</style>
