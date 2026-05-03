#!/usr/bin/env python3
import argparse
import json
import math
import subprocess
import sys
import threading
import time
import urllib.error
import urllib.parse
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


DEFAULT_TOKEN = "sk-codexclusterdev20260428token00000000000000000000"
NO_PROXY_OPENER = urllib.request.build_opener(urllib.request.ProxyHandler({}))

OPTION_KEYS = [
    "RetryTimes",
    "aggregate_group.smart_strategy_enabled",
    "aggregate_group.consecutive_failure_threshold",
    "aggregate_group.degrade_duration_seconds",
    "aggregate_group.cluster_degraded_weight_percent",
]


def shell_quote_sql(value):
    return "'" + str(value).replace("'", "''") + "'"


def run_cmd(args, check=True, capture=True):
    result = subprocess.run(
        args,
        text=True,
        stdout=subprocess.PIPE if capture else None,
        stderr=subprocess.PIPE if capture else None,
    )
    if check and result.returncode != 0:
        detail = (result.stderr or result.stdout or "").strip()
        raise RuntimeError(f"command failed ({result.returncode}): {' '.join(args)}\n{detail}")
    return result.stdout if capture else ""


def decode_body(raw):
    text = raw.decode("utf-8", errors="replace")
    if not text:
        return None, ""
    try:
        return json.loads(text), text
    except json.JSONDecodeError:
        return None, text


def calculate_effective_weight(weight, level, percent):
    if weight <= 0:
        return 0
    if level <= 0:
        return weight
    effective = float(weight)
    for _ in range(level):
        effective = math.ceil(effective * percent / 100.0)
        if effective <= 1:
            return 1
    return int(effective)


