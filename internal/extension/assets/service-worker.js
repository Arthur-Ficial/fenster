// fenster — Chrome extension service worker.
//
// Bridges Chrome's built-in LanguageModel (Gemini Nano) to fenster's Native
// Messaging host. The extension's only job is to:
//
//   1. Open a long-lived NM port to com.fullstackoptimization.fenster on
//      service-worker activation. The persistent port keeps the SW alive.
//   2. For each {type:"chat", payload:{messages,...}} frame from the host,
//      create a LanguageModel session, forward the prompt, and stream
//      back {type:"chunk", delta:"..."} frames followed by a {type:"done"}.
//   3. Surface availability + capabilities via {type:"availability"} on demand.
//
// The protocol is intentionally tiny so the Go side controls everything
// else (validation, OpenAI envelope shaping, MCP handling).

const HOST_NAME = "com.fullstackoptimization.fenster";

let port = null;
const sessions = new Map(); // id -> LanguageModel session

function send(msg) {
  try {
    port?.postMessage(msg);
  } catch (e) {
    console.warn("[fenster] send failed:", e);
  }
}

async function handleMessage(msg) {
  const { id, type } = msg || {};
  if (!type) return;

  switch (type) {
    case "ping":
      send({ id, type: "pong", payload: { version: chrome.runtime.getManifest().version } });
      return;

    case "availability": {
      let detail = { LanguageModel: typeof LanguageModel };
      if (typeof LanguageModel !== "undefined") {
        try { detail.availability = await LanguageModel.availability(); }
        catch (e) { detail.availability_err = String(e); }
      }
      send({ id, type: "availability", payload: detail });
      return;
    }

    case "chat":
      await handleChat(id, msg.payload || {});
      return;

    default:
      send({ id, type: "error", payload: { message: "unknown type: " + type } });
  }
}

async function handleChat(id, payload) {
  if (typeof LanguageModel === "undefined") {
    send({ id, type: "error", payload: {
      message: "Chrome's LanguageModel is not available on this profile/version. See chrome://flags/#prompt-api-for-gemini-nano and chrome://components/.",
      code: "language_model_unavailable",
    }});
    send({ id, type: "done", payload: { finish_reason: "stop" }});
    return;
  }
  const stream = !!payload.stream;
  const messages = payload.messages || [];

  // Map OpenAI messages[] to LanguageModel initialPrompts[].
  // LanguageModel expects {role: "system"|"user"|"assistant", content: "..."}
  // and offers system prompts via systemPrompt option.
  let systemPrompt = "";
  const initial = [];
  for (const m of messages) {
    const role = m.role;
    const content = typeof m.content === "string" ? m.content : (Array.isArray(m.content) ? m.content.filter(p => p.type === "text").map(p => p.text).join("") : "");
    if (role === "system") systemPrompt += (systemPrompt ? "\n" : "") + content;
    else initial.push({ role, content });
  }
  // Last user message is the trigger; everything before is history.
  let last = initial.pop();
  if (!last || last.role !== "user") {
    send({ id, type: "error", payload: { message: "last message must be user" }});
    send({ id, type: "done", payload: { finish_reason: "stop" }});
    return;
  }

  let session;
  try {
    const opts = { initialPrompts: initial };
    if (systemPrompt) opts.systemPrompt = systemPrompt;
    if (typeof payload.temperature === "number") opts.temperature = payload.temperature;
    if (typeof payload.top_k === "number") opts.topK = payload.top_k;
    if (payload.response_format && payload.response_format.type === "json_schema" && payload.response_format.json_schema) {
      opts.responseConstraint = payload.response_format.json_schema;
    } else if (payload.response_format && payload.response_format.type === "json_object") {
      opts.responseConstraint = { type: "object" };
    }
    session = await LanguageModel.create(opts);
  } catch (e) {
    send({ id, type: "error", payload: { message: "LanguageModel.create failed: " + String(e) }});
    send({ id, type: "done", payload: { finish_reason: "stop" }});
    return;
  }
  sessions.set(id, session);

  try {
    if (stream) {
      const readable = await session.promptStreaming(last.content);
      const reader = readable.getReader();
      while (true) {
        const { value, done } = await reader.read();
        if (done) break;
        send({ id, type: "chunk", payload: { delta: String(value || "") }});
      }
      send({ id, type: "done", payload: { finish_reason: "stop" }});
    } else {
      const result = await session.prompt(last.content);
      send({ id, type: "chunk", payload: { delta: String(result || "") }});
      send({ id, type: "done", payload: { finish_reason: "stop" }});
    }
  } catch (e) {
    send({ id, type: "error", payload: { message: "prompt failed: " + String(e) }});
    send({ id, type: "done", payload: { finish_reason: "stop" }});
  } finally {
    try { session.destroy?.(); } catch (_) {}
    sessions.delete(id);
  }
}

function connect() {
  try {
    port = chrome.runtime.connectNative(HOST_NAME);
  } catch (e) {
    console.error("[fenster] connectNative failed:", e);
    setTimeout(connect, 2000);
    return;
  }

  port.onMessage.addListener((msg) => {
    handleMessage(msg).catch((e) => {
      console.error("[fenster] handleMessage err:", e);
    });
  });

  port.onDisconnect.addListener(() => {
    const err = chrome.runtime.lastError;
    console.warn("[fenster] disconnected:", err && err.message);
    port = null;
    setTimeout(connect, 2000);
  });

  send({ id: "boot", type: "hello", payload: {
    version: chrome.runtime.getManifest().version,
    LanguageModel: typeof LanguageModel,
  }});
}

self.addEventListener("install", () => self.skipWaiting());
self.addEventListener("activate", () => connect());

// Also connect immediately when the SW evaluates (warm starts).
connect();
