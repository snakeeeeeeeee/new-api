#!/usr/bin/env python3
"""Sustained load tool for local new-api token fallback performance checks.

The script can start a fake upstream that streams responses without usage, then
load a local new-api endpoint so ResponseText2Usage fallback is exercised.

Example short run:
  python3 2dev/scripts/perf_sustained_load.py \
    --start-fake-upstream --target http://127.0.0.1:3000/v1/chat/completions \
    --api-key sk-local --duration 120 --concurrency 50 --body-size 200KB \
    --pid $(pgrep -n new-api)

Example sustained run:
  python3 2dev/scripts/perf_sustained_load.py \
    --start-fake-upstream --duration 1800 --concurrency 100 --body-size 200KB \
    --sample-interval 30 --pid $(pgrep -n new-api) --out-dir ./tmp/perf-run
"""

from __future__ import annotations

import argparse
import concurrent.futures
import http.client
import http.server
import json
import os
import pathlib
import queue
import socketserver
import ssl
import statistics
import subprocess
import threading
import time
import urllib.parse
import urllib.request
from dataclasses import dataclass


def parse_size(value: str) -> int:
    text = value.strip().upper()
    units = [("GB", 1024**3), ("MB", 1024**2), ("KB", 1024), ("B", 1)]
    for suffix, multiplier in units:
        if text.endswith(suffix):
            return int(float(text[: -len(suffix)]) * multiplier)
    return int(text)


def percentile(values: list[float], pct: float) -> float:
    if not values:
        return 0.0
    values = sorted(values)
    index = int((len(values) - 1) * pct)
    return values[index]


class ThreadingHTTPServer(socketserver.ThreadingMixIn, http.server.HTTPServer):
    daemon_threads = True
    allow_reuse_address = True


class FakeUpstreamHandler(http.server.BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"
    response_bytes = 200 * 1024
    chunk_bytes = 1024
    mode = "openai"
    payload_pattern = (
        "streaming fallback token text https://example.com/a/b?x=123&y=test "
        "E = mc^2 sum(x_i) >= sqrt(4) 中文测试 42 newline\n"
    )

    def log_message(self, fmt: str, *args: object) -> None:
        return

    def do_GET(self) -> None:
        if self.path == "/health":
            self._send_json({"ok": True})
            return
        self.send_error(404)

    def do_POST(self) -> None:
        content_length = int(self.headers.get("Content-Length", "0") or "0")
        if content_length:
            self.rfile.read(content_length)

        if self.mode == "claude" or self.path.endswith("/messages"):
            self._send_claude_stream()
        elif self.mode == "gemini" or "generateContent" in self.path:
            self._send_gemini_stream()
        else:
            self._send_openai_stream()

    def _payload_chunks(self):
        pattern = self.payload_pattern
        if not pattern:
            pattern = "x"
        remaining = self.response_bytes
        while remaining > 0:
            size = min(self.chunk_bytes, remaining)
            remaining -= size
            repeats = (size // len(pattern)) + 1
            yield (pattern * repeats)[:size]

    def _start_sse(self) -> None:
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream; charset=utf-8")
        self.send_header("Cache-Control", "no-cache")
        self.send_header("Connection", "close")
        self.end_headers()
        self.close_connection = True

    def _write_sse(self, payload: dict, event: str | None = None) -> None:
        if event:
            self.wfile.write(f"event: {event}\n".encode("utf-8"))
        self.wfile.write(b"data: ")
        self.wfile.write(json.dumps(payload, separators=(",", ":")).encode("utf-8"))
        self.wfile.write(b"\n\n")
        self.wfile.flush()

    def _send_openai_stream(self) -> None:
        self._start_sse()
        for chunk in self._payload_chunks():
            self._write_sse(
                {
                    "id": "fake-chatcmpl",
                    "object": "chat.completion.chunk",
                    "choices": [
                        {
                            "index": 0,
                            "delta": {"content": chunk},
                            "finish_reason": None,
                        }
                    ],
                }
            )
        self._write_sse(
            {
                "id": "fake-chatcmpl",
                "object": "chat.completion.chunk",
                "choices": [{"index": 0, "delta": {}, "finish_reason": "stop"}],
            }
        )
        self.wfile.write(b"data: [DONE]\n\n")

    def _send_claude_stream(self) -> None:
        self._start_sse()
        self._write_sse(
            {
                "type": "message_start",
                "message": {
                    "id": "fake-claude",
                    "type": "message",
                    "role": "assistant",
                    "content": [],
                    "model": "claude-sonnet-4-6",
                    "stop_reason": None,
                    "usage": {"input_tokens": 1},
                },
            },
            "message_start",
        )
        self._write_sse(
            {"type": "content_block_start", "index": 0, "content_block": {"type": "text", "text": ""}},
            "content_block_start",
        )
        for chunk in self._payload_chunks():
            self._write_sse(
                {"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": chunk}},
                "content_block_delta",
            )
        self._write_sse({"type": "content_block_stop", "index": 0}, "content_block_stop")
        self._write_sse({"type": "message_delta", "delta": {"stop_reason": "end_turn"}}, "message_delta")
        self._write_sse({"type": "message_stop"}, "message_stop")

    def _send_gemini_stream(self) -> None:
        self.send_response(200)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Connection", "close")
        self.end_headers()
        self.close_connection = True
        first = True
        self.wfile.write(b"[")
        for chunk in self._payload_chunks():
            if not first:
                self.wfile.write(b",")
            first = False
            payload = {
                "candidates": [
                    {
                        "content": {"parts": [{"text": chunk}], "role": "model"},
                        "finishReason": None,
                    }
                ]
            }
            self.wfile.write(json.dumps(payload, separators=(",", ":")).encode("utf-8"))
        self.wfile.write(b"]")

    def _send_json(self, payload: dict) -> None:
        body = json.dumps(payload).encode("utf-8")
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)


