package server

import (
	_ "embed"
	"net/http"
)

// rootHTML is the page Chrome loads when fenster spawns it. The Built-in AI
// APIs (LanguageModel) are exposed only on real http(s):// origins — not on
// about:blank — so fenster serves this minimal page from / for that purpose.
//
// The page surfaces a single button that, when clicked (real or synthetic),
// triggers the model download via LanguageModel.create() (Chrome enforces a
// user-gesture gate). Subsequent prompt() calls fan out from there.
const rootHTML = `<!doctype html>
<html><head><meta charset="utf-8"><title>fenster</title></head>
<body>
<h1>fenster bridge</h1>
<p>This page is loaded inside a fenster-controlled Chrome to expose the
Prompt API. Don't navigate elsewhere; fenster's CDP runner uses this tab.</p>
<button id="trigger">init Gemini Nano</button>
<pre id="status"></pre>
<script>
const trigger = document.getElementById('trigger');
const status = document.getElementById('status');
function log(msg) { status.textContent += msg + '\n'; }

window.fensterTriggerInit = async function() {
  if (typeof LanguageModel === 'undefined') {
    log('LanguageModel: undefined (open a real localhost origin and ensure the Prompt API flag is enabled)');
    return { ok: false, reason: 'LanguageModel-undefined' };
  }
  const avail = await LanguageModel.availability();
  log('availability: ' + avail);
  if (avail === 'available') return { ok: true, avail };
  if (avail === 'unavailable' || avail === 'no') return { ok: false, reason: avail };
  // 'downloadable' or 'downloading' — call create() to make Chrome trigger / continue the download.
  try {
    const session = await LanguageModel.create({
      monitor(m) {
        m.addEventListener('downloadprogress', (e) => {
          log('downloadprogress: ' + (e.loaded * 100).toFixed(1) + '%');
        });
      },
    });
    session.destroy?.();
    return { ok: true, avail: await LanguageModel.availability() };
  } catch (e) {
    log('create() error: ' + e);
    return { ok: false, reason: String(e) };
  }
};

trigger.addEventListener('click', () => {
  window.fensterTriggerInit().then(r => log('trigger result: ' + JSON.stringify(r)));
});
</script>
</body></html>`

func handleRoot() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(rootHTML))
	}
}
