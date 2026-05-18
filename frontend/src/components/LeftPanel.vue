<template>
  <aside class="left-panel">
    <header class="panel-header">
      <span class="panel-title">Query Builder</span>
    </header>

    <div class="panel-body">
      <!-- Tag builder -->
      <section class="section">
        <p class="section-label">OSM Tags</p>
        <div class="tag-list">
          <div v-for="(tag, i) in tags" :key="i" class="tag-row">
            <input v-model="tag.key"   class="tag-input" placeholder="key"   />
            <input v-model="tag.value" class="tag-input" placeholder="value" />
            <button class="btn-icon btn-remove" title="Remove" @click="removeTag(i)">✕</button>
          </div>
        </div>
        <button class="btn-secondary btn-full" @click="addTag">+ Add Tag</button>
      </section>

      <!-- Bounding box -->
      <section class="section">
        <p class="section-label">Bounding Box</p>
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

      <button class="btn-primary btn-full" @click="submitQuery">Query</button>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { ref } from 'vue'

interface Tag { key: string; value: string }
interface BBox { minLat: number; minLon: number; maxLat: number; maxLon: number }

const emit = defineEmits<{
  query: [params: { tags: Tag[]; bbox: BBox }]
}>()

const tags = ref<Tag[]>([{ key: 'amenity', value: 'cafe' }])
const bbox = ref<BBox>({ minLat: 25.7, minLon: -80.5, maxLat: 26.0, maxLon: -80.0 })

function addTag() { tags.value.push({ key: '', value: '' }) }
function removeTag(i: number) { tags.value.splice(i, 1) }

function submitQuery() {
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
  padding: 14px 16px;
  border-bottom: 1px solid #222b35;
  flex-shrink: 0;
}
.panel-title { font-size: 0.95rem; font-weight: 600; color: #c0ccd8; letter-spacing: 0.02em; }
.panel-body { flex: 1; overflow-y: auto; padding: 14px 16px; display: flex; flex-direction: column; gap: 20px; }

.section { display: flex; flex-direction: column; gap: 8px; }
.section-label { font-size: 0.75rem; font-weight: 600; text-transform: uppercase; letter-spacing: 0.08em; color: #5a7a8a; }

.tag-list { display: flex; flex-direction: column; gap: 6px; }
.tag-row { display: flex; gap: 6px; align-items: center; }
.tag-input {
  flex: 1; padding: 6px 8px; background: #0d1117; border: 1px solid #2a3040;
  border-radius: 4px; color: #d0dae4; font-size: 0.85rem;
}
.tag-input:focus { outline: none; border-color: #3a8fd4; }

.btn-icon { padding: 5px 8px; border: none; border-radius: 4px; cursor: pointer; font-size: 0.8rem; }
.btn-remove { background: #3a1a1a; color: #e07070; }
.btn-remove:hover { background: #4a2a2a; }

.btn-full { width: 100%; }
.btn-secondary {
  padding: 7px; background: #1a2535; border: 1px solid #2a3a4a; border-radius: 4px;
  color: #8ab0c8; font-size: 0.85rem; cursor: pointer;
}
.btn-secondary:hover { background: #243040; }
.btn-primary {
  padding: 10px; background: #1e5a9e; border: none; border-radius: 4px;
  color: #ffffff; font-size: 0.9rem; font-weight: 600; cursor: pointer; margin-top: auto;
}
.btn-primary:hover { background: #2a6aae; }
.btn-primary:active { background: #154a8e; }

.bbox-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
.bbox-field { display: flex; flex-direction: column; gap: 3px; }
.bbox-field span { font-size: 0.75rem; color: #6a8a9a; }
.bbox-input {
  padding: 6px 8px; background: #0d1117; border: 1px solid #2a3040;
  border-radius: 4px; color: #d0dae4; font-size: 0.85rem;
}
.bbox-input:focus { outline: none; border-color: #3a8fd4; }
</style>
