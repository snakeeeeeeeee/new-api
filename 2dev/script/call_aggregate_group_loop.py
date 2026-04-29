#!/usr/bin/env python3
import argparse
import json
import random
import statistics
import sys
import time
import urllib.error
import urllib.request
from concurrent.futures import ThreadPoolExecutor, as_completed

DEFAULT_MODEL_POOL = ["claude-opus-4-6", "claude-sonnet-4-6"]
OPENAI_CHAT_ENDPOINT = "/v1/chat/completions"
CLAUDE_MESSAGES_ENDPOINT = "/v1/messages"
NO_PROXY_OPENER = urllib.request.build_opener(urllib.request.ProxyHandler({}))


def normalize_base_url(base_url):
    return base_url.rstrip("/")


def normalize_token(token):
    token = token.strip()
    if token.startswith("sk-"):
        return token
    return f"sk-{token}"


def parse_extra_json(raw):
    if not raw:
        return {}
    try:
        value = json.loads(raw)
    except json.JSONDecodeError as exc:
        raise ValueError(f"--extra-json 不是合法 JSON: {exc}") from exc
    if not isinstance(value, dict):
        raise ValueError("--extra-json 必须是 JSON object")
    return value


def parse_model_list(raw):
    if not raw:
        return []
    return [item.strip() for item in raw.split(",") if item.strip()]


def choose_model(args):
    return random.choice(args.model_pool)


def build_affinity_key(args, index):
    if not args.affinity_key:
        return ""
    if args.vary_affinity:
        return f"{args.affinity_key}-{index}"
    return args.affinity_key


def apply_affinity(payload, args, index):
    affinity_key = build_affinity_key(args, index)
    if not affinity_key:
        return

    placements = args.affinity_placement
    if "all" in placements:
        if args.api_format == "claude":
            placements = ["metadata.user_id"]
        else:
            placements = [
                "prompt_cache_key",
                "user",
                "metadata.aggregate_route_affinity_key",
            ]

    for placement in placements:
        if placement == "prompt_cache_key":
            payload["prompt_cache_key"] = affinity_key
        elif placement == "user":
            payload["user"] = affinity_key
        elif placement == "metadata.user_id":
            metadata = payload.setdefault("metadata", {})
            if isinstance(metadata, dict):
                metadata["user_id"] = affinity_key
        elif placement == "metadata.aggregate_route_affinity_key":
            metadata = payload.setdefault("metadata", {})
            if isinstance(metadata, dict):
                metadata["aggregate_route_affinity_key"] = affinity_key


def build_openai_chat_payload(args, index, model):
    return {
        "model": model,
        "messages": [
            {
                "role": "user",
                "content": f"{args.message} #{index}",
            }
        ],
        "max_tokens": args.max_tokens,
        "stream": False,
    }


def build_claude_messages_payload(args, index, model):
    return {
        "model": model,
        "messages": [
            {
                "role": "user",
                "content": f"{args.message} #{index}",
            }
        ],
        "max_tokens": args.max_tokens,
        "stream": False,
    }


def build_payload(args, index):
    model = choose_model(args)
    if args.api_format == "claude":
        payload = build_claude_messages_payload(args, index, model)
    else:
        payload = build_openai_chat_payload(args, index, model)
    if args.temperature is not None:
        payload["temperature"] = args.temperature

    payload.update(parse_extra_json(args.extra_json))
    apply_affinity(payload, args, index)
    return payload


def decode_response_body(raw):
    text = raw.decode("utf-8", errors="replace")
    if not text:
        return None, ""
    try:
        return json.loads(text), text
    except json.JSONDecodeError:
        return None, text


def request_json(method, url, token, body=None, timeout=120, extra_headers=None):
    headers = {
        "Authorization": f"Bearer {normalize_token(token)}",
        "Content-Type": "application/json",
    }
    if extra_headers:
        headers.update(extra_headers)
    data = None
    if body is not None:
        data = json.dumps(body, ensure_ascii=False).encode("utf-8")

    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    started_at = time.monotonic()
    try:
        with NO_PROXY_OPENER.open(req, timeout=timeout) as resp:
            parsed, text = decode_response_body(resp.read())
            elapsed_ms = int((time.monotonic() - started_at) * 1000)
            return {
                "status": resp.status,
                "ok": 200 <= resp.status < 300,
                "elapsed_ms": elapsed_ms,
                "json": parsed,
                "text": text,
                "error": "",
            }
    except urllib.error.HTTPError as exc:
        parsed, text = decode_response_body(exc.read())
        elapsed_ms = int((time.monotonic() - started_at) * 1000)
        return {
            "status": exc.code,
            "ok": False,
            "elapsed_ms": elapsed_ms,
            "json": parsed,
            "text": text,
            "error": f"HTTP {exc.code}",
        }
    except Exception as exc:
        elapsed_ms = int((time.monotonic() - started_at) * 1000)
        return {
            "status": 0,
            "ok": False,
            "elapsed_ms": elapsed_ms,
            "json": None,
            "text": "",
            "error": str(exc),
        }


