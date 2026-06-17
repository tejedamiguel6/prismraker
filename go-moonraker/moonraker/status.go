package moonraker

import "encoding/json"

// StatusUpdate is the payload of a "notify_status_update" notification.
// Moonraker sends it as a two-element array: [ {objects...}, eventtime ].
//
//	[
//	  { "extruder": {"temperature": 215.0}, "toolhead": {"position": [...]} },
//	  123456.78
//	]
type StatusUpdate struct {
	Objects   map[string]json.RawMessage
	EventTime float64
}

// ParseStatusUpdate decodes the params of a notify_status_update notification.
func ParseStatusUpdate(params json.RawMessage) (StatusUpdate, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal(params, &raw); err != nil {
		return StatusUpdate{}, err
	}
	su := StatusUpdate{Objects: map[string]json.RawMessage{}}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw[0], &su.Objects); err != nil {
			return StatusUpdate{}, err
		}
	}
	if len(raw) > 1 {
		_ = json.Unmarshal(raw[1], &su.EventTime)
	}
	return su, nil
}

// Extruder holds the fields we care about from an extruder object. Add fields
// here as you discover what the U1 fork actually exposes.
//
// Note: the U1 has no per-extruder cumulative filament position; usage comes
// from the single print_stats.filament_used counter (see ledger.ObserveTotalFilament).
type Extruder struct {
	Temperature float64 `json:"temperature"`
	Target      float64 `json:"target"`
}
