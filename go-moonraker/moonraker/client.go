// Package moonraker is a small client for the Moonraker API
// (https://moonraker.readthedocs.io). It speaks the WebSocket JSON-RPC 2.0
// protocol, which is required to receive asynchronous status notifications.
//
// There is currently no widely-used Go client for Moonraker; this package aims
// to be a reusable foundation. It is intentionally dependency-light: gorilla
// for the websocket transport, stdlib for everything else.
package moonraker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// Client is a connection to a Moonraker host. It is safe for concurrent use.
type Client struct {
	url string

	mu      sync.Mutex
	conn    *websocket.Conn
	nextID  int64
	pending map[int64]chan rpcResponse

	// notifyHandlers are invoked for server-initiated JSON-RPC notifications
	// (method calls with no id), e.g. "notify_status_update".
	handlersMu sync.RWMutex
	handlers   map[string][]NotificationHandler

	closed atomic.Bool
}

// NotificationHandler receives the params of a JSON-RPC notification.
type NotificationHandler func(params json.RawMessage)

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
	ID      int64  `json:"id,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result"`
	Error   *rpcError       `json:"error"`
	ID      int64           `json:"id"`
	// Method/Params are populated for notifications (no id).
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string { return fmt.Sprintf("moonraker rpc error %d: %s", e.Code, e.Message) }

// New creates a client for a Moonraker websocket URL,
// e.g. "ws://printer.local:7125/websocket".
func New(wsURL string) *Client {
	return &Client{
		url:      wsURL,
		pending:  make(map[int64]chan rpcResponse),
		handlers: make(map[string][]NotificationHandler),
	}
}

// Connect dials the host and starts the read loop. Call Close to stop.
func (c *Client) Connect(ctx context.Context) error {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.url, nil)
	if err != nil {
		return fmt.Errorf("dial moonraker: %w", err)
	}
	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	go c.readLoop()
	return nil
}

// OnNotification registers a handler for a JSON-RPC notification method,
// e.g. "notify_status_update" or "notify_gcode_response".
func (c *Client) OnNotification(method string, h NotificationHandler) {
	c.handlersMu.Lock()
	defer c.handlersMu.Unlock()
	c.handlers[method] = append(c.handlers[method], h)
}

// Call performs a JSON-RPC request and unmarshals the result into out.
func (c *Client) Call(ctx context.Context, method string, params any, out any) error {
	id := atomic.AddInt64(&c.nextID, 1)
	ch := make(chan rpcResponse, 1)

	c.mu.Lock()
	c.pending[id] = ch
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return fmt.Errorf("not connected")
	}

	req := rpcRequest{JSONRPC: "2.0", Method: method, Params: params, ID: id}
	if err := conn.WriteJSON(req); err != nil {
		c.clearPending(id)
		return fmt.Errorf("write %s: %w", method, err)
	}

	select {
	case <-ctx.Done():
		c.clearPending(id)
		return ctx.Err()
	case resp := <-ch:
		if resp.Error != nil {
			return resp.Error
		}
		if out != nil && len(resp.Result) > 0 {
			return json.Unmarshal(resp.Result, out)
		}
		return nil
	}
}

// SubscribeStatus subscribes to printer object status updates. The objects map
// follows Moonraker's printer.objects.subscribe schema, where a nil value means
// "all fields". Updates arrive via the "notify_status_update" notification.
//
// For the U1's four toolheads you'll typically subscribe to objects like
// "extruder", "extruder1", "extruder2", "extruder3", "toolhead", and
// "print_stats". Inspect the actual fields the U1 fork exposes before relying
// on them — Snapmaker reworked ~15% of Moonraker for the multi-printhead system.
func (c *Client) SubscribeStatus(ctx context.Context, objects map[string]any) (json.RawMessage, error) {
	var result json.RawMessage
	err := c.Call(ctx, "printer.objects.subscribe", map[string]any{"objects": objects}, &result)
	return result, err
}

func (c *Client) readLoop() {
	for {
		c.mu.Lock()
		conn := c.conn
		c.mu.Unlock()
		if conn == nil {
			return
		}

		var resp rpcResponse
		if err := conn.ReadJSON(&resp); err != nil {
			if c.closed.Load() {
				return
			}
			// Connection dropped. A production build should reconnect with
			// backoff here; the scaffold simply stops the loop.
			return
		}

		if resp.ID != 0 {
			// Response to one of our calls.
			c.mu.Lock()
			ch, ok := c.pending[resp.ID]
			delete(c.pending, resp.ID)
			c.mu.Unlock()
			if ok {
				ch <- resp
			}
			continue
		}

		// Server notification (no id).
		if resp.Method != "" {
			c.handlersMu.RLock()
			hs := c.handlers[resp.Method]
			c.handlersMu.RUnlock()
			for _, h := range hs {
				h(resp.Params)
			}
		}
	}
}

func (c *Client) clearPending(id int64) {
	c.mu.Lock()
	delete(c.pending, id)
	c.mu.Unlock()
}

// Close shuts down the connection.
func (c *Client) Close() error {
	c.closed.Store(true)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil
	}
	_ = c.conn.WriteControl(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		time.Now().Add(time.Second))
	err := c.conn.Close()
	c.conn = nil
	return err
}
