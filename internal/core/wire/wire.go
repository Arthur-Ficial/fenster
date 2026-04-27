// Package wire holds the OpenAI-compatible JSON types fenster speaks.
//
// These types are PURE — no IO, no goroutines, no model. They mirror the
// wire format apfel emits at /v1/chat/completions, /v1/models, /health, and
// the error envelopes. The vendored apfel pytest suite at Tests/integration/
// is the spec.
//
// DRY rule: any "always-null when nil" JSON field uses jsonNullable (in
// nullable.go) so we don't repeat MarshalJSON boilerplate.
package wire

import "encoding/json"

// ModelID is the canonical model identifier exposed via /v1/models and
// chat completions response.model. fenster wraps Chrome's Prompt API
// (Gemini Nano), so the identifier reflects what is actually running.
const ModelID = "gemini-nano"

// ContextWindow advertised in /v1/models and /health.
const ContextWindow = 4096

// SupportedLanguagesFallback returns the languages we advertise when the
// backend doesn't report any. Apfel exposes Apple's set; fenster's default
// is Gemini Nano's currently-shipped set (en, ja, es).
func SupportedLanguagesFallback() []string {
	return []string{"en", "ja", "es"}
}

// Object kind strings.
const (
	ObjectChatCompletion      = "chat.completion"
	ObjectChatCompletionChunk = "chat.completion.chunk"
	ObjectModel               = "model"
	ObjectList                = "list"
)

// Finish reasons.
const (
	FinishStop          = "stop"
	FinishLength        = "length"
	FinishToolCalls     = "tool_calls"
	FinishContentFilter = "content_filter"
)

// ----- Request types -----

