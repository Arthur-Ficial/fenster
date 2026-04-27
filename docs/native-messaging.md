# fenster — Native Messaging protocol

## Framing

Each frame is a 4-byte little-endian unsigned length prefix followed by UTF-8-encoded JSON.

- Max message size **host → Chrome**: 1 MB (Chrome enforces; reject early with a clear error)
- Max message size **Chrome → host**: 4 GB (we cap at 16 MB for safety; configurable later)

```
+--------+--------+--------+--------+----------------------+
| len    | len    | len    | len    | UTF-8 JSON payload   |
| (LSB)  |        |        | (MSB)  | (len bytes)          |
+--------+--------+--------+--------+----------------------+
```

## Protocol envelope

Every message has at least these fields:

```json
{ "id": "<uuid>", "type": "<chat|chunk|done|error|hello>", "payload": { ... } }
```

- `id` ties chunks back to the originating HTTP request
- `type` determines payload shape
- `payload` is an open-ended object; clients ignore unknown keys

## Per-OS manifest

```json
{
  "name": "com.fullstackoptimization.fenster",
  "description": "fenster: OpenAI-compatible bridge to Gemini Nano",
  "path": "/absolute/path/to/fenster",
  "type": "stdio",
  "allowed_origins": ["chrome-extension://<EXTENSION_ID>/"]
}
```

Manifest install paths:

| OS | Path |
|---|---|
| macOS | `~/Library/Application Support/Google/Chrome/NativeMessagingHosts/com.fullstackoptimization.fenster.json` |
| Linux | `~/.config/google-chrome/NativeMessagingHosts/com.fullstackoptimization.fenster.json` |
| Windows | registry: `HKEY_CURRENT_USER\Software\Google\Chrome\NativeMessagingHosts\com.fullstackoptimization.fenster` (default value = absolute path to the JSON) |

### Per-browser variants (handled by `--browser=`)

- Chromium → `~/Library/Application Support/Chromium/NativeMessagingHosts/...`
- Edge → `~/Library/Application Support/Microsoft Edge/NativeMessagingHosts/...`
- Brave → `~/Library/Application Support/BraveSoftware/Brave-Browser/NativeMessagingHosts/...`

Linux and Windows have parallel paths.

## Windows footguns

1. Set stdin/stdout to **binary mode** (`_setmode(_fileno(stdin), _O_BINARY)`); CRLF translation will corrupt the length prefix. In Go, this is automatic — `os.Stdin` / `os.Stdout` are byte streams.
2. The registry has 32-bit and 64-bit views. Chrome checks the 32-bit view first. We write to both.
3. Manifest path inside the JSON must be absolute and use forward slashes or properly-escaped backslashes.
