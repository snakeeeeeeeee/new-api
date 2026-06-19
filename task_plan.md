# Task Plan: Claude/Bedrock OpenAI Tool Call Compatibility Fix

## Goal
Implement the requested Claude/OpenAI tool-call compatibility switch and fix OpenAI ChatCompletions `tool_calls/tool_call_id` histories so Claude/Bedrock upstreams do not receive `TOOL_USE_RESULT_MISMATCH` request bodies.

## Current Phase
Complete

## Phases
- [complete] Inspect supplied error dump and confirm raw OpenAI message/tool pairing.
- [complete] Trace OpenAI -> Claude -> AWS Bedrock conversion for assistant `content=null` plus `tool_calls`.
- [complete] Check failover/body reuse path and context trimming/normalization behavior.
- [complete] Implement global Claude compatibility switch and converter repair.
- [complete] Add unit coverage and frontend compatibility switch.
- [complete] Run docker dev integration test with the admin-created token.

## Key Questions
1. Does `RequestOpenAI2ClaudeMessage` preserve assistant messages whose `content` is null but `tool_calls` is non-empty?
2. Does any Claude compatibility normalization reorder, merge, or remove the `tool_use` immediately before Bedrock sees the body?
3. Does retry/failover reuse an already converted/cropped body instead of the original OpenAI body?
4. Is the Bedrock error index consistent with current conversion producing `messages.72` as a `tool_result` without a prior assistant `tool_use`?

## Decisions Made
| Decision | Rationale |
| --- | --- |
| Read-only investigation first | User explicitly asked to analyze and not directly modify code. |
| Add `claude.openai_tool_call_compat_enabled` default true | User requested a rollback switch in Compatibility Management with online default enabled. |
| Preserve `tool_use` by wrapping non-object or invalid arguments | Prevents losing the assistant `tool_use` while later `tool_result` remains. |
| Convert AWS pass-through OpenAI chat bodies before Bedrock | Pass-through previously could bypass OpenAI->Claude conversion in AWS Claude final format. |

## Errors Encountered
| Error | Attempt | Resolution |
| --- | --- | --- |
| Existing plan files described an older Cursor investigation | Initial read | Replaced planning notes with current Bedrock tool mismatch investigation notes. |
| Temporary probe compile failed due to `/dev/stdin` outside module | First probe attempt | Created a temporary file under `tmp/`, ran it, then deleted it. |
| Temporary probe initially used wrong helper signatures | Second probe attempt | Switched to correct `common.Any2Type[T]` signature and JSON decoder for multi-object dump parsing. |
| Existing Claude merge test expected orphan tool results to remain | First test run after implementation | Disabled the new compat switch in that legacy test, because the new default intentionally removes orphan tool results. |
| i18n insertion initially put two JSON keys on one line | Frontend locale update | Repaired all locale files with one key per line and reran frontend build. |