class FakeClaudeUpstream(BaseHTTPRequestHandler):
    counters = {"good": 0, "bad": 0, "other": 0}
    lock = threading.Lock()

    def log_message(self, fmt, *args):
        return

    def do_POST(self):
        parsed = urllib.parse.urlparse(self.path)
        length = int(self.headers.get("content-length") or "0")
        body = self.rfile.read(length) if length > 0 else b"{}"
        try:
            payload = json.loads(body.decode("utf-8"))
        except Exception:
            payload = {}
        route = "other"
        if parsed.path.startswith("/bad/"):
            route = "bad"
        elif parsed.path.startswith("/good/"):
            route = "good"
        with self.lock:
            self.counters[route] = self.counters.get(route, 0) + 1

        if route == "bad":
            self._write_json(
                500,
                {
                    "type": "error",
                    "error": {
                        "type": "api_error",
                        "message": "live aggregate degrade demo forced 500",
                    },
                },
            )
            return

        model = str(payload.get("model") or "claude-sonnet-4-6")
        self._write_json(
            200,
            {
                "id": f"msg_live_degrade_{int(time.time() * 1000)}",
                "type": "message",
                "role": "assistant",
                "model": model,
                "content": [{"type": "text", "text": "OK"}],
                "stop_reason": "end_turn",
                "stop_sequence": None,
                "usage": {"input_tokens": 1, "output_tokens": 1},
            },
        )

    def _write_json(self, status, body):
        raw = json.dumps(body).encode("utf-8")
        self.send_response(status)
        self.send_header("content-type", "application/json")
        self.send_header("content-length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)


class LiveDemoEnv:
    def __init__(self, args):
        self.args = args
        self.base_url = args.base_url.rstrip("/")
        self.snapshot = None
        self.fake_server = None
        self.fake_thread = None
        self.smart_keys = {
            args.primary_route: self.smart_key(args.primary_route),
            args.secondary_route: self.smart_key(args.secondary_route),
        }
        self.runtime_key = f"aggregate_group:state:{args.group}:{args.model}"

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
        ).strip()

    def psql_exec(self, sql):
        self.psql(sql)

    def psql_json(self, sql, default):
        raw = self.psql(sql)
        if not raw:
            return default
        return json.loads(raw)

    def redis(self, *items, check=True):
        return run_cmd(
            ["docker", "exec", self.args.redis_container, "redis-cli", *map(str, items)],
            check=check,
        ).strip()

    def redis_hgetall(self, key):
        raw = self.redis("hgetall", key, check=False)
        lines = raw.splitlines()
        if len(lines) % 2 != 0:
            return {}
        return {lines[i]: lines[i + 1] for i in range(0, len(lines), 2)}

    def redis_ttl(self, key):
        raw = self.redis("ttl", key, check=False)
        try:
            return int(raw)
        except ValueError:
            return -2

    def redis_restore_hash(self, key, data, ttl):
        self.redis("del", key, check=False)
        if not data:
            return
        args = ["hset", key]
        for field, value in data.items():
            args.extend([field, value])
        self.redis(*args)
        if ttl and ttl > 0:
            self.redis("expire", key, ttl)

    def smart_key(self, route_group):
        return f"aggregate_group:smart_state:{self.args.group}:{self.args.model}:{route_group}"

    def ensure_ready(self):
        health = run_cmd(
            [
                "docker",
                "inspect",
                "--format",
                "{{.State.Health.Status}}",
                self.args.app_container,
            ],
            check=False,
        ).strip()
        if health != "healthy":
            raise RuntimeError(f"{self.args.app_container} is not healthy: {health}")

    def restart_app(self):
        if self.args.no_restart:
            return
        print(f"Restarting {self.args.app_container} to reload DB options...", flush=True)
        run_cmd(["docker", "restart", self.args.app_container], capture=True)
        deadline = time.time() + self.args.restart_timeout
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
            ).strip()
            if health == "healthy":
                return
            time.sleep(1)
        raise RuntimeError(f"{self.args.app_container} did not become healthy")

    def start_fake_upstream(self):
        server = ThreadingHTTPServer((self.args.listen_host, self.args.fake_port), FakeClaudeUpstream)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        self.fake_server = server
        self.fake_thread = thread
        print(
            f"Fake upstream listening on {self.args.listen_host}:{self.args.fake_port}; "
            f"container URL host is {self.args.fake_upstream_host}",
            flush=True,
        )

    def stop_fake_upstream(self):
        if self.fake_server:
            self.fake_server.shutdown()
            self.fake_server.server_close()
            self.fake_server = None

    def snapshot_state(self):
        group_row = self.psql_json(
            f"""
            select row_to_json(t)
            from (
              select routing_mode, smart_routing_enabled, recovery_enabled,
                     recovery_interval_seconds, cluster_affinity_ttl_seconds,
                     route_affinity_strategy, route_affinity_key_sources,
                     retry_status_codes
              from aggregate_groups
              where name = {shell_quote_sql(self.args.group)}
            ) t
            """,
            {},
        )
        if not group_row:
            raise RuntimeError(f"aggregate group not found: {self.args.group}")

        target_rows = self.psql_json(
            f"""
            select coalesce(json_agg(row_to_json(t)), '[]'::json)
            from (
              select real_group, weight
              from aggregate_group_targets
              where aggregate_group_id = (
                select id from aggregate_groups where name = {shell_quote_sql(self.args.group)}
              )
                and real_group in ({shell_quote_sql(self.args.primary_route)}, {shell_quote_sql(self.args.secondary_route)})
              order by real_group
            ) t
            """,
            [],
        )
        target_names = {row.get("real_group") for row in target_rows}
        missing_targets = [
            name
            for name in [self.args.primary_route, self.args.secondary_route]
            if name not in target_names
        ]
        if missing_targets:
            raise RuntimeError(f"aggregate targets not found: {', '.join(missing_targets)}")

        channel_rows = self.psql_json(
            f"""
            select coalesce(json_agg(row_to_json(t)), '[]'::json)
            from (
              select id, name, "group", status, base_url, auto_ban
              from channels
              where id in ({int(self.args.primary_channel_id)}, {int(self.args.secondary_channel_id)})
              order by id
            ) t
            """,
            [],
        )
        if len(channel_rows) != 2:
            raise RuntimeError("expected both primary and secondary channels to exist")

        ability_rows = self.psql_json(
            f"""
            select coalesce(json_agg(row_to_json(t)), '[]'::json)
            from (
              select channel_id, model, enabled
              from abilities
              where channel_id in ({int(self.args.primary_channel_id)}, {int(self.args.secondary_channel_id)})
                and model = {shell_quote_sql(self.args.model)}
              order by channel_id, model
            ) t
            """,
            [],
        )
        ability_channel_ids = {int(row.get("channel_id")) for row in ability_rows}
        missing_abilities = [
            str(channel_id)
            for channel_id in [self.args.primary_channel_id, self.args.secondary_channel_id]
            if channel_id not in ability_channel_ids
        ]
        if missing_abilities:
            raise RuntimeError(
                f"model ability not found for channel(s) {', '.join(missing_abilities)} and model {self.args.model}"
            )

        options = {}
        for key in OPTION_KEYS:
            value = self.psql(f"select value from options where key = {shell_quote_sql(key)} limit 1")
            options[key] = value if value != "" else None

        redis_hashes = {}
        redis_ttls = {}
        for key in [*self.smart_keys.values(), self.runtime_key]:
            redis_hashes[key] = self.redis_hgetall(key)
            redis_ttls[key] = self.redis_ttl(key)

        self.snapshot = {
            "group": group_row,
            "targets": target_rows,
            "channels": channel_rows,
            "abilities": ability_rows,
            "options": options,
            "redis_hashes": redis_hashes,
            "redis_ttls": redis_ttls,
        }

    def setup_demo_state(self):
        good_base = f"http://{self.args.fake_upstream_host}:{self.args.fake_port}/good"
        bad_base = f"http://{self.args.fake_upstream_host}:{self.args.fake_port}/bad"
        self.psql_exec(
            f"""
            update aggregate_groups
            set routing_mode = 'cluster',
                smart_routing_enabled = true,
                retry_status_codes = '400-599',
                route_affinity_strategy = 'request_only',
                route_affinity_key_sources = ''
            where name = {shell_quote_sql(self.args.group)}
            """
        )
        self.psql_exec(
            f"""
            update aggregate_group_targets
            set weight = case
              when real_group = {shell_quote_sql(self.args.primary_route)} then {int(self.args.primary_weight)}
              when real_group = {shell_quote_sql(self.args.secondary_route)} then {int(self.args.secondary_weight)}
              else weight
            end
            where aggregate_group_id = (
              select id from aggregate_groups where name = {shell_quote_sql(self.args.group)}
            )
            """
        )
        self.psql_exec(
            f"""
            update channels
            set status = 1,
                auto_ban = 0,
                base_url = case
                  when id = {int(self.args.primary_channel_id)} then {shell_quote_sql(good_base)}
                  when id = {int(self.args.secondary_channel_id)} then {shell_quote_sql(bad_base)}
                  else base_url
                end
            where id in ({int(self.args.primary_channel_id)}, {int(self.args.secondary_channel_id)})
            """
        )
        self.psql_exec(
            f"""
            update abilities
            set enabled = true
            where channel_id in ({int(self.args.primary_channel_id)}, {int(self.args.secondary_channel_id)})
              and model = {shell_quote_sql(self.args.model)}
            """
        )
        self.set_option("RetryTimes", str(int(self.args.retry_times)))
        self.set_option("aggregate_group.smart_strategy_enabled", "true")
        self.set_option("aggregate_group.consecutive_failure_threshold", str(int(self.args.failure_threshold)))
        self.set_option("aggregate_group.degrade_duration_seconds", str(int(self.args.degrade_duration_seconds)))
        self.set_option(
            "aggregate_group.cluster_degraded_weight_percent",
            str(int(self.args.cluster_degraded_weight_percent)),
        )
        self.clear_affinity()
        self.clear_smart_states()
        self.clear_runtime_state()
        self.restart_app()

    def set_option(self, key, value):
        self.psql_exec(f"delete from options where key = {shell_quote_sql(key)}")
        self.psql_exec(
            "insert into options (key, value) values "
            f"({shell_quote_sql(key)}, {shell_quote_sql(value)})"
        )

    def restore_options(self):
        for key, value in self.snapshot["options"].items():
            self.psql_exec(f"delete from options where key = {shell_quote_sql(key)}")
            if value is not None:
                self.psql_exec(
                    "insert into options (key, value) values "
                    f"({shell_quote_sql(key)}, {shell_quote_sql(value)})"
                )

    def restore_db_state(self):
        group = self.snapshot["group"]
        self.psql_exec(
            f"""
            update aggregate_groups
            set routing_mode = {shell_quote_sql(group.get("routing_mode") or "failover")},
                smart_routing_enabled = {str(bool(group.get("smart_routing_enabled"))).lower()},
                recovery_enabled = {str(bool(group.get("recovery_enabled"))).lower()},
                recovery_interval_seconds = {int(group.get("recovery_interval_seconds") or 300)},
                cluster_affinity_ttl_seconds = {int(group.get("cluster_affinity_ttl_seconds") or 300)},
                route_affinity_strategy = {shell_quote_sql(group.get("route_affinity_strategy") or "platform_user")},
                route_affinity_key_sources = {shell_quote_sql(group.get("route_affinity_key_sources") or "")},
                retry_status_codes = {shell_quote_sql(group.get("retry_status_codes") or "")}
            where name = {shell_quote_sql(self.args.group)}
            """
        )
        for row in self.snapshot["targets"]:
            weight = row.get("weight")
            weight_sql = "null" if weight is None else str(int(weight))
            self.psql_exec(
                f"""
                update aggregate_group_targets
                set weight = {weight_sql}
                where aggregate_group_id = (
                  select id from aggregate_groups where name = {shell_quote_sql(self.args.group)}
                )
                  and real_group = {shell_quote_sql(row.get("real_group"))}
                """
            )
        for row in self.snapshot["channels"]:
            base_url = row.get("base_url") or ""
            auto_ban = row.get("auto_ban")
            auto_ban_sql = "null" if auto_ban is None else str(int(auto_ban))
            self.psql_exec(
                f"""
                update channels
                set status = {int(row.get("status"))},
                    base_url = {shell_quote_sql(base_url)},
                    auto_ban = {auto_ban_sql}
                where id = {int(row.get("id"))}
                """
            )
        for row in self.snapshot["abilities"]:
            self.psql_exec(
                f"""
                update abilities
                set enabled = {str(bool(row.get("enabled"))).lower()}
                where channel_id = {int(row.get("channel_id"))}
                  and model = {shell_quote_sql(row.get("model"))}
                """
            )
        self.restore_options()

    def restore_redis_state(self):
        self.clear_affinity()
        for key, data in self.snapshot["redis_hashes"].items():
            self.redis_restore_hash(key, data, self.snapshot["redis_ttls"].get(key, -2))

    def restore_state(self, keep_degrade_state=False):
        if not self.snapshot:
            return
        self.restore_db_state()
        if keep_degrade_state:
            self.clear_affinity()
        else:
            self.restore_redis_state()
        self.restart_app()

    def clear_affinity(self):
        for pattern in [
            "new-api:aggregate_route_affinity:v2*",
            "new-api:aggregate_route_affinity:v3*",
        ]:
            self.redis(
                "eval",
                "local keys=redis.call('keys', ARGV[1]); for _,k in ipairs(keys) do redis.call('del', k) end; return #keys",
                "0",
                pattern,
                check=False,
            )

    def clear_smart_states(self):
        for key in self.smart_keys.values():
            self.redis("del", key, check=False)

    def clear_runtime_state(self):
        self.redis("del", self.runtime_key, check=False)

    def smart_state(self, route_group):
        return self.redis_hgetall(self.smart_key(route_group))

    def last_log_id(self):
        raw = self.psql(
            f"""
            select coalesce(max(id), 0)
            from logs
            where "group" = {shell_quote_sql(self.args.group)}
              and model_name = {shell_quote_sql(self.args.model)}
            """
        )
        return int(raw or "0")

    def new_logs_after(self, after_id):
        rows = self.psql_json(
            f"""
            select coalesce(json_agg(row_to_json(t)), '[]'::json)
            from (
              select id, request_id, model_name, "group", channel_id, other
              from logs
              where id > {int(after_id)}
                and "group" = {shell_quote_sql(self.args.group)}
                and model_name = {shell_quote_sql(self.args.model)}
              order by id asc
            ) t
            """,
            [],
        )
        parsed = []
        for row in rows:
            other = row.get("other") or "{}"
            try:
                other_obj = json.loads(other)
            except json.JSONDecodeError:
                other_obj = {}
            admin_info = other_obj.get("admin_info") or {}
            row["admin_info"] = admin_info if isinstance(admin_info, dict) else {}
            parsed.append(row)
        return parsed

    def wait_new_logs(self, after_id):
        deadline = time.time() + self.args.log_wait_timeout
        while time.time() < deadline:
            rows = self.new_logs_after(after_id)
            if rows:
                return rows
            time.sleep(0.5)
        return []

    def call_api(self, index):
        url = f"{self.base_url}{self.args.endpoint}"
        affinity_key = f"live-degrade-demo-{int(time.time())}-{index}"
        body = {
            "model": self.args.model,
            "messages": [{"role": "user", "content": f"{self.args.message} #{index}"}],
            "max_tokens": self.args.max_tokens,
            "stream": False,
            "metadata": {
                "user_id": affinity_key,
                "aggregate_route_affinity_key": affinity_key,
            },
        }
        data = json.dumps(body).encode("utf-8")
        req = urllib.request.Request(
            url,
            data=data,
            method="POST",
            headers={
                "Authorization": f"Bearer {self.args.token}",
                "Content-Type": "application/json",
                "anthropic-version": self.args.claude_version,
            },
        )
        started = time.monotonic()
        try:
            with NO_PROXY_OPENER.open(req, timeout=self.args.request_timeout) as resp:
                parsed, text = decode_body(resp.read())
                return {
                    "ok": 200 <= resp.status < 300,
                    "status": resp.status,
                    "elapsed_ms": int((time.monotonic() - started) * 1000),
                    "json": parsed,
                    "text": text,
                    "error": "",
                }
        except urllib.error.HTTPError as exc:
            parsed, text = decode_body(exc.read())
            return {
                "ok": False,
                "status": exc.code,
                "elapsed_ms": int((time.monotonic() - started) * 1000),
                "json": parsed,
                "text": text,
                "error": f"HTTP {exc.code}",
            }
        except Exception as exc:
            return {
                "ok": False,
                "status": 0,
                "elapsed_ms": int((time.monotonic() - started) * 1000),
                "json": None,
                "text": "",
                "error": str(exc),
            }


