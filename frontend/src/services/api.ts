export interface Geometry {
  type: 'Point' | 'LineString' | 'Polygon'
  coordinates: unknown
}

export interface Feature {
  type: 'Feature'
  geometry: Geometry
  properties: Record<string, unknown>
}

export interface FeatureCollection {
  type: 'FeatureCollection'
  features: Feature[]
}

export interface FireRecord {
  latitude: number
  longitude: number
  brightness: number
  acq_date: string
  confidence: string
}

export interface AircraftState {
  icao24: string
  callsign: string
  country: string
  lat: number
  lon: number
  altitude: number   // metres
  velocity: number   // m/s
  heading: number    // degrees clockwise from north
  on_ground: boolean
  last_contact: number
}

export interface TelemetryEvent {
  id: string
  timestamp: string
  lat: number
  lon: number
  properties: Record<string, unknown>
}

const API_BASE = '/api'

async function apiFetch<T>(input: RequestInfo, init?: RequestInit): Promise<T> {
  const res = await fetch(input, init)
  if (!res.ok) {
    const text = await res.text().catch(() => res.statusText)
    throw new Error(`API error ${res.status}: ${text}`)
  }
  return res.json() as Promise<T>
}

export function getSpatialData(tags: string, bbox: string): Promise<FeatureCollection> {
  const params = new URLSearchParams({ tags, bbox })
  return apiFetch<FeatureCollection>(`${API_BASE}/spatial?${params}`)
}

export async function getSituationalData(): Promise<{ records: FireRecord[]; total: number }> {
  const res = await fetch(`${API_BASE}/situational`) // no limit param → backend returns all
  if (!res.ok) {
    const text = await res.text().catch(() => res.statusText)
    throw new Error(`API error ${res.status}: ${text}`)
  }
  const records = (await res.json()) as FireRecord[]
  const total = parseInt(res.headers.get('X-Total-Count') ?? '0', 10)
  return { records, total }
}

export function getAircraftData(): Promise<AircraftState[]> {
  return apiFetch<AircraftState[]>(`${API_BASE}/aircraft`)
}

export interface ShipState {
  mmsi: number
  name: string
  lat: number
  lon: number
  speed: number    // knots
  heading: number  // degrees
  ship_type: number
  last_seen: number // unix seconds
}

export interface CameraInfo {
  id: string
  name: string
  city: string
  lat: number
  lon: number
  snapshot_url: string
}

export function getShipData(): Promise<ShipState[]> {
  return apiFetch<ShipState[]>(`${API_BASE}/ships`)
}

export function getCameraData(): Promise<CameraInfo[]> {
  return apiFetch<CameraInfo[]>(`${API_BASE}/cameras`)
}

export function postTelemetry(event: TelemetryEvent): Promise<void> {
  return apiFetch<void>(`${API_BASE}/telemetry`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(event),
  })
}

/**
 * Opens a Server-Sent Events connection to /api/events with auto-reconnect.
 * Returns a cleanup function — call it to permanently close the stream (e.g. onUnmounted).
 */
export function openEventStream(
  onEvent: (event: TelemetryEvent) => void,
  onError?: (err: Event) => void,
  onOpen?: () => void,
): () => void {
  let es: EventSource | null = null
  let retryMs = 1_000
  let stopped = false

  function connect() {
    es = new EventSource(`${API_BASE}/events`)

    es.onopen = () => {
      retryMs = 1_000  // reset backoff on successful open
      onOpen?.()
    }

    es.onmessage = (e: MessageEvent) => {
      try {
        onEvent(JSON.parse(e.data) as TelemetryEvent)
      } catch {
        // silently skip malformed events
      }
    }

    es.onerror = (err: Event) => {
      onError?.(err)
      es?.close()
      if (!stopped) {
        setTimeout(connect, retryMs)
        retryMs = Math.min(retryMs * 2, 30_000)  // cap backoff at 30s
      }
    }
  }

  connect()
  return () => {
    stopped = true
    es?.close()
  }
}
