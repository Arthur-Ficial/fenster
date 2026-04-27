package backend

import "strings"

// stripJSONFence removes leading/trailing markdown fences (```json or ```)
// and whitespace from `s`. Apfel does this on its server side when the
// client asked for `response_format: json_object` so the result is ready to
// json.Marshal/Unmarshal without further cleanup.
//
// Examples:
//   "```json\n{...}\n```\n" -> "{...}"
//   "```\n{...}\n```"       -> "{...}"
//   "{...}"                 -> "{...}"
func stripJSONFence(s string) string {
	t := strings.TrimSpace(s)
	if !strings.HasPrefix(t, "```") {
		return t
	}
	// Drop the opening fence (```json or ```).
	nl := strings.IndexByte(t, '\n')
	if nl < 0 {
		return t
	}
	t = t[nl+1:]
	// Drop the trailing fence.
	if i := strings.LastIndex(t, "```"); i >= 0 {
		t = t[:i]
	}
	return strings.TrimSpace(t)
}
