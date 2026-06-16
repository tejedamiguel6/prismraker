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

	// The U1 reports one cumulative filament counter for the whole print
	// (print_stats.filament_used), not per-extruder. We attribute its deltas to
	// whichever tool is active — that's the multi-color fix.
	lastTotalPos float64
	totalPosSet  bool
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

// ObserveTotalFilament feeds the printer's single cumulative filament counter
// (print_stats.filament_used) and attributes the delta to the active tool. This
// is the U1-correct model: there is no per-extruder position, so usage is
// charged to whichever toolhead is selected when the counter advances.
func (l *Ledger) ObserveTotalFilament(cumulativeMM float64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.totalPosSet {
		if d := cumulativeMM - l.lastTotalPos; d > 0 {
			if t := l.toolheads[l.activeTool]; t != nil {
				t.UsedMM += d
			}
		}
	}
	l.lastTotalPos = cumulativeMM
	l.totalPosSet = true
	l.updatedAt = time.Now()
}

// SetTemp updates live temperature for a toolhead. nil leaves a field unchanged,
// which matters because Moonraker only streams temperature when it changes — a
// delta frame for an extruder often carries other fields but no temperature.
func (l *Ledger) SetTemp(index int, temp, target *float64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if t := l.toolheads[index]; t != nil {
		if temp != nil {
			t.Temperature = *temp
		}
		if target != nil {
			t.Target = *target
		}
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
