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
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	led := ledger.New(*toolheads)
	srv := api.New(led)

	mc := moonraker.New(*wsURL)
	mc.OnNotification("notify_status_update", func(params json.RawMessage) {
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
	// TODO: confirm the U1 fork's object/field names for the active tool and
	// per-extruder filament position (Snapmaker reworked ~15% of Moonraker).
	objects := map[string]any{
		"extruder":     nil,
		"extruder1":    nil,
		"extruder2":    nil,
		"extruder3":    nil,
		"toolhead":     nil,
		"print_stats":  nil,
	}
	if _, err := mc.SubscribeStatus(ctx, objects); err != nil {
		log.Fatalf("subscribe: %v", err)
	}
	log.Printf("subscribed to %d toolheads on %s", *toolheads, *wsURL)

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

// applyStatus maps a Moonraker status update onto the per-toolhead ledger.
//
// extruder -> T0, extruder1 -> T1, etc. TODO: verify this mapping and the
// active-tool signal against the U1 fork.
func applyStatus(led *ledger.Ledger, su moonraker.StatusUpdate) {
	for i, key := range []string{"extruder", "extruder1", "extruder2", "extruder3"} {
		raw, ok := su.Objects[key]
		if !ok {
			continue
		}
		var ex moonraker.Extruder
		if err := json.Unmarshal(raw, &ex); err != nil {
			continue
		}
		led.SetTemp(i, ex.Temperature, ex.Target)
		if ex.Position != 0 {
			led.ObservePosition(i, ex.Position)
		}
	}
	// TODO: read the active tool from the U1's toolhead/tool object and call
	// led.SetActive(idx) so usage is attributed correctly across color changes.
}
