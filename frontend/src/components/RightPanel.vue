<template>
  <aside class="right-panel">
    <header class="panel-header">
      <div class="tab-bar">
        <button
          :class="['tab', activeTab === 'fire' && 'tab-active']"
          @click="activeTab = 'fire'"
        >
          Fire Data
          <span v-if="fireRecords.length" class="tab-count">{{ fireRecords.length.toLocaleString() }}</span>
        </button>
        <button
          :class="['tab', activeTab === 'live' && 'tab-active']"
          @click="activeTab = 'live'"
        >
          Live Events
          <span v-if="anomalyCount" class="tab-badge">{{ anomalyCount }}</span>
        </button>
      </div>
      <button class="btn-refresh" title="Refresh FIRMS" @click="emit('refresh')">↻</button>
    </header>

    <div class="panel-body">
      <!-- FIRE DATA TAB -->
      <template v-if="activeTab === 'fire'">
        <div v-if="fireRecords.length === 0" class="empty">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.2" width="36" height="36" class="empty-icon">
            <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2z" />
            <path d="M12 6v6l4 2" />
          </svg>
          <p>Waiting for FIRMS feed…</p>
        </div>
        <div v-else>
          <div class="summary-line">
            {{ fireRecords.length.toLocaleString() }} records
          </div>
          <div class="event-list">
            <div v-for="(r, i) in fireRecords" :key="i" class="event-card fire-card">
              <div class="card-header">
                <span class="card-id">{{ r.latitude.toFixed(3) }}, {{ r.longitude.toFixed(3) }}</span>
                <span :class="['conf-dot', confidenceClass(r.confidence)]" :title="r.confidence" />
              </div>
              <dl class="card-body">
                <dt>Brightness</dt>
                <dd>
                  {{ r.brightness.toFixed(1) }} K
                  <span class="brightness-bar">
                    <span class="brightness-fill" :style="{ width: brightnessWidth(r.brightness) }" />
                  </span>
                </dd>
                <dt>Confidence</dt>
                <dd :class="'conf-' + confidenceClass(r.confidence)">{{ r.confidence }}</dd>
                <dt>Date</dt>
                <dd>{{ r.acq_date }}</dd>
              </dl>
            </div>
          </div>
        </div>
      </template>

      <!-- LIVE EVENTS TAB -->
      <template v-else>
        <div v-if="liveEvents.length === 0" class="empty">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.2" width="36" height="36" class="empty-icon">
            <path d="M5 12h14" /><path d="M12 5l7 7-7 7" />
            <path d="M1 1l22 22" stroke-dasharray="3 3" opacity="0.3" />
          </svg>
          <p>No live events yet</p>
          <p class="empty-sub">POST to /api/telemetry to send events</p>
        </div>
        <div v-else class="event-list">
          <div
            v-for="(ev, i) in liveEvents"
            :key="i"
            :class="['event-card', isAnomaly(ev) && 'anomaly']"
          >
            <div class="card-header">
              <span class="card-id">{{ ev.id }}</span>
              <span v-if="isAnomaly(ev)" class="anomaly-badge">
                <span class="anomaly-dot" />ANOMALY
              </span>
            </div>
            <dl class="card-body">
              <dt>Lat / Lon</dt>
              <dd>{{ ev.lat.toFixed(4) }}, {{ ev.lon.toFixed(4) }}</dd>
              <dt>Time</dt>
              <dd>{{ formatTs(ev.timestamp) }}</dd>
            </dl>
          </div>
        </div>
      </template>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import type { FireRecord, TelemetryEvent, AircraftState } from '../services/api'

const props = defineProps<{
  fireRecords: FireRecord[]
  liveEvents: TelemetryEvent[]
  aircraft: AircraftState[]
}>()
const emit = defineEmits<{ refresh: [] }>()

const activeTab = ref<'fire' | 'live'>('fire')

const anomalyCount = computed(() =>
  props.liveEvents.filter(ev => Boolean(ev.properties?.anomaly)).length,
)

function isAnomaly(ev: TelemetryEvent): boolean {
  return Boolean(ev.properties?.anomaly)
}

function formatTs(ts: string): string {
  try { return new Date(ts).toLocaleString() } catch { return ts }
}

