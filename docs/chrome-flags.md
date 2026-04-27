# fenster — required Chrome flags

When fenster spawns Chrome it uses:

```
chrome \
  --headless=new \
  --user-data-dir=$HOME/.fenster/profile \
  --load-extension=$HOME/.fenster/extension \
  --disable-extensions-except=$HOME/.fenster/extension \
  --enable-features=PromptAPIForGeminiNano,OptimizationGuideOnDeviceModel \
  --remote-debugging-port=9222 \
  --no-first-run \
  --no-default-browser-check \
  about:blank
```

## Why each flag

| Flag | Why |
|---|---|
| `--headless=new` | Chrome 112+ headless mode that supports extensions and service workers (the **legacy** `--headless` does not). |
| `--user-data-dir` | Dedicated profile so we don't conflict with the user's normal Chrome and so the Gemini Nano model download lives where we control it. |
| `--load-extension` | Loads the bundled MV3 extension from disk (we extract it from `embed.FS` to `~/.fenster/extension/` on first run). |
| `--disable-extensions-except` | Hardening: only our extension runs in this Chrome. |
| `--enable-features=PromptAPIForGeminiNano,OptimizationGuideOnDeviceModel` | Equivalent of the `chrome://flags` toggles that enable the Prompt API and the on-device model component. |
| `--remote-debugging-port=9222` | Lets `fenster doctor` and `fenster debug` attach via CDP for diagnostics. |
| `--no-first-run --no-default-browser-check` | Skip the welcome flow / "Make Chrome default?" prompt. |

## Flags we deliberately do NOT use

- `--disable-gpu` — kills Gemini Nano. The model needs GPU acceleration (or a 16 GB CPU fallback that we don't trigger by default).
- `--single-process` — incompatible with extensions and service workers.
- `--no-sandbox` — security regression. Only acceptable inside containers, never for end users.
