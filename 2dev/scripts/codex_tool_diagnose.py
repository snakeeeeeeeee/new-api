#!/usr/bin/env python3
"""
Automate Codex deferred-tool diagnostics against a new-api request dump.

The live mode starts request_dump, runs a Codex CLI repro against the target
base_url/model/key, collects dump events, then prints a short diagnosis. The
analyze-only mode accepts existing dump/Codex JSONL files and runs the same
diagnosis locally.
"""

from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request
from dataclasses import dataclass
from pathlib import Path
from typing import Any


DEFAULT_PROMPT = """请使用 Chrome 插件读取当前 Chrome 浏览器所有标签页的标题和 URL。
要求：必须实际调用可用工具完成，不要只描述计划；如果 Chrome 不可用，请在工具调用返回错误后再说明。"""

TERMINAL_RESPONSE_EVENTS = {
    "response.completed",
    "response.failed",
    "response.error",
    "response.incomplete",
    "response.cancelled",
}

SECRET_PATTERNS = [
    re.compile(r"sk-[A-Za-z0-9_\-]{12,}"),
    re.compile(r"Bearer\s+[A-Za-z0-9._\-]{12,}", re.IGNORECASE),
    re.compile(r"(OPENAI_API_KEY|api[_-]?key|authorization)(['\"\s:=]+)([^'\"\s,}]{8,})", re.IGNORECASE),
]


@dataclass
class HttpResult:
    status: int
    parsed: Any
    text: str


def redact(text: Any) -> str:
    value = "" if text is None else str(text)
    for pattern in SECRET_PATTERNS:
        if pattern.pattern.startswith("(OPENAI"):
            value = pattern.sub(r"\1\2<redacted>", value)
        else:
            value = pattern.sub(lambda m: m.group(0).split()[0] + " <redacted>" if m.group(0).lower().startswith("bearer") else "<redacted>", value)
    return value


def json_dumps(data: Any, *, indent: int | None = 2) -> str:
    return json.dumps(data, ensure_ascii=False, indent=indent, sort_keys=False)


def load_api_key(value: str | None) -> str:
    if not value:
        value = os.environ.get("CODEX_API_KEY")
    if not value:
        raise SystemExit("missing API key: pass --api-key env:NAME or set CODEX_API_KEY")
    if value.startswith("env:"):
        env_name = value[4:]
        env_value = os.environ.get(env_name)
        if not env_value:
            raise SystemExit(f"environment variable {env_name} is empty")
        return env_value
    return value


def split_csv_int(value: str | None) -> list[int]:
    if not value:
        return []
    result: list[int] = []
    for part in value.split(","):
        part = part.strip()
        if not part:
            continue
        result.append(int(part))
    return result


def split_csv_str(value: str | None) -> list[str]:
    if not value:
        return []
    return [part.strip() for part in value.split(",") if part.strip()]


def parse_header(value: str) -> tuple[str, str]:
    if ":" in value:
        key, raw = value.split(":", 1)
    elif "=" in value:
        key, raw = value.split("=", 1)
    else:
        raise SystemExit(f"invalid header {value!r}; use 'Name: value'")
    key = key.strip()
    raw = raw.strip()
    if not key:
        raise SystemExit(f"invalid header {value!r}; header name is empty")
    return key, raw


def admin_headers(args: argparse.Namespace) -> dict[str, str]:
    headers: dict[str, str] = {}
    for raw in args.admin_header or []:
        key, value = parse_header(raw)
        headers[key] = value
    env_auth = os.environ.get("NEWAPI_ADMIN_AUTH_HEADER")
    if env_auth and "Authorization" not in {k.title(): v for k, v in headers.items()}:
        key, value = parse_header(env_auth)
        headers[key] = value
    if args.admin_cookie or os.environ.get("NEWAPI_ADMIN_COOKIE"):
        headers["Cookie"] = args.admin_cookie or os.environ["NEWAPI_ADMIN_COOKIE"]
    if args.admin_user_id or os.environ.get("NEWAPI_ADMIN_USER_ID"):
        headers["New-Api-User"] = str(args.admin_user_id or os.environ["NEWAPI_ADMIN_USER_ID"])
    return headers


def decode_response(raw: bytes) -> tuple[Any, str]:
    text = raw.decode("utf-8", errors="replace")
    if not text:
        return None, ""
    try:
        return json.loads(text), text
    except json.JSONDecodeError:
        return None, text


