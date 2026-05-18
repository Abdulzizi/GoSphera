<template>
  <aside class="right-panel">
    <header class="panel-header">
      <span class="panel-title">Telemetry / FIRMS</span>
      <button class="btn-refresh" title="Refresh" @click="emit('refresh')">↻</button>
    </header>

    <div class="panel-body">
      <div v-if="events.length === 0" class="empty">
        <p>No data — waiting for FIRMS feed</p>
      </div>

      <div v-else class="event-list">
        <div
          v-for="(ev, i) in events"
          :key="i"
          :class="['event-card', isAnomaly(ev) && 'anomaly']"
        >
          <div class="card-header">
            <span class="card-id">{{ cardId(ev) }}</span>
            <span v-if="isAnomaly(ev)" class="anomaly-badge">ANOMALY</span>
          </div>
          <dl class="card-body">
            <template v-if="'latitude' in ev">
              <dt>Lat / Lon</dt>
              <dd>{{ (ev as FireRecord).latitude.toFixed(4) }}, {{ (ev as FireRecord).longitude.toFixed(4) }}</dd>
              <dt>Brightness</dt>
              <dd>{{ (ev as FireRecord).brightness.toFixed(1) }} K</dd>
              <dt>Confidence</dt>
              <dd>{{ (ev as FireRecord).confidence }}</dd>
              <dt>Date</dt>
              <dd>{{ (ev as FireRecord).acq_date }}</dd>
            </template>
            <template v-else>
              <dt>ID</dt>
              <dd>{{ (ev as TelemetryEvent).id }}</dd>
              <dt>Lat / Lon</dt>
              <dd>{{ (ev as TelemetryEvent).lat.toFixed(4) }}, {{ (ev as TelemetryEvent).lon.toFixed(4) }}</dd>
              <dt>Time</dt>
              <dd>{{ formatTs((ev as TelemetryEvent).timestamp) }}</dd>
            </template>
          </dl>
        </div>
      </div>
    </div>
  </aside>
</template>

<script setup lang="ts">
import type { FireRecord, TelemetryEvent } from '../services/api'

type AnyEvent = FireRecord | TelemetryEvent

const props = defineProps<{ events: AnyEvent[] }>()
const emit = defineEmits<{ refresh: [] }>()

function isAnomaly(ev: AnyEvent): boolean {
  if ('properties' in ev) {
    return Boolean((ev as TelemetryEvent).properties?.anomaly)
  }
  return false
}

function cardId(ev: AnyEvent): string {
  if ('id' in ev) return (ev as TelemetryEvent).id
  const fr = ev as FireRecord
  return `${fr.latitude.toFixed(2)}, ${fr.longitude.toFixed(2)}`
}

function formatTs(ts: string): string {
  try { return new Date(ts).toLocaleString() } catch { return ts }
}
</script>

<style scoped>
.right-panel {
  display: flex;
  flex-direction: column;
  height: 100%;
  background: #131920;
  border-left: 1px solid #222b35;
  overflow: hidden;
}
.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 14px 16px;
  border-bottom: 1px solid #222b35;
  flex-shrink: 0;
}
.panel-title { font-size: 0.95rem; font-weight: 600; color: #c0ccd8; }
.btn-refresh {
  padding: 4px 10px; background: #1a2535; border: 1px solid #2a3a4a;
  border-radius: 4px; color: #8ab0c8; cursor: pointer; font-size: 1rem;
}
.btn-refresh:hover { background: #243040; }

.panel-body { flex: 1; overflow-y: auto; padding: 12px; }
.empty { display: flex; align-items: center; justify-content: center; height: 120px; color: #4a6070; font-size: 0.85rem; text-align: center; }

.event-list { display: flex; flex-direction: column; gap: 8px; }
.event-card {
  background: #0d1117;
  border: 1px solid #222b35;
  border-radius: 6px;
  padding: 10px;
}
.event-card.anomaly {
  border-color: #7a1a1a;
  background: rgba(120, 25, 25, 0.12);
}
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 6px;
}
.card-id { font-size: 0.8rem; font-weight: 600; color: #3a8fd4; font-family: monospace; }
.anomaly-badge {
  font-size: 0.68rem;
  font-weight: 700;
  background: #8a1010;
  color: #ffaaaa;
  padding: 2px 6px;
  border-radius: 3px;
  letter-spacing: 0.05em;
}
.card-body {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 2px 10px;
  font-size: 0.78rem;
}
.card-body dt { color: #5a7a8a; }
.card-body dd { color: #b0c4d4; font-family: monospace; }
</style>
