package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Arthur-Ficial/fenster/internal/backend"
)

// newTestServer wraps a Mux + Backend in httptest. Tests run against this.
func newTestServer(t *testing.T, be backend.Backend) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(NewMux(Config{Backend: be}))
	t.Cleanup(srv.Close)
	return srv
}

func TestHealth_200_HasModelAvailable(t *testing.T) {
	s := newTestServer(t, backend.EchoBackend{})
	resp, err := http.Get(s.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["model_available"] != true {
		t.Fatalf("expected model_available true, got %+v", body)
	}
	if body["model"] != "gemini-nano" {
		t.Fatalf("expected fenster model id gemini-nano, got %v", body["model"])
	}
}

func TestModels_HasOneEntry(t *testing.T) {
	s := newTestServer(t, backend.EchoBackend{})
	resp, err := http.Get(s.URL + "/v1/models")
	if err != nil { t.Fatal(err) }
	defer resp.Body.Close()
	var body struct {
		Object string           `json:"object"`
		Data   []map[string]any `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body.Object != "list" || len(body.Data) != 1 {
		t.Fatalf("expected list with one model, got %+v", body)
	}
	if body.Data[0]["id"] != "gemini-nano" {
		t.Fatalf("expected gemini-nano id, got %v", body.Data[0]["id"])
	}
}

func TestChatCompletions_NonStreaming_200(t *testing.T) {
	s := newTestServer(t, backend.EchoBackend{})
	body := []byte(`{"model":"gemini-nano","messages":[{"role":"user","content":"hello"}]}`)
	resp, err := http.Post(s.URL+"/v1/chat/completions", "application/json", bytes.NewReader(body))
	if err != nil { t.Fatal(err) }
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d body %s", resp.StatusCode, b)
	}
	raw, _ := io.ReadAll(resp.Body)
	for _, want := range []string{
		`"object":"chat.completion"`,
		`"model":"gemini-nano"`,
		`"finish_reason":"stop"`,
		`"refusal":null`,
		`"logprobs":null`,
		`"prompt_tokens"`,
		`"total_tokens"`,
	} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("missing %s in %s", want, raw)
		}
	}
}

func TestChatCompletions_Streaming_EmitsSSEFrames(t *testing.T) {
	s := newTestServer(t, backend.EchoBackend{})
	body := []byte(`{"model":"gemini-nano","stream":true,"messages":[{"role":"user","content":"hello world"}]}`)
	resp, err := http.Post(s.URL+"/v1/chat/completions", "application/json", bytes.NewReader(body))
	if err != nil { t.Fatal(err) }
	defer resp.Body.Close()
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Fatalf("expected text/event-stream, got %s", got)
	}
	raw, _ := io.ReadAll(resp.Body)
	body2 := string(raw)
	for _, want := range []string{
		`data: {"id":"chatcmpl-`,
		`"object":"chat.completion.chunk"`,
		`"finish_reason":"stop"`,
		"data: [DONE]\n\n",
	} {
		if !strings.Contains(body2, want) {
			t.Errorf("missing %s in stream body:\n%s", want, body2)
		}
	}
}

func TestChatCompletions_StreamUsage_OnlyWhenIncludeUsage(t *testing.T) {
	s := newTestServer(t, backend.EchoBackend{})
	body := []byte(`{"model":"gemini-nano","stream":true,"stream_options":{"include_usage":true},"messages":[{"role":"user","content":"hi"}]}`)
	resp, err := http.Post(s.URL+"/v1/chat/completions", "application/json", bytes.NewReader(body))
	if err != nil { t.Fatal(err) }
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	body2 := string(raw)
	if !strings.Contains(body2, `"usage":{`) {
		t.Fatalf("expected usage chunk in stream when include_usage=true, got:\n%s", body2)
	}
}

func TestChatCompletions_EmptyMessages_400(t *testing.T) {
	s := newTestServer(t, backend.EchoBackend{})
	body := []byte(`{"model":"gemini-nano","messages":[]}`)
	resp, err := http.Post(s.URL+"/v1/chat/completions", "application/json", bytes.NewReader(body))
	if err != nil { t.Fatal(err) }
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	raw, _ := io.ReadAll(resp.Body)
	for _, want := range []string{
		`"error"`,
		`"type":"invalid_request_error"`,
		"messages",
		`"param":null`,
		`"code":null`,
	} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("missing %s in %s", want, raw)
		}
	}
}

