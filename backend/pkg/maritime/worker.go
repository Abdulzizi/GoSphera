package maritime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const aisstreamURL = "wss://stream.aisstream.io/v0/stream"

// Worker connects to AISStream.io and caches incoming ship positions.
type Worker struct {
	cache  *Cache
	apiKey string
}

func NewWorker(cache *Cache) *Worker {
	return &Worker{
		cache:  cache,
		apiKey: os.Getenv("AISSTREAM_API_KEY"),
	}
}

// Start launches the WebSocket listener in a background goroutine.
// If AISSTREAM_API_KEY is not set the worker logs a warning and returns immediately.
func (w *Worker) Start(ctx context.Context) {
	if w.apiKey == "" {
		fmt.Println("[AISStream] AISSTREAM_API_KEY not set — ship tracking disabled")
		return
	}
	go w.runLoop(ctx)
}

func (w *Worker) runLoop(ctx context.Context) {
	retryMs := 1_000
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := w.connect(ctx); err != nil {
			fmt.Printf("[AISStream] disconnected: %v — retrying in %dms\n", err, retryMs)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(retryMs) * time.Millisecond):
		}
		if retryMs < 60_000 {
			retryMs *= 2
		}
	}
}

// subscribeMsg is sent to AISStream after connecting to request position reports.
// BoundingBoxes format: [ [ [minLat, minLon], [maxLat, maxLon] ], … ]
type subscribeMsg struct {
	APIKey             string         `json:"APIKey"`
	BoundingBoxes      [][2][2]float64 `json:"BoundingBoxes"`
	FilterMessageTypes []string        `json:"FilterMessageTypes"`
}

// aismsg is the envelope for every message AISStream sends.
type aismsg struct {
	MessageType string `json:"MessageType"`
	MetaData    struct {
		MMSI      int     `json:"MMSI"`
		ShipName  string  `json:"ShipName"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		TimeUTC   string  `json:"time_utc"`
	} `json:"MetaData"`
	Message struct {
		PositionReport *struct {
			TrueHeading int     `json:"TrueHeading"`
			Sog         float64 `json:"Sog"` // speed over ground, knots
			Cog         float64 `json:"Cog"` // course over ground
		} `json:"PositionReport"`
		ShipStaticData *struct {
			Type int `json:"Type"`
		} `json:"ShipStaticData"`
	} `json:"Message"`
}

func (w *Worker) connect(ctx context.Context) error {
	dialer := websocket.DefaultDialer
	conn, _, err := dialer.DialContext(ctx, aisstreamURL, nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.Close()

	sub := subscribeMsg{
		APIKey: w.apiKey,
		// Global bounding box: [[minLat, minLon], [maxLat, maxLon]]
		BoundingBoxes:      [][2][2]float64{{{-90, -180}, {90, 180}}},
		FilterMessageTypes: []string{"PositionReport"},
	}
	if err := conn.WriteJSON(sub); err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}

	fmt.Println("[AISStream] connected and subscribed")

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		_, raw, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}

		var msg aismsg
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		if msg.MessageType != "PositionReport" || msg.Message.PositionReport == nil {
			continue
		}

		pr := msg.Message.PositionReport
		lat := msg.MetaData.Latitude
		lon := msg.MetaData.Longitude
		if lat == 0 && lon == 0 {
			continue // no position fix
		}

		heading := float64(pr.TrueHeading)
		if pr.TrueHeading == 511 { // 511 = not available per AIS spec
			heading = pr.Cog
		}

		ts := time.Now().Unix()
		if msg.MetaData.TimeUTC != "" {
			if t, err := time.Parse("2006-01-02 15:04:05.999999999 +0000 UTC", msg.MetaData.TimeUTC); err == nil {
				ts = t.Unix()
			}
		}

		w.cache.Set(ShipState{
			MMSI:    msg.MetaData.MMSI,
			Name:    strings.TrimSpace(msg.MetaData.ShipName),
			Lat:     lat,
			Lon:     lon,
			Speed:   pr.Sog,
			Heading: heading,
			LastSeen: ts,
		})
	}
}
