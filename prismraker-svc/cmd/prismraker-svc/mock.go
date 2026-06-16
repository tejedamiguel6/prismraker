package main

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/yourname/go-moonraker/moonraker"
)

// statusFeed is the slice of the Moonraker client that main depends on. Both the
// real *moonraker.Client and the mockClient below satisfy it, so -mock swaps the
// data source without touching the rest of the wiring.
type statusFeed interface {
	OnNotification(method string, h moonraker.NotificationHandler)
	Connect(ctx context.Context) error
	SubscribeStatus(ctx context.Context, objects map[string]any) (json.RawMessage, error)
	Close() error
}

// mockClient simulates a U1 running a multi-color print without any hardware. It
// emits notify_status_update notifications in the same two-element wire format
// the real machine uses, so ParseStatusUpdate and applyStatus run unchanged.
type mockClient struct {
	toolheads int
	interval  time.Duration

	mu       sync.Mutex
	handlers []moonraker.NotificationHandler

	// simulation state, indexed by toolhead.
	temp     []float64
	target   []float64
	position []float64
	active   int
}

func newMockClient(toolheads int) *mockClient {
	m := &mockClient{
		toolheads: toolheads,
		interval:  500 * time.Millisecond,
		temp:      make([]float64, toolheads),
		target:    make([]float64, toolheads),
		position:  make([]float64, toolheads),
	}
	for i := range m.temp {
		m.temp[i] = ambientTemp
	}
	return m
}

const (
	ambientTemp = 25.0
	printTemp   = 215.0
	// feedRate is mm of filament extruded per second on the active toolhead.
	feedRate = 2.5
	// toolDwell is how long a single color prints before the next tool takes over.
	toolDwell = 8 * time.Second
)

func (m *mockClient) OnNotification(_ string, h moonraker.NotificationHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers = append(m.handlers, h)
}

func (m *mockClient) Connect(ctx context.Context) error {
	go m.run(ctx)
	return nil
}

func (m *mockClient) SubscribeStatus(_ context.Context, _ map[string]any) (json.RawMessage, error) {
	return json.RawMessage(`{"eventtime":0}`), nil
}

func (m *mockClient) Close() error { return nil }

// run advances the simulation on a ticker and emits a status update each step,
// cycling the active toolhead to mimic color changes, until ctx is cancelled.
func (m *mockClient) run(ctx context.Context) {
	log.Printf("[mock] simulating %d toolheads, color change every %s", m.toolheads, toolDwell)
	tick := time.NewTicker(m.interval)
	defer tick.Stop()

	swap := time.NewTicker(toolDwell)
	defer swap.Stop()

	dt := m.interval.Seconds()
	for {
		select {
		case <-ctx.Done():
			return
		case <-swap.C:
			m.active = (m.active + 1) % m.toolheads
			log.Printf("[mock] color change -> T%d", m.active)
		case <-tick.C:
			m.step(dt)
			m.emit()
		}
	}
}

// step nudges temperatures toward their targets and advances filament position
// on the active toolhead. Inactive toolheads cool toward ambient.
func (m *mockClient) step(dt float64) {
	for i := 0; i < m.toolheads; i++ {
		if i == m.active {
			m.target[i] = printTemp
			m.position[i] += feedRate * dt
		} else {
			m.target[i] = 0
		}
		// First-order approach toward the target (or ambient when off).
		goal := m.target[i]
		if goal == 0 {
			goal = ambientTemp
		}
		m.temp[i] += (goal - m.temp[i]) * 0.2
	}
}

func (m *mockClient) emit() {
	keys := []string{"extruder", "extruder1", "extruder2", "extruder3"}
	objects := map[string]any{}
	for i := 0; i < m.toolheads && i < len(keys); i++ {
		objects[keys[i]] = map[string]any{
			"temperature": round1(m.temp[i]),
			"target":      m.target[i],
		}
	}

	// Mirror the real U1 frame shape: toolhead.extruder names the active tool,
	// and print_stats.filament_used is the single cumulative usage counter.
	var total float64
	for i := 0; i < m.toolheads; i++ {
		total += m.position[i]
	}
	objects["toolhead"] = map[string]any{"extruder": keys[m.active]}
	objects["print_stats"] = map[string]any{
		"filament_used": round1(total),
		"state":         "printing",
		"filename":      "mock_demo.gcode",
	}

	// Match Moonraker's wire format: [ {objects}, eventtime ].
	params, err := json.Marshal([]any{objects, 0.0})
	if err != nil {
		return
	}

	m.mu.Lock()
	handlers := append([]moonraker.NotificationHandler(nil), m.handlers...)
	m.mu.Unlock()
	for _, h := range handlers {
		h(params)
	}
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}
