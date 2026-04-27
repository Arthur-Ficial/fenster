package backend

import (
	cerr "github.com/Arthur-Ficial/fenster/internal/core/errors"
)

// Sentinels returned to the server layer; the server maps them to envelopes.
var (
	errBackendUnavailable = cerr.ErrModelUnavailable
)
