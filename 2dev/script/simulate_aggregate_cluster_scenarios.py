#!/usr/bin/env python3
import argparse
import json
import math
import subprocess
import sys
import time
import urllib.error
import urllib.request


DEFAULT_TOKEN = "sk-codexclusterdev20260428token00000000000000000000"
NO_PROXY_OPENER = urllib.request.build_opener(urllib.request.ProxyHandler({}))


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


class ScenarioEnv:
    def __init__(self, args):
        self.args = args
        self.base_url = args.base_url.rstrip("/")
        self.smart_keys = {
            args.primary_route: self.smart_key(args.primary_route),
            args.secondary_route: self.smart_key(args.secondary_route),
        }
        self.runtime_key = f"aggregate_group:state:{args.group}:{args.model}"
        self.snapshot = None

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
        if data:
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

    def snapshot_state(self):
        group_row = self.psql_json(
            f"""
            select row_to_json(t)
            from (
              select routing_mode, smart_routing_enabled, recovery_enabled, recovery_interval_seconds
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
        channel_rows = self.psql_json(
            f"""
            select coalesce(json_agg(row_to_json(t)), '[]'::json)
            from (
              select id, status
              from channels
              where id in ({self.args.primary_channel_id}, {self.args.secondary_channel_id})
              order by id
            ) t
            """,
            [],
        )
        ability_rows = self.psql_json(
            f"""
            select coalesce(json_agg(row_to_json(t)), '[]'::json)
            from (
              select channel_id, model, enabled
              from abilities
              where channel_id in ({self.args.primary_channel_id}, {self.args.secondary_channel_id})
                and model = {shell_quote_sql(self.args.model)}
              order by channel_id, model
            ) t
            """,
            [],
        )
        option_value = self.psql(
            "select value from options where key = 'aggregate_group.cluster_degraded_weight_percent' limit 1"
        )
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
            "cluster_degraded_weight_percent": option_value if option_value else None,
            "redis_hashes": redis_hashes,
            "redis_ttls": redis_ttls,
        }

    def restore_state(self):
        if not self.snapshot:
            return
        group = self.snapshot["group"]
        self.psql_exec(
            f"""
            update aggregate_groups
            set routing_mode = {shell_quote_sql(group["routing_mode"])},
                smart_routing_enabled = {str(bool(group["smart_routing_enabled"])).lower()},
                recovery_enabled = {str(bool(group["recovery_enabled"])).lower()},
                recovery_interval_seconds = {int(group["recovery_interval_seconds"])}
            where name = {shell_quote_sql(self.args.group)}
            """
        )
        for row in self.snapshot["targets"]:
            self.psql_exec(
                f"""
                update aggregate_group_targets
                set weight = {int(row["weight"])}
                where aggregate_group_id = (
                  select id from aggregate_groups where name = {shell_quote_sql(self.args.group)}
                )
                  and real_group = {shell_quote_sql(row["real_group"])}
                """
            )
        for row in self.snapshot["channels"]:
            self.psql_exec(
                f"update channels set status = {int(row['status'])} where id = {int(row['id'])}"
            )
        for row in self.snapshot["abilities"]:
            self.psql_exec(
                f"""
                update abilities
                set enabled = {str(bool(row["enabled"])).lower()}
                where channel_id = {int(row["channel_id"])}
                  and model = {shell_quote_sql(row["model"])}
                """
            )
        self.restore_cluster_degraded_weight_percent(
            self.snapshot.get("cluster_degraded_weight_percent")
        )
        self.clear_affinity()
        for key, data in self.snapshot["redis_hashes"].items():
            self.redis_restore_hash(key, data, self.snapshot["redis_ttls"].get(key, -2))
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

    def set_runtime_cluster_active_secondary(self):
        now = int(time.time())
        self.redis(
            "hset",
            self.runtime_key,
            "ActiveIndex",
            "1",
            "ActiveGroup",
            self.args.secondary_route,
            "RoutingMode",
            "cluster",
            "LastFailAt",
            "0",
            "LastSuccessAt",
            str(now),
            "LastSwitchAt",
            str(now),
            "ActiveSinceAt",
            str(now),
        )
        self.redis("expire", self.runtime_key, 86400)

    def set_smart_degraded(self, route_group, seconds=600, expired=False, weight_note="", degrade_level=1):
        now = int(time.time())
        until_ts = now - 1 if expired else now + seconds
        key = self.smart_keys[route_group]
        self.redis(
            "hset",
            key,
            "ConsecutiveFailures",
            "2",
            "ConsecutiveSlows",
            "0",
            "DegradedUntil",
            str(until_ts),
            "DegradeLevel",
            str(max(0, int(degrade_level))),
            "DegradedConsecutiveFailures",
            "0",
            "DegradedConsecutiveSlows",
            "0",
            "LastFailureAt",
            str(now - 1),
            "LastSlowAt",
            "0",
            "LastSuccessAt",
            "0",
            "LastTriggerReason",
            "consecutive_failures",
            "LastTriggerAt",
            str(now - 1),
        )
        self.redis("expire", key, 86400)
        if weight_note:
            print(f"    wrote degraded state for {route_group}: {weight_note}", flush=True)

    def apply_db_state(
        self,
        mode,
        primary_weight,
        secondary_weight,
        primary_enabled=True,
        secondary_enabled=True,
        recovery_interval=10,
    ):
        self.psql_exec(
            f"""
            update aggregate_groups
            set routing_mode = {shell_quote_sql(mode)},
                smart_routing_enabled = true,
                recovery_enabled = true,
                recovery_interval_seconds = {int(recovery_interval)}
            where name = {shell_quote_sql(self.args.group)}
            """
        )
        self.psql_exec(
            f"""
            update aggregate_group_targets
            set weight = case
              when real_group = {shell_quote_sql(self.args.primary_route)} then {int(primary_weight)}
              when real_group = {shell_quote_sql(self.args.secondary_route)} then {int(secondary_weight)}
              else weight
            end
            where aggregate_group_id = (
              select id from aggregate_groups where name = {shell_quote_sql(self.args.group)}
            )
            """
        )
        self.set_channel_enabled(self.args.primary_channel_id, primary_enabled)
        self.set_channel_enabled(self.args.secondary_channel_id, secondary_enabled)
        self.set_cluster_degraded_weight_percent(self.args.cluster_degraded_weight_percent)

    def set_channel_enabled(self, channel_id, enabled):
        status = 1 if enabled else 2
        self.psql_exec(f"update channels set status = {status} where id = {int(channel_id)}")
        self.psql_exec(
            f"""
            update abilities
            set enabled = {str(bool(enabled)).lower()}
            where channel_id = {int(channel_id)}
              and model = {shell_quote_sql(self.args.model)}
            """
        )

    def set_cluster_degraded_weight_percent(self, percent):
        self.psql_exec("delete from options where key = 'aggregate_group.cluster_degraded_weight_percent'")
        self.psql_exec(
            "insert into options (key, value) values "
            f"('aggregate_group.cluster_degraded_weight_percent', {shell_quote_sql(str(int(percent)))})"
        )

    def restore_cluster_degraded_weight_percent(self, percent):
        self.psql_exec("delete from options where key = 'aggregate_group.cluster_degraded_weight_percent'")
        if percent is not None:
            self.psql_exec(
                "insert into options (key, value) values "
                f"('aggregate_group.cluster_degraded_weight_percent', {shell_quote_sql(str(percent))})"
            )

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
              select id, created_at, request_id, model_name, "group", channel_id, other
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

    def docker_logs_since(self, since_ts):
        result = subprocess.run(
            ["docker", "logs", "--since", str(int(since_ts)), self.args.app_container],
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
        )
        return result.stdout or ""

    def call_api(self):
        url = f"{self.base_url}{self.args.endpoint}"
        body = {
            "model": self.args.model,
            "messages": [{"role": "user", "content": self.args.message}],
            "max_tokens": self.args.max_tokens,
            "stream": False,
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


def setup_cluster_healthy(env):
    env.apply_db_state("cluster", primary_weight=0, secondary_weight=200)
    env.restart_app()
    env.clear_affinity()
    env.clear_smart_states()
    env.clear_runtime_state()


def setup_cluster_degraded_reduced(env):
    env.apply_db_state("cluster", primary_weight=0, secondary_weight=200)
    env.restart_app()
    env.clear_affinity()
    env.clear_smart_states()
    env.clear_runtime_state()
    effective = max(1, math.ceil(200 * env.args.cluster_degraded_weight_percent / 100))
    env.set_smart_degraded(
        env.args.secondary_route,
        weight_note=f"weight=200 effective_weight={effective}",
    )


def setup_cluster_degraded_level2_reduced(env):
    env.apply_db_state("cluster", primary_weight=0, secondary_weight=200)
    env.restart_app()
    env.clear_affinity()
    env.clear_smart_states()
    env.clear_runtime_state()
    first = max(1, math.ceil(200 * env.args.cluster_degraded_weight_percent / 100))
    second = max(1, math.ceil(first * env.args.cluster_degraded_weight_percent / 100))
    env.set_smart_degraded(
        env.args.secondary_route,
        weight_note=f"weight=200 degrade_level=2 effective_weight={second}",
        degrade_level=2,
    )


def setup_cluster_zero_degraded(env):
    env.apply_db_state("cluster", primary_weight=100, secondary_weight=0)
    env.restart_app()
    env.clear_affinity()
    env.clear_smart_states()
    env.clear_runtime_state()
    env.set_smart_degraded(
        env.args.secondary_route,
        weight_note="weight=0 effective_weight=0",
    )


def setup_cluster_secondary_disabled(env):
    env.apply_db_state(
        "cluster",
        primary_weight=100,
        secondary_weight=200,
        primary_enabled=True,
        secondary_enabled=False,
    )
    env.restart_app()
    env.clear_affinity()
    env.clear_smart_states()
    env.clear_runtime_state()


def setup_cluster_primary_disabled(env):
    env.apply_db_state(
        "cluster",
        primary_weight=100,
        secondary_weight=200,
        primary_enabled=False,
        secondary_enabled=True,
    )
    env.restart_app()
    env.clear_affinity()
    env.clear_smart_states()
    env.clear_runtime_state()


def setup_cluster_both_disabled(env):
    env.apply_db_state(
        "cluster",
        primary_weight=100,
        secondary_weight=200,
        primary_enabled=False,
        secondary_enabled=False,
    )
    env.restart_app()
    env.clear_affinity()
    env.clear_smart_states()
    env.clear_runtime_state()


def setup_failover_degraded_skip(env):
    env.apply_db_state("failover", primary_weight=2, secondary_weight=200)
    env.restart_app()
    env.clear_affinity()
    env.clear_smart_states()
    env.clear_runtime_state()
    env.set_smart_degraded(env.args.primary_route)


def setup_failover_ignores_cluster_runtime(env):
    env.apply_db_state(
        "failover",
        primary_weight=2,
        secondary_weight=200,
        primary_enabled=True,
        secondary_enabled=False,
    )
    env.restart_app()
    env.clear_affinity()
    env.clear_smart_states()
    env.set_runtime_cluster_active_secondary()


def setup_cluster_degraded_expired(env):
    env.apply_db_state("cluster", primary_weight=0, secondary_weight=200)
    env.restart_app()
    env.clear_affinity()
    env.clear_smart_states()
    env.clear_runtime_state()
    env.set_smart_degraded(env.args.secondary_route, expired=True)


def latest_admin_log(logs):
    for row in reversed(logs):
        admin = row.get("admin_info") or {}
        if admin.get("aggregate_group"):
            return row
    return logs[-1] if logs else None


def use_channels(admin):
    values = admin.get("use_channel") or []
    return [str(item) for item in values]


def expect_success_route(route, mode):
    def check(env, response, logs, docker_logs):
        if not response["ok"]:
            return False, f"expected HTTP 2xx, got {response['status']} {response.get('error') or response.get('text')}"
        row = latest_admin_log(logs)
        if not row:
            return False, "expected consume log with admin_info, got none"
        admin = row.get("admin_info") or {}
        if admin.get("aggregate_routing_mode") != mode:
            return False, f"expected mode={mode}, got {admin.get('aggregate_routing_mode')}"
        if admin.get("route_group") != route:
            return False, f"expected route_group={route}, got {admin.get('route_group')}, use_channel={use_channels(admin)}"
        return True, f"route_group={route}, use_channel={use_channels(admin)}, log_id={row.get('id')}"

    return check


def expect_attempted_channel(channel_id, mode, reduced_route=None, effective_weight=None):
    def check(env, response, logs, docker_logs):
        row = latest_admin_log(logs)
        admin = (row.get("admin_info") or {}) if row else {}
        channels = use_channels(admin)
        channel_seen_in_log = f"channel#{channel_id}" in docker_logs
        channel_seen_in_consume_log = str(channel_id) in channels
        if row and admin.get("aggregate_routing_mode") != mode:
            return False, f"expected mode={mode}, got {admin.get('aggregate_routing_mode')}"
        if not channel_seen_in_consume_log and not channel_seen_in_log:
            return False, (
                f"expected channel {channel_id} attempted, got "
                f"http={response['status']} use_channel={channels}"
            )
        if reduced_route:
            needle = f"route_group={reduced_route}"
            if needle not in docker_logs or "reduced degraded route" not in docker_logs:
                return False, f"expected reduced degraded log for {reduced_route}"
            if effective_weight is not None and f"effective_weight={effective_weight}" not in docker_logs:
                return False, f"expected effective_weight={effective_weight} in reduced log"
        if response["ok"]:
            return True, f"route_group={admin.get('route_group')}, use_channel={channels}, log_id={row.get('id')}"
        return True, (
            f"channel#{channel_id} attempted; upstream/client ended with "
            f"status={response['status']} error={response.get('error')}"
        )

    return check


def expect_http_status(status):
    def check(env, response, logs, docker_logs):
        if response["status"] != status:
            return False, f"expected HTTP {status}, got {response['status']} {response.get('text') or response.get('error')}"
        return True, f"status={response['status']}, consume_logs={len(logs)}"

    return check


def build_scenarios(env):
    return [
        {
            "name": "cluster healthy high-weight secondary is attempted",
            "setup": setup_cluster_healthy,
            "check": expect_attempted_channel(env.args.secondary_channel_id, "cluster"),
        },
        {
            "name": "cluster degraded secondary is reduced and still attempted",
            "setup": setup_cluster_degraded_reduced,
            "check": expect_attempted_channel(
                env.args.secondary_channel_id,
                "cluster",
                reduced_route=env.args.secondary_route,
                effective_weight=max(1, math.ceil(200 * env.args.cluster_degraded_weight_percent / 100)),
            ),
        },
        {
            "name": "cluster degraded secondary level 2 is recursively reduced",
            "setup": setup_cluster_degraded_level2_reduced,
            "check": expect_attempted_channel(
                env.args.secondary_channel_id,
                "cluster",
                reduced_route=env.args.secondary_route,
                effective_weight=max(
                    1,
                    math.ceil(
                        math.ceil(200 * env.args.cluster_degraded_weight_percent / 100)
                        * env.args.cluster_degraded_weight_percent
                        / 100
                    ),
                ),
            ),
        },
        {
            "name": "cluster degraded secondary with weight 0 remains zero weight",
            "setup": setup_cluster_zero_degraded,
            "check": expect_success_route(env.args.primary_route, "cluster"),
        },
        {
            "name": "cluster secondary manually disabled is hard unavailable",
            "setup": setup_cluster_secondary_disabled,
            "check": expect_success_route(env.args.primary_route, "cluster"),
        },
        {
            "name": "cluster primary manually disabled attempts secondary",
            "setup": setup_cluster_primary_disabled,
            "check": expect_attempted_channel(env.args.secondary_channel_id, "cluster"),
        },
        {
            "name": "cluster both manually disabled returns 503",
            "setup": setup_cluster_both_disabled,
            "check": expect_http_status(503),
        },
        {
            "name": "failover still skips degraded primary and attempts secondary",
            "setup": setup_failover_degraded_skip,
            "check": expect_attempted_channel(env.args.secondary_channel_id, "failover"),
        },
        {
            "name": "failover ignores prior cluster runtime active secondary",
            "setup": setup_failover_ignores_cluster_runtime,
            "check": expect_success_route(env.args.primary_route, "failover"),
        },
        {
            "name": "cluster expired degraded secondary restores original weight and is attempted",
            "setup": setup_cluster_degraded_expired,
            "check": expect_attempted_channel(env.args.secondary_channel_id, "cluster"),
        },
    ]


def run_scenario(env, scenario):
    print(f"\n=== {scenario['name']} ===", flush=True)
    last_error = ""
    for attempt in range(1, env.args.attempts_per_scenario + 1):
        print(f"  attempt {attempt}/{env.args.attempts_per_scenario}", flush=True)
        scenario["setup"](env)
        after_id = env.last_log_id()
        since_ts = int(time.time()) - 1
        response = env.call_api()
        logs = env.wait_new_logs(after_id) if response["ok"] else env.new_logs_after(after_id)
        docker_logs = env.docker_logs_since(since_ts)
        ok, message = scenario["check"](env, response, logs, docker_logs)
        if ok:
            print(f"  PASS: {message}", flush=True)
            return {"name": scenario["name"], "passed": True, "message": message}
        last_error = message
        print(f"  retry needed: {message}", flush=True)
    print(f"  FAIL: {last_error}", flush=True)
    return {"name": scenario["name"], "passed": False, "message": last_error}


def main():
    parser = argparse.ArgumentParser(
        description="Run deterministic aggregate cluster/failover scenario simulation against local docker dev."
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
    parser.add_argument("--message", default="Reply OK only.")
    parser.add_argument("--max-tokens", type=int, default=2)
    parser.add_argument("--request-timeout", type=int, default=130)
    parser.add_argument("--log-wait-timeout", type=int, default=10)
    parser.add_argument("--attempts-per-scenario", type=int, default=2)
    parser.add_argument("--app-container", default="new-api-dev")
    parser.add_argument("--postgres-container", default="postgres-dev")
    parser.add_argument("--postgres-user", default="root")
    parser.add_argument("--postgres-db", default="new-api")
    parser.add_argument("--redis-container", default="redis-dev")
    parser.add_argument("--restart-timeout", type=int, default=45)
    parser.add_argument("--cluster-degraded-weight-percent", type=int, default=20)
    parser.add_argument("--no-restart", action="store_true")
    parser.add_argument("--report-json", default="")
    args = parser.parse_args()
    if args.cluster_degraded_weight_percent <= 0 or args.cluster_degraded_weight_percent > 100:
        print("--cluster-degraded-weight-percent 必须在 1 到 100 之间", file=sys.stderr)
        return 2

    env = ScenarioEnv(args)
    env.ensure_ready()
    env.snapshot_state()
    results = []
    try:
        for scenario in build_scenarios(env):
            results.append(run_scenario(env, scenario))
    finally:
        print("\n=== restore dev state ===", flush=True)
        env.restore_state()
        print("restored group/channel/smart-state snapshot", flush=True)

    passed = sum(1 for item in results if item["passed"])
    summary = {"total": len(results), "passed": passed, "failed": len(results) - passed, "results": results}
    print("\n=== summary ===", flush=True)
    print(json.dumps(summary, ensure_ascii=False, indent=2), flush=True)
    if args.report_json:
        with open(args.report_json, "w", encoding="utf-8") as handle:
            json.dump(summary, handle, ensure_ascii=False, indent=2)
    return 0 if summary["failed"] == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