def extract_response_summary(result, show_content):
    body = result.get("json")
    if not isinstance(body, dict):
        text = result.get("text") or result.get("error") or ""
        return text[:180].replace("\n", " ")

    if not result.get("ok"):
        return json.dumps(body, ensure_ascii=False)[:240]

    response_id = body.get("id", "-")
    response_model = body.get("model", "-")
    content = ""
    choices = body.get("choices") or []
    if choices:
        message = choices[0].get("message") or {}
        content = message.get("content") or ""
    claude_content = body.get("content") or []
    if not content and isinstance(claude_content, list):
        text_parts = []
        for item in claude_content:
            if isinstance(item, dict) and item.get("type") == "text":
                text_parts.append(item.get("text") or "")
        content = "".join(text_parts)
    if show_content and content:
        return f"id={response_id} model={response_model} content={content[:160]!r}"
    return f"id={response_id} model={response_model}"


def build_request_headers(args):
    if args.api_format != "claude":
        return {}
    return {
        "anthropic-version": args.claude_version,
    }


def call_once(args, index):
    url = f"{normalize_base_url(args.base_url)}{args.endpoint}"
    payload = build_payload(args, index)
    if args.show_start:
        print(
            f"[{index:03d}] START request_model={payload.get('model') or '-'}",
            flush=True,
        )
    result = request_json(
        "POST",
        url,
        args.token,
        payload,
        timeout=args.timeout,
        extra_headers=build_request_headers(args),
    )
    result["index"] = index
    result["request_model"] = payload.get("model", "")
    result["affinity_key"] = build_affinity_key(args, index)
    result["summary"] = extract_response_summary(result, args.show_content)
    return result


def percentile(values, ratio):
    if not values:
        return 0
    ordered = sorted(values)
    pos = int(round((len(ordered) - 1) * ratio))
    return ordered[pos]


def print_result(result):
    prefix = "OK " if result["ok"] else "ERR"
    affinity = (
        f" affinity={result['affinity_key']}" if result.get("affinity_key") else ""
    )
    print(
        f"[{result['index']:03d}] {prefix} "
        f"status={result['status']} elapsed={result['elapsed_ms']}ms"
        f" request_model={result.get('request_model') or '-'}"
        f"{affinity} {result['summary']}",
        flush=True,
    )


def print_summary(results):
    success = [r for r in results if r["ok"]]
    failed = [r for r in results if not r["ok"]]
    latencies = [r["elapsed_ms"] for r in success]
    status_counts = {}
    model_counts = {}
    for result in results:
        status_counts[result["status"]] = status_counts.get(result["status"], 0) + 1
        model = result.get("request_model") or "-"
        model_counts[model] = model_counts.get(model, 0) + 1

    summary = {
        "total": len(results),
        "success": len(success),
        "failed": len(failed),
        "status_counts": status_counts,
        "request_model_counts": model_counts,
        "latency_ms": {
            "min": min(latencies) if latencies else 0,
            "avg": int(statistics.mean(latencies)) if latencies else 0,
            "p50": percentile(latencies, 0.50),
            "p95": percentile(latencies, 0.95),
            "max": max(latencies) if latencies else 0,
        },
    }
    print("\n=== summary ===", flush=True)
    print(json.dumps(summary, ensure_ascii=False, indent=2), flush=True)


def fetch_token_logs(args):
    if args.show_token_logs <= 0:
        return
    url = f"{normalize_base_url(args.base_url)}/api/log/token"
    result = request_json("GET", url, args.token, timeout=args.timeout)
    print("\n=== token logs ===", flush=True)
    if not result["ok"] or not isinstance(result.get("json"), dict):
        detail = extract_response_summary(result, show_content=True)
        print(f"fetch token logs failed: status={result['status']} {detail}", flush=True)
        return
    payload = result["json"]
    logs = payload.get("data") or []
    for item in logs[: args.show_token_logs]:
        other = item.get("other") or {}
        if isinstance(other, str):
            try:
                other = json.loads(other)
            except json.JSONDecodeError:
                other = {}
        admin_info = other.get("admin_info") if isinstance(other, dict) else None
        if not isinstance(admin_info, dict):
            admin_info = {}
        print(
            json.dumps(
                {
                    "id": item.get("id"),
                    "created_at": item.get("created_at"),
                    "model": item.get("model_name"),
                    "group": item.get("group"),
                    "quota": item.get("quota"),
                    "aggregate_group": admin_info.get("aggregate_group"),
                    "routing_mode": admin_info.get("aggregate_routing_mode"),
                    "route_group": admin_info.get("route_group"),
                    "channel_id": admin_info.get("channel_id"),
                },
                ensure_ascii=False,
            ),
            flush=True,
        )


def run(args):
    results = []
    if args.concurrency <= 1:
        for index in range(1, args.times + 1):
            result = call_once(args, index)
            results.append(result)
            print_result(result)
            if not result["ok"] and args.stop_on_error:
                break
            if args.interval > 0 and index < args.times:
                time.sleep(args.interval)
    else:
        with ThreadPoolExecutor(max_workers=args.concurrency) as executor:
            futures = []
            for index in range(1, args.times + 1):
                futures.append(executor.submit(call_once, args, index))
                if args.interval > 0 and index < args.times:
                    time.sleep(args.interval)
            for future in as_completed(futures):
                result = future.result()
                results.append(result)
                print_result(result)

    results.sort(key=lambda item: item["index"])
    print_summary(results)
    fetch_token_logs(args)
    if any(not result["ok"] for result in results):
        return 1
    return 0