def http_json(method: str, base_url: str, path: str, headers: dict[str, str], payload: Any | None = None, timeout: int = 30) -> HttpResult:
    body = None
    request_headers = dict(headers)
    if payload is not None:
        body = json.dumps(payload, ensure_ascii=False, separators=(",", ":")).encode("utf-8")
        request_headers.setdefault("Content-Type", "application/json")
    url = base_url.rstrip("/") + path
    req = urllib.request.Request(url, data=body, method=method, headers=request_headers)
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            raw = resp.read()
            parsed, text = decode_response(raw)
            return HttpResult(resp.status, parsed, text)
    except urllib.error.HTTPError as exc:
        raw = exc.read()
        parsed, text = decode_response(raw)
        return HttpResult(exc.code, parsed, text)


def unwrap_api_data(parsed: Any) -> Any:
    if isinstance(parsed, dict) and "success" in parsed and "data" in parsed:
        if parsed.get("success") is not True:
            raise RuntimeError(parsed.get("message") or parsed)
        return parsed.get("data")
    return parsed


def request_dump_rule(args: argparse.Namespace) -> dict[str, Any]:
    user_ids = split_csv_int(args.dump_user_ids or os.environ.get("DUMP_USER_IDS"))
    if not user_ids:
        raise SystemExit("request_dump needs user id filter: pass --dump-user-ids 123 or set DUMP_USER_IDS")
    models = split_csv_str(args.dump_models)
    if not models and args.model:
        models = [args.model]
    rule: dict[str, Any] = {
        "user_ids": user_ids,
        "duration_seconds": args.dump_duration,
        "max_count": args.dump_max_count,
        "print_on": "all",
        "log_level": args.dump_log_level,
        "print_url": True,
        "print_headers": args.dump_headers,
        "print_body": args.dump_body,
        "print_upstream_body": True,
        "max_body_bytes": args.dump_max_body_bytes,
        "trace_responses_stream": True,
        "trace_responses_stream_key_events_only": args.dump_key_events_only,
        "max_stream_events_per_request": args.dump_max_stream_events,
    }
    optional_fields = {
        "token_names": split_csv_str(args.dump_token_names or os.environ.get("DUMP_TOKEN_NAMES")),
        "models": models,
        "paths": split_csv_str(args.dump_paths),
        "aggregate_groups": split_csv_str(args.dump_aggregate_groups),
        "keywords": split_csv_str(args.dump_keywords),
    }
    for key, value in optional_fields.items():
        if value:
            rule[key] = value
    return rule


def toml_string(value: str) -> str:
    return json.dumps(value)


def prepare_temp_codex_home(source_home: Path, api_key: str, output_dir: Path) -> Path:
    target = output_dir / "codex-home"
    target.mkdir(parents=True, exist_ok=True)
    source_config = source_home / "config.toml"
    if source_config.exists():
        shutil.copy2(source_config, target / "config.toml")
    else:
        (target / "config.toml").write_text("", encoding="utf-8")
    (target / "auth.json").write_text(json.dumps({"OPENAI_API_KEY": api_key}, indent=2), encoding="utf-8")

    # Keep the repro home small. The plugin cache is read by Codex, while runtime
    # state such as sessions/logs stays isolated in the temporary home.
    source_plugin_cache = source_home / "plugins" / "cache"
    if source_plugin_cache.exists():
        plugin_dir = target / "plugins"
        plugin_dir.mkdir(exist_ok=True)
        cache_link = plugin_dir / "cache"
        if not cache_link.exists():
            cache_link.symlink_to(source_plugin_cache, target_is_directory=True)
    for name in ("skills", "rules"):
        source = source_home / name
        link = target / name
        if source.exists() and not link.exists():
            link.symlink_to(source, target_is_directory=True)
    return target


