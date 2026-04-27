package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFileFlags_Empty(t *testing.T) {
	got, err := readFileFlags(nil)
	if err != nil || got != "" {
		t.Fatalf("nil flags should return empty: got %q err %v", got, err)
	}
}

func TestReadFileFlags_NonexistentRejected(t *testing.T) {
	_, err := readFileFlags([]string{"/tmp/fenster_no_such_file_xx"})
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
	var ex *exitError
	if !errors.As(err, &ex) || ex.code != exitInvalidArgs {
		t.Fatalf("expected exitInvalidArgs (%d), got %v", exitInvalidArgs, err)
	}
	if !strings.Contains(ex.Error(), "no such file") {
		t.Fatalf("expected 'no such file', got %q", ex.Error())
	}
}

func TestReadFileFlags_ImageRejected(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.jpg")
	if err := os.WriteFile(p, []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F'}, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := readFileFlags([]string{p})
	if err == nil {
		t.Fatal("expected image rejection")
	}
	if !strings.Contains(err.Error(), "image") && !strings.Contains(err.Error(), "text-only") {
		t.Fatalf("expected image/text-only error, got %v", err)
	}
}

func TestReadFileFlags_BinaryRejected(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.bin")
	if err := os.WriteFile(p, []byte{0xff, 0xfe, 0xfd, 0x80, 0x81, 0x82}, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := readFileFlags([]string{p})
	if err == nil {
		t.Fatal("expected binary rejection")
	}
	if !strings.Contains(err.Error(), "binary") && !strings.Contains(err.Error(), "UTF-8") && !strings.Contains(err.Error(), "text") {
		t.Fatalf("expected binary/UTF-8/text error, got %v", err)
	}
}

func TestReadFileFlags_TextAccepted(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.txt")
	if err := os.WriteFile(p, []byte("Hello, fenster."), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := readFileFlags([]string{p})
	if err != nil {
		t.Fatal(err)
	}
	if got != "Hello, fenster." {
		t.Fatalf("got %q", got)
	}
}

func TestReadFileFlags_MultipleConcatenated(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	_ = os.WriteFile(a, []byte("Fact A"), 0o644)
	_ = os.WriteFile(b, []byte("Fact B"), 0o644)
	got, err := readFileFlags([]string{a, b})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "Fact A") || !strings.Contains(got, "Fact B") {
		t.Fatalf("expected both facts, got %q", got)
	}
}

func TestCombinePromptAndFiles(t *testing.T) {
	cases := []struct {
		files, prompt, want string
	}{
		{"", "hi", "hi"},
		{"file body", "", "file body"},
		{"file body", "what?", "file body\n\nwhat?"},
	}
	for _, c := range cases {
		got, err := combinePromptAndFiles(c.files, c.prompt)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if got != c.want {
			t.Errorf("combine(%q,%q) = %q; want %q", c.files, c.prompt, got, c.want)
		}
	}
}
