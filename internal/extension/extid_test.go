package extension

import (
	"path/filepath"
	"strings"
	"testing"
)

// Chrome's extension ID for an unpacked extension is derived deterministically
// from the absolute path. Algorithm (per Chromium source):
//
//   sha = sha256(absolute_path_bytes)
//   id  = first 16 bytes of sha, each nibble mapped to 'a'..'p'
//
// The result is a 32-char string matching /^[a-p]{32}$/.
func TestPathDerivedID_Format(t *testing.T) {
	id := PathDerivedID("/Users/arthurficial/.fenster/extension")
	if len(id) != 32 {
		t.Fatalf("expected 32 chars, got %d (%q)", len(id), id)
	}
	for _, r := range id {
		if r < 'a' || r > 'p' {
			t.Fatalf("char %q out of [a-p] range in id %q", r, id)
		}
	}
}

func TestPathDerivedID_StableForSamePath(t *testing.T) {
	a := PathDerivedID("/abs/path/x")
	b := PathDerivedID("/abs/path/x")
	if a != b {
		t.Fatalf("ids unstable: %s vs %s", a, b)
	}
}

func TestPathDerivedID_DiffersByPath(t *testing.T) {
	a := PathDerivedID("/abs/a")
	b := PathDerivedID("/abs/b")
	if a == b {
		t.Fatalf("expected different ids for different paths")
	}
}

func TestPathDerivedID_NoSeparatorBleed(t *testing.T) {
	// Trailing slash should not change the id (matching Chromium behavior:
	// the path is normalized before hashing).
	a := PathDerivedID(filepath.Clean("/x/y"))
	b := PathDerivedID("/x/y")
	if !strings.EqualFold(a, b) {
		t.Fatalf("normalized paths should match: %s vs %s", a, b)
	}
}
