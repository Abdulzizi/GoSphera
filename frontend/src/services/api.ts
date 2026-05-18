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

export function getSituationalData(): Promise<FireRecord[]> {
  return apiFetch<FireRecord[]>(`${API_BASE}/situational`)
}

export function postTelemetry(event: TelemetryEvent): Promise<void> {
  return apiFetch<void>(`${API_BASE}/telemetry`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(event),
  })
}

/**
 * Opens a Server-Sent Events connection to /api/events.
 * Returns a cleanup function — call it to close the stream (e.g. onUnmounted).
 */
export function openEventStream(
  onEvent: (event: TelemetryEvent) => void,
  onError?: (err: Event) => void,
): () => void {
  const es = new EventSource(`${API_BASE}/events`)

  es.onmessage = (e: MessageEvent) => {
    try {
      onEvent(JSON.parse(e.data) as TelemetryEvent)
    } catch {
      // silently skip malformed events — never crash the stream
    }
  }

  if (onError) {
    es.onerror = onError
  }

  return () => es.close()
}
