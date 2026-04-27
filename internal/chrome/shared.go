package chrome

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

// SharedState lives at ~/.fenster/run/chrome.json and tells subsequent
// fenster processes "there's already a Chrome running for me, attach here".
type SharedState struct {
	PID    int    `json:"pid"`
	CDPURL string `json:"cdp_url"`
}

// SharedStatePath returns the well-known location.
func SharedStatePath() string {
	if v := os.Getenv("FENSTER_CHROME_STATE"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".fenster", "run", "chrome.json")
}

// LoadShared reads the file. Returns nil, nil if the file doesn't exist.
func LoadShared() (*SharedState, error) {
	path := SharedStatePath()
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var s SharedState
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// SaveShared atomically writes the state.
func SaveShared(s SharedState) error {
	path := SharedStatePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// IsAlive checks pid + CDP responsiveness.
func (s *SharedState) IsAlive(ctx context.Context) bool {
	if s == nil || s.PID == 0 {
		return false
	}
	// pid alive?
	if err := syscall.Kill(s.PID, 0); err != nil {
		return false
	}
	// CDP responsive?
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, "GET", s.CDPURL+"/json/version", nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// EnsureSharedChrome returns a CDP URL for a Chrome that fenster can use.
// If one is already running (per shared state file) it returns that URL.
// Otherwise it launches Chrome with the bootstrapped profile and feature
// flags, persists the state, and returns the new CDP URL.
//
// Concurrency: a coarse file lock at ~/.fenster/run/chrome.lock serializes
// concurrent EnsureSharedChrome calls so only one process actually launches
// Chrome.
func EnsureSharedChrome(ctx context.Context, opts LaunchOptions) (cdpURL string, launched bool, err error) {
	lock, err := acquireFileLock(SharedStatePath() + ".lock")
	if err != nil {
		return "", false, err
	}
	defer lock.release()

	if existing, err := LoadShared(); err == nil && existing != nil && existing.IsAlive(ctx) {
		return existing.CDPURL, false, nil
	}

	// Launch a new Chrome. Pick a random debug port.
	port, err := pickFreePort()
	if err != nil {
		return "", false, err
	}
	binary := opts.BinaryPath
	if binary == "" {
		binary = LocateBinary()
	}
	if binary == "" {
		return "", false, errors.New("chrome: could not locate Chrome (set FENSTER_BROWSER)")
	}
	profile := opts.ProfileDir
	if profile == "" {
		profile = DefaultProfileDir()
	}
	if err := os.MkdirAll(profile, 0o755); err != nil {
		return "", false, err
	}
	if err := bootstrapLocalState(profile); err != nil {
		return "", false, err
	}
	for _, name := range []string{"SingletonLock", "SingletonCookie", "SingletonSocket"} {
		_ = os.Remove(filepath.Join(profile, name))
	}

	args := []string{
		"--user-data-dir=" + profile,
		"--remote-debugging-port=" + strconv.Itoa(port),
		"--remote-allow-origins=*",
		"--no-first-run",
		"--no-default-browser-check",
		"--enable-features=PromptAPIForGeminiNano,OptimizationGuideOnDeviceModel,OptimizationGuideOnDeviceModelBypassPerfRequirement,AIPromptAPI,AIRewriterAPI,AISummarizationAPI",
		"about:blank",
	}
	cmd := exec.Command(binary, args...)
	// Detach: we don't want our process tree death to bring Chrome down,
	// AND we don't want Chrome's stdout/stderr noise in our logs.
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return "", false, fmt.Errorf("chrome: spawn: %w", err)
	}
	// Don't wait on the process; release it.
	go func() { _ = cmd.Wait() }()

	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	state := SharedState{PID: cmd.Process.Pid, CDPURL: url}

	// Wait for CDP to come up.
	deadline := time.Now().Add(20 * time.Second)
	for {
		if state.IsAlive(ctx) {
			break
		}
		if time.Now().After(deadline) {
			_ = cmd.Process.Kill()
			return "", false, errors.New("chrome: spawned process did not expose CDP within 20s")
		}
		time.Sleep(200 * time.Millisecond)
	}
	if err := SaveShared(state); err != nil {
		return "", false, err
	}
	return url, true, nil
}

func pickFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// fileLock is a tiny advisory lock backed by flock(2).
type fileLock struct{ f *os.File }

func acquireFileLock(path string) (*fileLock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return &fileLock{f: f}, nil
}

func (l *fileLock) release() {
	if l == nil || l.f == nil {
		return
	}
	_ = syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	_ = l.f.Close()
}
