# Progress

## 2026-06-12
- Created investigation plan files.
- Starting local code inspection and Cursor public documentation search.
- Found local source of `field messages is required`: OpenAI chat-completions validation in `relay/helper/valid_request.go`.
- Found separate `/v1/responses` route and DTO using `input`.
- Found new-api GitHub issue #2986 about Cursor compatibility; maintainer says Cursor request format is not standard OpenAI-compatible.
- Confirmed Cursor official BYOK docs: configure API keys in Cursor Settings > Models; requests route through Cursor servers for prompt building.
- Confirmed Portkey's Cursor integration flow: OpenAI API Key + Override OpenAI Base URL + custom model + select that model in Chat.
- Marked investigation phases complete.
