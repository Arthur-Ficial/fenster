// Package bridge is the IPC seam between fenster's supervisor process and
// the NM-host child Chrome spawns when an extension calls connectNative.
//
// Wire format: 4-byte little-endian length prefix + UTF-8 JSON, identical
// to Chrome's Native Messaging framing. This means the NM-host child can
// stream bytes verbatim between Chrome's stdio and our Unix socket — no
// re-framing required.
//
// Topology:
//
//	[ HTTP client ]
//	      ↓
//	[ supervisor (this process) ] ←→ [ Unix socket ] ←→ [ nm-host child ] ←→ [ Chrome stdio ] ←→ [ extension SW ] ←→ [ LanguageModel ]
//
// The supervisor sends Frame{type:"chat",...} into the socket and reads back
// Frame{type:"chunk"...} stream + Frame{type:"done"}. Multiple concurrent
// requests are demultiplexed by Frame.ID via Router.
package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/Arthur-Ficial/fenster/internal/nm"
)

// Frame is one envelope flowing both ways.
type Frame struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// Conn wraps a net.Conn with framed JSON I/O.
type Conn struct {
	c net.Conn
	mu sync.Mutex
}

// Read reads one frame.
func (c *Conn) Read() (Frame, error) {
	raw, err := nm.Read(c.c, 16*1024*1024)
	if err != nil {
		return Frame{}, err
	}
	var f Frame
	if err := json.Unmarshal(raw, &f); err != nil {
		return Frame{}, fmt.Errorf("bridge: bad json: %w", err)
	}
	return f, nil
}

// Write writes one frame.
func (c *Conn) Write(f Frame) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	b, err := json.Marshal(f)
	if err != nil {
		return err
	}
	return nm.Write(c.c, b)
}

// ReadRaw reads one length-prefixed payload without JSON-decoding it.
// Used by nm-host child to forward Chrome stdio bytes verbatim.
func (c *Conn) ReadRaw() ([]byte, error) {
	return nm.Read(c.c, 16*1024*1024)
}

// WriteRaw writes a pre-encoded payload as a single frame.
func (c *Conn) WriteRaw(b []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return nm.Write(c.c, b)
}

// Close closes the underlying connection.
func (c *Conn) Close() error { return c.c.Close() }

// ----- Server (supervisor side) -----

// Server listens on a Unix socket for a single nm-host child to connect.
type Server struct {
	sock     string
	listener net.Listener
	mu       sync.Mutex
}

// NewServer returns an unstarted Server.
func NewServer() *Server { return &Server{} }

// Listen binds on the given socket path. Removes any stale socket file.
func (s *Server) Listen(path string) error {
	_ = os.Remove(path)
	if err := os.MkdirAll(filepathDir(path), 0o755); err != nil {
		return err
	}
	l, err := net.Listen("unix", path)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.listener = l
	s.sock = path
	s.mu.Unlock()
	return nil
}

// Accept waits for a single connection.
func (s *Server) Accept() (*Conn, error) {
	s.mu.Lock()
	l := s.listener
	s.mu.Unlock()
	if l == nil {
		return nil, errors.New("bridge: server not listening")
	}
	c, err := l.Accept()
	if err != nil {
		return nil, err
	}
	return &Conn{c: c}, nil
}

// Close stops listening and removes the socket file.
func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		_ = s.listener.Close()
		s.listener = nil
	}
	if s.sock != "" {
		_ = os.Remove(s.sock)
		s.sock = ""
	}
	return nil
}

// ----- Client (nm-host child side) -----

// Dial connects to a supervisor socket with a timeout.
func Dial(path string, timeout time.Duration) (*Conn, error) {
	deadline := time.Now().Add(timeout)
	for {
		c, err := net.Dial("unix", path)
		if err == nil {
			return &Conn{c: c}, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("bridge: dial %s: %w", path, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// ----- Router -----

// Router demultiplexes incoming frames by ID into per-request channels.
type Router struct {
	mu      sync.Mutex
	pending map[string]chan Frame
}

// NewRouter returns a fresh Router.
func NewRouter() *Router { return &Router{pending: map[string]chan Frame{}} }

// Pending registers a channel for the given request id and returns it.
// The caller waits on this channel for chunk/done frames.
func (r *Router) Pending(id string) <-chan Frame {
	r.mu.Lock()
	defer r.mu.Unlock()
	ch, ok := r.pending[id]
	if !ok {
		ch = make(chan Frame, 16)
		r.pending[id] = ch
	}
	return ch
}

// Deliver pushes a frame to the registered channel for its ID. If no channel
// is registered, the frame is dropped (caller is gone).
func (r *Router) Deliver(f Frame) {
	r.mu.Lock()
	ch, ok := r.pending[f.ID]
	r.mu.Unlock()
	if !ok {
		return
	}
	select {
	case ch <- f:
	default:
		// Channel full — drop. Reasonable for slow consumers; the
		// alternative is a deadlock.
	}
}

// Forget releases the channel for the given id. Subsequent Deliver calls
// for the same id are dropped.
func (r *Router) Forget(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if ch, ok := r.pending[id]; ok {
		close(ch)
		delete(r.pending, id)
	}
}

// ----- Helpers -----

// filepathDir is filepath.Dir but with no platform import.
func filepathDir(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' || p[i] == '\\' {
			return p[:i]
		}
	}
	return "."
}

// AsyncCopy reads from src and forwards every frame to dst. Returns when
// either side EOFs or errors.
func AsyncCopy(ctx context.Context, src, dst *Conn, onFrame func(Frame)) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		f, err := src.Read()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if onFrame != nil {
			onFrame(f)
		}
		if err := dst.Write(f); err != nil {
			return err
		}
	}
}
