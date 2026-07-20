#!/usr/bin/env python3
import argparse
import gzip
import json
import secrets
import subprocess
import sys
import threading
import time
import urllib.error
import urllib.parse
import urllib.request
from datetime import datetime, timezone
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path


NO_PROXY_OPENER = urllib.request.build_opener(urllib.request.ProxyHandler({}))
OPTION_DEFAULTS = {
    "RetryTimes": "0",
    "GroupRatio": "{}",
    "ModelRatio": "{}",
    "CompletionRatio": "{}",
    "claude.response_integrity_fallback_enabled": "false",
    "claude.response_integrity_first_block_timeout_seconds": "30",
    "error_snapshot.enabled": "false",
    "error_snapshot.ttl_minutes": "60",
    "error_snapshot.max_storage_mib": "256",
    "error_snapshot.max_files": "1000",
    "error_snapshot.priority_user_ids": "",
    "error_snapshot.priority_channel_ids": "",
}


def require(condition, message):
    if not condition:
        raise AssertionError(message)


def sql_str(value):
    if value is None:
        return "null"
    return "'" + str(value).replace("'", "''") + "'"


def run_cmd(args, check=True):
    result = subprocess.run(
        args,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    if check and result.returncode != 0:
        detail = (result.stderr or result.stdout or "").strip()
        raise RuntimeError(
            f"command failed ({result.returncode}): {' '.join(args)}\n{detail}"
        )
    return result.stdout.strip()


def decode_json(raw):
    text = raw.decode("utf-8", errors="replace")
    if not text:
        return None, ""
    try:
        return json.loads(text), text
    except json.JSONDecodeError:
        return None, text


class FakeClaudeUpstream(BaseHTTPRequestHandler):
    events = []
    lock = threading.Lock()
    log_path = None
    protocol_version = "HTTP/1.1"

    def log_message(self, fmt, *args):
        return

    @classmethod
    def reset(cls, log_path):
        with cls.lock:
            cls.events = []
            cls.log_path = log_path

    @classmethod
    def scenario_routes(cls, scenario):
        with cls.lock:
            return [
                event["route"]
                for event in cls.events
                if event["scenario"] == scenario
            ]

    def do_POST(self):
        length = int(self.headers.get("content-length") or "0")
        raw = self.rfile.read(length) if length > 0 else b"{}"
        try:
            payload = json.loads(raw.decode("utf-8"))
        except Exception:
            payload = {}

        route = self.path.strip("/").split("/", 1)[0]
        if route not in {"bad", "good"}:
            route = "unknown"
        prompt = self.extract_prompt(payload)
        scenario = self.extract_scenario(prompt)
        stream = bool(payload.get("stream"))
        event = {
            "timestamp": datetime.now(timezone.utc).isoformat(),
            "route": route,
            "scenario": scenario,
            "stream": stream,
            "path": self.path,
        }
        with self.lock:
            self.events.append(event)
            if self.log_path is not None:
                with self.log_path.open("a", encoding="utf-8") as handle:
                    handle.write(json.dumps(event, ensure_ascii=False) + "\n")

        try:
            if stream:
                self.handle_stream(route, scenario, payload)
            else:
                self.handle_non_stream(route, scenario, payload)
        except (BrokenPipeError, ConnectionResetError):
            return

    @staticmethod
    def extract_prompt(payload):
        values = []
        for message in payload.get("messages") or []:
            if not isinstance(message, dict):
                continue
            content = message.get("content")
            if isinstance(content, str):
                values.append(content)
                continue
            if isinstance(content, list):
                for block in content:
                    if isinstance(block, dict) and isinstance(block.get("text"), str):
                        values.append(block["text"])
        return " ".join(values)

    @staticmethod
    def extract_scenario(prompt):
        marker = "scenario:"
        for part in prompt.split():
            if part.startswith(marker):
                return part[len(marker) :].strip()
        return "unknown"

    def handle_non_stream(self, route, scenario, payload):
        empty_on_bad = scenario in {
            "legacy-empty-off",
            "legacy-empty-off-again",
            "fallback-empty",
            "priority-fallback",
            "retry-excluded",
            "all-fail",
        }
        empty_on_good = scenario == "all-fail"
        if (route == "bad" and empty_on_bad) or (route == "good" and empty_on_good):
            self.write_json(
                200,
                self.message_payload(payload, scenario, route, content=[]),
            )
            return
        self.write_json(
            200,
            self.message_payload(
                payload,
                scenario,
                route,
                content=[{"type": "text", "text": f"{route}:{scenario}"}],
            ),
        )

    def handle_stream(self, route, scenario, payload):
        self.send_response(200)
        self.send_header("content-type", "text/event-stream")
        self.send_header("cache-control", "no-cache")
        self.send_header("connection", "close")
        self.end_headers()

        if route == "bad" and scenario == "stream-eof":
            self.write_sse(self.message_start(payload, scenario, route))
            return

        if route == "bad" and scenario in {"stream-timeout", "snapshot-enabled"}:
            self.write_sse(self.message_start(payload, scenario, route))
            time.sleep(2.5)
            self.write_valid_stream(payload, scenario, route)
            return

        if route == "bad" and scenario == "stream-incomplete":
            self.write_sse(self.message_start(payload, scenario, route))
            self.write_sse(
                {
                    "type": "content_block_start",
                    "index": 0,
                    "content_block": {"type": "text", "text": ""},
                }
            )
            self.write_sse(
                {
                    "type": "content_block_delta",
                    "index": 0,
                    "delta": {"type": "text_delta", "text": "partial"},
                }
            )
            return

        paced = route == "bad" and scenario == "stream-normal"
        self.write_valid_stream(payload, scenario, route, paced=paced)

    def message_payload(self, payload, scenario, route, content):
        return {
            "id": f"msg_{route}_{scenario}_{int(time.time() * 1000)}",
            "type": "message",
            "role": "assistant",
            "model": str(payload.get("model") or "claude-integrity-test"),
            "content": content,
            "stop_reason": "end_turn",
            "stop_sequence": None,
            "usage": {"input_tokens": 3, "output_tokens": 2 if content else 0},
        }

    def message_start(self, payload, scenario, route):
        return {
            "type": "message_start",
            "message": {
                "id": f"msg_{route}_{scenario}_{int(time.time() * 1000)}",
                "type": "message",
                "role": "assistant",
                "model": str(payload.get("model") or "claude-integrity-test"),
                "content": [],
                "stop_reason": None,
                "stop_sequence": None,
                "usage": {"input_tokens": 3, "output_tokens": 0},
            },
        }

    def write_valid_stream(self, payload, scenario, route, paced=False):
        self.write_sse(self.message_start(payload, scenario, route))
        self.write_sse(
            {
                "type": "content_block_start",
                "index": 0,
                "content_block": {"type": "text", "text": ""},
            }
        )
        self.write_sse(
            {
                "type": "content_block_delta",
                "index": 0,
                "delta": {"type": "text_delta", "text": f"{route}:part-1"},
            }
        )
        if paced:
            time.sleep(0.45)
        self.write_sse(
            {
                "type": "content_block_delta",
                "index": 0,
                "delta": {"type": "text_delta", "text": ":part-2"},
            }
        )
        self.write_sse({"type": "content_block_stop", "index": 0})
        self.write_sse(
            {
                "type": "message_delta",
                "delta": {"stop_reason": "end_turn", "stop_sequence": None},
                "usage": {"output_tokens": 2},
            }
        )
        self.write_sse({"type": "message_stop"})

    def write_sse(self, body):
        raw = json.dumps(body, separators=(",", ":")).encode("utf-8")
        event_type = str(body.get("type") or "message")
        self.wfile.write(b"event: " + event_type.encode("utf-8") + b"\n")
        self.wfile.write(b"data: " + raw + b"\n\n")
        self.wfile.flush()

    def write_json(self, status, body):
        raw = json.dumps(body, separators=(",", ":")).encode("utf-8")
        self.send_response(status)
        self.send_header("content-type", "application/json")
        self.send_header("content-length", str(len(raw)))
        self.send_header("connection", "close")
        self.end_headers()
        self.wfile.write(raw)


class ClaudeIntegrityDockerTest:
    def __init__(self, args):
        self.args = args
        self.base_url = args.base_url.rstrip("/")
        self.namespace = args.namespace
        self.model = f"claude-integrity-{self.namespace}"
        self.bad_group = f"{self.namespace}_bad"
        self.good_group = f"{self.namespace}_good"
        self.aggregate_group = f"{self.namespace}_aggregate"
        self.username = f"{self.namespace}_user"
        self.password = f"Ci!{secrets.token_hex(7)}"
        self.token_key = secrets.token_hex(24)
        self.root_access_token = secrets.token_hex(16)
        self.root_user_id = None
        self.root_original_access_token = None
        self.user_id = None
        self.token_id = None
        self.bad_channel_id = None
        self.good_channel_id = None
        self.aggregate_group_id = None
        self.aggregate_payload = None
        self.option_snapshot = {}
        self.initial_error_snapshot_count = 0
        self.fake_server = None
        self.tests = []
        self.result = "running"
        self.error = ""
        self.started_at = int(time.time())
        self.log_dir = Path(args.log_dir)
        self.log_dir.mkdir(parents=True, exist_ok=True)
        self.fake_log = self.log_dir / f"{self.namespace}-fake-upstream.jsonl"
        self.report_path = self.log_dir / f"{self.namespace}-report.json"

    def log(self, message):
        print(f"[{datetime.now().astimezone().isoformat()}] {message}", flush=True)

    def passed(self, name, detail=""):
        self.tests.append({"name": name, "status": "passed", "detail": detail})
        self.log(f"PASS {name}{': ' + detail if detail else ''}")

    def psql(self, sql):
        return run_cmd(
            [
                "docker",
                "exec",
                self.args.postgres_container,
                "psql",
                "-U",
                self.args.postgres_user,
                "-d",
                self.args.postgres_db,
                "-At",
                "-v",
                "ON_ERROR_STOP=1",
                "-c",
                sql,
            ]
        )

    def psql_scalar(self, sql):
        raw = self.psql(sql)
        return next((line.strip() for line in raw.splitlines() if line.strip()), "")

    def psql_json(self, sql, default):
        raw = self.psql_scalar(sql)
        return json.loads(raw) if raw else default

    def redis(self, *args):
        return run_cmd(
            ["docker", "exec", self.args.redis_container, "redis-cli", *args],
            check=False,
        )

    def admin_headers(self):
        require(self.root_user_id is not None, "root user is not initialized")
        return {
            "Authorization": f"Bearer {self.root_access_token}",
            "New-Api-User": str(self.root_user_id),
        }

    def request(self, method, path, body=None, headers=None, timeout=None):
        request_headers = {"Content-Type": "application/json"}
        if headers:
            request_headers.update(headers)
        data = None if body is None else json.dumps(body).encode("utf-8")
        req = urllib.request.Request(
            f"{self.base_url}{path}",
            data=data,
            method=method,
            headers=request_headers,
        )
        try:
            with NO_PROXY_OPENER.open(
                req, timeout=timeout or self.args.request_timeout
            ) as response:
                raw = response.read()
                payload, text = decode_json(raw)
                return response.status, payload, text
        except urllib.error.HTTPError as exc:
            raw = exc.read()
            payload, text = decode_json(raw)
            return exc.code, payload, text

    def request_api(self, method, path, body=None):
        status, payload, text = self.request(
            method, path, body=body, headers=self.admin_headers()
        )
        require(status == 200, f"{method} {path} returned HTTP {status}: {text}")
        require(
            isinstance(payload, dict) and payload.get("success") is True,
            f"{method} {path} failed: {payload or text}",
        )
        return payload.get("data")

    def request_raw(self, method, path, body=None, headers=None):
        request_headers = {}
        if headers:
            request_headers.update(headers)
        data = None
        if body is not None:
            request_headers["Content-Type"] = "application/json"
            data = json.dumps(body).encode("utf-8")
        req = urllib.request.Request(
            f"{self.base_url}{path}",
            data=data,
            method=method,
            headers=request_headers,
        )
        try:
            with NO_PROXY_OPENER.open(req, timeout=self.args.request_timeout) as response:
                return response.status, response.read(), dict(response.headers.items())
        except urllib.error.HTTPError as exc:
            return exc.code, exc.read(), dict(exc.headers.items())

    def gateway_request(self, scenario, stream=False):
        return self.request(
            "POST",
            "/v1/messages",
            body={
                "model": self.model,
                "max_tokens": 16,
                "stream": stream,
                "messages": [
                    {"role": "user", "content": f"scenario:{scenario} verify"}
                ],
            },
            headers={
                "Authorization": f"Bearer sk-{self.token_key}",
                "anthropic-version": "2023-06-01",
            },
            timeout=self.args.stream_request_timeout if stream else None,
        )

    def gateway_stream(self, scenario):
        body = {
            "model": self.model,
            "max_tokens": 16,
            "stream": True,
            "messages": [
                {"role": "user", "content": f"scenario:{scenario} verify"}
            ],
        }
        req = urllib.request.Request(
            f"{self.base_url}/v1/messages",
            data=json.dumps(body).encode("utf-8"),
            method="POST",
            headers={
                "Content-Type": "application/json",
                "Authorization": f"Bearer sk-{self.token_key}",
                "anthropic-version": "2023-06-01",
            },
        )
        started = time.monotonic()
        with NO_PROXY_OPENER.open(
            req, timeout=self.args.stream_request_timeout
        ) as response:
            lines = []
            first_delta = None
            while True:
                raw = response.readline()
                if not raw:
                    break
                line = raw.decode("utf-8", errors="replace")
                lines.append(line)
                if "part-1" in line and first_delta is None:
                    first_delta = time.monotonic() - started
            total = time.monotonic() - started
            return response.status, "".join(lines), first_delta, total

    def ensure_ready(self):
        deadline = time.time() + self.args.ready_timeout
        while time.time() < deadline:
            health = run_cmd(
                [
                    "docker",
                    "inspect",
                    "--format",
                    "{{.State.Health.Status}}",
                    self.args.app_container,
                ],
                check=False,
            )
            if health == "healthy":
                status, payload, _ = self.request("GET", "/api/status")
                if status == 200 and isinstance(payload, dict):
                    if payload.get("success") is True:
                        return
            time.sleep(1)
        raise RuntimeError(f"{self.args.app_container} did not become healthy")

    def start_fake_upstream(self):
        FakeClaudeUpstream.reset(self.fake_log)
        self.fake_server = ThreadingHTTPServer(
            (self.args.listen_host, self.args.fake_port), FakeClaudeUpstream
        )
        thread = threading.Thread(target=self.fake_server.serve_forever, daemon=True)
        thread.start()
        self.log(
            f"fake Claude upstream listening on {self.args.listen_host}:{self.args.fake_port}"
        )

    def stop_fake_upstream(self):
        if self.fake_server is not None:
            self.fake_server.shutdown()
            self.fake_server.server_close()
            self.fake_server = None

    def prepare_admin_access(self):
        root = self.psql_json(
            """
            select json_build_object('id', id, 'access_token', access_token)
            from users
            where role >= 100 and deleted_at is null
            order by id limit 1
            """,
            {},
        )
        require(root, "root user not found")
        self.root_user_id = int(root["id"])
        self.root_original_access_token = root.get("access_token")
        self.psql(
            f"update users set access_token = {sql_str(self.root_access_token)} "
            f"where id = {self.root_user_id}"
        )

        option_data = self.request_api("GET", "/api/option/") or []
        current = {
            str(item.get("key")): str(item.get("value"))
            for item in option_data
            if isinstance(item, dict)
        }
        for key, default in OPTION_DEFAULTS.items():
            db_value = self.psql_scalar(
                f"select value from options where key = {sql_str(key)} limit 1"
            )
            self.option_snapshot[key] = {
                "exists": db_value != "",
                "value": current.get(key, db_value or default),
            }
        self.initial_error_snapshot_count = int(
            self.psql_scalar("select count(*) from error_snapshots") or 0
        )
        self.passed("runtime configuration snapshot")

    def update_option(self, key, value):
        self.request_api(
            "PUT", "/api/option/", {"key": key, "value": value}
        )

    def install_real_groups(self):
        raw = self.option_snapshot["GroupRatio"]["value"]
        try:
            group_ratios = json.loads(raw) if raw else {}
        except json.JSONDecodeError as exc:
            raise RuntimeError(f"invalid existing GroupRatio: {exc}") from exc
        require(isinstance(group_ratios, dict), "existing GroupRatio is not an object")
        group_ratios[self.bad_group] = 1
        group_ratios[self.good_group] = 1
        self.update_option(
            "GroupRatio", json.dumps(group_ratios, separators=(",", ":"))
        )
        for key in ("ModelRatio", "CompletionRatio"):
            raw = self.option_snapshot[key]["value"]
            try:
                ratios = json.loads(raw) if raw else {}
            except json.JSONDecodeError as exc:
                raise RuntimeError(f"invalid existing {key}: {exc}") from exc
            require(isinstance(ratios, dict), f"existing {key} is not an object")
            ratios[self.model] = 1
            self.update_option(key, json.dumps(ratios, separators=(",", ":")))

    def create_entities(self):
        self.request_api(
            "POST",
            "/api/user/",
            {
                "username": self.username,
                "password": self.password,
                "display_name": self.username,
                "role": 10,
            },
        )
        self.user_id = int(
            self.psql_scalar(
                f"select id from users where username = {sql_str(self.username)} "
                "and deleted_at is null limit 1"
            )
        )
        self.psql(
            f"update users set status=1, quota=1000000000, used_quota=0, "
            f"request_count=0, \"group\"='UserGroup-admin' where id={self.user_id}"
        )
        self.redis("del", f"user:{self.user_id}")
        now = int(time.time())
        self.token_id = int(
            self.psql_scalar(
                f"""
                insert into tokens (
                  user_id, key, status, name, created_time, accessed_time,
                  expired_time, remain_quota, unlimited_quota, model_limits_enabled,
                  model_limits, used_quota, "group", cross_group_retry
                ) values (
                  {self.user_id}, {sql_str(self.token_key)}, 1,
                  {sql_str(self.namespace + '_token')}, {now}, 0, -1,
                  1000000000, true, false, '', 0,
                  {sql_str(self.aggregate_group)}, false
                ) returning id
                """
            )
        )

        self.create_channel("bad", self.bad_group)
        self.create_channel("good", self.good_group)
        rows = self.psql_json(
            f"""
            select coalesce(json_agg(json_build_object('id', id, 'name', name)), '[]'::json)
            from channels where tag={sql_str(self.namespace)}
            """,
            [],
        )
        by_name = {row["name"]: int(row["id"]) for row in rows}
        self.bad_channel_id = by_name.get(f"{self.namespace}_bad_channel")
        self.good_channel_id = by_name.get(f"{self.namespace}_good_channel")
        require(self.bad_channel_id and self.good_channel_id, f"missing channels: {rows}")
        self.request_api("POST", "/api/channel/tag/enabled", {"tag": self.namespace})

        self.aggregate_payload = {
            "name": self.aggregate_group,
            "display_name": f"Claude integrity {self.namespace}",
            "description": "Temporary Docker integrity regression group",
            "status": 1,
            "group_ratio": 1,
            "routing_mode": "failover",
            "smart_routing_enabled": False,
            "recovery_enabled": True,
            "recovery_interval_seconds": 30,
            "cluster_affinity_ttl_seconds": 60,
            "route_affinity_strategy": "off",
            "route_affinity_scope": "shared",
            "route_affinity_key_sources": [],
            "retry_status_codes": "500-599",
            "visible_user_groups": ["UserGroup-admin"],
            "targets": [
                {"real_group": self.bad_group, "weight": 100, "rpm_limit": 0},
                {"real_group": self.good_group, "weight": 100, "rpm_limit": 0},
            ],
            "client_route_pools": {
                "enabled": False,
                "claude_code_cli": {
                    "enabled": False,
                    "fallback_to_default": True,
                    "targets": [],
                },
            },
            "smart_strategy_config": None,
            "route_model_group_ratio_overrides": [],
        }
        data = self.request_api(
            "POST", "/api/aggregate_group/", self.aggregate_payload
        )
        self.aggregate_group_id = int(data["id"])
        self.aggregate_payload["id"] = self.aggregate_group_id
        self.passed(
            "isolated aggregate entities created",
            f"bad={self.bad_channel_id} good={self.good_channel_id}",
        )

    def create_channel(self, route, group):
        self.request_api(
            "POST",
            "/api/channel/",
            {
                "mode": "single",
                "channel": {
                    "type": 14,
                    "key": f"fake-{route}-{self.namespace}",
                    "status": 1,
                    "name": f"{self.namespace}_{route}_channel",
                    "weight": 100,
                    "base_url": (
                        f"http://{self.args.fake_upstream_host}:"
                        f"{self.args.fake_port}/{route}"
                    ),
                    "models": self.model,
                    "group": group,
                    "auto_ban": 0,
                    "priority": 100,
                    "tag": self.namespace,
                    "channel_info": {"is_multi_key": False},
                    "settings": "",
                },
            },
        )

    def update_retry_status_codes(self, value):
        payload = dict(self.aggregate_payload)
        payload["retry_status_codes"] = value
        data = self.request_api("PUT", "/api/aggregate_group/", payload)
        require(
            data.get("retry_status_codes") == value,
            f"aggregate retry_status_codes did not update: {data}",
        )
        self.aggregate_payload = payload

    def reset_route_state(self):
        key = f"aggregate_group:state:{self.aggregate_group}:{self.model}"
        self.redis("del", key)

    def update_error_snapshot_settings(
        self, enabled, priority_user_ids=None, priority_channel_ids=None
    ):
        return self.request_api(
            "PUT",
            "/api/request_dump/error_snapshots/settings",
            {
                "enabled": bool(enabled),
                "ttl_minutes": 60,
                "max_storage_mib": 256,
                "max_files": 1000,
                "priority_user_ids": priority_user_ids or [],
                "priority_channel_ids": priority_channel_ids or [],
            },
        )

    def error_snapshot_rows(self):
        if self.user_id is None:
            return []
        return self.psql_json(
            f"""
            select coalesce(json_agg(row_to_json(t) order by created_at, id), '[]'::json)
            from (
              select id, request_id, channel_id, retry_index, status_code,
                     error_code, capture_level, final_outcome, relative_path,
                     compressed_size, payload_truncated, created_at
              from error_snapshots
              where user_id={self.user_id}
              order by created_at, id
            ) t
            """,
            [],
        )

    def wait_for_new_error_snapshots(self, before_ids, expected):
        deadline = time.time() + self.args.log_wait_timeout
        latest = []
        while time.time() < deadline:
            rows = self.error_snapshot_rows()
            latest = [row for row in rows if row["id"] not in before_ids]
            if len(latest) >= expected and all(
                row.get("final_outcome") != "pending" for row in latest
            ):
                return latest
            time.sleep(0.1)
        raise AssertionError(
            f"expected {expected} finalized error snapshots, got {latest}"
        )

    def snapshot_detail(self, snapshot_id):
        return self.request_api(
            "GET", f"/api/request_dump/error_snapshots/{snapshot_id}"
        )

    def max_consume_log_id(self):
        value = self.psql_scalar(
            f"select coalesce(max(id), 0) from logs where user_id={self.user_id} and type=2"
        )
        return int(value or 0)

    def consume_logs_after(self, log_id, expected, wait=True):
        deadline = time.time() + (self.args.log_wait_timeout if wait else 0)
        while True:
            rows = self.psql_json(
                f"""
                select coalesce(json_agg(row_to_json(t) order by id), '[]'::json)
                from (
                  select id, channel_id, quota, prompt_tokens, completion_tokens,
                         request_id,
                         case when coalesce(other, '')='' then '{{}}'::json else other::json end as other
                  from logs
                  where user_id={self.user_id} and type=2 and id>{int(log_id)}
                  order by id
                ) t
                """,
                [],
            )
            if len(rows) >= expected or time.time() >= deadline:
                return rows
            time.sleep(0.2)

    def require_routes(self, scenario, expected):
        routes = FakeClaudeUpstream.scenario_routes(scenario)
        require(routes == expected, f"{scenario} routes {routes}, expected {expected}")

    def verify_legacy_switch_off(self, scenario):
        self.reset_route_state()
        before = self.max_consume_log_id()
        status, payload, text = self.gateway_request(scenario)
        require(status == 200, f"legacy empty failed: HTTP {status} {payload or text}")
        require(isinstance(payload, dict), f"legacy response is not JSON: {text}")
        require(payload.get("content") == [], f"legacy empty content changed: {payload}")
        self.require_routes(scenario, ["bad"])
        logs = self.consume_logs_after(before, 1)
        require(len(logs) == 1, f"legacy request should settle once: {logs}")

    def verify_error_snapshot_disabled(self):
        before_ids = {row["id"] for row in self.error_snapshot_rows()}
        self.verify_legacy_switch_off("legacy-empty-off")
        time.sleep(0.3)
        after_ids = {row["id"] for row in self.error_snapshot_rows()}
        require(after_ids == before_ids, "disabled error snapshot capture wrote a row")
        self.passed("error snapshot switch off does not capture failures")

    def verify_non_stream_fallback(self):
        scenario = "fallback-empty"
        before_ids = {row["id"] for row in self.error_snapshot_rows()}
        self.reset_route_state()
        before = self.max_consume_log_id()
        status, payload, text = self.gateway_request(scenario)
        require(status == 200, f"fallback failed: HTTP {status} {payload or text}")
        require(
            payload["content"][0]["text"] == f"good:{scenario}",
            f"unexpected fallback payload: {payload}",
        )
        self.require_routes(scenario, ["bad", "good"])
        logs = self.consume_logs_after(before, 1)
        require(len(logs) == 1, f"fallback should create one consume log: {logs}")
        require(
            int(logs[0]["channel_id"]) == self.good_channel_id,
            f"failed attempt polluted final billing log: {logs}",
        )
        snapshots = self.wait_for_new_error_snapshots(before_ids, 1)
        require(len(snapshots) == 1, f"unexpected fallback snapshots: {snapshots}")
        snapshot = snapshots[0]
        require(snapshot["channel_id"] == self.bad_channel_id, str(snapshot))
        require(snapshot["capture_level"] == "summary", str(snapshot))
        require(snapshot["final_outcome"] == "fallback_succeeded", str(snapshot))
        require(snapshot["error_code"] == "claude_content_block_missing", str(snapshot))
        detail = self.snapshot_detail(snapshot["id"])
        envelope = detail.get("payload") or {}
        require("client_request" not in envelope, str(envelope))
        require("upstream_request" not in envelope, str(envelope))
        require("upstream_response" in envelope, str(envelope))
        request_id = urllib.parse.quote(snapshot["request_id"], safe="")
        page = self.request_api(
            "GET", f"/api/request_dump/error_snapshots?request_id={request_id}"
        )
        require(page.get("total") == 1, f"request-id list mismatch: {page}")
        self.passed(
            "non-stream empty content falls back across child groups",
            "RetryTimes=3 still called bad once and billed good once",
        )

    def verify_priority_error_snapshot(self):
        self.update_error_snapshot_settings(
            True,
            priority_user_ids=[self.user_id],
            priority_channel_ids=[self.bad_channel_id],
        )
        scenario = "priority-fallback"
        before_ids = {row["id"] for row in self.error_snapshot_rows()}
        self.reset_route_state()
        status, payload, text = self.gateway_request(scenario)
        require(status == 200, f"priority fallback failed: {payload or text}")
        self.require_routes(scenario, ["bad", "good"])
        snapshots = self.wait_for_new_error_snapshots(before_ids, 1)
        require(len(snapshots) == 1, str(snapshots))
        snapshot = snapshots[0]
        require(snapshot["capture_level"] == "priority", str(snapshot))
        detail = self.snapshot_detail(snapshot["id"])
        envelope = detail.get("payload") or {}
        client = envelope.get("client_request") or {}
        upstream = envelope.get("upstream_request") or {}
        require("scenario:priority-fallback" in client.get("body", ""), str(client))
        require("scenario:priority-fallback" in upstream.get("body", ""), str(upstream))
        serialized = json.dumps(envelope, ensure_ascii=False)
        require(self.token_key not in serialized, "client API key leaked into snapshot")
        self.update_error_snapshot_settings(True)
        self.passed("priority snapshot captures sanitized client and upstream requests")

    def verify_retry_status_exclusion(self):
        self.update_retry_status_codes("500-501,503-599")
        scenario = "retry-excluded"
        self.reset_route_state()
        before = self.max_consume_log_id()
        status, payload, text = self.gateway_request(scenario)
        require(status == 502, f"excluded 502 returned HTTP {status}: {payload or text}")
        self.require_routes(scenario, ["bad"])
        time.sleep(0.5)
        logs = self.consume_logs_after(before, 0, wait=False)
        require(not logs, f"failed protected request was billed: {logs}")
        self.update_retry_status_codes("500-599")
        self.passed("aggregate retry_status_codes exclusion is honored")

    def verify_stream_fallback(self, scenario):
        self.reset_route_state()
        before = self.max_consume_log_id()
        status, _, text = self.gateway_request(scenario, stream=True)
        require(status == 200, f"{scenario} failed: HTTP {status} {text}")
        require("good:part-1" in text, f"{scenario} did not return good stream: {text}")
        self.require_routes(scenario, ["bad", "good"])
        logs = self.consume_logs_after(before, 1)
        require(len(logs) == 1, f"{scenario} should settle once: {logs}")
        require(int(logs[0]["channel_id"]) == self.good_channel_id, str(logs))

    def verify_inflight_snapshot(self):
        scenario = "snapshot-enabled"
        self.reset_route_state()
        before = self.max_consume_log_id()
        result = {}

        def invoke():
            try:
                result["value"] = self.gateway_request(scenario, stream=True)
            except Exception as exc:
                result["error"] = exc

        thread = threading.Thread(target=invoke, daemon=True)
        thread.start()
        deadline = time.time() + 3
        while time.time() < deadline:
            if FakeClaudeUpstream.scenario_routes(scenario) == ["bad"]:
                break
            time.sleep(0.02)
        require(thread.is_alive(), "snapshot request completed before switch update")
        self.update_option("claude.response_integrity_fallback_enabled", False)
        thread.join(self.args.stream_request_timeout)
        require(not thread.is_alive(), "snapshot request did not finish")
        require("error" not in result, f"snapshot request error: {result.get('error')}")
        status, _, text = result["value"]
        require(status == 200 and "good:part-1" in text, text)
        self.require_routes(scenario, ["bad", "good"])
        logs = self.consume_logs_after(before, 1)
        require(len(logs) == 1 and int(logs[0]["channel_id"]) == self.good_channel_id, str(logs))
        self.verify_legacy_switch_off("legacy-empty-off-again")
        self.update_option("claude.response_integrity_fallback_enabled", True)
        self.passed("in-flight request keeps enabled snapshot while new requests see switch off")

    def verify_normal_realtime_stream(self):
        scenario = "stream-normal"
        self.reset_route_state()
        before = self.max_consume_log_id()
        status, text, first_delta, total = self.gateway_stream(scenario)
        require(status == 200, f"normal stream HTTP {status}")
        require("event: message_stop" in text, f"normal stream incomplete: {text}")
        require(first_delta is not None, f"first delta not observed: {text}")
        require(
            total - first_delta >= 0.25,
            f"stream was buffered to completion: first={first_delta:.3f}s total={total:.3f}s",
        )
        self.require_routes(scenario, ["bad"])
        logs = self.consume_logs_after(before, 1)
        require(len(logs) == 1 and int(logs[0]["channel_id"]) == self.bad_channel_id, str(logs))
        self.passed(
            "valid stream remains real-time on first child group",
            f"first_delta={first_delta:.3f}s total={total:.3f}s",
        )

    def verify_incomplete_stream(self):
        scenario = "stream-incomplete"
        before_ids = {row["id"] for row in self.error_snapshot_rows()}
        self.reset_route_state()
        before = self.max_consume_log_id()
        status, _, text = self.gateway_request(scenario, stream=True)
        require(status == 200, f"incomplete stream HTTP {status}: {text}")
        require("claude_stream_incomplete" in text, f"missing SSE error: {text}")
        self.require_routes(scenario, ["bad"])
        logs = self.consume_logs_after(before, 1)
        require(len(logs) == 1, f"incomplete stream should settle once: {logs}")
        require(int(logs[0]["channel_id"]) == self.bad_channel_id, str(logs))
        admin_info = (logs[0].get("other") or {}).get("admin_info") or {}
        require(admin_info.get("claude_stream_incomplete") is True, str(logs))
        snapshots = self.wait_for_new_error_snapshots(before_ids, 1)
        require(len(snapshots) == 1, str(snapshots))
        snapshot = snapshots[0]
        require(snapshot["final_outcome"] == "stream_incomplete", str(snapshot))
        detail = self.snapshot_detail(snapshot["id"])
        require((detail.get("payload") or {}).get("stream"), str(detail))
        self.passed("post-commit truncation emits SSE error without fallback and is marked")

    def verify_all_failed(self):
        scenario = "all-fail"
        before_ids = {row["id"] for row in self.error_snapshot_rows()}
        self.reset_route_state()
        before = self.max_consume_log_id()
        status, payload, text = self.gateway_request(scenario)
        require(status == 502, f"all-fail returned HTTP {status}: {payload or text}")
        require(isinstance(payload, dict), f"all-fail response is not structured: {text}")
        error = payload.get("error") or {}
        require(
            error.get("code") == "claude_content_block_missing",
            f"unexpected all-fail error: {payload}",
        )
        self.require_routes(scenario, ["bad", "good"])
        time.sleep(0.5)
        logs = self.consume_logs_after(before, 0, wait=False)
        require(not logs, f"all-fail request was billed: {logs}")
        snapshots = self.wait_for_new_error_snapshots(before_ids, 2)
        require(len(snapshots) == 2, str(snapshots))
        require(len({row["request_id"] for row in snapshots}) == 1, str(snapshots))
        require(
            all(row["final_outcome"] == "final_failure" for row in snapshots),
            str(snapshots),
        )
        require(
            {row["channel_id"] for row in snapshots}
            == {self.bad_channel_id, self.good_channel_id},
            str(snapshots),
        )
        self.passed("all child groups failing returns structured unbilled 502")

    def verify_error_snapshot_management(self):
        status = self.request_api("GET", "/api/request_dump/error_snapshots/status")
        require(status.get("settings", {}).get("enabled") is True, str(status))
        require(
            str(status.get("storage_path", "")).endswith("/error-snapshots"),
            str(status),
        )
        page = self.request_api(
            "GET",
            f"/api/request_dump/error_snapshots?user_id={self.user_id}&p=1&page_size=100",
        )
        items = page.get("items") or []
        require(page.get("total") == len(items) and items, str(page))

        selected = items[0]
        detail = self.snapshot_detail(selected["id"])
        require(detail.get("snapshot", {}).get("id") == selected["id"], str(detail))
        download_status, compressed, headers = self.request_raw(
            "GET",
            f"/api/request_dump/error_snapshots/{selected['id']}/download",
            headers=self.admin_headers(),
        )
        require(download_status == 200, f"snapshot download HTTP {download_status}")
        require(compressed.startswith(b"\x1f\x8b"), "download is not gzip")
        decoded = gzip.decompress(compressed)
        require(len(decoded) <= 128 * 1024, f"snapshot exceeds bound: {len(decoded)}")
        require(
            "application/gzip" in headers.get("Content-Type", ""),
            str(headers),
        )

        self.request_api(
            "DELETE", f"/api/request_dump/error_snapshots/{selected['id']}"
        )
        exists = self.psql_scalar(
            "select count(*) from error_snapshots where id=" + sql_str(selected["id"])
        )
        require(exists == "0", f"deleted snapshot still indexed: {exists}")
        self.request_api("POST", "/api/request_dump/error_snapshots/cleanup")

        if self.initial_error_snapshot_count == 0:
            self.request_api("DELETE", "/api/request_dump/error_snapshots")
            remaining = self.psql_scalar("select count(*) from error_snapshots")
            require(remaining == "0", f"clear all left {remaining} snapshots")
            self.passed(
                "error snapshot detail, gzip download, delete, cleanup, and clear APIs"
            )
        else:
            for snapshot in self.error_snapshot_rows():
                self.request_api(
                    "DELETE", f"/api/request_dump/error_snapshots/{snapshot['id']}"
                )
            self.passed(
                "error snapshot detail, gzip download, delete, and cleanup APIs",
                "clear-all skipped because snapshots existed before this test",
            )

    def run(self):
        self.ensure_ready()
        self.start_fake_upstream()
        self.prepare_admin_access()
        self.install_real_groups()
        self.create_entities()
        self.update_option("RetryTimes", 3)
        self.update_option("claude.response_integrity_first_block_timeout_seconds", 30)
        self.update_option("claude.response_integrity_fallback_enabled", False)
        self.update_error_snapshot_settings(False)

        self.verify_error_snapshot_disabled()
        self.passed("switch off preserves legacy empty-content response")

        self.update_option("claude.response_integrity_fallback_enabled", True)
        self.update_error_snapshot_settings(True)
        for timeout_value in (30, 45, 60):
            self.update_option(
                "claude.response_integrity_first_block_timeout_seconds",
                timeout_value,
            )
            stored = self.psql_scalar(
                "select value from options where key="
                + sql_str("claude.response_integrity_first_block_timeout_seconds")
            )
            require(stored == str(timeout_value), f"timeout {timeout_value} not stored: {stored}")
        self.passed("30/45/60 second timeout values hot-save without restart")
        self.update_option("claude.response_integrity_first_block_timeout_seconds", 1)

        self.verify_non_stream_fallback()
        self.verify_priority_error_snapshot()
        self.verify_retry_status_exclusion()
        self.verify_stream_fallback("stream-eof")
        self.passed("EOF before first content block falls back")
        self.verify_stream_fallback("stream-timeout")
        self.passed("absolute first-block timeout cancels attempt and falls back")
        self.verify_inflight_snapshot()
        self.verify_normal_realtime_stream()
        self.verify_incomplete_stream()
        self.verify_all_failed()
        self.verify_error_snapshot_management()
        self.result = "passed"

    def restore_options(self):
        if not self.option_snapshot or self.root_user_id is None:
            return
        for key, snapshot in self.option_snapshot.items():
            value = snapshot["value"]
            try:
                self.update_option(key, value)
            except Exception:
                self.psql(
                    "insert into options(key, value) values "
                    f"({sql_str(key)}, {sql_str(value)}) "
                    "on conflict(key) do update set value=excluded.value"
                )
            if not snapshot["exists"]:
                self.psql(f"delete from options where key={sql_str(key)}")

    def cleanup_entities(self):
        self.reset_route_state()
        if self.aggregate_group_id is None:
            value = self.psql_scalar(
                "select id from aggregate_groups where name="
                f"{sql_str(self.aggregate_group)} limit 1"
            )
            self.aggregate_group_id = int(value) if value else None
        if self.user_id is None:
            value = self.psql_scalar(
                "select id from users where username="
                f"{sql_str(self.username)} limit 1"
            )
            self.user_id = int(value) if value else None
        if self.token_id is None:
            value = self.psql_scalar(
                "select id from tokens where name="
                f"{sql_str(self.namespace + '_token')} limit 1"
            )
            self.token_id = int(value) if value else None
        resolved_channels = self.psql_json(
            "select coalesce(json_agg(id order by id), '[]'::json) "
            f"from channels where tag={sql_str(self.namespace)}",
            [],
        )
        if self.bad_channel_id is not None:
            resolved_channels.append(self.bad_channel_id)
        if self.good_channel_id is not None:
            resolved_channels.append(self.good_channel_id)
        channel_ids = sorted({int(value) for value in resolved_channels})

        for snapshot in self.error_snapshot_rows():
            try:
                self.request_api(
                    "DELETE", f"/api/request_dump/error_snapshots/{snapshot['id']}"
                )
            except Exception as exc:
                self.log(f"snapshot {snapshot['id']} cleanup failed: {exc}")

        if self.aggregate_group_id is not None:
            try:
                self.request_api(
                    "DELETE", f"/api/aggregate_group/{self.aggregate_group_id}"
                )
            except Exception as exc:
                self.log(f"aggregate cleanup API fallback: {exc}")
            self.psql(
                f"delete from aggregate_groups where id={self.aggregate_group_id}"
            )
        for channel_id in channel_ids:
            try:
                self.request_api("DELETE", f"/api/channel/{channel_id}")
            except Exception as exc:
                self.log(f"channel {channel_id} cleanup API fallback: {exc}")

        if self.user_id is not None:
            self.psql(f"delete from logs where user_id={self.user_id}")
        if self.token_id is not None:
            self.psql(f"delete from tokens where id={self.token_id}")
        if channel_ids:
            joined = ",".join(str(value) for value in channel_ids)
            self.psql(f"delete from abilities where channel_id in ({joined})")
            self.psql(f"delete from channels where id in ({joined})")
        if self.user_id is not None:
            self.psql(f"delete from admin_menu_permissions where user_id={self.user_id}")
            self.psql(f"delete from users where id={self.user_id}")
            self.redis("del", f"user:{self.user_id}")

    def restore_admin_access(self):
        if self.root_user_id is not None:
            self.psql(
                f"update users set access_token={sql_str(self.root_original_access_token)} "
                f"where id={self.root_user_id}"
            )

    def write_report(self):
        report = {
            "namespace": self.namespace,
            "result": self.result,
            "error": self.error,
            "started_at": self.started_at,
            "finished_at": int(time.time()),
            "model": self.model,
            "tests": self.tests,
            "fake_upstream_events": FakeClaudeUpstream.events,
            "restored_options": self.option_snapshot,
        }
        with self.report_path.open("w", encoding="utf-8") as handle:
            json.dump(report, handle, ensure_ascii=False, indent=2)
            handle.write("\n")

    def cleanup(self):
        try:
            self.restore_options()
            self.cleanup_entities()
        finally:
            try:
                self.restore_admin_access()
            finally:
                self.stop_fake_upstream()
        self.write_report()
        self.log(f"cleanup complete; report={self.report_path}")


def build_parser():
    namespace = "ci" + datetime.now().strftime("%H%M%S") + secrets.token_hex(2)
    parser = argparse.ArgumentParser(
        description="Docker-dev regression for Claude content-block integrity fallback."
    )
    parser.add_argument("--namespace", default=namespace)
    parser.add_argument("--base-url", default="http://localhost:3001")
    parser.add_argument("--listen-host", default="127.0.0.1")
    parser.add_argument("--fake-upstream-host", default="host.docker.internal")
    parser.add_argument("--fake-port", type=int, default=19146)
    parser.add_argument("--app-container", default="new-api-dev")
    parser.add_argument("--postgres-container", default="postgres-dev")
    parser.add_argument("--postgres-user", default="root")
    parser.add_argument("--postgres-db", default="new-api")
    parser.add_argument("--redis-container", default="redis-dev")
    parser.add_argument(
        "--log-dir", default="tmp/claude-response-integrity-validation"
    )
    parser.add_argument("--request-timeout", type=float, default=20)
    parser.add_argument("--stream-request-timeout", type=float, default=30)
    parser.add_argument("--ready-timeout", type=float, default=180)
    parser.add_argument("--log-wait-timeout", type=float, default=12)
    return parser


def main():
    args = build_parser().parse_args()
    test = ClaudeIntegrityDockerTest(args)
    exit_code = 0
    try:
        test.run()
    except Exception as exc:
        test.result = "failed"
        test.error = str(exc)
        test.log(f"FAIL {exc}")
        exit_code = 1
    finally:
        try:
            test.cleanup()
        except Exception as cleanup_exc:
            test.error = f"{test.error}; cleanup failed: {cleanup_exc}".strip("; ")
            test.log(f"CLEANUP_FAIL {cleanup_exc}")
            try:
                test.restore_admin_access()
            finally:
                test.stop_fake_upstream()
            test.write_report()
            exit_code = 1
    return exit_code


if __name__ == "__main__":
    sys.exit(main())