def latest_admin_log(logs):
    for row in reversed(logs):
        admin = row.get("admin_info") or {}
        if admin.get("aggregate_group"):
            return row
    return logs[-1] if logs else None


def print_round_state(env, index, response, logs):
    state = env.smart_state(env.args.secondary_route)
    now = int(time.time())
    level = int(state.get("DegradeLevel") or "0")
    degraded_until = int(state.get("DegradedUntil") or "0")
    ttl = max(0, degraded_until - now)
    consecutive_failures = int(state.get("ConsecutiveFailures") or "0")
    degraded_failures = int(state.get("DegradedConsecutiveFailures") or "0")
    effective = calculate_effective_weight(
        env.args.secondary_weight,
        level,
        env.args.cluster_degraded_weight_percent,
    )
    row = latest_admin_log(logs)
    admin = (row.get("admin_info") or {}) if row else {}
    use_channel = admin.get("use_channel") or []
    route_group = admin.get("route_group") or "-"
    with FakeClaudeUpstream.lock:
        counters = dict(FakeClaudeUpstream.counters)
    print(
        "[%03d] http=%s elapsed=%sms route=%s use_channel=%s "
        "fake_good=%d fake_bad=%d level=L%d weight=%d/%d ttl=%ss "
        "failures=%d degraded_failures=%d"
        % (
            index,
            response["status"],
            response["elapsed_ms"],
            route_group,
            ",".join(map(str, use_channel)) if use_channel else "-",
            counters.get("good", 0),
            counters.get("bad", 0),
            level,
            env.args.secondary_weight,
            effective,
            ttl,
            consecutive_failures,
            degraded_failures,
        ),
        flush=True,
    )
    if not response["ok"]:
        detail = response.get("text") or response.get("error") or ""
        print(f"      response_error={detail[:240].replace(chr(10), ' ')}", flush=True)


