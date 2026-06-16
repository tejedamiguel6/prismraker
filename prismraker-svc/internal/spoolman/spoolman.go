// Package spoolman is a small client for the Spoolman REST API
// (https://github.com/Donkie/Spoolman). Prismraker reads spool color/name/weight
// from it so each toolhead card shows the real filament, and (later, M3) will
// decrement the correct spool on a multi-color print.
package spoolman

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Client talks to a Spoolman server, e.g. New("http://localhost:7912").
type Client struct {
	base string
	http *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		base: strings.TrimRight(baseURL, "/"),
		http: &http.Client{Timeout: 10 * time.Second},
	}
}

// Spool is the subset of Spoolman's spool object Prismraker uses. See
// /api/v1/spool/{id}; many more fields exist.
type Spool struct {
	ID              int      `json:"id"`
	RemainingWeight float64  `json:"remaining_weight"` // grams
	Filament        Filament `json:"filament"`
}

type Filament struct {
	Name     string `json:"name"`      // e.g. "Galaxy Black"
	Material string `json:"material"`  // e.g. "PLA"
	ColorHex string `json:"color_hex"` // e.g. "1A1A1A" (no leading #)
	Vendor   Vendor `json:"vendor"`
}

type Vendor struct {
	Name string `json:"name"` // e.g. "Polymaker"
}

// DisplayName is a human label for the spool, falling back gracefully.
func (s Spool) DisplayName() string {
	parts := make([]string, 0, 2)
	if s.Filament.Vendor.Name != "" {
		parts = append(parts, s.Filament.Vendor.Name)
	}
	if s.Filament.Name != "" {
		parts = append(parts, s.Filament.Name)
	}
	if len(parts) == 0 {
		return fmt.Sprintf("Spool #%d", s.ID)
	}
	return strings.Join(parts, " ")
}

// ColorHexCSS returns the color as a CSS hex ("#1A1A1A"), or "" if unknown.
func (s Spool) ColorHexCSS() string {
	h := strings.TrimPrefix(s.Filament.ColorHex, "#")
	if h == "" {
		return ""
	}
	return "#" + h
}

// Spool fetches a single spool by id.
func (c *Client) Spool(ctx context.Context, id int) (*Spool, error) {
	url := fmt.Sprintf("%s/api/v1/spool/%d", c.base, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get spool %d: %w", id, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get spool %d: status %d", id, resp.StatusCode)
	}
	var s Spool
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return nil, fmt.Errorf("decode spool %d: %w", id, err)
	}
	return &s, nil
}