def run_codex_repro(args: argparse.Namespace, output_dir: Path) -> tuple[int, Path]:
    api_key = load_api_key(args.api_key)
    source_home = Path(args.source_codex_home or os.environ.get("CODEX_HOME") or Path.home() / ".codex").expanduser()
    codex_home = prepare_temp_codex_home(source_home, api_key, output_dir)
    workdir = Path(args.workdir or output_dir / "work").expanduser()
    workdir.mkdir(parents=True, exist_ok=True)
    provider = "codex_tool_diagnose"
    prompt = args.prompt or DEFAULT_PROMPT
    command = [
        args.codex_binary,
        "exec",
        "--json",
        "--ephemeral",
        "--skip-git-repo-check",
        "--dangerously-bypass-approvals-and-sandbox",
        "-C",
        str(workdir),
        "-m",
        args.model,
        "-c",
        f"model_provider={toml_string(provider)}",
        "-c",
        f"model_providers.{provider}.name={toml_string(provider)}",
        "-c",
        f"model_providers.{provider}.base_url={toml_string(args.api_base_url.rstrip('/'))}",
        "-c",
        f"model_providers.{provider}.wire_api={toml_string('responses')}",
        "-c",
        f"model_providers.{provider}.requires_openai_auth=true",
        prompt,
    ]
    env = os.environ.copy()
    env["CODEX_HOME"] = str(codex_home)
    env["OPENAI_API_KEY"] = api_key
    codex_jsonl = output_dir / "codex-events.jsonl"
    started = time.time()
    try:
        proc = subprocess.run(
            command,
            cwd=str(workdir),
            env=env,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            timeout=args.codex_timeout,
            check=False,
        )
        output = proc.stdout or ""
        rc = proc.returncode
    except subprocess.TimeoutExpired as exc:
        output = (exc.stdout or "") + (exc.stderr or "")
        rc = 124
        output += f"\n[codex_tool_diagnose] Codex timed out after {args.codex_timeout}s\n"
    elapsed = int((time.time() - started) * 1000)
    codex_jsonl.write_text(redact(output) + f"\n[codex_tool_diagnose] return_code={rc} elapsed_ms={elapsed}\n", encoding="utf-8")
    return rc, codex_jsonl


def load_events_file(path: Path) -> list[dict[str, Any]]:
    text = path.read_text(encoding="utf-8", errors="replace")
    stripped = text.strip()
    if not stripped:
        return []
    try:
        parsed = json.loads(stripped)
        data = unwrap_api_data(parsed)
        if isinstance(data, dict) and isinstance(data.get("events"), list):
            return [item for item in data["events"] if isinstance(item, dict)]
        if isinstance(data, dict) and isinstance(data.get("stage"), str):
            return [data]
        if isinstance(data, list):
            return [item for item in data if isinstance(item, dict)]
    except Exception:
        pass

    concatenated = load_concatenated_json_objects(stripped)
    if concatenated:
        events: list[dict[str, Any]] = []
        for item in concatenated:
            data = unwrap_api_data(item)
            if isinstance(data, dict) and isinstance(data.get("events"), list):
                events.extend(event for event in data["events"] if isinstance(event, dict))
            elif isinstance(data, dict) and isinstance(data.get("stage"), str):
                events.append(data)
        if events:
            return events

    events: list[dict[str, Any]] = []
    for line in text.splitlines():
        candidate = line.strip()
        if not candidate:
            continue
        if "request_dump " in candidate:
            candidate = candidate.split("request_dump ", 1)[1].strip()
        try:
            item = json.loads(candidate)
        except json.JSONDecodeError:
            continue
        data = unwrap_api_data(item)
        if isinstance(data, dict) and isinstance(data.get("events"), list):
            events.extend(item for item in data["events"] if isinstance(item, dict))
        elif isinstance(data, dict):
            events.append(data)
    return events


def load_concatenated_json_objects(text: str) -> list[Any]:
    decoder = json.JSONDecoder()
    items: list[Any] = []
    index = 0
    length = len(text)
    while index < length:
        while index < length and text[index].isspace():
            index += 1
        if index >= length:
            break
        if text[index] not in "[{":
            next_start = min([pos for pos in (text.find("{", index + 1), text.find("[", index + 1)) if pos != -1], default=-1)
            if next_start == -1:
                break
            index = next_start
        try:
            item, next_index = decoder.raw_decode(text, index)
        except json.JSONDecodeError:
            next_start = min([pos for pos in (text.find("{", index + 1), text.find("[", index + 1)) if pos != -1], default=-1)
            if next_start == -1:
                break
            index = next_start
            continue
        items.append(item)
        index = next_index
    return items


def load_codex_jsonl(path: Path | None) -> list[dict[str, Any]]:
    if not path:
        return []
    items: list[dict[str, Any]] = []
    text = path.read_text(encoding="utf-8", errors="replace")
    for line in text.splitlines():
        line = line.strip()
        if not line.startswith("{"):
            continue
        try:
            parsed = json.loads(line)
        except json.JSONDecodeError:
            continue
        if isinstance(parsed, dict):
            items.append(parsed)
    return items


def find_tool_arrays(obj: Any) -> list[list[Any]]:
    arrays: list[list[Any]] = []
    if isinstance(obj, dict):
        tools = obj.get("tools")
        if isinstance(tools, list):
            arrays.append(tools)
        for value in obj.values():
            arrays.extend(find_tool_arrays(value))
    elif isinstance(obj, list):
        for value in obj:
            arrays.extend(find_tool_arrays(value))
    return arrays


