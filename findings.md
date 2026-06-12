# Findings

## Local Code
- `/v1/chat/completions` is registered as `RelayFormatOpenAI`; `/v1/responses` is registered separately as `RelayFormatOpenAIResponses` in `router/relay-router.go`.
- `relay/helper/valid_request.go` returns `field messages is required` only after parsing an OpenAI-compatible chat request whose relay mode is chat completions and whose `messages` array is empty or absent.
- `relay/helper/valid_request.go` validates `/v1/responses` requests with `input`, not `messages`, via `GetAndValidateResponsesRequest`.
- `dto.GeneralOpenAIRequest` supports `messages`, `reasoning_effort`, and `reasoning`; `dto.OpenAIResponsesRequest` supports `input`, `reasoning.effort`, and related Responses API fields.
- Model mapping happens after request validation in `relay/helper/model_mapped.go`, so a missing `messages` validation error happens before `spt-gpt-5.5 -> gpt-5.5` mapping can matter.

## External Research
- GitHub issue QuantumNous/new-api#2986 is specifically about Cursor compatibility. A maintainer comment says Cursor's request is not standard OpenAI-compatible format, and another user reported traffic appears to be routed through Cursor servers.
- Cursor's official BYOK page says users add keys in Cursor Settings > Models, and supported BYOK providers include OpenAI, Anthropic, Google, Azure OpenAI, and AWS Bedrock.
- Cursor's official BYOK page says API-key requests are routed through Cursor servers for final prompt building, and the key is sent to Cursor's backend with each request but not persisted.
- Cursor's official BYOK page says custom API keys only work with chat models; tab completion remains Cursor built-in.
- Portkey's Cursor integration docs describe the practical custom gateway flow: enable OpenAI API Key, enable Override OpenAI Base URL, add a custom model, enable it, then select it in Chat.
- Portkey's Cursor docs warn not to name the custom model like a real provider model id such as `gpt-4o` or longer strings containing `gpt-4`, because Cursor may recognize/reroute it as a built-in provider model.

## Working Hypotheses
- The reported error likely means Cursor sent a Responses-style payload with `input`, or another non-chat-completions payload, to an endpoint that new-api treated as `/v1/chat/completions`.
- This is unlikely to be caused by missing reasoning/thinking level. Missing `reasoning` would not remove `messages`, and request validation fails before channel model mapping or upstream reasoning handling.
- The model name `spt-gpt-5.5` is risky for Cursor because it contains a `gpt-5.5`-like provider identifier. A neutral name like `spt_gateway` or `acme-ai-gateway` is less likely to trigger Cursor provider-specific logic.
- If new-api is only reachable from the user's LAN/VPN/localhost, Cursor BYOK can fail because requests come from Cursor's backend, not directly from the desktop process.
