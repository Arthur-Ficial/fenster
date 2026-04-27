package wire

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// encodeNullableString encodes a *string as a JSON string, or null when nil.
// DRY: every "*string but always present" field uses this.
func encodeNullableString(p *string) (json.RawMessage, error) {
	if p == nil {
		return json.RawMessage("null"), nil
	}
	b, err := json.Marshal(*p)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// encodeContent encodes a *Content into the JSON value (string, array, or null).
func encodeContent(c *Content) (json.RawMessage, error) {
	if c == nil {
		return json.RawMessage("null"), nil
	}
	if c.Parts != nil {
		return json.Marshal(c.Parts)
	}
	return json.Marshal(c.Text)
}

// decodeContent inverts encodeContent.
func decodeContent(raw json.RawMessage) (*Content, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	switch raw[0] {
	case '"':
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		return &Content{Text: s}, nil
	case '[':
		var parts []ContentPart
		if err := json.Unmarshal(raw, &parts); err != nil {
			return nil, err
		}
		return &Content{Parts: parts}, nil
	default:
		return nil, fmt.Errorf("content must be string or array, got %s", string(raw))
	}
}

// marshalAlwaysPresent re-emits the marshalled bytes of v but ensures
// the named keys appear with explicit JSON nulls when absent. This keeps
// MarshalJSON callers DRY: list the keys that must always be present, and
// the helper will inject `"key":null` if json.Marshal omitted them.
func marshalAlwaysPresent(v any, alwaysKeys ...string) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	// Decode into an ordered-ish map so we can ensure presence.
	var existing map[string]json.RawMessage
	if err := json.Unmarshal(b, &existing); err != nil {
		return nil, err
	}
	for _, k := range alwaysKeys {
		if _, ok := existing[k]; !ok {
			existing[k] = json.RawMessage("null")
		}
	}
	return json.Marshal(existing)
}
