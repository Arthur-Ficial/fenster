package errors

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNewError_AlwaysHasNullParamAndCode(t *testing.T) {
	env := New("bad request", InvalidRequest)
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, want := range []string{`"message":"bad request"`, `"type":"invalid_request_error"`, `"param":null`, `"code":null`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in %s", want, got)
		}
	}
}

func TestHTTPStatusFor(t *testing.T) {
	cases := map[string]int{
		InvalidRequest:        400,
		ContextLengthExceeded: 400,
		Authentication:        401,
		Forbidden:             403,
		RateLimit:             429,
		NotImplemented:        501,
		ServerError:           500,
		"surprise_kind":       500,
	}
	for k, want := range cases {
		if got := HTTPStatus(k); got != want {
			t.Errorf("HTTPStatus(%q) = %d, want %d", k, got, want)
		}
	}
}

func TestSentinels_HaveTypeAndDefaultMessage(t *testing.T) {
	for _, e := range []*Sentinel{
		ErrEmptyMessages, ErrImageContent, ErrLogprobs, ErrN, ErrPresencePenalty,
		ErrFrequencyPenalty, ErrStop, ErrInvalidJSON, ErrMaxTokensInvalid, ErrTemperatureInvalid,
	} {
		if e.Type == "" || e.Message == "" {
			t.Errorf("sentinel missing fields: %+v", e)
		}
		if e.Status == 0 {
			t.Errorf("sentinel missing status: %+v", e)
		}
	}
}
