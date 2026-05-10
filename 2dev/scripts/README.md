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