def summarize_tools_from_body(text: str) -> dict[str, Any]:
    summary: dict[str, Any] = {
        "parse_ok": False,
        "tool_names": [],
        "tool_search_available": False,
        "shell_command_available": False,
        "node_repl_mentioned": False,
    }
    if not text:
        return summary
    summary["tool_search_available"] = "tool_search" in text
    summary["shell_command_available"] = "shell_command" in text
    summary["node_repl_mentioned"] = "node_repl" in text
    try:
        root = json.loads(text)
    except json.JSONDecodeError:
        return summary
    summary["parse_ok"] = True
    names: list[str] = []
    for tools in find_tool_arrays(root):
        for raw_tool in tools:
            if not isinstance(raw_tool, dict):
                continue
            tool_type = str(raw_tool.get("type") or "").strip()
            name = str(raw_tool.get("name") or "").strip()
            function = raw_tool.get("function")
            if not name and isinstance(function, dict):
                name = str(function.get("name") or "").strip()
            if not name:
                name = tool_type
            label = f"{tool_type}:{name}" if tool_type and name else name or tool_type
            if label and label not in names:
                names.append(label)
    names.sort()
    joined = "\n".join(names)
    summary["tool_names"] = names
    summary["tool_search_available"] = summary["tool_search_available"] or "tool_search" in joined
    summary["shell_command_available"] = summary["shell_command_available"] or "shell_command" in joined
    summary["node_repl_mentioned"] = summary["node_repl_mentioned"] or "node_repl" in joined
    return summary


def bool_from_summary(summary: Any, key: str) -> bool:
    return isinstance(summary, dict) and summary.get(key) is True


def text_contains(obj: Any, needle: str) -> bool:
    return needle in json.dumps(obj, ensure_ascii=False, sort_keys=True)


def analyze_codex_items(items: list[dict[str, Any]]) -> dict[str, Any]:
    command_count = 0
    mcp_calls: list[str] = []
    agent_messages: list[str] = []
    chrome_skill_read = False
    tool_search_text = False
    node_repl_text = False
    for event in items:
        item = event.get("item")
        if not isinstance(item, dict):
            if text_contains(event, "tool_search"):
                tool_search_text = True
            if text_contains(event, "node_repl"):
                node_repl_text = True
            continue
        item_type = item.get("type")
        if item_type == "command_execution":
            command_count += 1
            command = str(item.get("command") or "")
            output = str(item.get("aggregated_output") or "")
            if "chrome" in command.lower() and "SKILL.md" in command:
                chrome_skill_read = True
            if "control-chrome" in output or "Chrome Plugin" in output:
                chrome_skill_read = True
        elif item_type == "mcp_tool_call":
            server = str(item.get("server") or "")
            tool = str(item.get("tool") or "")
            label = f"{server}.{tool}" if server else tool
            if label:
                mcp_calls.append(label)
        elif item_type == "agent_message":
            text = str(item.get("text") or "")
            if text:
                agent_messages.append(text)
        if text_contains(item, "tool_search"):
            tool_search_text = True
        if text_contains(item, "node_repl"):
            node_repl_text = True
    return {
        "command_execution_count": command_count,
        "mcp_calls": mcp_calls,
        "mcp_call_count": len(mcp_calls),
        "node_repl_mcp_call_count": sum(1 for call in mcp_calls if call.startswith("node_repl.")),
        "agent_message_count": len(agent_messages),
        "last_agent_message": agent_messages[-1] if agent_messages else "",
        "chrome_skill_read": chrome_skill_read,
        "tool_search_text_seen": tool_search_text,
        "node_repl_text_seen": node_repl_text,
    }


