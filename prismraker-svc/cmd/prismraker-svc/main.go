// Command prismraker-svc watches a Snapmaker U1's Moonraker feed, tracks
// filament usage per-toolhead, and serves normalized color state to the
// prismraker-ui React dashboard.
//
// This is a runnable scaffold: it connects, subscribes, wires the ledger, and
// serves the API. The TODOs mark where U1-fork-specific field names must be
// confirmed against a live machine before the per-color accounting is trusted.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/yourname/go-moonraker/moonraker"
	"github.com/yourname/prismraker-svc/internal/api"
	"github.com/yourname/prismraker-svc/internal/ledger"
)

func main() {
	wsURL := flag.String("moonraker", "ws://printer.local:7125/websocket", "Moonraker websocket URL")
	listen := flag.String("listen", ":8420", "address for the prismraker API/UI server")
	toolheads := flag.Int("toolheads", 4, "number of toolheads (U1 = 4)")
	mock := flag.Bool("mock", false, "run without hardware, feeding a simulated multi-color print")
	capturePath := flag.String("capture", "", "if set, append raw Moonraker frames to this JSONL file for field validation")
	spoolmanURL := flag.String("spoolman", "", "Spoolman base URL for spool colors/weights, e.g. http://localhost:7912")
	spools := flag.String("spools", "", "comma-separated Spoolman spool ids per toolhead, e.g. 1,2,3,4 (blank = unassigned)")
	spoolmanSync := flag.Bool("spoolman-sync", false, "actually decrement Spoolman spools as filament is used (default: dry-run, log only)")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var cap *capture
	if *capturePath != "" {
		c, err := newCapture(*capturePath)
		if err != nil {
			log.Fatalf("capture: %v", err)
		}
		cap = c
		defer cap.Close()
		log.Printf("[capture] recording raw frames to %s", *capturePath)
	}

	led := ledger.New(*toolheads)
	srv := api.New(led)

	// Spool colors/names/weights from Spoolman, refreshed periodically. With no
	// Spoolman configured but -mock set, fall back to demo spools so the
	// dashboard still shows swatches.
	if *spoolmanURL != "" {
		mode := "dry-run"
		if *spoolmanSync {
			mode = "sync (decrements spools)"
		}
		log.Printf("spoolman: %s, toolheads -> spools [%s], %s", *spoolmanURL, *spools, mode)
		go runSpoolman(ctx, led, *spoolmanURL, *spools, *spoolmanSync, srv.Broadcast)
	} else if *mock {
		assignDemoSpools(led)
	}

	var mc statusFeed
	if *mock {
		log.Printf("[mock] no printer connection; using simulated data")
		mc = newMockClient(*toolheads)
	} else {
		mc = moonraker.New(*wsURL)
	}
	mc.OnNotification("notify_status_update", func(params json.RawMessage) {
		if cap != nil {
			cap.record("notify_status_update", params)
		}
		su, err := moonraker.ParseStatusUpdate(params)
		if err != nil {
			log.Printf("parse status update: %v", err)
			return
		}
		applyStatus(led, su)
		srv.Broadcast()
	})

	if err := mc.Connect(ctx); err != nil {
		log.Fatalf("connect moonraker: %v", err)
	}
	defer mc.Close()

	// Subscribe to the four extruders + toolhead + print_stats. nil = all fields.
	// Field names confirmed against a live U1 (Klipper fork 1.4.x): active tool is
	// toolhead.extruder; usage is print_stats.filament_used (one cumulative
	// counter, not per-extruder); temps are extruderN.temperature/target.
	objects := map[string]any{
		"extruder":    nil,
		"extruder1":   nil,
		"extruder2":   nil,
		"extruder3":   nil,
		"toolhead":    nil,
		"print_stats": nil,
	}
	snapshot, err := mc.SubscribeStatus(ctx, objects)
	if err != nil {
		log.Fatalf("subscribe: %v", err)
	}
	// Moonraker only streams fields that change, so steady-state temps never
	// arrive as deltas. Seed full state from the subscribe response, whose shape
	// is {"eventtime":N, "status": {objects...}}.
	if len(snapshot) > 0 {
		var snap struct {
			Status map[string]json.RawMessage `json:"status"`
		}
		if err := json.Unmarshal(snapshot, &snap); err == nil && snap.Status != nil {
			applyStatus(led, moonraker.StatusUpdate{Objects: snap.Status})
		}
	}
	source := *wsURL
	if *mock {
		source = "mock"
	}
	log.Printf("subscribed to %d toolheads on %s", *toolheads, source)

	go func() {
		log.Printf("prismraker API listening on %s", *listen)
		if err := http.ListenAndServe(*listen, srv.Routes()); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down")
	_ = os.Stdout.Sync()
}

// applyStatus maps a Moonraker status update onto the per-toolhead ledger,
// using the field layout confirmed on a live U1 (extruder -> T0, extruder1 -> T1…):
//   - active tool        = toolhead.extruder ("extruder1" -> index 1)
//   - per-tool temps      = extruderN.temperature / .target (only when changed)
//   - filament usage      = print_stats.filament_used (one cumulative counter,
//     attributed to the active tool)
//
// Frames carry only changed fields, so temperature/target are decoded as
// pointers and left untouched when absent.
func applyStatus(led *ledger.Ledger, su moonraker.StatusUpdate) {
	// Active tool first, so usage in this same frame is attributed correctly.
	if raw, ok := su.Objects["toolhead"]; ok {
		var th struct {
			Extruder string `json:"extruder"`
		}
		if json.Unmarshal(raw, &th) == nil && th.Extruder != "" {
			led.SetActive(extruderIndex(th.Extruder))
		}
	}

	for i, key := range []string{"extruder", "extruder1", "extruder2", "extruder3"} {
		raw, ok := su.Objects[key]
		if !ok {
			continue
		}
		var ex struct {
			Temperature *float64 `json:"temperature"`
			Target      *float64 `json:"target"`
		}
		if json.Unmarshal(raw, &ex) != nil {
			continue
		}
		if ex.Temperature != nil || ex.Target != nil {
			led.SetTemp(i, ex.Temperature, ex.Target)
		}
	}

	if raw, ok := su.Objects["print_stats"]; ok {
		var ps struct {
			FilamentUsed float64 `json:"filament_used"`
		}
		if json.Unmarshal(raw, &ps) == nil && ps.FilamentUsed > 0 {
			led.ObserveTotalFilament(ps.FilamentUsed)
		}
	}
}

// extruderIndex maps a Moonraker extruder name to a toolhead index:
// "extruder" -> 0, "extruder1" -> 1, "extruder2" -> 2, "extruder3" -> 3.
func extruderIndex(name string) int {
	switch name {
	case "extruder1":
		return 1
	case "extruder2":
		return 2
	case "extruder3":
		return 3
	default:
		return 0
	}
}
