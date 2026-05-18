<template>
  <nav class="top-nav">
    <div class="nav-left">
      <svg class="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
        <circle cx="12" cy="12" r="10" />
        <ellipse cx="12" cy="12" rx="4" ry="10" />
        <line x1="2" y1="12" x2="22" y2="12" />
        <line x1="12" y1="2" x2="12" y2="22" />
      </svg>
      <span class="nav-wordmark">GoSphera</span>
    </div>

    <div class="nav-center">
      <span class="stat-chip">
        <span class="dot dot-orange" />
        <template v-if="firmsCount === 0">FIRMS: loading…</template>
        <template v-else-if="firmsFetched < firmsCount">
          FIRMS: {{ firmsFetched.toLocaleString() }} of {{ firmsCount.toLocaleString() }}
        </template>
        <template v-else>FIRMS: {{ firmsCount.toLocaleString() }} records</template>
      </span>
      <span :class="['stat-chip', sseConnected ? 'chip-green' : 'chip-red']">
        <span :class="['dot', sseConnected ? 'dot-green dot-pulse' : 'dot-red']" />
        SSE: {{ sseConnected ? 'connected' : 'disconnected' }}
      </span>
    </div>

    <div class="nav-right">
      <button class="nav-btn" title="Toggle map style (coming soon)">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" width="16" height="16">
          <rect x="3" y="3" width="7" height="7" rx="1" />
          <rect x="14" y="3" width="7" height="7" rx="1" />
          <rect x="3" y="14" width="7" height="7" rx="1" />
          <rect x="14" y="14" width="7" height="7" rx="1" />
        </svg>
        Map Style
      </button>
    </div>
  </nav>
</template>

<script setup lang="ts">
defineProps<{
  firmsCount: number
  firmsFetched: number
  sseConnected: boolean
}>()
</script>

<style scoped>
.top-nav {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 16px;
  background: #0d1117;
  border-bottom: 1px solid #1e2a35;
  height: 48px;
  flex-shrink: 0;
  z-index: 100;
}

.nav-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.nav-icon {
  width: 22px;
  height: 22px;
  color: #3a8fd4;
  flex-shrink: 0;
}

.nav-wordmark {
  font-size: 1rem;
  font-weight: 700;
  color: #e0eaf4;
  letter-spacing: 0.04em;
}

.nav-center {
  display: flex;
  align-items: center;
  gap: 10px;
}

.stat-chip {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  background: #131920;
  border: 1px solid #1e2a35;
  border-radius: 20px;
  font-size: 0.78rem;
  color: #8ab0c8;
  white-space: nowrap;
}

.chip-green { border-color: #1a4a2a; }
.chip-red   { border-color: #4a1a1a; }

.dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
}

.dot-orange { background: #f97316; }
.dot-green  { background: #22c55e; }
.dot-red    { background: #ef4444; }

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.4; }
}
.dot-pulse { animation: pulse 2s ease-in-out infinite; }

.nav-right {
  display: flex;
  align-items: center;
}

.nav-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 5px 12px;
  background: #131920;
  border: 1px solid #1e2a35;
  border-radius: 6px;
  color: #8ab0c8;
  font-size: 0.8rem;
  cursor: pointer;
  transition: background 0.15s;
}
.nav-btn:hover { background: #1a2535; }
</style>