def run_demo(env):
    print(
        "\nDemo setup:\n"
        f"  group={env.args.group} mode=cluster model={env.args.model}\n"
        f"  good route: {env.args.primary_route} channel#{env.args.primary_channel_id} weight={env.args.primary_weight}\n"
        f"  bad route : {env.args.secondary_route} channel#{env.args.secondary_channel_id} weight={env.args.secondary_weight}\n"
        f"  threshold={env.args.failure_threshold}, percent={env.args.cluster_degraded_weight_percent}%, "
        f"duration={env.args.degrade_duration_seconds}s, retry_times={env.args.retry_times}\n",
        flush=True,
    )
    print(
        "Open the aggregate runtime topology now. Refresh after each printed round to watch L1/L2/L3.",
        flush=True,
    )
    reached_level = 0
    for index in range(1, env.args.rounds + 1):
        after_id = env.last_log_id()
        response = env.call_api(index)
        logs = env.wait_new_logs(after_id) if response["ok"] else env.new_logs_after(after_id)
        print_round_state(env, index, response, logs)
        state = env.smart_state(env.args.secondary_route)
        reached_level = max(reached_level, int(state.get("DegradeLevel") or "0"))
        if reached_level >= env.args.target_level:
            print(f"Target degrade level L{env.args.target_level} reached.", flush=True)
            break
        time.sleep(env.args.pause)
    return reached_level


