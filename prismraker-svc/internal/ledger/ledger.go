// Package ledger tracks filament usage independently for each toolhead and
// attributes it to the correct spool — the thing the stock Fluidd/Mainsail +
// Spoolman stack gets wrong on multi-color prints (fluidd #1269, mainsail #1927).
package ledger

import (
	"sync"
	"time"
)

// Toolhead is the live, normalized state of a single U1 printhead.
type Toolhead struct {
	Index       int     `json:"index"`
	Name        string  `json:"name"`         // e.g. "T0"
	ColorHex    string  `json:"colorHex"`     // swatch, e.g. "#1A1A1A"
	ColorName   string  `json:"colorName"`    // e.g. "Galaxy Black"
	SpoolID     int     `json:"spoolId"`      // Spoolman spool id, 0 = none
	Temperature float64 `json:"temperature"`
	Target      float64 `json:"target"`
	Active      bool    `json:"active"` // currently selected tool

	UsedMM        float64 `json:"usedMm"`        // filament consumed this print
	RemainingGram float64 `json:"remainingGram"` // from Spoolman, if known

	lastPos    float64 // last cumulative extruder position seen
	lastPosSet bool
}

// Ledger holds state for all toolheads and the active tool.
type Ledger struct {
	mu         sync.RWMutex
	toolheads  map[int]*Toolhead
	activeTool int
	updatedAt  time.Time
}

func New(count int) *Ledger {
	l := &Ledger{toolheads: make(map[int]*Toolhead, count)}
	for i := 0; i < count; i++ {
		name := "T" + string(rune('0'+i))
		l.toolheads[i] = &Toolhead{Index: i, Name: name}
	}
	return l
}

// SetActive records which tool is currently printing. Usage deltas are
// attributed to whichever tool is active when the extruder advances.
func (l *Ledger) SetActive(index int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i, t := range l.toolheads {
		t.Active = i == index
	}
	l.activeTool = index
	l.updatedAt = time.Now()
}

// ObservePosition feeds a new cumulative extruder position for a toolhead and
// accumulates the delta as used filament. Negative deltas (retraction/reset)
// are ignored for accounting.
func (l *Ledger) ObservePosition(index int, cumulativeMM float64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	t := l.toolheads[index]
	if t == nil {
		return
	}
	if t.lastPosSet {
		if d := cumulativeMM - t.lastPos; d > 0 {
			t.UsedMM += d
		}
	}
	t.lastPos = cumulativeMM
	t.lastPosSet = true
	l.updatedAt = time.Now()
}

// SetTemp updates live temperature for a toolhead.
func (l *Ledger) SetTemp(index int, temp, target float64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if t := l.toolheads[index]; t != nil {
		t.Temperature, t.Target = temp, target
	}
}

// AssignSpool binds a Spoolman spool (with color metadata) to a toolhead.
func (l *Ledger) AssignSpool(index, spoolID int, colorHex, colorName string, remainingGram float64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if t := l.toolheads[index]; t != nil {
		t.SpoolID = spoolID
		t.ColorHex = colorHex
		t.ColorName = colorName
		t.RemainingGram = remainingGram
	}
}

// Snapshot returns a copy of all toolhead state for the API layer.
func (l *Ledger) Snapshot() []Toolhead {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]Toolhead, 0, len(l.toolheads))
	for i := 0; i < len(l.toolheads); i++ {
		if t := l.toolheads[i]; t != nil {
			out = append(out, *t)
		}
	}
	return out
}