@dataclass
class Result:
    status: int
    latency_ms: float
    overloaded: bool
    error: str = ""


def build_request_body(model: str, body_size: int, stream: bool) -> bytes:
    prompt = "p" * max(0, body_size)
    payload = {
        "model": model,
        "stream": stream,
        "messages": [{"role": "user", "content": prompt}],
    }
    return json.dumps(payload, separators=(",", ":")).encode("utf-8")


def send_request(target: str, api_key: str, body: bytes, timeout: float) -> Result:
    parsed = urllib.parse.urlparse(target)
    conn_cls = http.client.HTTPSConnection if parsed.scheme == "https" else http.client.HTTPConnection
    port = parsed.port
    host = parsed.hostname or "127.0.0.1"
    path = parsed.path or "/"
    if parsed.query:
        path += "?" + parsed.query

    context = ssl._create_unverified_context() if parsed.scheme == "https" else None
    start = time.perf_counter()
    conn = None
    try:
        if parsed.scheme == "https":
            conn = conn_cls(host, port=port, timeout=timeout, context=context)
        else:
            conn = conn_cls(host, port=port, timeout=timeout)
        headers = {"Content-Type": "application/json"}
        if api_key:
            headers["Authorization"] = f"Bearer {api_key}"
        conn.request("POST", path, body=body, headers=headers)
        resp = conn.getresponse()
        payload = resp.read()
        latency_ms = (time.perf_counter() - start) * 1000
        overloaded = resp.status == 503 and b"system cpu overloaded" in payload
        return Result(resp.status, latency_ms, overloaded)
    except Exception as exc:  # noqa: BLE001
        latency_ms = (time.perf_counter() - start) * 1000
        return Result(0, latency_ms, False, type(exc).__name__ + ": " + str(exc))
    finally:
        if conn is not None:
            conn.close()


def worker(args: argparse.Namespace, deadline: float, result_queue: queue.Queue[Result]) -> None:
    body = build_request_body(args.model, args.body_size, args.stream)
    while time.time() < deadline:
        result_queue.put(send_request(args.target, args.api_key, body, args.timeout))


