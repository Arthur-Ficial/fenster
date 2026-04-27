#!/usr/bin/env python3
"""Minimal MCP server fixture that takes 10 seconds to respond to initialize."""

import json
import sys
import time


def read_message():
    line = sys.stdin.readline()
    if not line:
        return None
    return json.loads(line.strip())


def send(msg):
    sys.stdout.write(json.dumps(msg, separators=(",", ":")) + "\n")
    sys.stdout.flush()


def respond(msg_id, result):
    send({"jsonrpc": "2.0", "id": msg_id, "result": result})


def handle(msg):
    method = msg.get("method")
    msg_id = msg.get("id")

    if method == "initialize":
        time.sleep(10)  # Slow startup
        respond(msg_id, {
            "protocolVersion": "2025-06-18",
            "capabilities": {"tools": {}},
            "serverInfo": {"name": "slow-startup-mcp", "version": "1.0.0"},
        })
    elif method == "notifications/initialized":
        return
    elif method == "tools/list":
        respond(msg_id, {"tools": [{
            "name": "echo",
            "description": "Echo back the input",
            "inputSchema": {"type": "object", "properties": {"text": {"type": "string"}}},
        }]})
    elif method == "tools/call":
        args = json.loads(msg.get("params", {}).get("arguments", "{}"))
        respond(msg_id, {"content": [{"type": "text", "text": args.get("text", "")}]})


def main():
    while True:
        msg = read_message()
        if msg is None:
            break
        handle(msg)


if __name__ == "__main__":
    main()
