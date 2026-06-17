package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// capture appends raw Moonraker notification frames to a JSONL file, one frame
// per line. This is the validation tool from docs/ROADMAP.md: capture real
// notify_status_update payloads from a live U1 so the per-extruder object/field
// names and the active-tool signal can be confirmed before trusting accounting.
type capture struct {
	mu sync.Mutex
	f  *os.File
	n  int
}

// captureFrame is one recorded line: a timestamp, the notification method, and
// the verbatim params Moonraker sent.
type captureFrame struct {
	Time   string          `json:"time"`
	Seq    int             `json:"seq"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

func newCapture(path string) (*capture, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create capture file: %w", err)
	}
	return &capture{f: f}, nil
}

// record writes one frame. Safe for concurrent use.
func (c *capture) record(method string, params json.RawMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n++
	line, err := json.Marshal(captureFrame{
		Time:   time.Now().Format(time.RFC3339Nano),
		Seq:    c.n,
		Method: method,
		Params: params,
	})
	if err != nil {
		return
	}
	if _, err := c.f.Write(append(line, '\n')); err != nil {
		log.Printf("[capture] write: %v", err)
	}
}

func (c *capture) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	log.Printf("[capture] wrote %d frames to %s", c.n, c.f.Name())
	return c.f.Close()
}
