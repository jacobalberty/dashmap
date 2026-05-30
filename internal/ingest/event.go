package ingest

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// eventJSON holds the fields we care about from event.json.  All values are
// strings in the actual files; unknown extra fields (e.g. "street") are stored
// only in the raw bytes.
type eventJSON struct {
	Timestamp string `json:"timestamp"`
	City      string `json:"city"`
	EstLat    string `json:"est_lat"`
	EstLon    string `json:"est_lon"`
	Reason    string `json:"reason"`
	Camera    string `json:"camera"`
}

// parsedEvent is the result of parsing one event.json.
type parsedEvent struct {
	EventTS time.Time
	City    string
	EstLat  *float64
	EstLon  *float64
	Reason  string
	Camera  string
	Raw     []byte // verbatim bytes for events.raw JSONB
}

// parseEventJSON parses the bytes of one event.json file.
func parseEventJSON(data []byte) (*parsedEvent, error) {
	var ej eventJSON
	if err := json.Unmarshal(data, &ej); err != nil {
		return nil, fmt.Errorf("event.json unmarshal: %w", err)
	}

	var eventTS time.Time
	if ej.Timestamp != "" {
		t, err := time.Parse("2006-01-02T15:04:05", ej.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("event.json timestamp %q: %w", ej.Timestamp, err)
		}
		// Tesla timestamps are naive wall-clock — treat as UTC.
		eventTS = time.Date(t.Year(), t.Month(), t.Day(),
			t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
	}

	pe := &parsedEvent{
		EventTS: eventTS,
		City:    ej.City,
		Reason:  ej.Reason,
		Camera:  ej.Camera,
		Raw:     data,
	}
	if v, err := strconv.ParseFloat(ej.EstLat, 64); err == nil {
		pe.EstLat = &v
	}
	if v, err := strconv.ParseFloat(ej.EstLon, 64); err == nil {
		pe.EstLon = &v
	}
	return pe, nil
}
