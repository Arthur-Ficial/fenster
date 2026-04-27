package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Arthur-Ficial/fenster/internal/bridge"
	"github.com/Arthur-Ficial/fenster/internal/nm"
)

// runNMHost is the entry point Chrome executes when the extension calls
// connectNative("com.fullstackoptimization.fenster"). Chrome connects our
// stdin/stdout to the extension's port. We forward those bytes verbatim
// to/from the supervisor's Unix socket.
//
// Frame format on both pipes is identical: 4-byte LE length + JSON. So this
// process is a pure relay; no parsing required.
func runNMHost(ctx context.Context) error {
	sock := defaultBridgeSock()
	c, err := bridge.Dial(sock, 5*time.Second)
	if err != nil {
		return fmt.Errorf("nm-host: cannot reach supervisor at %s: %w (run `fenster --serve` first)", sock, err)
	}
	defer c.Close()

	// Two copy goroutines: stdin (from Chrome) -> socket; socket -> stdout (to Chrome).
	var wg sync.WaitGroup
	wg.Add(2)
	errCh := make(chan error, 2)

	go func() {
		defer wg.Done()
		errCh <- copyNMToBridge(os.Stdin, c)
	}()
	go func() {
		defer wg.Done()
		errCh <- copyBridgeToNM(c, os.Stdout)
	}()
	wg.Wait()
	close(errCh)
	for e := range errCh {
		if e != nil && e != io.EOF {
			return e
		}
	}
	return nil
}

func copyNMToBridge(r io.Reader, c *bridge.Conn) error {
	for {
		raw, err := nm.Read(r, 16*1024*1024)
		if err != nil {
			return err
		}
		// Send to supervisor as a single bridge frame body.
		if err := c.WriteRaw(raw); err != nil {
			return err
		}
	}
}

func copyBridgeToNM(c *bridge.Conn, w io.Writer) error {
	for {
		raw, err := c.ReadRaw()
		if err != nil {
			return err
		}
		if err := nm.Write(w, raw); err != nil {
			return err
		}
	}
}

func defaultBridgeSock() string {
	if v := os.Getenv("FENSTER_BRIDGE_SOCK"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/fenster-bridge.sock"
	}
	dir := filepath.Join(home, ".fenster", "run")
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, "b.sock")
}
