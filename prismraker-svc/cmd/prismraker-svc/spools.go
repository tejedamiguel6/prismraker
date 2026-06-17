package main

import (
	"log"
	"strconv"
	"strings"

	"github.com/yourname/prismraker-svc/internal/spoolsvc"
)

// parseSpoolMap parses a "-spools" value like "1,2,,4" into a toolhead-index ->
// spool-id map. Position i maps to toolhead Ti; blank entries are skipped so you
// can leave a toolhead unassigned. This is now only an optional startup
// convenience — assignment normally happens by clicking a toolhead in the UI.
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

// demoSpools is the canned catalog used in -mock mode (no Spoolman server), so
// the spool picker and swatches work offline.
func demoSpools() []spoolsvc.Option {
	return []spoolsvc.Option{
		{ID: 1001, Name: "Polymaker Galaxy Black", ColorHex: "#1c1c20", RemainingGram: 642},
		{ID: 1002, Name: "Polymaker Signal Red", ColorHex: "#d7263d", RemainingGram: 318},
		{ID: 1003, Name: "Polymaker Cyan", ColorHex: "#1aa7c4", RemainingGram: 41},
		{ID: 1004, Name: "Polymaker Bone White", ColorHex: "#ece5d8", RemainingGram: 770},
		{ID: 1005, Name: "Polymaker Sunflower Yellow", ColorHex: "#f4c20d", RemainingGram: 510},
		{ID: 1006, Name: "Polymaker Forest Green", ColorHex: "#2e7d32", RemainingGram: 233},
	}
}
