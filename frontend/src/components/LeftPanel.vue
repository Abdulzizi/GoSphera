<template>
  <aside class="left-panel">
    <header class="panel-header">
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" width="16" height="16">
        <circle cx="11" cy="11" r="8" /><line x1="21" y1="21" x2="16.65" y2="16.65" />
      </svg>
      <span class="panel-title">Query Builder</span>
    </header>

    <div class="panel-body">
      <!-- Tag builder -->
      <section class="section">
        <p class="section-label">OSM Tags</p>
        <div class="tag-list">
          <div v-for="(tag, i) in tags" :key="i" class="tag-row">
            <input v-model="tag.key"   class="tag-input mono" placeholder="key"   />
            <input v-model="tag.value" class="tag-input mono" placeholder="value" />
            <button class="btn-icon btn-remove" title="Remove" @click="removeTag(i)">✕</button>
          </div>
        </div>
        <button class="btn-secondary btn-full" @click="addTag">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="13" height="13">
            <circle cx="12" cy="12" r="10" /><line x1="12" y1="8" x2="12" y2="16" /><line x1="8" y1="12" x2="16" y2="12" />
          </svg>
          Add Tag
        </button>
      </section>

      <!-- Bounding box -->
      <section class="section">
        <p class="section-label">Bounding Box</p>

        <!-- City presets -->
        <div class="preset-row">
          <button
            v-for="(preset, name) in presets"
            :key="name"
            class="preset-btn"
            @click="applyPreset(preset)"
          >{{ name }}</button>
        </div>

        <div class="bbox-grid">
          <label class="bbox-field">
            <span>Min Lat</span>
            <input v-model.number="bbox.minLat" type="number" step="0.01" class="bbox-input" />
          </label>
          <label class="bbox-field">
            <span>Min Lon</span>
            <input v-model.number="bbox.minLon" type="number" step="0.01" class="bbox-input" />
          </label>
          <label class="bbox-field">
            <span>Max Lat</span>
            <input v-model.number="bbox.maxLat" type="number" step="0.01" class="bbox-input" />
          </label>
          <label class="bbox-field">
            <span>Max Lon</span>
            <input v-model.number="bbox.maxLon" type="number" step="0.01" class="bbox-input" />
          </label>
        </div>
      </section>

      <button class="btn-primary btn-full" :disabled="isLoading" @click="submitQuery">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="15" height="15">
          <circle cx="11" cy="11" r="8" /><line x1="21" y1="21" x2="16.65" y2="16.65" />
        </svg>
        <span v-if="isLoading">Querying…</span>
        <span v-else>Query</span>
      </button>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { ref } from 'vue'

interface Tag  { key: string; value: string }
interface BBox { minLat: number; minLon: number; maxLat: number; maxLon: number }

const props = defineProps<{ isLoading: boolean }>()
const emit  = defineEmits<{ query: [params: { tags: Tag[]; bbox: BBox }] }>()

const tags = ref<Tag[]>([{ key: 'amenity', value: 'cafe' }])
const bbox = ref<BBox>({ minLat: 25.7, minLon: -80.5, maxLat: 26.0, maxLon: -80.0 })

const presets: Record<string, BBox> = {
  Miami:   { minLat: 25.7,  minLon: -80.5, maxLat: 26.0,  maxLon: -80.0 },
  Jakarta: { minLat: -6.4,  minLon: 106.7, maxLat: -6.1,  maxLon: 107.0 },
  London:  { minLat: 51.4,  minLon: -0.3,  maxLat: 51.6,  maxLon: 0.1   },
}

function applyPreset(preset: BBox) { bbox.value = { ...preset } }

function addTag()            { tags.value.push({ key: '', value: '' }) }
function removeTag(i: number){ tags.value.splice(i, 1) }

function submitQuery() {
  if (props.isLoading) return
  emit('query', {
    tags: tags.value.filter(t => t.key.trim() && t.value.trim()),
    bbox: bbox.value,
  })
}
</script>

<style scoped>
.left-panel {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: #131920;
  border-right: 1px solid #222b35;
  overflow: hidden;
}

.panel-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 14px 16px;
  border-bottom: 1px solid #222b35;
  flex-shrink: 0;
  color: #6a9ab8;
}

.panel-title { font-size: 0.95rem; font-weight: 600; color: #c0ccd8; letter-spacing: 0.02em; }
.panel-body  { flex: 1; overflow-y: auto; padding: 14px 16px; display: flex; flex-direction: column; gap: 20px; }

.section       { display: flex; flex-direction: column; gap: 8px; }
.section-label { font-size: 0.72rem; font-weight: 700; text-transform: uppercase; letter-spacing: 0.08em; color: #4a6a7a; }

.tag-list { display: flex; flex-direction: column; gap: 6px; }
.tag-row  { display: flex; gap: 6px; align-items: center; }

.tag-input {
  flex: 1; min-width: 0; padding: 6px 8px;
  background: #0d1117; border: 1px solid #2a3040;
  border-radius: 4px; color: #d0dae4; font-size: 0.83rem;
}
.tag-input:focus { outline: none; border-color: #3a8fd4; }
.mono { font-family: 'SF Mono', 'Consolas', monospace; }

.btn-icon   { padding: 5px 8px; border: none; border-radius: 4px; cursor: pointer; font-size: 0.8rem; }
.btn-remove { background: #3a1a1a; color: #e07070; }
.btn-remove:hover { background: #4a2a2a; }

.btn-full { width: 100%; }

.btn-secondary {
  display: flex; align-items: center; justify-content: center; gap: 6px;
  padding: 7px; background: #1a2535; border: 1px solid #2a3a4a; border-radius: 4px;
  color: #8ab0c8; font-size: 0.83rem; cursor: pointer;
}
.btn-secondary:hover { background: #243040; }

.btn-primary {
  display: flex; align-items: center; justify-content: center; gap: 7px;
  padding: 10px; background: #1e5a9e; border: none; border-radius: 4px;
  color: #fff; font-size: 0.9rem; font-weight: 600; cursor: pointer; margin-top: auto;
  transition: background 0.15s;
}
.btn-primary:hover:not(:disabled) { background: #2a6aae; }
.btn-primary:active:not(:disabled){ background: #154a8e; }
.btn-primary:disabled { background: #1a3a5a; color: #6a8aa8; cursor: default; }

/* Preset chips */
.preset-row { display: flex; gap: 6px; flex-wrap: wrap; }
.preset-btn {
  padding: 4px 10px; background: #0d1117; border: 1px solid #2a3a4a;
  border-radius: 12px; color: #6a9ab8; font-size: 0.76rem; cursor: pointer;
  transition: background 0.12s, color 0.12s;
}
.preset-btn:hover { background: #1a2a3a; color: #3a8fd4; border-color: #3a5a7a; }

/* Single-column bbox so labels never clip */
.bbox-grid  { display: grid; grid-template-columns: 1fr; gap: 8px; }
.bbox-field { display: flex; flex-direction: column; gap: 3px; }
.bbox-field span { font-size: 0.73rem; color: #5a7a8a; }

.bbox-input {
  padding: 6px 8px; background: #0d1117; border: 1px solid #2a3040;
  border-radius: 4px; color: #d0dae4; font-size: 0.83rem;
}
.bbox-input:focus { outline: none; border-color: #3a8fd4; }
</style>
