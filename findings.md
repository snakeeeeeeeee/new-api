# Findings

## Requirements
- Analyze a new-api failover bug where an OpenAI ChatCompletions streaming request from Codex Desktop fails on a backup Claude/Bedrock channel.
- Do not edit business code in this pass.
- Focus on assistant `content=null` plus `tool_calls`, tool result conversion, consecutive assistant message merging, failover reuse, and context trimming atomicity.

## Error Dump
- Target `request_id`: `20260619014235551618433BwdNyxFx`.
- First attempt: `channel_id=77`, `channel_name=doge_kiro-bug`, status `429`, upstream says server busy.
- Failover attempts: `channel_id=102`, `channel_name=self-local_claude-re-kiro`, status `502`.
- Bedrock error: `TOOL_USE_RESULT_MISMATCH`, message says `messages.72.content` has more `toolResult` blocks than previous turn `toolUse` blocks.
- The logged raw body exists only on the first 429 record; the failover 502 records in the supplied dump do not include raw bodies.

## Raw OpenAI Request
- Parsed raw body has `model=claude-opus-4-8`, `stream=true`, `messages=168`, `tools=21`.
- The OpenAI input has 43 assistant tool calls and 43 tool results.
- Every `role=tool` `tool_call_id` in the raw OpenAI input matches some assistant `tool_calls[].id`; no global missing IDs found.
- Around indices 68-73:
  - `messages[68]` assistant text.
  - `messages[69]` assistant `content=null`, `tool_calls[0].id=tooluse_yV0i4a3eYAOVJSuU6zCU6w`.
  - `messages[70]` tool result for that id.
  - `messages[71]` assistant text.
  - `messages[72]` assistant `content=null`, `tool_calls[0].id=tooluse_ctlGdCLQDMMUmI2OLhaPOZ`.
  - `messages[73]` tool result for that id.

## Conversion Code
- `relay/channel/aws/adaptor.go` handles OpenAI -> Claude for Bedrock by calling `claude.RequestOpenAI2ClaudeMessage`, then `relaycommon.NormalizeClaudeRequestCompat`.
- `relay/channel/claude/relay-claude.go` currently copies assistant `ToolCalls` into an intermediate `dto.Message` when `message.Role == "assistant" && message.ToolCalls != nil`.
- The same function replaces any `fmtMessage.Content == nil` with string content `"..."` before Claude conversion.
- If a message has `ToolCalls != nil`, the converter parses message content and appends `tool_use` blocks from `ParseToolCalls`.
- This suggests assistant `content=null` plus `tool_calls` is not obviously dropped in the first OpenAI -> Claude conversion pass.
- A temporary local probe against the supplied raw body produced 136 converted Claude messages from 168 OpenAI messages.
- The converted output contains repeated sequences of `assistant` text, then another `assistant` message containing `tool_use`, then a `user` message containing `tool_result`.
- `relaycommon.NormalizeClaudeRequestCompat` rejects that converted request locally with `claude_tool_result_mismatch` when `ToolProtocolValidationMode=reject`.
- The probe also logged `tool call function arguments is not a map[string]any` for at least one `tool_call`; the converter currently `continue`s on argument parse failure, which drops the `tool_use` block while leaving the later `tool_result`.

## Retry And Pass-Through
- `controller/relay.go` retry loop restores `c.Request.Body` from `BodyStorage` for every attempt, and `TextHelper` deep-copies `info.Request`. Normal retry/failover does not appear to reuse the previously converted request object.
- If global/channel pass-through is enabled, `TextHelper` can bypass `ConvertOpenAIRequest`.
- `shouldApplyClaudeCompatToOpenAIPassthrough` currently applies only when `info.ApiType == APITypeAnthropic`, not `APITypeAws`; AWS pass-through can therefore bypass OpenAI->Claude conversion/compat handling in `TextHelper`.
- `relay/channel/aws/relay-aws.go` has a second pass-through branch in `buildAwsRequestBody` that reads original body storage again and deletes only `model`/`stream` before forwarding, unless Claude passthrough compat is enabled.

## Files Of Interest
- `/Users/zhangyu/code/go/new-api/relay/channel/claude/relay-claude.go`
- `/Users/zhangyu/code/go/new-api/relay/channel/aws/adaptor.go`
- `/Users/zhangyu/code/go/new-api/relay/channel/aws/relay-aws.go`
- `/Users/zhangyu/code/go/new-api/relay/common` normalization files
- `/Users/zhangyu/code/go/new-api/middleware/distributor.go`
- `/Users/zhangyu/code/go/new-api/relay/relay.go` or equivalent relay retry path

## Working Hypotheses
- The raw OpenAI request is paired; the mismatch is likely introduced after parsing.
- The most likely areas are Claude compatibility normalization/reordering after conversion, or retry/failover request-body reuse where pass-through mode may read original OpenAI body in a Claude/Bedrock request path.
- Bedrock's `messages.72.content` may refer to the converted Claude message index, not raw OpenAI index.

## Current Conclusion
- The strongest reproduced root cause is not raw OpenAI pairing loss; it is conversion loss.
- `relay/channel/claude/relay-claude.go` silently skips a `tool_use` when `toolCall.Function.Arguments` cannot unmarshal into `map[string]any`.
- In the supplied request, `openai[130]` has malformed/non-object `function.arguments`: `{"code" title="Open health page in browser": ...}`. That means the converter appends no `tool_use` for `tooluse_9TMs5EoTCfvHXTAJTKSkUI`, while `openai[131]` still becomes a `tool_result`.
- This reproduces local `claude_tool_result_mismatch` before Bedrock: `messages.100.content.0.tool_use_id` references a missing previous `tool_use`.
- Separately, the converter leaves repeated `assistant text -> assistant tool_use -> user tool_result` sequences because it only merges adjacent same-role messages when both original contents are strings. This is risky for Bedrock/Anthropic tool protocol and should be normalized by merging assistant text blocks with following assistant tool_use blocks.
- Failover itself appears less likely as the direct cause: retry restores the original request body and deep-copies `info.Request` per attempt. However, AWS pass-through settings can bypass expected OpenAI->Claude conversion and compat handling, so channel 102 settings matter.

## Implemented Fix
- Added global option `claude.openai_tool_call_compat_enabled`, default `true`, exposed in Compatibility Management under Claude compatibility.
- OpenAI->Claude converter now preserves assistant `tool_calls` even when `content=null`.
- Tool-call `arguments` conversion now follows the compatibility rules:
  - empty string -> `{}`;
  - JSON object -> original object;
  - JSON array/string/number/bool/null -> `{"value": parsed}`;
  - invalid JSON -> `{"_raw_arguments": raw}` with truncated/masked logging.
- Adjacent assistant text plus assistant tool calls are normalized into one Claude assistant message containing text and `tool_use` blocks.
- Consecutive matching tool results are merged into one Claude user message.
- Orphan/incomplete tool interaction groups are trimmed before the final Claude validator runs.
- AWS/Bedrock pass-through now detects OpenAI chat-style request bodies and converts them through the OpenAI->Claude converter before deleting `model/stream` and sending to Bedrock.
