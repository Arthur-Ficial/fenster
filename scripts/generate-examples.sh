#!/usr/bin/env bash
set -euo pipefail

OUT="${OUT:-EXAMPLE.md}"
BASE_URL="${FENSTER_BASE_URL:-http://127.0.0.1:11434}"
MODEL="${FENSTER_MODEL:-gemini-nano}"
TIMEOUT="${FENSTER_EXAMPLE_TIMEOUT:-180}"
COUNT=0

require() {
    if ! command -v "$1" >/dev/null 2>&1; then
        echo "error: required command not found: $1" >&2
        exit 1
    fi
}

request_payload() {
    local prompt="$1"
    jq -nc --arg model "$MODEL" --arg content "$prompt" \
        '{model:$model,messages:[{role:"user",content:$content}]}'
}

system_payload() {
    local system="$1"
    local prompt="$2"
    jq -nc --arg model "$MODEL" --arg system "$system" --arg content "$prompt" \
        '{model:$model,messages:[{role:"system",content:$system},{role:"user",content:$content}]}'
}

json_payload() {
    local prompt="$1"
    jq -nc --arg model "$MODEL" --arg content "$prompt" \
        '{model:$model,messages:[{role:"user",content:$content}],response_format:{type:"json_object"}}'
}

stream_payload() {
    local prompt="$1"
    jq -nc --arg model "$MODEL" --arg content "$prompt" \
        '{model:$model,stream:true,stream_options:{include_usage:true},messages:[{role:"user",content:$content}]}'
}

extract_content() {
    jq -r '.choices[0].message.content // .error.message // empty'
}

curl_json() {
    local payload="$1"
    curl -sS --max-time "$TIMEOUT" -X POST "$BASE_URL/v1/chat/completions" \
        -H 'Content-Type: application/json' \
        -d "$payload"
}