// ChatCompletionRequest is the JSON body of POST /v1/chat/completions.
type ChatCompletionRequest struct {
	Model            string           `json:"model"`
	Messages         []Message        `json:"messages"`
	Stream           *bool            `json:"stream,omitempty"`
	StreamOptions    *StreamOptions   `json:"stream_options,omitempty"`
	Temperature      *float64         `json:"temperature,omitempty"`
	MaxTokens        *int             `json:"max_tokens,omitempty"`
	Seed             *int             `json:"seed,omitempty"`
	Tools            []Tool           `json:"tools,omitempty"`
	ToolChoice       *json.RawMessage `json:"tool_choice,omitempty"`
	ResponseFormat   *ResponseFormat  `json:"response_format,omitempty"`
	Logprobs         *bool            `json:"logprobs,omitempty"`
	N                *int             `json:"n,omitempty"`
	Stop             *json.RawMessage `json:"stop,omitempty"`
	PresencePenalty  *float64         `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64         `json:"frequency_penalty,omitempty"`
	User             *string          `json:"user,omitempty"`

	// apfel JSON-API extensions (passed through; unused in fenster M1)
	XContextStrategy      *string `json:"x_context_strategy,omitempty"`
	XContextMaxTurns      *int    `json:"x_context_max_turns,omitempty"`
	XContextOutputReserve *int    `json:"x_context_output_reserve,omitempty"`
}

// StreamOptions is the OpenAI-compatible streaming flag bag.
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

// IsStream returns true when the client requested SSE streaming.
func (r *ChatCompletionRequest) IsStream() bool {
	return r.Stream != nil && *r.Stream
}

// IncludeUsageInStream returns true when stream + stream_options.include_usage.
func (r *ChatCompletionRequest) IncludeUsageInStream() bool {
	return r.IsStream() && r.StreamOptions != nil && r.StreamOptions.IncludeUsage
}

// ----- Message + Content -----

// Message is one entry in messages[]. content+refusal are always-encoded
// (null when nil) on assistant role to match apfel's wire shape.
type Message struct {
	Role       string     `json:"-"`
	Content    *Content   `json:"-"`
	Refusal    *string    `json:"-"`
	ToolCalls  []ToolCall `json:"-"`
	ToolCallID string     `json:"-"`
	Name       string     `json:"-"`
}

// rawMessage is the on-wire representation; we use it for both directions
// so encode/decode stay symmetric.
type rawMessage struct {
	Role       string          `json:"role"`
	Content    json.RawMessage `json:"content,omitempty"`
	Refusal    json.RawMessage `json:"refusal,omitempty"`
	ToolCalls  []ToolCall      `json:"tool_calls,omitempty"`
	ToolCallID string          `json:"tool_call_id,omitempty"`
	Name       string          `json:"name,omitempty"`
}

// MarshalJSON: assistant always emits content+refusal (null when nil);
// other roles emit content as the value or omit if truly absent.
func (m Message) MarshalJSON() ([]byte, error) {
	r := rawMessage{Role: m.Role, ToolCalls: m.ToolCalls, ToolCallID: m.ToolCallID, Name: m.Name}
	contentBytes, err := encodeContent(m.Content)
	if err != nil {
		return nil, err
	}
	refusalBytes, err := encodeNullableString(m.Refusal)
	if err != nil {
		return nil, err
	}
	if m.Role == "assistant" {
		// Always present, even when nil.
		r.Content = contentBytes
		r.Refusal = refusalBytes
		return marshalAlwaysPresent(r, "content", "refusal")
	}
	if m.Content != nil {
		r.Content = contentBytes
	}
	if m.Refusal != nil {
		r.Refusal = refusalBytes
	}
	return json.Marshal(r)
}

// UnmarshalJSON parses both string- and array-shaped content.
func (m *Message) UnmarshalJSON(data []byte) error {
	var r rawMessage
	if err := json.Unmarshal(data, &r); err != nil {
		return err
	}
	m.Role = r.Role
	m.ToolCalls = r.ToolCalls
	m.ToolCallID = r.ToolCallID
	m.Name = r.Name
	if len(r.Content) > 0 && string(r.Content) != "null" {
		c, err := decodeContent(r.Content)
		if err != nil {
			return err
		}
		m.Content = c
	}
	if len(r.Refusal) > 0 && string(r.Refusal) != "null" {
		var s string
		if err := json.Unmarshal(r.Refusal, &s); err != nil {
			return err
		}
		m.Refusal = &s
	}
	return nil
}

// Content is "text" | "parts[]". Apfel accepts both. parts[] containing
// image_url is rejected during validation, not here.
type Content struct {
	Text  string
	Parts []ContentPart
}

// HasImage returns true when the content array contains an image_url part.
func (c *Content) HasImage() bool {
	if c == nil {
		return false
	}
	for _, p := range c.Parts {
		if p.Type == "image_url" {
			return true
		}
	}
	return false
}

// AsString flattens parts to a concatenated text or returns Text directly.
func (c *Content) AsString() string {
	if c == nil {
		return ""
	}
	if c.Text != "" || len(c.Parts) == 0 {
		return c.Text
	}
	var out string
	for _, p := range c.Parts {
		if p.Type == "text" {
			out += p.Text
		}
	}
	return out
}

// TextContent is the helper used everywhere a one-shot string content is needed.
func TextContent(s string) *Content { return &Content{Text: s} }

// MarshalJSON for Content emits a string when Text is set, an array when
// Parts is set. Used for direct (non-Message) marshalling.
func (c Content) MarshalJSON() ([]byte, error) { return encodeContent(&c) }

// UnmarshalJSON for Content accepts either a JSON string or a JSON array.
func (c *Content) UnmarshalJSON(data []byte) error {
	got, err := decodeContent(data)
	if err != nil {
		return err
	}
	if got != nil {
		*c = *got
	}
	return nil
}

// ContentPart is one entry of an array-shaped content.
type ContentPart struct {
	Type     string        `json:"type"`
	Text     string        `json:"text,omitempty"`
	ImageURL *ContentImage `json:"image_url,omitempty"`
}

// ContentImage is the image_url payload (rejected in validation).
type ContentImage struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

// ----- Tools -----

// Tool is a function-tool definition.
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction is the function spec inside Tool.
type ToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// ToolCall is one entry of message.tool_calls or delta.tool_calls.
type ToolCall struct {
	Index    *int             `json:"index,omitempty"`
	ID       string           `json:"id,omitempty"`
	Type     string           `json:"type,omitempty"`
	Function ToolCallFunction `json:"function,omitempty"`
}

// ToolCallFunction is name + JSON-stringified arguments.
type ToolCallFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// ResponseFormat is the OpenAI response_format object.
type ResponseFormat struct {
	Type       string          `json:"type"`
	JSONSchema json.RawMessage `json:"json_schema,omitempty"`
}

// IsJSONObject reports whether response_format requests JSON mode.
func (r *ResponseFormat) IsJSONObject() bool {
	return r != nil && (r.Type == "json_object" || r.Type == "json_schema")
}

// ----- Response types -----

// ChatCompletionResponse is the non-streaming response body.
type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// NewChatResponse is the standard factory; sets Object correctly.
func NewChatResponse(id string, created int64, model string, choices []Choice, usage Usage) ChatCompletionResponse {
	return ChatCompletionResponse{ID: id, Object: ObjectChatCompletion, Created: created, Model: model, Choices: choices, Usage: usage}
}

// Choice is one entry of choices[] in non-streaming responses.
// Logprobs is always emitted as JSON null.
type Choice struct {
	Index        int     `json:"-"`
	Message      Message `json:"-"`
	FinishReason string  `json:"-"`
}

// MarshalJSON for Choice writes logprobs:null always.
func (c Choice) MarshalJSON() ([]byte, error) {
	type aux struct {
		Index        int             `json:"index"`
		Message      Message         `json:"message"`
		FinishReason string          `json:"finish_reason"`
		Logprobs     json.RawMessage `json:"logprobs"`
	}
	return json.Marshal(aux{Index: c.Index, Message: c.Message, FinishReason: c.FinishReason, Logprobs: json.RawMessage("null")})
}

// UnmarshalJSON for Choice mirrors aux.
func (c *Choice) UnmarshalJSON(data []byte) error {
	type aux struct {
		Index        int     `json:"index"`
		Message      Message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	}
	var a aux
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	c.Index = a.Index
	c.Message = a.Message
	c.FinishReason = a.FinishReason
	return nil
}

// Usage is the prompt/completion/total token block.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ----- Streaming -----

// ChatCompletionChunk is one streaming chunk body.
type ChatCompletionChunk struct {
	ID      string        `json:"id"`
	Object  string        `json:"object"`
	Created int64         `json:"created"`
	Model   string        `json:"model"`
	Choices []ChunkChoice `json:"choices"`
	Usage   *Usage        `json:"usage,omitempty"`
}

// NewChunk is the standard factory; sets Object correctly.
func NewChunk(id string, created int64, model string, choices []ChunkChoice) ChatCompletionChunk {
	return ChatCompletionChunk{ID: id, Object: ObjectChatCompletionChunk, Created: created, Model: model, Choices: choices}
}

// ChunkChoice is one streaming-choice entry. logprobs always null;
// finish_reason emitted as null when nil and as string when set.
type ChunkChoice struct {
	Index        int     `json:"-"`
	Delta        Delta   `json:"-"`
	FinishReason *string `json:"-"`
}

// MarshalJSON for ChunkChoice always emits logprobs:null and finish_reason
// (null or string).
func (c ChunkChoice) MarshalJSON() ([]byte, error) {
	type aux struct {
		Index        int             `json:"index"`
		Delta        Delta           `json:"delta"`
		FinishReason json.RawMessage `json:"finish_reason"`
		Logprobs     json.RawMessage `json:"logprobs"`
	}
	a := aux{Index: c.Index, Delta: c.Delta, Logprobs: json.RawMessage("null")}
	fr, err := encodeNullableString(c.FinishReason)
	if err != nil {
		return nil, err
	}
	a.FinishReason = fr
	return json.Marshal(a)
}

// UnmarshalJSON for ChunkChoice.
func (c *ChunkChoice) UnmarshalJSON(data []byte) error {
	type aux struct {
		Index        int    `json:"index"`
		Delta        Delta  `json:"delta"`
		FinishReason string `json:"finish_reason"`
	}
	var a aux
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	c.Index = a.Index
	c.Delta = a.Delta
	if a.FinishReason != "" {
		fr := a.FinishReason
		c.FinishReason = &fr
	}
	return nil
}

// Delta is one streaming-delta payload.
type Delta struct {
	Role      string     `json:"role,omitempty"`
	Content   *string    `json:"content,omitempty"`
	Refusal   *string    `json:"refusal,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}
