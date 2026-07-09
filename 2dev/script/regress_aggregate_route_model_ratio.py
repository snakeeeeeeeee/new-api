#!/usr/bin/env python3
import argparse
import http.cookiejar
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


def require(condition, message):
    if not condition:
        raise AssertionError(message)


def parse_log_other(log):
    other = log.get("other") or {}
    if isinstance(other, str):
        other = json.loads(other) if other else {}
    require(isinstance(other, dict), f"invalid log other payload: {other}")
    return other


class FakeOpenAIUpstream(BaseHTTPRequestHandler):
    namespace = ""
    log_path = None
    events = []
    lock = threading.Lock()

    def log_message(self, fmt, *args):
        return

    def do_POST(self):
        length = int(self.headers.get("content-length") or "0")
        raw = self.rfile.read(length) if length > 0 else b"{}"
        try:
            body = json.loads(raw.decode("utf-8"))
        except Exception:
            body = {}

        route = self.path.strip("/").split("/", 1)[0] or "unknown"
        messages = body.get("messages") or []
        prompt = " ".join(
            str(item.get("content") or "")
            for item in messages
            if isinstance(item, dict)
        )
        status = 500 if route == "low" and "force-high" in prompt else 200
        event = {
            "timestamp": datetime.now(timezone.utc).isoformat(),
            "namespace": self.namespace,
            "route": route,
            "status": status,
            "path": self.path,
            "model": body.get("model"),
            "prompt": prompt,
        }
        with self.lock:
            self.events.append(event)
            if self.log_path is not None:
                with self.log_path.open("a", encoding="utf-8") as handle:
                    handle.write(json.dumps(event, ensure_ascii=False) + "\n")

        if status != 200:
            self.write_json(
                status,
                {
                    "error": {
                        "message": "forced low-route failure for ratio regression",
                        "type": "upstream_error",
                        "code": "ratio_regression_low_failure",
                    }
                },
            )
            return

        model = str(body.get("model") or "gpt-4o-mini")
        self.write_json(
            200,
            {
                "id": f"chatcmpl-{self.namespace}-{route}-{int(time.time() * 1000)}",
                "object": "chat.completion",
                "created": int(time.time()),
                "model": model,
                "choices": [
                    {
                        "index": 0,
                        "message": {
                            "role": "assistant",
                            "content": f"route={route} run={self.namespace}",
                        },
                        "finish_reason": "stop",
                    }
                ],
                "usage": {
                    "prompt_tokens": 100,
                    "completion_tokens": 20,
                    "total_tokens": 120,
                },
            },
        )

    def write_json(self, status, body):
        raw = json.dumps(body).encode("utf-8")
        self.send_response(status)
        self.send_header("content-type", "application/json")
        self.send_header("content-length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)


class RegressionEnv:
    def __init__(self, args):
        self.args = args
        self.base_url = args.base_url.rstrip("/")
        self.namespace = args.namespace
        self.model = args.model
        self.aggregate_group = f"{self.namespace}_aggregate"
        self.low_group = args.low_group
        self.high_group = args.high_group
        self.username = f"rmr_{datetime.now().strftime('%H%M%S')}_{secrets.token_hex(2)}"
        self.password = f"Rmr!{secrets.token_hex(6)}"
        self.user_access_token = secrets.token_hex(16)
        self.token_key = secrets.token_hex(24)
        self.root_access_token = secrets.token_hex(16)
        self.root_user_id = None
        self.root_original_access_token = None
        self.user_id = None
        self.token_id = None
        self.low_channel_id = None
        self.high_channel_id = None
        self.aggregate_group_id = None
        self.aggregate_payload = None
        self.fake_server = None
        self.started_at = int(time.time())
        self.started_iso = datetime.now(timezone.utc).isoformat()
        self.tests = []
        self.consume_logs = []
        self.result = "running"
        self.error = ""
        self.log_dir = Path(args.log_dir)
        self.log_dir.mkdir(parents=True, exist_ok=True)
        self.runner_log = self.log_dir / f"{self.namespace}-runner.log"
        self.fake_log = self.log_dir / f"{self.namespace}-fake-upstream.jsonl"
        self.report_path = self.log_dir / f"{self.namespace}-report.json"

    def log(self, message):
        line = f"[{datetime.now().astimezone().isoformat()}] {message}"
        print(line, flush=True)
        with self.runner_log.open("a", encoding="utf-8") as handle:
            handle.write(line + "\n")

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

    def psql_json(self, sql, default):
        raw = self.psql(sql)
        return json.loads(raw) if raw else default

    def psql_scalar(self, sql):
        raw = self.psql(sql)
        for line in raw.splitlines():
            value = line.strip()
            if value:
                return value
        return ""

    def redis(self, *args):
        return run_cmd(
            ["docker", "exec", self.args.redis_container, "redis-cli", *args],
            check=False,
        )

    def ensure_ready(self):
        deadline = time.time() + self.args.ready_timeout
        last_health = ""
        while time.time() < deadline:
            last_health = run_cmd(
                [
                    "docker",
                    "inspect",
                    "--format",
                    "{{.State.Health.Status}}",
                    self.args.app_container,
                ],
                check=False,
            )
            if last_health == "healthy":
                status, payload = self.request_json("GET", "/api/status", auth=False)
                if status == 200 and payload.get("success") is True:
                    return
            time.sleep(1)
        raise RuntimeError(
            f"{self.args.app_container} did not become healthy: {last_health}"
        )

    def start_fake_upstream(self):
        FakeOpenAIUpstream.namespace = self.namespace
        FakeOpenAIUpstream.log_path = self.fake_log
        FakeOpenAIUpstream.events = []
        self.fake_server = ThreadingHTTPServer(
            (self.args.listen_host, self.args.fake_port), FakeOpenAIUpstream
        )
        thread = threading.Thread(target=self.fake_server.serve_forever, daemon=True)
        thread.start()
        self.log(
            f"fake upstream listening on {self.args.listen_host}:{self.args.fake_port}"
        )

    def stop_fake_upstream(self):
        if self.fake_server is not None:
            self.fake_server.shutdown()
            self.fake_server.server_close()
            self.fake_server = None

    def admin_headers(self):
        require(self.root_user_id is not None, "root user is not initialized")
        return {
            "Authorization": f"Bearer {self.root_access_token}",
            "New-Api-User": str(self.root_user_id),
        }

    def request_json(self, method, path, body=None, auth=True, opener=None, headers=None):
        request_headers = {"Content-Type": "application/json"}
        if auth:
            request_headers.update(self.admin_headers())
        if headers:
            request_headers.update(headers)
        data = None if body is None else json.dumps(body).encode("utf-8")
        req = urllib.request.Request(
            f"{self.base_url}{path}",
            data=data,
            method=method,
            headers=request_headers,
        )
        request_opener = opener or NO_PROXY_OPENER
        try:
            with request_opener.open(req, timeout=self.args.request_timeout) as response:
                payload, text = decode_json(response.read())
                return response.status, payload if isinstance(payload, dict) else {"raw": text}
        except urllib.error.HTTPError as exc:
            payload, text = decode_json(exc.read())
            return exc.code, payload if isinstance(payload, dict) else {"raw": text}

    def require_api_success(self, method, path, body=None, opener=None, headers=None):
        status, payload = self.request_json(
            method, path, body=body, auth=opener is None, opener=opener, headers=headers
        )
        require(status == 200, f"{method} {path} returned HTTP {status}: {payload}")
        require(payload.get("success") is True, f"{method} {path} failed: {payload}")
        return payload.get("data")

    def prepare_admin_access(self):
        root = self.psql_json(
            """
            select json_build_object(
              'id', id,
              'access_token', case when access_token is null then null else btrim(access_token) end
            )
            from users
            where role >= 100 and deleted_at is null
            order by id asc
            limit 1
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
        self.passed("temporary admin API access", f"root_user_id={self.root_user_id}")

    def create_test_admin(self):
        self.require_api_success(
            "POST",
            "/api/user/",
            {
                "username": self.username,
                "password": self.password,
                "display_name": self.username,
                "role": 10,
            },
        )
        row = self.psql_json(
            f"""
            select json_build_object('id', id)
            from users
            where username = {sql_str(self.username)} and deleted_at is null
            limit 1
            """,
            {},
        )
        require(row, "test admin was not inserted")
        self.user_id = int(row["id"])
        now = int(time.time())
        self.psql(
            f"""
            update users
            set role = 10,
                status = 1,
                quota = 1000000000,
                used_quota = 0,
                request_count = 0,
                "group" = 'UserGroup-admin',
                access_token = {sql_str(self.user_access_token)},
                setting = '{{}}'
            where id = {self.user_id}
            """
        )
        self.psql(
            f"""
            insert into admin_menu_permissions (user_id, menu_key, created_time, updated_time)
            values
              ({self.user_id}, 'aggregate_group', {now}, {now}),
              ({self.user_id}, 'channel', {now}, {now}),
              ({self.user_id}, 'log_dashboard', {now}, {now})
            on conflict (user_id, menu_key) do nothing
            """
        )
        self.redis("del", f"user:{self.user_id}")
        self.passed("dedicated test admin created", f"user={self.username} id={self.user_id}")

    def login_test_admin(self):
        cookie_jar = http.cookiejar.CookieJar()
        opener = urllib.request.build_opener(
            urllib.request.ProxyHandler({}), urllib.request.HTTPCookieProcessor(cookie_jar)
        )
        status, payload = self.request_json(
            "POST",
            "/api/user/login",
            body={"username": self.username, "password": self.password},
            auth=False,
            opener=opener,
        )
        require(status == 200 and payload.get("success") is True, f"login failed: {payload}")
        require(list(cookie_jar), "login did not return a session cookie")
        self.user_opener = opener
        self.passed("test admin session login")

    def create_token(self):
        now = int(time.time())
        token_name = f"{self.namespace}_token"
        self.token_id = int(
            self.psql_scalar(
                f"""
                insert into tokens (
                  user_id, key, status, name, created_time, accessed_time,
                  expired_time, remain_quota, unlimited_quota, model_limits_enabled,
                  model_limits, used_quota, "group", cross_group_retry
                ) values (
                  {self.user_id}, {sql_str(self.token_key)}, 1, {sql_str(token_name)},
                  {now}, 0, -1, 1000000000, true, false, '', 0,
                  {sql_str(self.aggregate_group)}, false
                ) returning id
                """
            )
        )
        self.passed("dedicated API token created", f"token_id={self.token_id}")

    def channel_payload(self, route, real_group):
        return {
            "mode": "single",
            "channel": {
                "type": 1,
                "key": f"fake-{route}-{self.namespace}",
                "status": 1,
                "name": f"{self.namespace}_{route}_channel",
                "weight": 100,
                "base_url": f"http://{self.args.fake_upstream_host}:{self.args.fake_port}/{route}",
                "models": self.model,
                "group": real_group,
                "auto_ban": 0,
                "priority": 999999,
                "tag": self.namespace,
                "channel_info": {"is_multi_key": False},
                "settings": "",
            },
        }

    def create_channels(self):
        self.require_api_success(
            "POST", "/api/channel/", self.channel_payload("low", self.low_group)
        )
        self.require_api_success(
            "POST", "/api/channel/", self.channel_payload("high", self.high_group)
        )
        rows = self.psql_json(
            f"""
            select coalesce(json_agg(row_to_json(t) order by name), '[]'::json)
            from (
              select id, name
              from channels
              where name in (
                {sql_str(self.namespace + '_low_channel')},
                {sql_str(self.namespace + '_high_channel')}
              )
            ) t
            """,
            [],
        )
        by_name = {row["name"]: int(row["id"]) for row in rows}
        self.low_channel_id = by_name.get(f"{self.namespace}_low_channel")
        self.high_channel_id = by_name.get(f"{self.namespace}_high_channel")
        require(self.low_channel_id and self.high_channel_id, f"channel IDs missing: {rows}")
        self.require_api_success(
            "POST", "/api/channel/tag/enabled", {"tag": self.namespace}
        )
        self.passed(
            "fake upstream channels created through admin API",
            f"low={self.low_channel_id} high={self.high_channel_id}",
        )

    def build_aggregate_payload(self, status=1):
        return {
            "name": self.aggregate_group,
            "display_name": f"Route model ratio {self.namespace}",
            "description": f"Retained Docker regression record {self.namespace}",
            "status": status,
            "group_ratio": 0.5,
            "routing_mode": "failover",
            "smart_routing_enabled": False,
            "recovery_enabled": True,
            "recovery_interval_seconds": 300,
            "cluster_affinity_ttl_seconds": 300,
            "route_affinity_strategy": "off",
            "route_affinity_scope": "shared",
            "route_affinity_key_sources": [],
            "retry_status_codes": "500-599",
            "visible_user_groups": ["UserGroup-admin"],
            "targets": [
                {"real_group": self.low_group, "weight": 100, "rpm_limit": 0},
                {"real_group": self.high_group, "weight": 100, "rpm_limit": 0},
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
            "route_model_group_ratio_overrides": [
                {
                    "real_group": self.low_group,
                    "model_name": self.model,
                    "group_ratio": 9,
                    "enabled": False,
                },
                {
                    "real_group": self.high_group,
                    "model_name": self.model,
                    "group_ratio": 3.25,
                    "enabled": True,
                },
            ],
        }

    def create_aggregate_group(self):
        self.aggregate_payload = self.build_aggregate_payload()
        data = self.require_api_success(
            "POST", "/api/aggregate_group/", self.aggregate_payload
        )
        self.aggregate_group_id = int(data["id"])
        self.aggregate_payload["id"] = self.aggregate_group_id
        require(
            data.get("enabled_route_model_group_ratio_override_count") == 1,
            f"unexpected enabled rule count: {data}",
        )
        self.passed(
            "aggregate and exact route-model rules created through admin API",
            f"aggregate_id={self.aggregate_group_id}",
        )

    def verify_admin_apis(self):
        data = self.require_api_success(
            "GET", f"/api/aggregate_group/{self.aggregate_group_id}"
        )
        rules = data.get("route_model_group_ratio_overrides") or []
        require(len(rules) == 2, f"expected 2 rules: {rules}")
        enabled = [rule for rule in rules if rule.get("enabled")]
        require(
            len(enabled) == 1
            and enabled[0].get("real_group") == self.high_group
            and float(enabled[0].get("group_ratio")) == 3.25,
            f"enabled rule round trip failed: {rules}",
        )
        for group in (self.low_group, self.high_group):
            models = self.require_api_success(
                "GET", f"/api/aggregate_group/models?group={urllib.parse.quote(group)}"
            )
            require(self.model in models, f"{self.model} missing from {group}: {models}")
        runtime = self.require_api_success(
            "GET",
            f"/api/aggregate_group/{self.aggregate_group_id}/runtime?model={urllib.parse.quote(self.model)}",
        )
        require(runtime.get("selected_model") == self.model, f"runtime model mismatch: {runtime}")
        self.passed("aggregate detail, exact-model list, and runtime APIs")

    def verify_pricing(self):
        deadline = time.time() + self.args.pricing_wait_timeout
        detail = {}
        while time.time() < deadline:
            status, payload = self.request_json(
                "GET", "/api/pricing", auth=False, opener=self.user_opener
            )
            require(
                status == 200 and payload.get("success") is True,
                f"pricing failed: {payload}",
            )
            details = payload.get("model_group_ratio_details") or {}
            detail = (details.get(self.model) or {}).get(self.aggregate_group) or {}
            if detail.get("dynamic_route") is True:
                break
            time.sleep(1)
        require(detail.get("dynamic_route") is True, f"dynamic route missing: {detail}")
        require(float(detail.get("max_ratio")) == 3.25, f"max ratio mismatch: {detail}")
        self.passed("pricing exposes highest reachable dynamic ratio", "max_ratio=3.25")

    def call_gateway(self, label):
        status, payload = self.request_json(
            "POST",
            "/v1/chat/completions",
            body={
                "model": self.model,
                "messages": [{"role": "user", "content": f"{label} {self.namespace}"}],
                "max_tokens": 32,
                "stream": False,
            },
            auth=False,
            headers={"Authorization": f"Bearer sk-{self.token_key}"},
        )
        require(status == 200, f"gateway request {label} failed HTTP {status}: {payload}")
        choices = payload.get("choices") or []
        require(choices, f"gateway request {label} returned no choices: {payload}")
        return payload

    def load_consume_logs(self, expected_count):
        deadline = time.time() + self.args.log_wait_timeout
        logs = []
        while time.time() < deadline:
            logs = self.psql_json(
                f"""
                select coalesce(json_agg(row_to_json(t) order by id), '[]'::json)
                from (
                  select id, created_at, type, quota, prompt_tokens, completion_tokens,
                         channel_id, token_id, "group", request_id,
                         case when coalesce(other, '') = '' then '{{}}'::json else other::json end as other
                  from logs
                  where user_id = {self.user_id}
                    and token_id = {self.token_id}
                    and model_name = {sql_str(self.model)}
                    and type = 2
                    and created_at >= {self.started_at}
                  order by id
                ) t
                """,
                [],
            )
            if len(logs) >= expected_count:
                return logs
            time.sleep(0.25)
        raise AssertionError(f"expected {expected_count} consume logs, got {logs}")

    def verify_gateway_billing(self):
        low_response = self.call_gateway("normal-low")
        low_text = low_response["choices"][0]["message"]["content"]
        require("route=low" in low_text, f"first request did not use low route: {low_text}")
        logs = self.load_consume_logs(1)
        low_log = logs[0]
        low_other = low_log["other"]
        require(float(low_other.get("group_ratio")) == 0.5, f"low ratio mismatch: {low_log}")
        require(
            not low_other.get("route_model_group_ratio_applied"),
            f"disabled low rule unexpectedly applied: {low_log}",
        )
        self.passed("disabled route rule falls back to aggregate ratio", "group_ratio=0.5")

        high_response = self.call_gateway("force-high")
        high_text = high_response["choices"][0]["message"]["content"]
        require("route=high" in high_text, f"retry did not use high route: {high_text}")
        logs = self.load_consume_logs(2)
        require(len(logs) == 2, f"expected exactly 2 consume logs: {logs}")
        high_log = logs[-1]
        high_other = high_log["other"]
        require(float(high_other.get("group_ratio")) == 3.25, f"high ratio mismatch: {high_log}")
        require(
            high_other.get("route_model_group_ratio_applied") is True,
            f"route rule metadata missing: {high_log}",
        )
        require(
            high_other.get("route_model_ratio_real_group") == self.high_group,
            f"real group metadata mismatch: {high_log}",
        )
        require(
            high_other.get("route_model_ratio_model_name") == self.model,
            f"model metadata mismatch: {high_log}",
        )
        require(
            int(high_log["quota"]) > int(low_log["quota"]),
            f"high-route quota should exceed low-route quota: {logs}",
        )
        self.consume_logs = logs
        self.passed(
            "retry refreshes to enabled high-route final ratio exactly once",
            f"group_ratio=3.25 low_quota={low_log['quota']} high_quota={high_log['quota']}",
        )

        with FakeOpenAIUpstream.lock:
            route_statuses = [
                (event["route"], event["status"])
                for event in FakeOpenAIUpstream.events
            ]
        require(
            route_statuses == [("low", 200), ("low", 500), ("high", 200)],
            f"unexpected fake-upstream call sequence: {route_statuses}",
        )
        self.passed("fake-upstream route sequence", str(route_statuses))

    def verify_usage_log_visibility(self):
        high_log = self.consume_logs[-1]
        request_id = urllib.parse.quote(str(high_log["request_id"]))
        admin_data = self.require_api_success(
            "GET", f"/api/log/?type=2&request_id={request_id}&p=1&page_size=10"
        )
        admin_items = admin_data.get("items") or []
        require(len(admin_items) == 1, f"admin usage log missing: {admin_data}")
        admin_other = parse_log_other(admin_items[0])
        require(
            admin_other.get("route_model_group_ratio_applied") is True,
            f"admin route ratio metadata missing: {admin_items[0]}",
        )
        require(
            admin_other.get("route_model_ratio_real_group") == self.high_group,
            f"admin real group metadata mismatch: {admin_items[0]}",
        )

        self_data = self.require_api_success(
            "GET",
            f"/api/log/self?type=2&request_id={request_id}&p=1&page_size=10",
            opener=self.user_opener,
            headers={"New-Api-User": str(self.user_id)},
        )
        self_items = self_data.get("items") or []
        require(len(self_items) == 1, f"self usage log missing: {self_data}")
        self_other = parse_log_other(self_items[0])
        require(
            float(self_other.get("group_ratio")) == 3.25,
            f"self usage log lost final ratio: {self_items[0]}",
        )
        forbidden_keys = {
            "route_model_group_ratio_applied",
            "route_model_group_ratio",
            "route_model_ratio_aggregate_group",
            "route_model_ratio_real_group",
            "route_model_ratio_model_name",
        }
        leaked_keys = sorted(forbidden_keys.intersection(self_other))
        require(not leaked_keys, f"self usage log leaked admin metadata: {leaked_keys}")
        self.passed(
            "usage log route-ratio metadata is admin-only",
            "admin metadata retained; self API keeps only final group_ratio=3.25",
        )

    def run(self):
        self.ensure_ready()
        self.start_fake_upstream()
        self.prepare_admin_access()
        self.create_test_admin()
        self.login_test_admin()
        self.create_token()
        self.create_channels()
        self.create_aggregate_group()
        self.verify_admin_apis()
        self.verify_pricing()
        self.verify_gateway_billing()
        self.verify_usage_log_visibility()
        self.result = "passed"
        self.write_report(active=True)
        self.log(f"READY_FOR_BROWSER_CHECK namespace={self.namespace}")
        print(
            f"BROWSER_LOGIN username={self.username} password={self.password}",
            flush=True,
        )
        if self.args.pause_for_browser:
            input("Press Enter after browser verification to disable retained test entities: ")

    def disable_entities(self):
        self.log("disabling retained test entities")
        if self.aggregate_group_id and self.aggregate_payload:
            payload = dict(self.aggregate_payload)
            payload["status"] = 2
            self.require_api_success("PUT", "/api/aggregate_group/", payload)
        if self.low_channel_id or self.high_channel_id:
            self.require_api_success(
                "POST", "/api/channel/tag/disabled", {"tag": self.namespace}
            )
        if self.token_id:
            self.psql(f"update tokens set status = 2 where id = {self.token_id}")
        if self.user_id:
            try:
                self.require_api_success(
                    "POST", "/api/user/manage", {"id": self.user_id, "action": "disable"}
                )
            except Exception as exc:
                self.log(f"user disable API fallback: {exc}")
                self.psql(f"update users set status = 2 where id = {self.user_id}")
                self.redis("del", f"user:{self.user_id}")
            self.psql(
                f"update users set access_token = null where id = {self.user_id}"
            )

    def restore_admin_access(self):
        if self.root_user_id is None:
            return
        self.psql(
            f"update users set access_token = {sql_str(self.root_original_access_token)} "
            f"where id = {self.root_user_id}"
        )

    def entity_statuses(self):
        statuses = {}
        if self.user_id is not None:
            statuses["user_status"] = self.psql_scalar(
                f"select status from users where id = {self.user_id}"
            )
        if self.token_id is not None:
            statuses["token_status"] = self.psql_scalar(
                f"select status from tokens where id = {self.token_id}"
            )
        if self.aggregate_group_id is not None:
            statuses["aggregate_status"] = self.psql_scalar(
                f"select status from aggregate_groups where id = {self.aggregate_group_id}"
            )
        channel_ids = [
            channel_id
            for channel_id in (self.low_channel_id, self.high_channel_id)
            if channel_id is not None
        ]
        if channel_ids:
            statuses["channel_statuses"] = self.psql_json(
                "select coalesce(json_agg(json_build_object('id', id, 'status', status) order by id), '[]'::json) "
                f"from channels where id in ({','.join(str(value) for value in channel_ids)})",
                [],
            )
        return statuses

    def write_report(self, active=False):
        application_logs = sorted(self.log_dir.glob("oneapi-*.log"))
        report = {
            "namespace": self.namespace,
            "result": self.result,
            "error": self.error,
            "started_at": self.started_at,
            "started_iso": self.started_iso,
            "finished_at": None if active else int(time.time()),
            "finished_iso": None if active else datetime.now(timezone.utc).isoformat(),
            "active_for_browser_check": active,
            "entities": {
                "user_id": self.user_id,
                "username": self.username,
                "token_id": self.token_id,
                "token_name": f"{self.namespace}_token",
                "aggregate_group_id": self.aggregate_group_id,
                "aggregate_group": self.aggregate_group,
                "low_group": self.low_group,
                "high_group": self.high_group,
                "low_channel_id": self.low_channel_id,
                "high_channel_id": self.high_channel_id,
                "channel_tag": self.namespace,
                "model": self.model,
            },
            "tests": self.tests,
            "consume_logs": self.consume_logs,
            "entity_statuses": {} if active else self.entity_statuses(),
            "log_files": {
                "runner": str(self.runner_log),
                "fake_upstream": str(self.fake_log),
                "report": str(self.report_path),
                "application": str(application_logs[-1]) if application_logs else "",
            },
            "filters": {
                "namespace": self.namespace,
                "username": self.username,
                "aggregate_group": self.aggregate_group,
                "token_id": self.token_id,
                "model": self.model,
            },
        }
        with self.report_path.open("w", encoding="utf-8") as handle:
            json.dump(report, handle, ensure_ascii=False, indent=2)
            handle.write("\n")

    def cleanup(self):
        try:
            self.disable_entities()
        finally:
            try:
                self.restore_admin_access()
            finally:
                self.stop_fake_upstream()
        self.write_report(active=False)
        self.log(
            f"retained disabled records and logs; report={self.report_path} "
            f"statuses={self.entity_statuses()}"
        )


def build_parser():
    namespace = "agratio" + datetime.now().strftime("%Y%m%d%H%M%S") + secrets.token_hex(2)
    parser = argparse.ArgumentParser(
        description="Docker-dev regression for aggregate child-route exact-model ratios."
    )
    parser.add_argument("--namespace", default=namespace)
    parser.add_argument("--base-url", default="http://localhost:3001")
    parser.add_argument("--model", default="claude-sonnet-4-6")
    parser.add_argument("--low-group", default="test_grok")
    parser.add_argument("--high-group", default="test-group2")
    parser.add_argument("--listen-host", default="127.0.0.1")
    parser.add_argument("--fake-upstream-host", default="host.docker.internal")
    parser.add_argument("--fake-port", type=int, default=19087)
    parser.add_argument("--app-container", default="new-api-dev")
    parser.add_argument("--postgres-container", default="postgres-dev")
    parser.add_argument("--postgres-user", default="root")
    parser.add_argument("--postgres-db", default="new-api")
    parser.add_argument("--redis-container", default="redis-dev")
    parser.add_argument("--log-dir", default="logs-dev")
    parser.add_argument("--request-timeout", type=float, default=20)
    parser.add_argument("--ready-timeout", type=float, default=120)
    parser.add_argument("--pricing-wait-timeout", type=float, default=75)
    parser.add_argument("--log-wait-timeout", type=float, default=10)
    parser.add_argument("--pause-for-browser", action="store_true")
    return parser


def main():
    args = build_parser().parse_args()
    env = RegressionEnv(args)
    exit_code = 0
    try:
        env.run()
    except Exception as exc:
        env.result = "failed"
        env.error = str(exc)
        env.log(f"FAIL {exc}")
        exit_code = 1
    finally:
        try:
            env.cleanup()
        except Exception as cleanup_exc:
            env.error = f"{env.error}; cleanup failed: {cleanup_exc}".strip("; ")
            env.log(f"CLEANUP_FAIL {cleanup_exc}")
            try:
                env.restore_admin_access()
            finally:
                env.stop_fake_upstream()
            env.write_report(active=False)
            exit_code = 1
    return exit_code


if __name__ == "__main__":
    sys.exit(main())
