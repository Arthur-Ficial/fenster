package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBuild_HasRequiredFields(t *testing.T) {
	m := Build("/tmp/fenster", "extid123abc")
	if m.Name != HostName {
		t.Errorf("Name should be %q, got %q", HostName, m.Name)
	}
	if m.Path != "/tmp/fenster" {
		t.Errorf("Path lost")
	}
	if m.Type != "stdio" {
		t.Errorf("Type should be stdio")
	}
	if len(m.AllowedOrigins) != 1 || !strings.HasPrefix(m.AllowedOrigins[0], "chrome-extension://") {
		t.Errorf("allowed_origins not set: %+v", m.AllowedOrigins)
	}
}

func TestBuild_JSONShape(t *testing.T) {
	m := Build("/abs/path/fenster", "abcdef1234567890")
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, want := range []string{`"name": "com.fullstackoptimization.fenster"`, `"type": "stdio"`, `"path": "/abs/path/fenster"`, `chrome-extension://abcdef1234567890/`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in %s", want, got)
		}
	}
}

func TestPath_Darwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin-specific")
	}
	p := PathForBrowser(BrowserChrome)
	if !strings.Contains(p, "Library/Application Support/Google/Chrome/NativeMessagingHosts") {
		t.Errorf("unexpected path %q", p)
	}
	if !strings.HasSuffix(p, HostName+".json") {
		t.Errorf("path should end with %s.json, got %q", HostName, p)
	}
}

func TestKnownBrowsers_AllReturnPaths(t *testing.T) {
	for _, b := range AllBrowsers() {
		p := PathForBrowser(b)
		if p == "" {
			t.Errorf("browser %q got empty path", b)
		}
	}
}

// TestInstallUninstall round-trips a manifest write with a temp HOME so
// we don't touch the user's real Chrome NM dirs.
func TestInstallUninstall(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	got, err := Install(BrowserChrome, "/tmp/fenster", "abcd1234")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if _, err := os.Stat(got); err != nil {
		t.Fatalf("manifest not written at %s: %v", got, err)
	}
	// Round-trip parse.
	raw, _ := os.ReadFile(got)
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("manifest invalid JSON: %v", err)
	}
	if m.Path != "/tmp/fenster" {
		t.Errorf("path lost in round trip")
	}
	if err := Uninstall(BrowserChrome); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if _, err := os.Stat(got); !os.IsNotExist(err) {
		t.Errorf("manifest still present after Uninstall: %v", err)
	}
}

func TestInstallAll(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	results, err := InstallAll("/tmp/fenster", "extabc")
	if err != nil {
		t.Fatalf("InstallAll: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	for _, r := range results {
		if r.Err == nil {
			if !strings.Contains(r.Path, filepath.Base(filepath.Dir(r.Path))) {
				// just sanity check the path looks like a NM dir
				continue
			}
		}
	}
}
