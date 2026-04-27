// Package errors holds fenster's OpenAI-compatible error envelope and the
// catalogue of fixed error sentinels (one per pytest-asserted rejection).
//
// DRY rule: every place that returns an error to a client uses the
// sentinel + Render so we never hand-craft the JSON shape twice.
package errors

import "encoding/json"

// Type strings — apfel uses these; fenster mirrors.
const (
	InvalidRequest        = "invalid_request_error"
	Authentication        = "authentication_error"
	Forbidden             = "forbidden"
	RateLimit             = "rate_limit_error"
	ServerError           = "server_error"
	ContextLengthExceeded = "context_length_exceeded"
	NotImplemented        = "not_implemented_error"
)

// Envelope is the OpenAI error wrapper.
type Envelope struct {
	Error Body `json:"error"`
}

// Body mirrors OpenAI exactly. param/code are always present (null when none).
type Body struct {
	Message string          `json:"message"`
	Type    string          `json:"type"`
	Param   json.RawMessage `json:"param"`
	Code    json.RawMessage `json:"code"`
}

// New builds a fresh envelope with explicit JSON nulls for param/code.
func New(message, errType string) Envelope {
	return Envelope{Error: Body{
		Message: message, Type: errType,
		Param: json.RawMessage("null"),
		Code:  json.RawMessage("null"),
	}}
}

// HTTPStatus maps an error type to its standard HTTP status.
func HTTPStatus(errType string) int {
	switch errType {
	case InvalidRequest, ContextLengthExceeded:
		return 400
	case Authentication:
		return 401
	case Forbidden:
		return 403
	case RateLimit:
		return 429
	case NotImplemented:
		return 501
	default:
		return 500
	}
}

// Sentinel is a pre-built error: type + status + canonical message. Validation
// code uses these directly, ensuring every rejection path is named.
type Sentinel struct {
	Type    string
	Status  int
	Message string
}

// Envelope returns the Envelope for this sentinel.
func (s *Sentinel) Envelope() Envelope { return New(s.Message, s.Type) }

// Error implements the standard error interface so sentinels are returnable
// from functions that produce `error`. errors.As(target, &Sentinel{}) finds them.
func (s *Sentinel) Error() string { return s.Message }

// Wire-asserted rejection messages. These exact strings are what apfel
// returns; fenster matches them so apfel's pytest asserts pass.
var (
	ErrEmptyMessages = &Sentinel{InvalidRequest, 400,
		"'messages' must contain at least one message"}
	ErrImageContent = &Sentinel{InvalidRequest, 400,
		"Image content is not supported by the Gemini Nano on-device model"}
	ErrLogprobs = &Sentinel{InvalidRequest, 400,
		"Parameter 'logprobs' is not supported by the Gemini Nano on-device model."}
	ErrN = &Sentinel{InvalidRequest, 400,
		"Parameter 'n' is not supported by the Gemini Nano on-device model. Only n=1 is allowed."}
	ErrPresencePenalty = &Sentinel{InvalidRequest, 400,
		"Parameter 'presence_penalty' is not supported by the Gemini Nano on-device model."}
	ErrFrequencyPenalty = &Sentinel{InvalidRequest, 400,
		"Parameter 'frequency_penalty' is not supported by the Gemini Nano on-device model."}
	ErrStop = &Sentinel{InvalidRequest, 400,
		"Parameter 'stop' is not supported by the Gemini Nano on-device model."}
	ErrInvalidJSON = &Sentinel{InvalidRequest, 400,
		"Invalid JSON"}
	ErrMaxTokensInvalid = &Sentinel{InvalidRequest, 400,
		"'max_tokens' must be a positive integer"}
	ErrTemperatureInvalid = &Sentinel{InvalidRequest, 400,
		"'temperature' must be non-negative"}
	ErrModelUnavailable = &Sentinel{ServerError, 503,
		"Model is unavailable"}
	ErrNotImplemented = &Sentinel{NotImplemented, 501,
		"This endpoint is not implemented by fenster"}
)