function confidenceClass(conf: string): string {
  const c = conf.toLowerCase()
  if (c === 'high' || c === 'h') return 'high'
  if (c === 'nominal' || c === 'n') return 'nominal'
  return 'low'
}

function brightnessWidth(b: number): string {
  // VIIRS brightness typically 300–450 K; clamp to 0-100%
  const pct = Math.min(100, Math.max(0, ((b - 300) / 150) * 100))
  return `${pct}%`
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
  align-items: center;
  justify-content: space-between;
  padding: 0 12px 0 0;
  border-bottom: 1px solid #222b35;
  flex-shrink: 0;
}

.tab-bar { display: flex; }

.tab {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 14px 14px 12px;
  background: none;
  border: none;
  border-bottom: 2px solid transparent;
  color: #5a7a8a;
  font-size: 0.83rem;
  font-weight: 500;
  cursor: pointer;
  transition: color 0.15s;
}
.tab:hover { color: #8ab0c8; }
.tab-active {
  color: #3a8fd4;
  border-bottom-color: #3a8fd4;
}

.tab-count {
  padding: 1px 5px;
  background: #1a2a3a;
  border-radius: 10px;
  font-size: 0.7rem;
  color: #6a9ab8;
}

.tab-badge {
  padding: 1px 5px;
  background: #7a1010;
  border-radius: 10px;
  font-size: 0.7rem;
  color: #ffaaaa;
}

.btn-refresh {
  padding: 4px 10px;
  background: #1a2535;
  border: 1px solid #2a3a4a;
  border-radius: 4px;
  color: #8ab0c8;
  cursor: pointer;
  font-size: 1rem;
}
.btn-refresh:hover { background: #243040; }

.panel-body { flex: 1; overflow-y: auto; padding: 12px; }

.summary-line {
  font-size: 0.75rem;
  color: #4a6a7a;
  margin-bottom: 10px;
  padding: 0 2px;
}

.empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 10px;
  min-height: 160px;
  color: #4a6070;
  font-size: 0.85rem;
  text-align: center;
}
.empty-icon { color: #2a4050; }
.empty-sub  { font-size: 0.75rem; color: #3a5060; }

.event-list { display: flex; flex-direction: column; gap: 8px; }

.event-card {
  background: #0d1117;
  border: 1px solid #222b35;
  border-radius: 6px;
  padding: 10px;
}
.event-card.anomaly {
  border-color: #7a1a1a;
  border-left: 3px solid #ef4444;
  background: rgba(120, 25, 25, 0.12);
}

/* Fire card */
.fire-card { border-left: 3px solid #f97316; }

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 6px;
}
.card-id {
  font-size: 0.78rem;
  font-weight: 600;
  color: #3a8fd4;
  font-family: monospace;
}

/* Confidence dot */
.conf-dot {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  flex-shrink: 0;
}
.conf-dot.high    { background: #22c55e; }
.conf-dot.nominal { background: #eab308; }
.conf-dot.low     { background: #f97316; }

/* Anomaly badge */
.anomaly-badge {
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 0.68rem;
  font-weight: 700;
  background: #8a1010;
  color: #ffaaaa;
  padding: 2px 7px;
  border-radius: 3px;
  letter-spacing: 0.05em;
}

@keyframes pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.5; transform: scale(1.3); }
}
.anomaly-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #ef4444;
  animation: pulse 1.2s ease-in-out infinite;
}

.card-body {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 3px 10px;
  font-size: 0.77rem;
}
.card-body dt { color: #5a7a8a; }
.card-body dd { color: #b0c4d4; font-family: monospace; display: flex; align-items: center; gap: 8px; }

/* Confidence text colour */
.conf-high    { color: #22c55e; }
.conf-nominal { color: #eab308; }
.conf-low     { color: #f97316; }

/* Brightness bar */
.brightness-bar {
  display: inline-block;
  width: 50px;
  height: 4px;
  background: #1a2535;
  border-radius: 2px;
  overflow: hidden;
  vertical-align: middle;
}
.brightness-fill {
  display: block;
  height: 100%;
  background: linear-gradient(to right, #f97316, #ef4444);
  border-radius: 2px;
  transition: width 0.3s;
}
</style>
