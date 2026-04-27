// Package nm implements the Chrome Native Messaging stdio framing protocol:
// each message is a 4-byte little-endian unsigned length prefix followed by
// UTF-8-encoded JSON. Chrome's documented bounds:
//
//	host -> Chrome: ≤ 1 MB per message
//	Chrome -> host: ≤ 4 GB per message  (we cap incoming at 16 MB by default)
//
// fenster's host process is launched by Chrome itself (Native Messaging
// hosts are spawned by the browser, not by the user), so stdin/stdout are
// the bridge. Both directions are full-duplex; readers and writers should
// run on separate goroutines.
package nm

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// DefaultMaxMessageSize is the cap fenster enforces on incoming messages.
// Chrome's hard ceiling is 4 GB but we don't expect to need that much.
const DefaultMaxMessageSize = 16 * 1024 * 1024

// DefaultMaxOutgoing is Chrome's enforced ceiling for host -> Chrome messages.
// Writing more than this is rejected by Chrome anyway; we fail fast locally.
const DefaultMaxOutgoing = 1 * 1024 * 1024

// ErrMessageTooLarge is returned when a payload exceeds the configured cap.
var ErrMessageTooLarge = errors.New("nm: message exceeds size cap")

// Write writes payload as a single Native Messaging frame to w.
func Write(w io.Writer, payload []byte) error {
	if len(payload) > DefaultMaxOutgoing {
		return fmt.Errorf("%w: %d bytes (max %d)", ErrMessageTooLarge, len(payload), DefaultMaxOutgoing)
	}
	hdr := make([]byte, 4)
	binary.LittleEndian.PutUint32(hdr, uint32(len(payload)))
	if _, err := w.Write(hdr); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}

// Read reads a single Native Messaging frame from r. It returns io.EOF on a
// clean end-of-stream and io.ErrUnexpectedEOF on a truncated frame.
//
// maxSize must be > 0; payloads larger than maxSize fail with ErrMessageTooLarge
// (and the rest of the stream is undefined — caller should disconnect).
func Read(r io.Reader, maxSize int) ([]byte, error) {
	if maxSize <= 0 {
		maxSize = DefaultMaxMessageSize
	}
	hdr := make([]byte, 4)
	n, err := io.ReadFull(r, hdr)
	switch {
	case err == nil:
		// fall through
	case err == io.EOF && n == 0:
		return nil, io.EOF
	case err == io.ErrUnexpectedEOF, err == io.EOF:
		return nil, io.ErrUnexpectedEOF
	default:
		return nil, err
	}
	length := binary.LittleEndian.Uint32(hdr)
	if int(length) > maxSize {
		return nil, fmt.Errorf("%w: %d bytes (max %d)", ErrMessageTooLarge, length, maxSize)
	}
	if length == 0 {
		return []byte{}, nil
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		if err == io.EOF {
			return nil, io.ErrUnexpectedEOF
		}
		return nil, err
	}
	return payload, nil
}