def analyze_events(events: list[dict[str, Any]], codex_items: list[dict[str, Any]] | None = None) -> dict[str, Any]:
    codex_items = codex_items or []
    raw_events = [event for event in events if event.get("stage") == "raw_request"]
    upstream_events = [event for event in events if event.get("stage") == "upstream_request"]
    stream_events = [event for event in events if event.get("stage") == "responses_stream_event"]
    stream_summaries = [event for event in events if event.get("stage") == "responses_stream_summary"]
    relay_errors = [event for event in events if event.get("stage") == "relay_error"]

    raw_tool_search = any(bool_from_summary(event.get("request_summary"), "tool_search_available") or text_contains(event.get("raw_body", ""), "tool_search") for event in raw_events)
    raw_shell = any(text_contains(event.get("request_summary", {}), "shell_command") or text_contains(event.get("raw_body", ""), "shell_command") for event in raw_events)
    raw_chrome = any(bool_from_summary(event.get("request_summary"), "chrome_mentioned") or text_contains(event.get("raw_body", ""), "Chrome") or text_contains(event.get("raw_body", ""), "chrome") for event in raw_events)

    upstream_body_summaries = [summarize_tools_from_body(str(event.get("upstream_body") or "")) for event in upstream_events]
    upstream_tool_search = any(summary["tool_search_available"] for summary in upstream_body_summaries)
    upstream_shell = any(summary["shell_command_available"] for summary in upstream_body_summaries)
    upstream_node_repl = any(summary["node_repl_mentioned"] for summary in upstream_body_summaries)
    upstream_tool_names = sorted({name for summary in upstream_body_summaries for name in summary["tool_names"]})

    stream_tool_names = sorted({str(event.get("stream_tool_name") or "") for event in stream_events if event.get("stream_tool_name")})
    stream_item_types = sorted({str(event.get("stream_item_type") or "") for event in stream_events if event.get("stream_item_type")})
    terminal_events = [event for event in stream_events if event.get("stream_event_type") in TERMINAL_RESPONSE_EVENTS]
    terminal_reasons = [str(event.get("stream_stop_reason") or "") for event in stream_summaries if event.get("stream_stop_reason")]
    stream_tool_search = "tool_search" in stream_tool_names
    stream_shell = "shell_command" in stream_tool_names
    stream_function_calls = [event for event in stream_events if event.get("stream_item_type") == "function_call" or event.get("stream_tool_name")]
    codex = analyze_codex_items(codex_items)

    codes: list[str] = []
    findings: list[str] = []
    next_actions: list[str] = []

    if relay_errors:
        codes.append("relay_error")
        err = relay_errors[-1]
        findings.append(f"请求命中了 relay_error：status={err.get('status_code')} code={err.get('error_code') or '-'} message={err.get('error_message') or '-'}")
        next_actions.append("先处理 relay_error；工具链问题需要在请求成功或正常流式返回时再判定。")

    if not raw_events:
        codes.append("dump_not_matched")
        findings.append("没有 raw_request 事件，说明 dump 规则没有匹配到 Codex 请求，或请求没有打到这个 new-api 实例。")
        next_actions.append("放宽 dump 过滤：只保留 user_ids，先去掉 token_names/models/paths；确认 Codex base_url 指向当前实例。")
    else:
        findings.append(f"dump 捕获 raw_request={len(raw_events)} upstream_request={len(upstream_events)} stream_event={len(stream_events)} stream_summary={len(stream_summaries)}。")

    if raw_events and not raw_tool_search:
        codes.append("client_missing_deferred_tools")
        findings.append("raw_request 里没有 tool_search，这表示 Codex 客户端没有把 deferred tool 暴露给模型。")
        next_actions.append("检查本机 Codex 插件/配置/版本；这类情况不是 new-api 转发丢工具。")

    if raw_tool_search and upstream_events and not upstream_tool_search:
        codes.append("gateway_dropped_deferred_tools")
        if upstream_shell:
            findings.append("raw_request 有 tool_search，但 upstream_request 没有；上游体里只看到普通 function tool，例如 shell_command。")
            next_actions.append("这最像 Responses 转 Chat fallback 或兼容转换丢弃非 function tool；Codex 请求应路由到原生 Responses/OAuth 透传路径。")
        else:
            findings.append("raw_request 有 tool_search，但 upstream_request 没有，说明当前 new-api 实例在出站前丢了 deferred tool。")
            next_actions.append("检查该渠道/聚合分组是否进入 chat fallback、OpenAI-compatible 转换、或不支持 Responses 的账号。")

    if raw_tool_search and not upstream_events:
        codes.append("upstream_body_not_captured")
        findings.append("raw_request 有 tool_search，但没有 upstream_request 事件，无法确认是不是出站转换丢工具。")
        next_actions.append("确认 dump 开启 print_upstream_body，并尽量在最靠近问题的那一层 new-api/sub2api 抓取。")

    if upstream_tool_search and not stream_function_calls:
        codes.append("upstream_model_no_tool_call")
        findings.append("upstream_request 保留了 tool_search，但响应流里没有任何 function/tool item。")
        next_actions.append("优先查最终上游账号/模型是否真正支持 Codex deferred/client tools，以及是否被 sub2api 账号策略降级。")

    if upstream_tool_search and stream_function_calls and not stream_tool_search:
        codes.append("model_skipped_deferred_tool_search")
        if stream_shell:
            findings.append("上游保留了 tool_search，但模型响应流只调用了普通工具（包含 shell_command），没有调用 tool_search。")
        else:
            findings.append("上游保留了 tool_search，但模型响应流没有调用 tool_search。")
        next_actions.append("这种更偏最终上游模型/账号能力或调度路径；用同请求切换已知正常账号做 A/B 对照。")

    if stream_tool_search and codex["node_repl_mcp_call_count"] == 0:
        codes.append("tool_search_not_followed_by_client_tool")
        findings.append("响应流里出现了 tool_search，但 Codex JSONL 没有 node_repl MCP 调用。")
        next_actions.append("查 tool_search 返回内容和 Codex 客户端插件执行；问题可能在客户端工具发现结果或插件注册。")

    if codex["node_repl_mcp_call_count"] > 0:
        codes.append("client_tool_chain_reached")
        findings.append(f"Codex 客户端已经调用 node_repl MCP {codex['node_repl_mcp_call_count']} 次，说明 deferred tool 到客户端执行链路至少打通了一次。")

    if codex["chrome_skill_read"] and codex["mcp_call_count"] == 0 and raw_tool_search and upstream_events and not upstream_tool_search:
        codes.append("classic_codex_stops_after_skill_due_tool_loss")
        findings.append("Codex 已读取 Chrome skill，但没有任何 MCP 调用；结合 upstream 丢 tool_search，符合“说要干活但停住”的典型链路。")

    if codex["chrome_skill_read"] and codex["mcp_call_count"] == 0 and upstream_tool_search and not stream_tool_search:
        codes.append("classic_codex_stops_after_skill_without_deferred_call")
        findings.append("Codex 已读取 Chrome skill，但模型没有发起 tool_search，因此客户端没有机会进入 node_repl/Chrome 执行。")

    if not codes:
        codes.append("inconclusive")
        findings.append("现有证据不足以下结论，需要更多 raw/upstream body 或 Codex JSONL。")
        next_actions.append("用脚本 live mode 重新抓取，或导出 request_dump events + Codex --json 输出再分析。")

    return {
        "verdict_codes": sorted(set(codes)),
        "findings": findings,
        "next_actions": list(dict.fromkeys(next_actions)),
        "evidence": {
            "event_counts": {
                "raw_request": len(raw_events),
                "upstream_request": len(upstream_events),
                "responses_stream_event": len(stream_events),
                "responses_stream_summary": len(stream_summaries),
                "relay_error": len(relay_errors),
            },
            "raw": {
                "tool_search_available": raw_tool_search,
                "shell_command_seen": raw_shell,
                "chrome_mentioned": raw_chrome,
            },
            "upstream": {
                "tool_search_available": upstream_tool_search,
                "shell_command_available": upstream_shell,
                "node_repl_mentioned": upstream_node_repl,
                "tool_names": upstream_tool_names[:80],
            },
            "stream": {
                "tool_names": stream_tool_names,
                "item_types": stream_item_types,
                "function_or_tool_event_count": len(stream_function_calls),
                "terminal_event_count": len(terminal_events),
                "terminal_reasons": terminal_reasons,
            },
            "codex_cli": codex,
        },
    }


