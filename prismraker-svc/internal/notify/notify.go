// Package notify emits color-aware events (low spool, color change, fault) to
// external sinks. The scaffold ships a webhook sink; ntfy/Discord/Telegram are
// thin wrappers over the same Event shape.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type Kind string

const (
	KindLowSpool    Kind = "low_spool"
	KindColorChange Kind = "color_change"
	KindToolFault   Kind = "tool_fault"
)

type Event struct {
	Kind      Kind      `json:"kind"`
	Toolhead  int       `json:"toolhead"`
	ColorName string    `json:"colorName"`
	Message   string    `json:"message"`
	Time      time.Time `json:"time"`
}

// Sink delivers events somewhere.
type Sink interface {
	Send(ctx context.Context, e Event) error
}

// WebhookSink POSTs the event as JSON to a URL. ntfy and Discord both accept
// this with minor body tweaks.
type WebhookSink struct {
	URL    string
	Client *http.Client
}

func NewWebhookSink(url string) *WebhookSink {
	return &WebhookSink{URL: url, Client: &http.Client{Timeout: 10 * time.Second}}
}

func (w *WebhookSink) Send(ctx context.Context, e Event) error {
	body, err := json.Marshal(e)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := w.Client.Do(req)
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

// LogSink prints events to stdout — handy for development.
type LogSink struct{}

func (LogSink) Send(_ context.Context, e Event) error {
	log.Printf("[notify] %s T%d %s: %s", e.Kind, e.Toolhead, e.ColorName, e.Message)
	return nil
}