def ps_stats(pid: int | None) -> dict[str, float]:
    if not pid:
        return {}
    try:
        output = subprocess.check_output(["ps", "-p", str(pid), "-o", "%cpu=,rss="], text=True).strip()
        if not output:
            return {}
        cpu_text, rss_text = output.split()[:2]
        return {"cpu_pct": float(cpu_text), "rss_mb": int(rss_text) / 1024.0}
    except Exception:
        return {}


def fetch_stats(stats_url: str, api_key: str, user_id: int) -> dict[str, float]:
    if not stats_url:
        return {}
    try:
        request = urllib.request.Request(stats_url)
        if api_key:
            request.add_header("Authorization", f"Bearer {api_key}")
        if user_id:
            request.add_header("New-Api-User", str(user_id))
        with urllib.request.urlopen(request, timeout=5) as response:
            payload = json.loads(response.read().decode("utf-8"))
        data = payload.get("data", payload)
        memory = data.get("memory_stats", {})
        cache = data.get("cache_stats", {})
        return {
            "heap_alloc_mb": memory.get("alloc", 0) / 1024 / 1024,
            "heap_sys_mb": memory.get("sys", 0) / 1024 / 1024,
            "goroutines": memory.get("num_goroutine", 0),
            "gc": memory.get("num_gc", 0),
            "memory_buffers": cache.get("active_memory_buffers", 0),
            "disk_files": cache.get("active_disk_files", 0),
        }
    except Exception:
        return {}


def capture_pprof(pprof_base: str, out_dir: pathlib.Path, seconds: int) -> None:
    if not pprof_base:
        return
    out_dir.mkdir(parents=True, exist_ok=True)
    stamp = int(time.time())
    urls = {
        f"cpu-{stamp}.pprof": f"{pprof_base.rstrip('/')}/profile?seconds={seconds}",
        f"heap-{stamp}.pprof": f"{pprof_base.rstrip('/')}/heap",
    }
    for filename, url in urls.items():
        try:
            urllib.request.urlretrieve(url, out_dir / filename)
        except Exception as exc:  # noqa: BLE001
            print(f"[pprof] failed to fetch {url}: {exc}", flush=True)


def summarize(results: list[Result]) -> dict[str, float | int]:
    latencies = [item.latency_ms for item in results if item.status > 0]
    return {
        "requests": len(results),
        "success": sum(1 for item in results if 200 <= item.status < 300),
        "status_503": sum(1 for item in results if item.status == 503),
        "system_cpu_overloaded": sum(1 for item in results if item.overloaded),
        "errors": sum(1 for item in results if item.error),
        "p95_ms": round(percentile(latencies, 0.95), 2),
        "p99_ms": round(percentile(latencies, 0.99), 2),
        "avg_ms": round(statistics.mean(latencies), 2) if latencies else 0,
    }


def print_window(label: str, results: list[Result], extra: dict[str, float]) -> None:
    summary = summarize(results)
    merged = {**summary, **{k: round(v, 2) for k, v in extra.items()}}
    print(label + " " + json.dumps(merged, ensure_ascii=False, sort_keys=True), flush=True)