def render_report(report: dict[str, Any], output_dir: Path | None = None) -> str:
    lines = [
        "# Codex Tool Diagnose Report",
        "",
        "## Verdict",
    ]
    for code in report["verdict_codes"]:
        lines.append(f"- `{code}`")
    lines.extend(["", "## Findings"])
    for item in report["findings"]:
        lines.append(f"- {item}")
    lines.extend(["", "## Next Actions"])
    if report["next_actions"]:
        for item in report["next_actions"]:
            lines.append(f"- {item}")
    else:
        lines.append("- 暂无额外动作。")
    lines.extend(["", "## Evidence", "```json", json_dumps(report["evidence"]), "```"])
    if output_dir:
        lines.extend(["", "## Artifacts", f"- `{output_dir}`"])
    return "\n".join(lines) + "\n"


def run_live(args: argparse.Namespace) -> int:
    if not args.admin_base_url:
        raise SystemExit("live mode needs --admin-base-url or NEWAPI_ADMIN_BASE_URL")
    if not args.api_base_url:
        raise SystemExit("live mode needs --api-base-url")
    if not args.model:
        raise SystemExit("live mode needs --model")

    output_dir = Path(args.output_dir or f"tmp/codex-tool-diagnose-{time.strftime('%Y%m%d-%H%M%S')}").expanduser()
    output_dir.mkdir(parents=True, exist_ok=True)
    headers = admin_headers(args)
    rule = request_dump_rule(args)
    (output_dir / "dump-rule.json").write_text(json_dumps(rule), encoding="utf-8")

    stopped = False
    try:
        clear = http_json("POST", args.admin_base_url, "/api/request_dump/clear", headers, timeout=args.http_timeout)
        if clear.status >= 400:
            raise RuntimeError(f"clear dump failed HTTP {clear.status}: {redact(clear.text[:500])}")
        start = http_json("POST", args.admin_base_url, "/api/request_dump/start", headers, payload=rule, timeout=args.http_timeout)
        if start.status >= 400:
            raise RuntimeError(f"start dump failed HTTP {start.status}: {redact(start.text[:500])}")
        unwrap_api_data(start.parsed)
        print(f"[diagnose] request_dump started: user_ids={rule['user_ids']} model={rule.get('models') or '*'}", flush=True)

        rc, codex_jsonl = run_codex_repro(args, output_dir)
        if rc != 0:
            print(f"[diagnose] codex exited with rc={rc}; continuing to collect dump events", file=sys.stderr, flush=True)

        time.sleep(args.post_run_wait)
        events_result = http_json("GET", args.admin_base_url, "/api/request_dump/events?after_id=0&limit=200", headers, timeout=args.http_timeout)
        if events_result.status >= 400:
            raise RuntimeError(f"events dump failed HTTP {events_result.status}: {redact(events_result.text[:500])}")
        events_data = unwrap_api_data(events_result.parsed)
        events_file = output_dir / "dump-events.json"
        events_file.write_text(redact(json_dumps(events_data)), encoding="utf-8")
        events = events_data.get("events", []) if isinstance(events_data, dict) else []

        stop = http_json("POST", args.admin_base_url, "/api/request_dump/stop", headers, timeout=args.http_timeout)
        stopped = True
        if stop.status >= 400:
            print(f"[diagnose] stop dump failed HTTP {stop.status}: {redact(stop.text[:500])}", file=sys.stderr, flush=True)

        report = analyze_events([event for event in events if isinstance(event, dict)], load_codex_jsonl(codex_jsonl))
        report_file = output_dir / "report.json"
        report_md_file = output_dir / "report.md"
        report_file.write_text(json_dumps(report), encoding="utf-8")
        report_md = render_report(report, output_dir)
        report_md_file.write_text(report_md, encoding="utf-8")
        print(report_md)
        return 2 if "dump_not_matched" in report["verdict_codes"] else 0
    finally:
        if not stopped:
            try:
                http_json("POST", args.admin_base_url, "/api/request_dump/stop", headers, timeout=args.http_timeout)
            except Exception as exc:  # noqa: BLE001 - best-effort cleanup for an ops script.
                print(f"[diagnose] stop dump cleanup failed: {redact(exc)}", file=sys.stderr)


