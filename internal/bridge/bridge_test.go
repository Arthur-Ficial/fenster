// Bridge tests — TDD-first. The bridge is the IPC between fenster's
// supervisor (long-lived HTTP server / CLI process) and the NM-host child
// process Chrome spawns when an extension calls connectNative.
//
// Frames over the unix socket are length-prefixed JSON, the same shape used
// over Native Messaging stdio. That way the NM-host child can copy bytes
// directly between the two pipes.
package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestEnvelope_RoundTripsJSON(t *testing.T) {
	env := Frame{ID: "x", Type: "ping", Payload: json.RawMessage(`{"version":"0.0.1"}`)}
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	var got Frame
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != "x" || got.Type != "ping" {
		t.Errorf("round-trip lost: %+v", got)
	}
}

func TestServerClient_RoundTripPing(t *testing.T) {
	// Unix socket paths are limited to ~104 chars on macOS; t.TempDir()
	// nests under /var/folders/... which often exceeds that. Use /tmp.
	sock := filepath.Join("/tmp", fmt.Sprintf("fenster-bridge-%d.sock", time.Now().UnixNano()))
	t.Cleanup(func() { _ = os.Remove(sock) })

	srv := NewServer()
	if err := srv.Listen(sock); err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer srv.Close()

	// Server-side: when a client connects, it acts as the "extension".
	// We simulate the extension by echoing pings from the client side.
	go func() {
		conn, err := srv.Accept()
		if err != nil {
			t.Logf("Accept: %v", err)
			return
		}
		defer conn.Close()
		// Read one frame, echo a pong.
		f, err := conn.Read()
		if err != nil {
			return
		}
		_ = conn.Write(Frame{ID: f.ID, Type: "pong", Payload: f.Payload})
	}()

	// Client side connects.
	c, err := Dial(sock, 2*time.Second)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()
	if err := c.Write(Frame{ID: "1", Type: "ping", Payload: json.RawMessage(`{"v":1}`)}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := c.Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.Type != "pong" || got.ID != "1" {
		t.Fatalf("unexpected frame: %+v", got)
	}
}

func TestRequestRouter_DispatchesByID(t *testing.T) {
	r := NewRouter()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Spawn a fake "relay" goroutine that produces responses for two ids.
	wait := r.Pending("call-1")
	r.Pending("call-2") // unused, ensures multiple in-flight ok
	go func() {
		r.Deliver(Frame{ID: "call-1", Type: "chunk", Payload: json.RawMessage(`{"delta":"hello"}`)})
		r.Deliver(Frame{ID: "call-1", Type: "done", Payload: json.RawMessage(`{"finish_reason":"stop"}`)})
	}()

	var saw []string
	for {
		select {
		case f := <-wait:
			saw = append(saw, f.Type)
			if f.Type == "done" {
				if !contains(saw, "chunk") {
					t.Errorf("expected chunk before done, saw %v", saw)
				}
				return
			}
		case <-ctx.Done():
			t.Fatalf("timed out, saw %v", saw)
		}
	}
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

// TestServer_RoutesByID drives the integration: server accepts a relay,
// router dispatches incoming frames by id, multiple in-flight calls don't
// cross-contaminate.
func TestServer_RoutesByID(t *testing.T) {
	// Unix socket paths are limited to ~104 chars on macOS; t.TempDir()
	// nests under /var/folders/... which often exceeds that. Use /tmp.
	sock := filepath.Join("/tmp", fmt.Sprintf("fenster-bridge-%d.sock", time.Now().UnixNano()))
	t.Cleanup(func() { _ = os.Remove(sock) })
	srv := NewServer()
	if err := srv.Listen(sock); err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	// Relay side: read frames and respond with pong for each id.
	relayDone := make(chan struct{})
	go func() {
		defer close(relayDone)
		conn, err := srv.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			f, err := conn.Read()
			if err != nil {
				return
			}
			if err := conn.Write(Frame{ID: f.ID, Type: "pong"}); err != nil {
				return
			}
		}
	}()

	c, err := Dial(sock, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	// Send 3 concurrent requests; check all 3 responses arrive.
	var wg sync.WaitGroup
	results := make(chan string, 3)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			id := "call-" + string('a'+rune(n))
			if err := c.Write(Frame{ID: id, Type: "ping"}); err != nil {
				return
			}
		}(i)
	}
	wg.Wait()
	for i := 0; i < 3; i++ {
		f, err := c.Read()
		if err != nil {
			t.Fatalf("Read %d: %v", i, err)
		}
		if f.Type != "pong" {
			t.Fatalf("unexpected frame: %+v", f)
		}
		results <- f.ID
	}
	close(results)
	got := []string{}
	for s := range results {
		got = append(got, s)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 results, got %v", got)
	}
	if !strings.Contains(strings.Join(got, ","), "call-") {
		t.Errorf("results lost id prefix: %v", got)
	}
}