def build_parser():
    parser = argparse.ArgumentParser(
        description=(
            "Live aggregate cluster degrade demo. It uses real gateway requests and a local fake "
            "Claude upstream, then prints the Redis smart-state after each round."
        )
    )
    parser.add_argument("--base-url", default="http://localhost:3001")
    parser.add_argument("--endpoint", default="/v1/messages?beta=true")
    parser.add_argument("--token", default=DEFAULT_TOKEN)
    parser.add_argument("--claude-version", default="2023-06-01")
    parser.add_argument("--group", default="codex_cluster_dev_20260428")
    parser.add_argument("--model", default="claude-sonnet-4-6")
    parser.add_argument("--primary-route", default="claude-re-kiro")
    parser.add_argument("--secondary-route", default="claude-re-kiro-003")
    parser.add_argument("--primary-channel-id", type=int, default=5)
    parser.add_argument("--secondary-channel-id", type=int, default=12)
    parser.add_argument("--primary-weight", type=int, default=1)
    parser.add_argument("--secondary-weight", type=int, default=200)
    parser.add_argument("--retry-times", type=int, default=0)
    parser.add_argument("--failure-threshold", type=int, default=2)
    parser.add_argument("--degrade-duration-seconds", type=int, default=600)
    parser.add_argument("--cluster-degraded-weight-percent", type=int, default=20)
    parser.add_argument("--target-level", type=int, default=3)
    parser.add_argument("--rounds", type=int, default=30)
    parser.add_argument("--pause", type=float, default=5.0)
    parser.add_argument("--hold-seconds", type=int, default=60)
    parser.add_argument("--keep-degrade-state", action="store_true")
    parser.add_argument("--message", default="Reply OK only.")
    parser.add_argument("--max-tokens", type=int, default=2)
    parser.add_argument("--request-timeout", type=int, default=60)
    parser.add_argument("--log-wait-timeout", type=int, default=10)
    parser.add_argument("--listen-host", default="0.0.0.0")
    parser.add_argument("--fake-upstream-host", default="host.docker.internal")
    parser.add_argument("--fake-port", type=int, default=19082)
    parser.add_argument("--app-container", default="new-api-dev")
    parser.add_argument("--postgres-container", default="postgres-dev")
    parser.add_argument("--postgres-user", default="root")
    parser.add_argument("--postgres-db", default="new-api")
    parser.add_argument("--redis-container", default="redis-dev")
    parser.add_argument("--restart-timeout", type=int, default=45)
    parser.add_argument("--no-restart", action="store_true")
    return parser


