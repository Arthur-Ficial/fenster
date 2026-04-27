package backend

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Arthur-Ficial/fenster/internal/bridge"
	"github.com/Arthur-Ficial/fenster/internal/core/ids"
	"github.com/Arthur-Ficial/fenster/internal/core/tokens"
	"github.com/Arthur-Ficial/fenster/internal/core/wire"
)

// ChromeBackend talks to the Chrome extension via the bridge socket.
// The supervisor accepts an nm-host child connection; this backend writes
// chat frames into that channel and reads chunks/done back.
type ChromeBackend struct {
	server *bridge.Server
	router *bridge.Router

	connMu sync.Mutex
	conn   *bridge.Conn // currently-connected nm-host relay
}

// NewChromeBackend listens on the bridge socket and waits for the nm-host
// child to connect. It does NOT block — Listen returns immediately; the
// first Chat() call waits for a relay to appear.
func NewChromeBackend(socketPath string) (*ChromeBackend, error) {
	srv := bridge.NewServer()
	if err := srv.Listen(socketPath); err != nil {
		return nil, fmt.Errorf("chrome backend: listen %s: %w", socketPath, err)
	}
	cb := &ChromeBackend{server: srv, router: bridge.NewRouter()}
	go cb.acceptLoop()
	return cb, nil
}

func (b *ChromeBackend) acceptLoop() {
	for {
		conn, err := b.server.Accept()
		if err != nil {
			return
		}
		b.connMu.Lock()
		b.conn = conn
		b.connMu.Unlock()
		go b.readLoop(conn)
	}
}

func (b *ChromeBackend) readLoop(conn *bridge.Conn) {
	for {
		f, err := conn.Read()
		if err != nil {
			b.connMu.Lock()
			if b.conn == conn {
				b.conn = nil
			}
			b.connMu.Unlock()
			return
		}
		b.router.Deliver(f)
	}
}

func (b *ChromeBackend) currentConn() *bridge.Conn {
	b.connMu.Lock()
	defer b.connMu.Unlock()
	return b.conn
}

// Health returns Available iff a relay is connected.
func (b *ChromeBackend) Health(ctx context.Context) (Health, error) {
	if b.currentConn() == nil {
		return Health{
			Available: false,
			Detail:    "Chrome extension not connected. Install the extension at chrome://extensions/ → Load unpacked → ~/.fenster/extension, then reload Chrome.",
			ContextWindow: wire.ContextWindow,
		}, nil
	}
	return Health{Available: true, ContextWindow: wire.ContextWindow, SupportedLanguages: wire.SupportedLanguagesFallback()}, nil
}

// Chat sends a chat frame and aggregates chunks until done.
func (b *ChromeBackend) Chat(ctx context.Context, req *wire.ChatCompletionRequest) (Result, error) {
	ch, cancel, err := b.send(ctx, req)
	if err != nil {
		return Result{}, err
	}
	defer cancel()

	var content string
	res := Result{}
	timeout := time.After(60 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return Result{}, ctx.Err()
		case <-timeout:
			return Result{}, errors.New("chrome backend: timeout waiting for response")
		case f, ok := <-ch:
			if !ok {
				goto done
			}
			switch f.Type {
			case "chunk":
				var p struct {
					Delta string `json:"delta"`
				}
				_ = json.Unmarshal(f.Payload, &p)
				content += p.Delta
			case "done":
				goto done
			case "error":
				var p struct {
					Message string `json:"message"`
				}
				_ = json.Unmarshal(f.Payload, &p)
				return Result{}, fmt.Errorf("chrome: %s", p.Message)
			}
		}
	}
done:
	res.Content = content
	res.FinishReason = wire.FinishStop
	res.Usage = tokens.Usage{Prompt: tokens.Estimate(promptText(req)), Completion: tokens.Estimate(content)}
	return res, nil
}

// ChatStream streams chunks back via a Chunk channel.
func (b *ChromeBackend) ChatStream(ctx context.Context, req *wire.ChatCompletionRequest) (<-chan Chunk, error) {
	frames, cancel, err := b.send(ctx, req)
	if err != nil {
		return nil, err
	}
	out := make(chan Chunk, 16)
	go func() {
		defer close(out)
		defer cancel()
		var total string
		for {
			select {
			case <-ctx.Done():
				return
			case f, ok := <-frames:
				if !ok {
					return
				}
				switch f.Type {
				case "chunk":
					var p struct {
						Delta string `json:"delta"`
					}
					_ = json.Unmarshal(f.Payload, &p)
					total += p.Delta
					out <- Chunk{ContentDelta: p.Delta}
				case "done":
					out <- Chunk{
						FinishReason: wire.FinishStop,
						Usage: &tokens.Usage{
							Prompt:     tokens.Estimate(promptText(req)),
							Completion: tokens.Estimate(total),
						},
					}
					return
				case "error":
					var p struct {
						Message string `json:"message"`
					}
					_ = json.Unmarshal(f.Payload, &p)
					out <- Chunk{Err: fmt.Errorf("chrome: %s", p.Message)}
					return
				}
			}
		}
	}()
	return out, nil
}

// Close shuts the bridge socket.
func (b *ChromeBackend) Close() error {
	if b.server != nil {
		_ = b.server.Close()
	}
	return nil
}

// send builds a chat frame, registers a router channel, writes it to the
// connected relay, and returns the receive channel + a cancel func.
func (b *ChromeBackend) send(ctx context.Context, req *wire.ChatCompletionRequest) (<-chan bridge.Frame, func(), error) {
	conn := b.currentConn()
	if conn == nil {
		return nil, func() {}, errors.New("chrome backend: extension not connected")
	}
	id := ids.ChatCompletion()
	ch := b.router.Pending(id)
	cancel := func() { b.router.Forget(id) }

	payload, err := json.Marshal(map[string]any{
		"messages":        req.Messages,
		"stream":          req.IsStream(),
		"temperature":     req.Temperature,
		"response_format": req.ResponseFormat,
	})
	if err != nil {
		cancel()
		return nil, func() {}, err
	}
	if err := conn.Write(bridge.Frame{ID: id, Type: "chat", Payload: payload}); err != nil {
		cancel()
		return nil, func() {}, err
	}
	return ch, cancel, nil
}

func promptText(req *wire.ChatCompletionRequest) string {
	var s string
	for _, m := range req.Messages {
		s += m.Role + ": " + m.Content.AsString() + "\n"
	}
	return s
}

// Compile-time interface check.
var _ Backend = (*ChromeBackend)(nil)
