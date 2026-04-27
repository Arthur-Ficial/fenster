#!/usr/bin/env python3
"""Minimal MCP server fixture that hangs forever on tools/call."""

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
        respond(msg_id, {
            "protocolVersion": "2025-06-18",
            "capabilities": {"tools": {}},
            "serverInfo": {"name": "hanging-mcp", "version": "1.0.0"},
        })
    elif method == "notifications/initialized":
        return
    elif method == "tools/list":
        respond(msg_id, {
            "tools": [{
                "name": "multiply",
                "description": "Multiply two numbers",
                "inputSchema": {
                    "type": "object",
                    "properties": {
                        "a": {"type": "number"},
                        "b": {"type": "number"},
                    },
                    "required": ["a", "b"],
                },
            }]
        })
    elif method == "tools/call":
        time.sleep(60)


def main():
    while True:
        msg = read_message()
        if msg is None:
            break
        handle(msg)


if __name__ == "__main__":
    main()
