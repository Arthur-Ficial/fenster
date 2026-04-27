# fenster — platforms

| Platform | apfel | fenster |
|---|---|---|
| macOS 26+ | yes | macOS 13+ (Ventura) |
| Windows | no | Windows 10/11 |
| Linux | no | Linux desktop (with GPU) |
| ChromeOS | no | Chromebook Plus only |
| iOS / Android | no | no |

## Hardware floor

- **GPU** with >4 GB VRAM (preferred), or
- **CPU** with ≥16 GB RAM and ≥4 cores (fallback path; meaningfully slower)
- ≥22 GB free disk (Gemini Nano evicts under 10 GB free)
- Chrome 138+ (Origin Trial flag for Prompt API; Chromium-based browsers like Edge / Brave / Opera should also work via `--browser=...`)

## What fenster cannot do

- Run on Android. Native Messaging on Android Chrome doesn't expose the Prompt API.
- Run on iOS. No headless Chrome on iOS.
- Run on a typical no-GPU Linux server. `--disable-gpu` kills Gemini Nano. fenster is a desktop tool.
