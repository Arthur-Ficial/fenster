// Native Messaging framing: 4-byte little-endian unsigned length prefix
// followed by UTF-8 JSON payload. Chrome enforces 1 MB host→Chrome,
// 4 GB Chrome→host. We cap incoming at 16 MB by default for safety.
package nm

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestWrite_Frame_LittleEndianLength(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, []byte(`{"hi":1}`)); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if buf.Len() != 4+len(`{"hi":1}`) {
		t.Fatalf("framed length wrong: %d", buf.Len())
	}
	got := binary.LittleEndian.Uint32(buf.Bytes()[:4])
	if got != uint32(len(`{"hi":1}`)) {
		t.Fatalf("LE length got %d want %d", got, len(`{"hi":1}`))
	}
}

func TestRead_RoundTrips(t *testing.T) {
	var buf bytes.Buffer
	payload := []byte(`{"id":"x","type":"hello"}`)
	if err := Write(&buf, payload); err != nil {
		t.Fatal(err)
	}
	got, err := Read(&buf, DefaultMaxMessageSize)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("round-trip mismatch: %s vs %s", got, payload)
	}
}

func TestRead_RejectsOversized(t *testing.T) {
	// Pretend a sender claims a 32 MB payload but our cap is 1 MB.
	var buf bytes.Buffer
	hdr := make([]byte, 4)
	binary.LittleEndian.PutUint32(hdr, 32*1024*1024)
	buf.Write(hdr)
	_, err := Read(&buf, 1024*1024)
	if err == nil {
		t.Fatal("expected error on oversize length")
	}
	if !errors.Is(err, ErrMessageTooLarge) {
		t.Fatalf("expected ErrMessageTooLarge, got %v", err)
	}
}

func TestRead_EOFOnClean(t *testing.T) {
	_, err := Read(&bytes.Buffer{}, DefaultMaxMessageSize)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF, got %v", err)
	}
}

func TestRead_TruncatedHeader(t *testing.T) {
	buf := bytes.NewReader([]byte{1, 2, 3}) // only 3 bytes of length prefix
	_, err := Read(buf, DefaultMaxMessageSize)
	if err == nil {
		t.Fatal("expected error on truncated header")
	}
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("expected ErrUnexpectedEOF, got %v", err)
	}
}

func TestWrite_RefusesOversized(t *testing.T) {
	var buf bytes.Buffer
	big := strings.Repeat("a", DefaultMaxOutgoing+1)
	if err := Write(&buf, []byte(big)); err == nil {
		t.Fatal("expected oversize write to fail")
	}
}
