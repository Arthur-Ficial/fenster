# fenster — Chrome extension (MV3)

This is the Chrome extension that lives inside fenster's headless Chrome instance. fenster's Go binary bundles this directory via `embed.FS` (`internal/extension/embed.go`) and extracts it to `~/.fenster/extension/` on first run.

## Files

- `manifest.json` — MV3 manifest with `nativeMessaging` permission.
- `service-worker.js` — connects to the native messaging host (`com.fullstackoptimization.fenster`) and bridges Prompt API calls.

## M0 status

The service worker connects, says hello, and echoes incoming messages. Real Prompt API wiring lands in M1.

## Don't edit by hand in production

If you need to change anything here, also re-run `make build` so the changes are baked into the embedded binary. The on-disk copy at `~/.fenster/extension/` is overwritten by the binary on every launch.