def main():
    args = build_parser().parse_args()
    if args.cluster_degraded_weight_percent <= 0 or args.cluster_degraded_weight_percent > 100:
        print("--cluster-degraded-weight-percent must be between 1 and 100", file=sys.stderr)
        return 2
    if args.primary_weight < 0 or args.secondary_weight < 0:
        print("weights must be non-negative", file=sys.stderr)
        return 2
    if args.retry_times < 0:
        print("--retry-times must be non-negative", file=sys.stderr)
        return 2

    env = LiveDemoEnv(args)
    env.ensure_ready()
    env.snapshot_state()
    exit_code = 0
    try:
        env.start_fake_upstream()
        env.setup_demo_state()
        reached = run_demo(env)
        if reached < args.target_level:
            print(
                f"Target L{args.target_level} was not reached after {args.rounds} rounds. "
                "Increase --rounds or lower --target-level.",
                flush=True,
            )
            exit_code = 1
        if args.hold_seconds > 0:
            print(f"Holding current topology state for {args.hold_seconds}s before restore...", flush=True)
            time.sleep(args.hold_seconds)
    finally:
        print("\nRestoring dev DB/options/channel state...", flush=True)
        try:
            env.restore_state(keep_degrade_state=args.keep_degrade_state)
            if args.keep_degrade_state:
                print("DB/options restored; generated Redis degrade/runtime state was kept.", flush=True)
            else:
                print("DB/options and Redis smart/runtime state restored.", flush=True)
        finally:
            env.stop_fake_upstream()
    return exit_code


if __name__ == "__main__":
    sys.exit(main())
