// Package extension embeds fenster's MV3 Chrome extension into the binary.
//
// On first run (and every restart) the supervisor extracts these files to
// ~/.fenster/extension/ and points Chrome's --load-extension flag at that
// directory. We always overwrite — the on-disk copy is a cache, not state.
package extension

import "embed"

//go:embed assets/*
var Assets embed.FS
