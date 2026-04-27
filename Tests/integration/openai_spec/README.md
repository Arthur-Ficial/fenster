# Vendored OpenAI API Spec

Source: https://github.com/openai/openai-openapi (branch `2025-03-21`)

Downloaded: 2026-04-12

Used by `openapi_conformance_test.py` to validate apfel's HTTP responses against
the official OpenAI API schema at runtime. Vendored locally so tests are hermetic
(no network fetch at test time).

To refresh: `curl -sL https://raw.githubusercontent.com/openai/openai-openapi/2025-03-21/openapi.yaml -o openapi.yaml`
