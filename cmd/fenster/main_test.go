package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestVersionFlag(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"--version"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("--version returned error: %v", err)
	}
}

func TestDoctorSubcommand_Runs(t *testing.T) {
	cmd := newRootCmd()
	cmd.SetArgs([]string{"doctor", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		// FAIL is acceptable on minimal environments; we only require no panic.
		t.Logf("doctor returned %v (acceptable)", err)
	}
}

// findFreePort grabs an OS-allocated port for the test.
func findFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port
}

// TestServeFlag_RealServer brings up runServeMode against a free port,
// hits /health and /v1/models, asserts wire shape, then cancels the ctx.
func TestServeFlag_RealServer(t *testing.T) {
	port := findFreePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- runServeMode(ctx, port, "", false) }()

	// Wait for the listener.
	base := "http://127.0.0.1:" + itoa(port)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(base + "/health")
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				t.Fatalf("health status %d", resp.StatusCode)
			}
			var body map[string]any
			_ = json.NewDecoder(resp.Body).Decode(&body)
			if body["model"] != "apple-foundationmodel" {
				t.Fatalf("unexpected model %v", body["model"])
			}
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	resp, err := http.Get(base + "/v1/models")
	if err != nil {
		t.Fatalf("/v1/models: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("/v1/models %d: %s", resp.StatusCode, raw)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("runServeMode err: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not shut down within timeout")
	}
}

// TestServeFlag_PortInUse_Fails proves we surface bind errors.
func TestServeFlag_PortInUse_Fails(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()
	port := l.Addr().(*net.TCPAddr).Port
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := runServeMode(ctx, port, "", false); err == nil {
		t.Fatal("expected bind error, got nil")
	} else if !strings.Contains(err.Error(), "listen") {
		t.Fatalf("unexpected error %q", err)
	}
}

func itoa(n int) string {
	const dig = "0123456789"
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{dig[n%10]}, b...)
		n /= 10
	}
	return string(b)
}
