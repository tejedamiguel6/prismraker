package main

import (
	"context"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/yourname/prismraker-svc/internal/ledger"
	"github.com/yourname/prismraker-svc/internal/spoolman"
)

// parseSpoolMap parses a "-spools" value like "1,2,,4" into a toolhead-index ->
// spool-id map. Position i maps to toolhead Ti; blank entries are skipped so you
// can leave a toolhead unassigned.
func parseSpoolMap(csv string) map[int]int {
	m := map[int]int{}
	for i, field := range strings.Split(csv, ",") {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		id, err := strconv.Atoi(field)
		if err != nil {
			log.Printf("spoolman: ignoring invalid spool id %q for T%d", field, i)
			continue
		}
		m[i] = id
	}
	return m
}

// refreshSpools fetches each mapped spool from Spoolman and assigns its color,
// name, and remaining weight to the toolhead. Errors are logged per-spool so one
// missing spool doesn't blank the others.
func refreshSpools(ctx context.Context, led *ledger.Ledger, sc *spoolman.Client, mapping map[int]int) {
	for idx, id := range mapping {
		s, err := sc.Spool(ctx, id)
		if err != nil {
			log.Printf("spoolman: T%d spool %d: %v", idx, id, err)
			continue
		}
		led.AssignSpool(idx, s.ID, s.ColorHexCSS(), s.DisplayName(), s.RemainingWeight)
	}
}

// runSpoolman wires Spoolman into the ledger: an immediate fetch, then a periodic
// refresh so remaining-weight stays current. Blocks until ctx is cancelled.
func runSpoolman(ctx context.Context, led *ledger.Ledger, url, spools string, onChange func()) {
	sc := spoolman.New(url)
	mapping := parseSpoolMap(spools)
	if len(mapping) == 0 {
		log.Printf("spoolman: no -spools mapping given; nothing to fetch")
		return
	}
	refreshSpools(ctx, led, sc, mapping)
	onChange()

	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			refreshSpools(ctx, led, sc, mapping)
			onChange()
		}
	}
}

// assignDemoSpools fills the toolheads with canned colored spools so the
// dashboard shows real swatches in -mock mode without a Spoolman server.
func assignDemoSpools(led *ledger.Ledger) {
	demo := []struct {
		hex, name string
		grams     float64
	}{
		{"#1c1c20", "Polymaker Galaxy Black", 642},
		{"#d7263d", "Polymaker Signal Red", 318},
		{"#1aa7c4", "Polymaker Cyan", 41},
		{"#ece5d8", "Polymaker Bone White", 770},
	}
	for i, d := range demo {
		led.AssignSpool(i, 1000+i, d.hex, d.name, d.grams)
	}
}