write_block() {
    local command="$1"
    local output="$2"
    cat <<BLOCK

\`\`\`
${command}
\`\`\`

\`\`\`\`
$(printf '%s\n' "$output" | fold -s -w 88)
\`\`\`\`

---
BLOCK
}

run_prompt() {
    local prompt="$1"
    local payload
    payload="$(request_payload "$prompt")"
    COUNT=$((COUNT + 1))
    printf "  [%d] %s ...\n" "$COUNT" "${prompt:0:72}" >&2
    local raw
    raw="$(curl_json "$payload")"
    local output
    output="$(printf '%s' "$raw" | extract_content)"
    write_block "\$ curl -sS -X POST ${BASE_URL}/v1/chat/completions \\
  -H 'Content-Type: application/json' \\
  -d '${payload}' | jq -r '.choices[0].message.content'" "$output"
}

run_system() {
    local system="$1"
    local prompt="$2"
    local payload
    payload="$(system_payload "$system" "$prompt")"
    COUNT=$((COUNT + 1))
    printf "  [%d] system + %s ...\n" "$COUNT" "${prompt:0:64}" >&2
    local raw
    raw="$(curl_json "$payload")"
    local output
    output="$(printf '%s' "$raw" | extract_content)"
    write_block "\$ curl -sS -X POST ${BASE_URL}/v1/chat/completions \\
  -H 'Content-Type: application/json' \\
  -d '${payload}' | jq -r '.choices[0].message.content'" "$output"
}

run_json_mode() {
    local prompt="$1"
    local payload
    payload="$(json_payload "$prompt")"
    COUNT=$((COUNT + 1))
    printf "  [%d] json mode: %s ...\n" "$COUNT" "${prompt:0:64}" >&2
    local raw
    raw="$(curl_json "$payload")"
    local output
    output="$(printf '%s' "$raw" | extract_content)"
    write_block "\$ curl -sS -X POST ${BASE_URL}/v1/chat/completions \\
  -H 'Content-Type: application/json' \\
  -d '${payload}' | jq -r '.choices[0].message.content'" "$output"
}

run_stream() {
    local prompt="$1"
    local payload
    payload="$(stream_payload "$prompt")"
    COUNT=$((COUNT + 1))
    printf "  [%d] stream: %s ...\n" "$COUNT" "${prompt:0:64}" >&2
    local output
    output="$(curl -sS --max-time "$TIMEOUT" -N -X POST "$BASE_URL/v1/chat/completions" \
        -H 'Content-Type: application/json' \
        -d "$payload")"
    write_block "\$ curl -sS -N -X POST ${BASE_URL}/v1/chat/completions \\
  -H 'Content-Type: application/json' \\
  -d '${payload}'" "$output"
}

require curl
require jq

echo "Checking ${BASE_URL}/health ..." >&2
HEALTH="$(curl -sS --max-time 10 "$BASE_URL/health")"
if [[ "$(printf '%s' "$HEALTH" | jq -r '.model_available')" != "true" ]]; then
    echo "error: fenster health did not report model_available=true" >&2
    printf '%s\n' "$HEALTH" >&2
    exit 1
fi

PROOF_PAYLOAD="$(request_payload "Capital of Austria? One word.")"
PROOF_RAW="$(curl_json "$PROOF_PAYLOAD")"
PROOF_CONTENT="$(printf '%s' "$PROOF_RAW" | extract_content)"
if [[ -z "$PROOF_CONTENT" || "$PROOF_CONTENT" == Echo:* ]]; then
    echo "error: proof prompt did not hit the real model" >&2
    printf '%s\n' "$PROOF_RAW" >&2
    exit 1
fi

OS_VER="$(sw_vers -productVersion 2>/dev/null || uname -sr)"
CHIP="$(sysctl -n machdep.cpu.brand_string 2>/dev/null | sed 's/^Apple //')"
DATE="$(date -u +"%Y-%m-%d")"
VERSION="$(printf '%s' "$HEALTH" | jq -r '.version')"
LANGS="$(printf '%s' "$HEALTH" | jq -r '.supported_languages | join(", ")')"

echo "Generating ${OUT} from real fenster responses ..." >&2
{
cat <<HEADER
# Real Examples - Gemini Nano through fenster

Every response below is **real, unedited output** from Chrome's on-device Gemini Nano
through \`fenster\`'s OpenAI-compatible HTTP server. The generator fails if the
server is the echo backend; these examples were produced from a live server with
\`model_available=true\`.

This file was generated by \`scripts/generate-examples.sh\`.

> fenster v${VERSION} | ${MODEL} | ${OS_VER} | ${CHIP} | ${DATE} | languages: ${LANGS}

## Table of Contents

1. [Server Health](#1-server-health)
2. [Basic Q&A](#2-basic-qa)
3. [Reasoning and Math](#3-reasoning-and-math)
4. [Coding](#4-coding)
5. [Translation](#5-translation)
6. [System Prompt, JSON, and Streaming](#6-system-prompt-json-and-streaming)

---

## 1. Server Health

\`\`\`
\$ curl -sS ${BASE_URL}/health | jq .
\`\`\`

\`\`\`\`json
$(printf '%s' "$HEALTH" | jq .)
\`\`\`\`

---

## 2. Basic Q&A
HEADER

run_prompt "Capital of Austria? One word."
run_prompt "Explain what fenster does in two concise sentences."
run_prompt "What is the speed of light in km/s? Give the standard rounded value."

echo "## 3. Reasoning and Math"
run_prompt "What is 17 times 23? Return just the answer and one short check."
run_prompt "If all roses are flowers and some flowers fade quickly, do all roses fade quickly? Explain in two sentences."
run_prompt "A notebook costs 3 euros. A pen costs 2 euros more than the notebook. What is the total cost of one notebook and one pen?"

echo "## 4. Coding"
run_prompt "Write a Python function named is_prime that checks whether an integer is prime."
run_prompt "Find the bug in this Python code and show the corrected line: for i in range(10): if i = 5: print(i)"
run_prompt "Explain binary search time complexity in one short paragraph."

echo "## 5. Translation"
run_prompt "Translate to German: The command finished without errors."
run_prompt "Translate to Japanese: Hello, how are you?"
run_prompt "Translate to Spanish: The weather is beautiful today."

echo "## 6. System Prompt, JSON, and Streaming"
run_system "Reply with terse shell-style answers. No preamble." "What is recursion?"
run_json_mode "Return a compact JSON object with keys city and country for Vienna."
run_stream "Count from 1 to 5, separated by commas."
} > "$OUT"

echo "" >&2
echo "Done: ${COUNT} model examples written to ${OUT}" >&2