def run_analyze(args: argparse.Namespace) -> int:
    if not args.events_file:
        raise SystemExit("analyze mode needs --events-file")
    events = load_events_file(Path(args.events_file))
    codex_items = load_codex_jsonl(Path(args.codex_jsonl)) if args.codex_jsonl else []
    report = analyze_events(events, codex_items)
    report_md = render_report(report)
    if args.output_dir:
        output_dir = Path(args.output_dir).expanduser()
        output_dir.mkdir(parents=True, exist_ok=True)
        (output_dir / "report.json").write_text(json_dumps(report), encoding="utf-8")
        (output_dir / "report.md").write_text(report_md, encoding="utf-8")
    print(report_md)
    return 0


def run_self_test() -> int:
    raw = {
        "stage": "raw_request",
        "request_summary": {
            "tool_search_available": True,
            "tool_names": ["function:shell_command", "tool_search:tool_search"],
            "chrome_mentioned": True,
        },
    }
    upstream_dropped = {
        "stage": "upstream_request",
        "upstream_body": json.dumps({"tools": [{"type": "function", "function": {"name": "shell_command"}}]}),
    }
    codex_stopped = [
        {"type": "item.completed", "item": {"type": "command_execution", "command": "cat chrome/SKILL.md", "aggregated_output": "Chrome Plugin"}},
        {"type": "item.completed", "item": {"type": "agent_message", "text": "现在查找 node_repl 工具。"}},
    ]
    report = analyze_events([raw, upstream_dropped], codex_stopped)
    assert "gateway_dropped_deferred_tools" in report["verdict_codes"], report
    assert "classic_codex_stops_after_skill_due_tool_loss" in report["verdict_codes"], report

    client_missing = analyze_events([{"stage": "raw_request", "request_summary": {"tool_names": ["function:shell_command"]}}], [])
    assert "client_missing_deferred_tools" in client_missing["verdict_codes"], client_missing

    upstream_kept = {
        "stage": "upstream_request",
        "upstream_body": json.dumps({"tools": [{"type": "tool_search", "name": "tool_search"}]}),
    }
    no_tool = analyze_events([raw, upstream_kept, {"stage": "responses_stream_event", "stream_event_type": "response.completed"}], [])
    assert "upstream_model_no_tool_call" in no_tool["verdict_codes"], no_tool

    codex_ok = analyze_events(
        [raw, upstream_kept, {"stage": "responses_stream_event", "stream_tool_name": "tool_search", "stream_item_type": "function_call"}],
        [{"type": "item.started", "item": {"type": "mcp_tool_call", "server": "node_repl", "tool": "js"}}],
    )
    assert "client_tool_chain_reached" in codex_ok["verdict_codes"], codex_ok

    print("self-test ok")
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Diagnose Codex deferred-tool loss with new-api request_dump.")
    sub = parser.add_subparsers(dest="command")

    live = sub.add_parser("live", help="start request_dump, run Codex CLI, collect events, and diagnose")
    live.add_argument("--admin-base-url", default=os.environ.get("NEWAPI_ADMIN_BASE_URL"), help="new-api admin base URL, for example http://localhost:3001")
    live.add_argument("--admin-header", action="append", help="admin HTTP header, repeatable, for example 'Authorization: Bearer ...'")
    live.add_argument("--admin-cookie", help="admin Cookie header value")
    live.add_argument("--admin-user-id", help="New-Api-User header value")
    live.add_argument("--api-base-url", required=False, default=os.environ.get("CODEX_API_BASE_URL"), help="target OpenAI-compatible /v1 base URL")
    live.add_argument("--api-key", default="env:CODEX_API_KEY", help="target API key or env:NAME")
    live.add_argument("--model", default=os.environ.get("CODEX_MODEL"), help="Codex model to test")
    live.add_argument("--prompt", help="custom Codex repro prompt")
    live.add_argument("--codex-binary", default=os.environ.get("CODEX_BINARY", "codex"))
    live.add_argument("--source-codex-home", help="Codex home to copy config/plugin cache links from; defaults to CODEX_HOME or ~/.codex")
    live.add_argument("--workdir", help="working directory for codex exec; defaults inside output dir")
    live.add_argument("--output-dir", help="artifact directory; defaults under tmp/")
    live.add_argument("--codex-timeout", type=int, default=180)
    live.add_argument("--http-timeout", type=int, default=30)
    live.add_argument("--post-run-wait", type=float, default=1.0)
    live.add_argument("--dump-user-ids", default=os.environ.get("DUMP_USER_IDS"), help="required comma-separated user IDs")
    live.add_argument("--dump-token-names", default=os.environ.get("DUMP_TOKEN_NAMES"), help="optional comma-separated token names")
    live.add_argument("--dump-models", default=os.environ.get("DUMP_MODELS"), help="optional comma-separated model filters; defaults to --model")
    live.add_argument("--dump-paths", default=os.environ.get("DUMP_PATHS"), help="optional comma-separated path filters")
    live.add_argument("--dump-aggregate-groups", default=os.environ.get("DUMP_AGGREGATE_GROUPS"), help="optional comma-separated aggregate group filters")
    live.add_argument("--dump-keywords", default=os.environ.get("DUMP_KEYWORDS"), help="optional comma-separated keyword filters")
    live.add_argument("--dump-duration", type=int, default=300)
    live.add_argument("--dump-max-count", type=int, default=20)
    live.add_argument("--dump-log-level", default="debug", choices=["debug", "info", "warn", "error"])
    live.add_argument("--dump-body", action="store_true", help="also store raw request body; request_summary is captured even when disabled")
    live.add_argument("--dump-headers", action="store_true", help="also store non-sensitive request headers")
    live.add_argument("--dump-key-events-only", action="store_true", help="only capture key stream events")
    live.add_argument("--dump-max-body-bytes", type=int, default=1024 * 1024)
    live.add_argument("--dump-max-stream-events", type=int, default=1000)

    analyze = sub.add_parser("analyze", help="diagnose from existing request_dump events and optional Codex JSONL")
    analyze.add_argument("--events-file", required=True, help="request_dump events JSON/API response/log file")
    analyze.add_argument("--codex-jsonl", help="Codex CLI --json output")
    analyze.add_argument("--output-dir", help="optional report output directory")

    sub.add_parser("self-test", help="run built-in analyzer tests")
    return parser


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    if args.command == "live":
        return run_live(args)
    if args.command == "analyze":
        return run_analyze(args)
    if args.command == "self-test":
        return run_self_test()
    parser.print_help()
    return 2


if __name__ == "__main__":
    raise SystemExit(main())
