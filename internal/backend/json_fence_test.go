package backend

import "testing"

func TestStripJSONFence(t *testing.T) {
	cases := map[string]string{
		"":                                  "",
		"  ":                                "",
		"{\"a\":1}":                         `{"a":1}`,
		"```json\n{\"a\":1}\n```":          `{"a":1}`,
		"```json\n{\"a\":1}\n```\n":        `{"a":1}`,
		"```\n{\"a\":1}\n```":              `{"a":1}`,
		"  ```json\n{\"a\":1}\n```  ":      `{"a":1}`,
	}
	for in, want := range cases {
		if got := stripJSONFence(in); got != want {
			t.Errorf("stripJSONFence(%q) = %q; want %q", in, got, want)
		}
	}
}
