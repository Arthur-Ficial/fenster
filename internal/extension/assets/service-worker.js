// fenster service worker — M0 stub.
//
// Connects to the native messaging host on startup and keeps the port open.
// Per Chrome docs, an open native messaging port keeps the service worker
// alive indefinitely — this is the keystone that avoids MV3's 30-second
// idle timeout. M1 will wire this up to Prompt API calls.

const HOST_NAME = "com.fullstackoptimization.fenster";

let port = null;

function connect() {
  try {
    port = chrome.runtime.connectNative(HOST_NAME);
  } catch (e) {
    console.error("[fenster] connectNative failed:", e);
    return;
  }

  port.onMessage.addListener((msg) => {
    // M1: dispatch by msg.type — chat / tools / health.
    console.log("[fenster] received:", msg);
    // Echo for now so the framing roundtrip can be tested.
    port.postMessage({ id: msg.id, type: "echo", payload: msg });
  });

  port.onDisconnect.addListener(() => {
    const err = chrome.runtime.lastError;
    console.warn("[fenster] disconnected:", err && err.message);
    port = null;
    // Reconnect with backoff. M1 will tune this.
    setTimeout(connect, 1000);
  });

  // Send a hello so the host knows the SW is alive.
  port.postMessage({ id: "boot", type: "hello", payload: { version: "0.0.1" } });
}

self.addEventListener("activate", () => {
  connect();
});

// Also connect immediately when the SW evaluates (covers warm starts).
connect();