def main():
    parser = argparse.ArgumentParser(
        description="Loop-call a new-api aggregate-group token and observe cluster distribution/RPM.",
        epilog=(
            "示例：\n"
            "  python3 2dev/script/call_aggregate_group_loop.py "
            "--token sk-xxx --times 30\n"
            "  python3 2dev/script/call_aggregate_group_loop.py "
            "--token sk-xxx --times 30 "
            "--affinity-key user-a\n"
            "  python3 2dev/script/call_aggregate_group_loop.py "
            "--token sk-xxx --models claude-opus-4-6,claude-sonnet-4-6 --times 60 "
            "--affinity-key user --vary-affinity --show-token-logs 20\n"
            "  python3 2dev/script/call_aggregate_group_loop.py "
            "--token sk-xxx --api-format claude --times 60 "
            "--affinity-key user --vary-affinity --show-token-logs 20"
        ),
        formatter_class=argparse.RawTextHelpFormatter,
    )
    parser.add_argument("--base-url", default="http://localhost:3001")
    parser.add_argument("--endpoint", help="不传时根据 --api-format 自动选择")
    parser.add_argument("--token", required=True, help="支持传 sk-xxx 或裸 token")
    parser.add_argument(
        "--api-format",
        choices=["openai", "claude"],
        default="openai",
        help="openai 使用 /v1/chat/completions；claude 使用 /v1/messages 原生格式",
    )
    parser.add_argument(
        "--claude-version",
        default="2023-06-01",
        help="--api-format claude 时使用的 anthropic-version 请求头",
    )
    parser.add_argument("--model", help="固定单个模型；不传时默认随机跑 opus / sonnet")
    parser.add_argument(
        "--models",
        help="逗号分隔的随机模型池；例如 claude-opus-4-6,claude-sonnet-4-6",
    )
    parser.add_argument("--times", type=int, default=20)
    parser.add_argument("--concurrency", type=int, default=1)
    parser.add_argument("--interval", type=float, default=0.2)
    parser.add_argument("--timeout", type=int, default=120)
    parser.add_argument("--max-tokens", type=int, default=16)
    parser.add_argument("--temperature", type=float)
    parser.add_argument("--message", default="Reply with OK only.")
    parser.add_argument(
        "--affinity-key",
        help="固定亲和 key；默认不设置。固定 key 适合观察粘性，配合 --vary-affinity 可观察多 key 分散。",
    )
    parser.add_argument(
        "--affinity-placement",
        action="append",
        choices=[
            "all",
            "prompt_cache_key",
            "user",
            "metadata.user_id",
            "metadata.aggregate_route_affinity_key",
        ],
        default=None,
        help="亲和 key 写入位置，可重复；默认 all。claude 模式下 all 等同 metadata.user_id。",
    )
    parser.add_argument("--vary-affinity", action="store_true")
    parser.add_argument(
        "--extra-json",
        default="",
        help='合并到请求体的 JSON object，例如 \'{"metadata":{"case":"cluster"}}\'',
    )
    parser.add_argument("--show-content", action="store_true")
    parser.add_argument(
        "--show-start",
        action="store_true",
        help="每次请求发出前打印 START；适合手动禁用/启用渠道时观察是否仍在发请求。",
    )
    parser.add_argument("--show-token-logs", type=int, default=0)
    parser.add_argument("--stop-on-error", action="store_true")
    parser.add_argument("--seed", type=int, help="设置随机种子，便于复现模型选择序列")
    args = parser.parse_args()

    if args.times <= 0:
        print("--times 必须大于 0", file=sys.stderr)
        return 2
    if args.concurrency <= 0:
        print("--concurrency 必须大于 0", file=sys.stderr)
        return 2
    if args.model and args.models:
        print("--model 和 --models 不能同时使用", file=sys.stderr)
        return 2
    if args.models:
        args.model_pool = parse_model_list(args.models)
    elif args.model:
        args.model_pool = [args.model.strip()]
    else:
        args.model_pool = DEFAULT_MODEL_POOL
    if not args.model_pool:
        print("模型池不能为空", file=sys.stderr)
        return 2
    if args.seed is not None:
        random.seed(args.seed)
    if not args.endpoint:
        if args.api_format == "claude":
            args.endpoint = CLAUDE_MESSAGES_ENDPOINT
        else:
            args.endpoint = OPENAI_CHAT_ENDPOINT
    if args.affinity_placement is None:
        args.affinity_placement = ["all"]
    if args.api_format != "claude":
        args.claude_version = ""
    print(f"model_pool={','.join(args.model_pool)}", flush=True)
    print(f"api_format={args.api_format} endpoint={args.endpoint}", flush=True)
    return run(args)


if __name__ == "__main__":
    sys.exit(main())
