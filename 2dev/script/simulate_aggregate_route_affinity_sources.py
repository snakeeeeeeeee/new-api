#!/usr/bin/env python3
import argparse
import json
import subprocess
import sys
import time
import urllib.error
import urllib.request


DEFAULT_TOKEN = "sk-codexclusterdev20260428token00000000000000000000"
NO_PROXY_OPENER = urllib.request.build_opener(urllib.request.ProxyHandler({}))


def shell_quote_sql(value):
    return "'" + str(value).replace("'", "''") + "'"


def run_cmd(args, check=True):
    result = subprocess.run(args, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if check and result.returncode != 0:
        detail = (result.stderr or result.stdout or "").strip()
        raise RuntimeError(f"command failed ({result.returncode}): {' '.join(args)}\n{detail}")
    return result.stdout.strip()


def decode_body(raw):
    text = raw.decode("utf-8", errors="replace")
    if not text:
        return None, ""
    try:
        return json.loads(text), text
    except json.JSONDecodeError:
        return None, text


class AffinityScenario:
    def __init__(self, args):
        self.args = args
        self.base_url = args.base_url.rstrip("/")
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
        )

    def psql_json(self, sql, default):
        raw = self.psql(sql)
        if not raw:
            return default
        return json.loads(raw)

    def redis(self, *items, check=True):
        return run_cmd(["docker", "exec", self.args.redis_container, "redis-cli", *map(str, items)], check=check)

    def clear_route_affinity(self):
        deleted = self.redis(
            "eval",
            "local keys=redis.call('keys', ARGV[1]); if #keys > 0 then return redis.call('del', unpack(keys)) end; return 0",
            "0",
            "new-api:aggregate_route_affinity:v3*",
            check=False,
        )
        print(f"cleared_route_affinity_keys={deleted or 0}")

    def snapshot_state(self):
        group = self.psql_json(
            f"""
            select row_to_json(t)
            from (
              select routing_mode, route_affinity_strategy, route_affinity_key_sources
              from aggregate_groups
              where name = {shell_quote_sql(self.args.group)}
            ) t
            """,
            {},
        )
        if not group:
            raise RuntimeError(f"aggregate group not found: {self.args.group}")
        targets = self.psql_json(
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
        if len(targets) != 2:
            raise RuntimeError(
                f"expected both primary/secondary targets under {self.args.group}, got: {targets}"
            )
        self.snapshot = {"group": group, "targets": targets}

    def apply_request_only_config(self):
        sources = [
            {"type": "header", "key": "X-Aggregate-Affinity-Key"},
            {"type": "query", "key": "aggregate_route_affinity_key"},
            {"type": "gjson", "path": "metadata.aggregate_route_affinity_key"},
            {"type": "gjson", "path": "metadata.user_id"},
            {"type": "gjson", "path": "prompt_cache_key"},
            {"type": "gjson", "path": "user"},
            {"type": "gjson", "path": "cachedContent"},
        ]
        self.psql(
            f"""
            update aggregate_groups
            set routing_mode = 'cluster',
                route_affinity_strategy = 'request_only',
                route_affinity_key_sources = {shell_quote_sql(json.dumps(sources, ensure_ascii=False))}
            where name = {shell_quote_sql(self.args.group)}
            """
        )

    def set_weights(self, primary_weight, secondary_weight):
        self.psql(
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

    def restore_state(self):
        if not self.snapshot or self.args.no_restore:
            return
        group = self.snapshot["group"]
        self.psql(
            f"""
            update aggregate_groups
            set routing_mode = {shell_quote_sql(group.get("routing_mode") or "failover")},
                route_affinity_strategy = {shell_quote_sql(group.get("route_affinity_strategy") or "platform_user")},
                route_affinity_key_sources = {shell_quote_sql(group.get("route_affinity_key_sources") or "")}
            where name = {shell_quote_sql(self.args.group)}
            """
        )
        for target in self.snapshot["targets"]:
            self.psql(
                f"""
                update aggregate_group_targets
                set weight = {int(target.get("weight") or 0)}
                where aggregate_group_id = (
                  select id from aggregate_groups where name = {shell_quote_sql(self.args.group)}
                )
                  and real_group = {shell_quote_sql(target.get("real_group"))}
                """
            )
        self.clear_route_affinity()
        print("restored_original_group_state=true")

    def build_payload(self, affinity_key):
        if self.args.api_format == "claude":
            payload = {
                "model": self.args.model,
                "messages": [{"role": "user", "content": self.args.message}],
                "max_tokens": self.args.max_tokens,
                "stream": False,
            }
            if affinity_key:
                payload["metadata"] = {"aggregate_route_affinity_key": affinity_key}
            return payload
        payload = {
            "model": self.args.model,
            "messages": [{"role": "user", "content": self.args.message}],
            "max_tokens": self.args.max_tokens,
            "stream": False,
        }
        if affinity_key:
            payload["metadata"] = {"aggregate_route_affinity_key": affinity_key}
            payload["user"] = affinity_key
        return payload

    def call_api(self, label, affinity_key):
        payload = self.build_payload(affinity_key)
        body = json.dumps(payload).encode("utf-8")
        url = self.base_url + self.args.endpoint
        before_log_id = self.max_log_id()
        req = urllib.request.Request(url, data=body, method="POST")
        req.add_header("Authorization", f"Bearer {self.args.token}")
        req.add_header("Content-Type", "application/json")
        if self.args.api_format == "claude":
            req.add_header("anthropic-version", self.args.claude_version)
        if affinity_key and self.args.use_header_affinity:
            req.add_header("X-Aggregate-Affinity-Key", affinity_key)
        started = time.time()
        status = None
        request_id = ""
        try:
            with NO_PROXY_OPENER.open(req, timeout=self.args.timeout) as resp:
                status = resp.status
                request_id = resp.headers.get("X-Oneapi-Request-Id", "")
                decode_body(resp.read())
        except urllib.error.HTTPError as exc:
            status = exc.code
            request_id = exc.headers.get("X-Oneapi-Request-Id", "")
            decode_body(exc.read())
        except Exception as exc:
            raise RuntimeError(f"{label} request failed before response: {exc}") from exc
        elapsed_ms = int((time.time() - started) * 1000)
        time.sleep(self.args.log_wait)
        admin = self.fetch_admin_info(request_id)
        if not admin:
            admin = self.fetch_admin_info_after(before_log_id)
        print(
            f"{label}: status={status} elapsed={elapsed_ms}ms request_id={request_id or '-'} "
            f"route_group={admin.get('route_group') or '-'} affinity={json.dumps(admin.get('aggregate_route_affinity') or {}, ensure_ascii=False)}"
        )
        return admin

    def fetch_admin_info(self, request_id):
        if not request_id:
            return {}
        raw = self.psql(
            f"""
            select coalesce(other::jsonb->'admin_info', '{{}}'::jsonb)::text
            from logs
            where request_id = {shell_quote_sql(request_id)}
            order by id desc
            limit 1
            """
        )
        if not raw:
            return {}
        return json.loads(raw)

    def max_log_id(self):
        raw = self.psql("select coalesce(max(id), 0) from logs")
        try:
            return int(raw or 0)
        except ValueError:
            return 0

    def fetch_admin_info_after(self, before_log_id):
        raw = self.psql(
            f"""
            select coalesce(other::jsonb->'admin_info', '{{}}'::jsonb)::text
            from logs
            where id > {int(before_log_id)}
              and "group" = {shell_quote_sql(self.args.group)}
              and model_name = {shell_quote_sql(self.args.model)}
            order by id desc
            limit 1
            """
        )
        if not raw:
            return {}
        return json.loads(raw)

    def require_route(self, label, admin, expected_route, expect_affinity_fp):
        route = admin.get("route_group")
        if route != expected_route:
            raise RuntimeError(f"{label}: expected route_group={expected_route}, got={route}, admin={admin}")
        affinity = admin.get("aggregate_route_affinity") or {}
        has_fp = bool(affinity.get("key_fp"))
        if expect_affinity_fp and not has_fp:
            raise RuntimeError(f"{label}: expected aggregate route affinity key_fp, admin={admin}")
        if not expect_affinity_fp and has_fp:
            raise RuntimeError(f"{label}: expected no request affinity key_fp, admin={admin}")

    def run(self):
        self.snapshot_state()
        try:
            self.apply_request_only_config()
            self.clear_route_affinity()

            self.set_weights(100, 0)
            first = self.call_api("bind person-a on primary", "person-a")
            self.require_route("bind person-a on primary", first, self.args.primary_route, True)

            self.set_weights(0, 100)
            sticky = self.call_api("person-a should stay on primary", "person-a")
            self.require_route("person-a should stay on primary", sticky, self.args.primary_route, True)

            new_person = self.call_api("person-b should follow current weight", "person-b")
            self.require_route("person-b should follow current weight", new_person, self.args.secondary_route, True)

            missing = self.call_api("missing identifier should not write affinity", "")
            self.require_route("missing identifier should not write affinity", missing, self.args.secondary_route, False)
            print("scenario_result=PASS")
        finally:
            self.restore_state()


def parse_args():
    parser = argparse.ArgumentParser(
        description="Simulate aggregate route affinity request identity extraction in docker dev."
    )
    parser.add_argument("--base-url", default="http://localhost:3001")
    parser.add_argument("--token", default=DEFAULT_TOKEN)
    parser.add_argument("--group", default="codex_cluster_dev_20260428")
    parser.add_argument("--primary-route", default="claude-re-kiro")
    parser.add_argument("--secondary-route", default="claude-re-kiro-003")
    parser.add_argument("--api-format", choices=["claude", "openai"], default="claude")
    parser.add_argument("--endpoint", default="/v1/messages?beta=true")
    parser.add_argument("--model", default="claude-sonnet-4-6")
    parser.add_argument("--message", default="Reply OK only.")
    parser.add_argument("--max-tokens", type=int, default=2)
    parser.add_argument("--timeout", type=int, default=120)
    parser.add_argument("--log-wait", type=float, default=0.5)
    parser.add_argument("--use-header-affinity", action="store_true")
    parser.add_argument("--claude-version", default="2023-06-01")
    parser.add_argument("--postgres-container", default="postgres-dev")
    parser.add_argument("--postgres-user", default="root")
    parser.add_argument("--postgres-db", default="new-api")
    parser.add_argument("--redis-container", default="redis-dev")
    parser.add_argument("--no-restore", action="store_true")
    return parser.parse_args()


def main():
    args = parse_args()
    if args.api_format == "openai" and args.endpoint == "/v1/messages?beta=true":
        args.endpoint = "/v1/chat/completions"
    try:
        AffinityScenario(args).run()
    except Exception as exc:
        print(f"scenario_result=FAIL error={exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
