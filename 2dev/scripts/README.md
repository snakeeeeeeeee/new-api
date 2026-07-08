# Diagnostic Scripts

This directory contains local diagnostic tools for relay/upstream behavior.

## Prompt Token Overhead Probe

Use `prompt_injection_probe.py` to compare the same request across two or more
targets. It checks whether one target reports a stable extra input-token
overhead.

```bash
export BASELINE_KEY='sk-baseline'
export SUSPECT_KEY='sk-suspect'

python3 2dev/scripts/prompt_injection_probe.py \
  --target baseline anthropic x-api-key https://supertoken.cc env:BASELINE_KEY claude-opus-4-7 \
  --target suspect anthropic bearer https://supertoken.cc env:SUSPECT_KEY claude-opus-4-7 \
  --baseline baseline \
  --repeats 2 \
  --jsonl ./tmp/prompt-probe.jsonl
```

For a stronger leak attempt on only the suspect target:

```bash
python3 2dev/scripts/prompt_injection_probe.py \
  --target baseline anthropic x-api-key https://supertoken.cc env:BASELINE_KEY claude-opus-4-7 \
  --target suspect anthropic bearer https://supertoken.cc env:SUSPECT_KEY claude-opus-4-7 \
  --baseline baseline \
  --repeats 2 \
  --aggressive-leak-probes \
  --leak-target suspect \
  --leak-max-tokens 1200 \
  --jsonl ./tmp/prompt-probe-aggressive.jsonl
```

Read `effective_input`, not only `input`. It includes Anthropic cache fields
when present.

## Prompt Leak Conversation Probe

Use `prompt_leak_conversation_probe.py` to run several turns in one or more new
conversations and detect whether hidden/system prompt text can be elicited.

```bash
export TEST_KEY='sk-test'

python3 2dev/scripts/prompt_leak_conversation_probe.py \
  --provider anthropic \
  --auth bearer \
  --url https://supertoken.cc \
  --api-key env:TEST_KEY \
  --model claude-opus-4-7 \
  --sessions 2 \
  --max-tokens 1200 \
  --jsonl ./tmp/leak-probe.jsonl
```

OpenAI-compatible endpoint example:

```bash
export TEST_KEY='sk-test'

python3 2dev/scripts/prompt_leak_conversation_probe.py \
  --provider openai \
  --auth bearer \
  --url https://your-gateway.example.com \
  --api-key env:TEST_KEY \
  --model claude-opus-4-7 \
  --sessions 2 \
  --jsonl ./tmp/leak-probe-openai.jsonl
```

The verdict is a black-box signal. A strong leak signal means the model repeatedly
returned non-user instruction text, but it does not prove the exact internal
request body of an upstream provider. To prove what this gateway sends, capture
the final outbound request body before it leaves new-api or use mitmproxy on the
new-api outbound connection.

## Codex Deferred Tool Diagnose

Use `codex_tool_diagnose.py` when Codex says it will use Chrome/Node tools but
then stops without doing work. The script can start request dump, run a Codex CLI
repro, collect events, and report whether `tool_search` was lost by the gateway,
kept upstream but skipped by the model, or never sent by the client.

Live capture:

```bash
export CODEX_API_KEY='sk-test'

python3 2dev/scripts/codex_tool_diagnose.py live \
  --admin-base-url http://localhost:3001 \
  --admin-header 'Authorization: Bearer <admin-token>' \
  --admin-user-id 1 \
  --api-base-url https://your-new-api.example.com/v1 \
  --model gpt-5.5 \
  --dump-user-ids 14 \
  --dump-token-names lyc-codex
```

The live mode writes artifacts under `tmp/codex-tool-diagnose-*`, including
`dump-events.json`, `codex-events.jsonl`, `report.json`, and `report.md`. It
uses a temporary `CODEX_HOME`, so the target API key does not overwrite the
current Codex config.

Analyze existing dump/Codex files:

```bash
python3 2dev/scripts/codex_tool_diagnose.py analyze \
  --events-file ./tmp/dump-events.json \
  --codex-jsonl ./tmp/codex-events.jsonl
```

Quick analyzer test:

```bash
python3 2dev/scripts/codex_tool_diagnose.py self-test
```