def start_fake_upstream(args: argparse.Namespace) -> ThreadingHTTPServer | None:
    if not args.start_fake_upstream:
        return None
    FakeUpstreamHandler.response_bytes = args.response_size
    FakeUpstreamHandler.chunk_bytes = args.fake_chunk_size
    FakeUpstreamHandler.mode = args.fake_mode
    server = ThreadingHTTPServer((args.fake_host, args.fake_port), FakeUpstreamHandler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    print(
        f"[fake-upstream] mode={args.fake_mode} url=http://{args.fake_host}:{args.fake_port} "
        f"response_size={args.response_size}",
        flush=True,
    )
    return server


def run_load(args: argparse.Namespace) -> int:
    server = start_fake_upstream(args)
    deadline = time.time() + args.duration
    result_queue: queue.Queue[Result] = queue.Queue()
    window: list[Result] = []
    all_results: list[Result] = []
    out_dir = pathlib.Path(args.out_dir) if args.out_dir else None

    if out_dir:
        out_dir.mkdir(parents=True, exist_ok=True)

    next_sample = time.time() + args.sample_interval
    next_pprof = time.time() + args.pprof_interval if args.pprof_base and out_dir else float("inf")

    with concurrent.futures.ThreadPoolExecutor(max_workers=args.concurrency) as executor:
        futures = [executor.submit(worker, args, deadline, result_queue) for _ in range(args.concurrency)]
        while time.time() < deadline or any(not item.done() for item in futures):
            try:
                result = result_queue.get(timeout=0.2)
                window.append(result)
                all_results.append(result)
            except queue.Empty:
                pass

            now = time.time()
            if now >= next_sample:
                extra = {**ps_stats(args.pid), **fetch_stats(args.stats_url, args.stats_key, args.stats_user_id)}
                print_window(f"[window {int(now)}]", window, extra)
                if out_dir:
                    with (out_dir / "samples.jsonl").open("a", encoding="utf-8") as fh:
                        fh.write(json.dumps({"ts": now, "summary": summarize(window), "extra": extra}) + "\n")
                window = []
                next_sample = now + args.sample_interval

            if now >= next_pprof:
                threading.Thread(
                    target=capture_pprof,
                    args=(args.pprof_base, out_dir, args.pprof_seconds),
                    daemon=True,
                ).start()
                next_pprof = now + args.pprof_interval

    while not result_queue.empty():
        result = result_queue.get()
        window.append(result)
        all_results.append(result)

    if window:
        extra = {**ps_stats(args.pid), **fetch_stats(args.stats_url, args.stats_key, args.stats_user_id)}
        print_window(f"[window {int(time.time())}]", window, extra)

    print_window("[total]", all_results, {**ps_stats(args.pid), **fetch_stats(args.stats_url, args.stats_key, args.stats_user_id)})
    if server:
        server.shutdown()
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--target", default="http://127.0.0.1:3000/v1/chat/completions")
    parser.add_argument("--api-key", default=os.environ.get("NEW_API_KEY", ""))
    parser.add_argument("--model", default="claude-sonnet-4-6")
    parser.add_argument("--duration", type=int, default=120)
    parser.add_argument("--concurrency", type=int, default=50)
    parser.add_argument("--body-size", type=parse_size, default=parse_size("200KB"))
    parser.add_argument("--response-size", type=parse_size, default=parse_size("200KB"))
    parser.add_argument("--stream", action=argparse.BooleanOptionalAction, default=True)
    parser.add_argument("--timeout", type=float, default=120)
    parser.add_argument("--sample-interval", type=int, default=30)
    parser.add_argument("--pid", type=int, default=0)
    parser.add_argument("--stats-url", default="")
    parser.add_argument("--stats-key", default=os.environ.get("NEW_API_ADMIN_KEY", ""))
    parser.add_argument("--stats-user-id", type=int, default=int(os.environ.get("NEW_API_ADMIN_USER_ID", "0") or "0"))
    parser.add_argument("--pprof-base", default="")
    parser.add_argument("--pprof-interval", type=int, default=300)
    parser.add_argument("--pprof-seconds", type=int, default=30)
    parser.add_argument("--out-dir", default="")
    parser.add_argument("--start-fake-upstream", action="store_true")
    parser.add_argument("--fake-host", default="127.0.0.1")
    parser.add_argument("--fake-port", type=int, default=19081)
    parser.add_argument("--fake-mode", choices=["openai", "claude", "gemini"], default="openai")
    parser.add_argument("--fake-chunk-size", type=parse_size, default=parse_size("1KB"))
    return parser


def main() -> int:
    args = build_parser().parse_args()
    return run_load(args)


if __name__ == "__main__":
    raise SystemExit(main())
