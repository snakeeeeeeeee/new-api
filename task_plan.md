# Cursor Custom Base URL Investigation

## Goal
Explain why Cursor calling this new-api deployment returns `field messages is required`, and document how Cursor should be configured with a custom OpenAI-compatible base URL, API key, and model mapping.

## Phases
- [complete] Inspect local routing and relay request parsing for chat/completions vs responses behavior.
- [complete] Research Cursor custom API/base URL/model behavior from public sources.
- [complete] Correlate findings with the reported `spt-gpt-5.5 -> gpt-5.5` mapping and likely request shape.
- [complete] Provide actionable configuration and debugging steps.

## Decisions
- No business code edits planned unless a concrete compatibility issue is confirmed.
- The likely fix is configuration/routing, not a code change: use a neutral Cursor custom model name, make the gateway publicly reachable, and ensure Cursor's request lands on the matching OpenAI-compatible endpoint.

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| GitHub search URL interpreted by zsh glob | `curl` without quoting query URL | Retried with quoted/encoded URL. |
