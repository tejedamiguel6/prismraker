// Package spoolsvc owns runtime spool state: the catalog of spools the user can
// choose from (live from Spoolman, or a demo set when offline), which spool is
// assigned to each toolhead, and the background loop that decrements the right
// spool as filament is consumed.
//
// This is what makes the dashboard interactive: the UI lists Options and calls
// Assign when the user clicks a toolhead — no more -spools CLI flag.
package spoolsvc

import (
	"context"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/yourname/prismraker-svc/internal/ledger"
	"github.com/yourname/prismraker-svc/internal/spoolman"
)

// Option is one spool the user can load onto a toolhead.
type Option struct {
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	ColorHex      string  `json:"colorHex"`
	RemainingGram float64 `json:"remainingGram"`
}

// Service coordinates spool assignment and (when Spoolman is live) usage
// reconciliation. It is safe for concurrent use.
type Service struct {
	led    *ledger.Ledger
	client *spoolman.Client // nil => offline/demo mode
	sync   bool             // actually decrement Spoolman vs. dry-run log

	mu       sync.Mutex
	demo     []Option        // catalog when client == nil
	mapping  map[int]int     // toolhead index -> spool id
	reported map[int]float64 // mm already charged to Spoolman per toolhead
}

// New builds a service. Pass client=nil for offline mode, in which case demo is
// used as the catalog so the picker still works without a Spoolman server.
func New(led *ledger.Ledger, client *spoolman.Client, sync bool, demo []Option) *Service {
	return &Service{
		led:      led,
		client:   client,
		sync:     sync,
		demo:     demo,
		mapping:  map[int]int{},
		reported: map[int]float64{},
	}
}

// List returns the spools the user can pick from, sorted by id.
func (s *Service) List(ctx context.Context) ([]Option, error) {
	if s.client == nil {
		s.mu.Lock()
		out := append([]Option(nil), s.demo...)
		s.mu.Unlock()
		sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
		return out, nil
	}
	spools, err := s.client.Spools(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Option, 0, len(spools))
	for _, sp := range spools {
		out = append(out, Option{
			ID:            sp.ID,
			Name:          sp.DisplayName(),
			ColorHex:      sp.ColorHexCSS(),
			RemainingGram: sp.RemainingWeight,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// Assign loads the spool with spoolID onto a toolhead. spoolID == 0 clears it.
// Future consumption (not past usage) is charged to the new spool.
func (s *Service) Assign(ctx context.Context, toolhead, spoolID int) error {
	if spoolID == 0 {
		return s.Clear(toolhead)
	}

	var opt Option
	if s.client != nil {
		sp, err := s.client.Spool(ctx, spoolID)
		if err != nil {
			return err
		}
		opt = Option{ID: sp.ID, Name: sp.DisplayName(), ColorHex: sp.ColorHexCSS(), RemainingGram: sp.RemainingWeight}
	} else {
		o, ok := s.demoByID(spoolID)
		if !ok {
			return errUnknownSpool(spoolID)
		}
		opt = o
	}

	s.led.AssignSpool(toolhead, opt.ID, opt.ColorHex, opt.Name, opt.RemainingGram)

	s.mu.Lock()
	s.mapping[toolhead] = spoolID
	// Charge only filament used from now on to the freshly-loaded spool.
	s.reported[toolhead] = usedMM(s.led, toolhead)
	s.mu.Unlock()
	return nil
}

// Clear unassigns a toolhead's spool.
func (s *Service) Clear(toolhead int) error {
	s.led.AssignSpool(toolhead, 0, "", "", 0)
	s.mu.Lock()
	delete(s.mapping, toolhead)
	delete(s.reported, toolhead)
	s.mu.Unlock()
	return nil
}

// Run reconciles usage and refreshes remaining weights every 30s until ctx is
// cancelled. It's a no-op loop when Spoolman isn't configured. onChange is
// called after each refresh so the API can broadcast to the UI.
func (s *Service) Run(ctx context.Context, onChange func()) {
	if s.client == nil {
		return // nothing to reconcile in demo mode
	}
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			s.reconcile(ctx)
			s.refresh(ctx)
			onChange()
		}
	}
}

// minReportMM avoids a flurry of tiny Spoolman writes.
const minReportMM = 1.0

func (s *Service) reconcile(ctx context.Context) {
	for _, t := range s.led.Snapshot() {
		s.mu.Lock()
		id, ok := s.mapping[t.Index]
		already := s.reported[t.Index]
		s.mu.Unlock()
		if !ok {
			continue
		}
		delta := t.UsedMM - already
		if delta < minReportMM {
			continue
		}
		if s.sync {
			if err := s.client.UseLength(ctx, id, delta); err != nil {
				log.Printf("spoolman: T%d spool %d use: %v", t.Index, id, err)
				continue // leave reported unchanged so we retry next tick
			}
			log.Printf("spoolman: T%d spool %d -= %.1fmm", t.Index, id, delta)
		} else {
			log.Printf("spoolman: [dry-run] T%d spool %d would -= %.1fmm (pass -spoolman-sync to apply)", t.Index, id, delta)
		}
		s.mu.Lock()
		s.reported[t.Index] = t.UsedMM
		s.mu.Unlock()
	}
}

func (s *Service) refresh(ctx context.Context) {
	s.mu.Lock()
	snapshot := make(map[int]int, len(s.mapping))
	for idx, id := range s.mapping {
		snapshot[idx] = id
	}
	s.mu.Unlock()

	for idx, id := range snapshot {
		sp, err := s.client.Spool(ctx, id)
		if err != nil {
			log.Printf("spoolman: refresh T%d spool %d: %v", idx, id, err)
			continue
		}
		s.led.AssignSpool(idx, sp.ID, sp.ColorHexCSS(), sp.DisplayName(), sp.RemainingWeight)
	}
}

func (s *Service) demoByID(id int) (Option, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, o := range s.demo {
		if o.ID == id {
			return o, true
		}
	}
	return Option{}, false
}

// usedMM returns a toolhead's current cumulative usage from the ledger.
func usedMM(led *ledger.Ledger, toolhead int) float64 {
	for _, t := range led.Snapshot() {
		if t.Index == toolhead {
			return t.UsedMM
		}
	}
	return 0
}

type errUnknownSpool int

func (e errUnknownSpool) Error() string { return "unknown spool id" }