func TestChatCompletions_LogprobsTrue_400(t *testing.T) {
	s := newTestServer(t, backend.EchoBackend{})
	body := []byte(`{"model":"gemini-nano","logprobs":true,"messages":[{"role":"user","content":"hi"}]}`)
	resp, err := http.Post(s.URL+"/v1/chat/completions", "application/json", bytes.NewReader(body))
	if err != nil { t.Fatal(err) }
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestChatCompletions_NTwo_400(t *testing.T) {
	s := newTestServer(t, backend.EchoBackend{})
	body := []byte(`{"model":"gemini-nano","n":2,"messages":[{"role":"user","content":"hi"}]}`)
	resp, err := http.Post(s.URL+"/v1/chat/completions", "application/json", bytes.NewReader(body))
	if err != nil { t.Fatal(err) }
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestChatCompletions_BadJSON_400(t *testing.T) {
	s := newTestServer(t, backend.EchoBackend{})
	resp, err := http.Post(s.URL+"/v1/chat/completions", "application/json", strings.NewReader(`{not json`))
	if err != nil { t.Fatal(err) }
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCompletions_501(t *testing.T) {
	s := newTestServer(t, backend.EchoBackend{})
	resp, err := http.Post(s.URL+"/v1/completions", "application/json", strings.NewReader(`{}`))
	if err != nil { t.Fatal(err) }
	defer resp.Body.Close()
	if resp.StatusCode != 501 {
		t.Fatalf("expected 501, got %d", resp.StatusCode)
	}
}

func TestEmbeddings_501(t *testing.T) {
	s := newTestServer(t, backend.EchoBackend{})
	resp, err := http.Post(s.URL+"/v1/embeddings", "application/json", strings.NewReader(`{}`))
	if err != nil { t.Fatal(err) }
	defer resp.Body.Close()
	if resp.StatusCode != 501 {
		t.Fatalf("expected 501, got %d", resp.StatusCode)
	}
}

func TestCORS_PreflightReturns204(t *testing.T) {
	s := httptest.NewServer(NewMux(Config{Backend: backend.EchoBackend{}, EnableCORS: true}))
	defer s.Close()
	req, _ := http.NewRequest(http.MethodOptions, s.URL+"/v1/chat/completions", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 204 {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	if resp.Header.Get("Access-Control-Allow-Origin") == "" {
		t.Fatalf("expected ACAO header")
	}
}

func TestCORS_DisabledByDefault(t *testing.T) {
	s := newTestServer(t, backend.EchoBackend{}) // EnableCORS:false
	req, _ := http.NewRequest(http.MethodOptions, s.URL+"/v1/chat/completions", nil)
	req.Header.Set("Origin", "http://example.com")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	// Without --cors enabled, OPTIONS still resolves but the ACAO header
	// is empty (apfel default behaviour).
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected empty ACAO when CORS disabled, got %s", got)
	}
}

func TestUnknownPath_404(t *testing.T) {
	s := newTestServer(t, backend.EchoBackend{})
	resp, err := http.Get(s.URL + "/v1/totally-not-real")
	if err != nil { t.Fatal(err) }
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestMethodNotAllowed_405(t *testing.T) {
	s := newTestServer(t, backend.EchoBackend{})
	resp, err := http.Get(s.URL + "/v1/chat/completions")
	if err != nil { t.Fatal(err) }
	defer resp.Body.Close()
	if resp.StatusCode != 405 {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}
}
