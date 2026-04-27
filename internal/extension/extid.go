package extension

import (
	"crypto/sha256"
	"path/filepath"
)

// PathDerivedID returns the extension ID Chrome assigns to an unpacked
// extension loaded with --load-extension=<absolutePath>. The ID is
// derived deterministically:
//
//	sha = sha256(filepath.Clean(absolutePath))
//	id  = first 16 bytes of sha, each nibble mapped to 'a'..'p'
//
// (Per chromium/src/components/crx_file/id_util.cc.)
func PathDerivedID(absolutePath string) string {
	clean := filepath.Clean(absolutePath)
	h := sha256.Sum256([]byte(clean))
	out := make([]byte, 32)
	for i := 0; i < 16; i++ {
		// Each byte → two chars 'a'..'p'.
		out[i*2] = 'a' + (h[i] >> 4)
		out[i*2+1] = 'a' + (h[i] & 0x0f)
	}
	return string(out)
}
