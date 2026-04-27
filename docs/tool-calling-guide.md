# fenster — tool calling guide

The Chrome Prompt API does **not** support OpenAI-style function calling natively. fenster fakes it on the host side. The trick is OpenAI-compatible enough that `openai.OpenAI(...).chat.completions.create(tools=...)` works as clients expect.

## How the shim works

OpenAI-style request:

```json
{
  "model": "gemini-nano",
  "messages": [...],
  "tools": [{
    "type": "function",
    "function": {
      "name": "get_weather",
      "parameters": {"type": "object", "properties": {"city": {"type": "string"}}, "required": ["city"]}
    }
  }],
  "tool_choice": {"type": "function", "function": {"name": "get_weather"}}
}
```

fenster translates this into a Prompt API call with:

1. **System prompt** describing the available tools (concatenated to the user's system prompt).
2. **`responseConstraint`** = a JSON Schema that forces the model to emit either a `{tool_call: {name, arguments}}` or `{content: "..."}` shape:

   ```json
   {
     "anyOf": [
       {
         "type": "object",
         "required": ["tool_call"],
         "properties": {
           "tool_call": {
             "type": "object",
             "required": ["name", "arguments"],
             "properties": {
               "name": {"enum": ["get_weather"]},
               "arguments": {"type": "object", "properties": {"city": {"type": "string"}}, "required": ["city"]}
             }
           }
         }
       },
       {
         "type": "object",
         "required": ["content"],
         "properties": {"content": {"type": "string"}}
       }
     ]
   }
   ```

3. **Host-side parsing.** The Prompt API output is JSON-validated against the constraint, then unwrapped into an OpenAI-shaped `tool_calls[]` or `content`. Parsing errors are 500-classed for now (TODO: graceful retry with stricter prompt).

## Limits

- Quality with a 3B model is meaningfully below cloud function-calling. Manage user expectations.
- Multi-tool selection works but is sometimes flaky on long tool lists. Keep `tools.length` small.
- `tool_choice` forcing a specific function works because we narrow the constraint's `name.enum` to that function's name only.
- Streaming: `tool_calls[]` deltas are emitted in chunks (`{tool_calls: [{index: 0, function: {name: "...", arguments: "{\"city\":"}}]}` etc.) to match OpenAI's wire format. Internally fenster waits for the JSON to validate before splitting into deltas — this means the first tool-call chunk has higher latency than a content stream.
