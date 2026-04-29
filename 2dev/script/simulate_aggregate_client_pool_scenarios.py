#!/usr/bin/env python3
import argparse
import json
import subprocess
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


class ClientPoolScenario:
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
        return run_cmd(
            ["docker", "exec", self.args.redis_container, "redis-cli", *map(str, items)],
            check=check,
        )

    def clear_affinity(self):
        self.redis(
            "eval",
            "local keys=redis.call('keys', ARGV[1]); for _,k in ipairs(keys) do redis.call('del', k) end; return #keys",
            "0",
            "new-api:aggregate_route_affinity:v3*",
            check=False,
        )

    def snapshot_state(self):
        group_row = self.psql_json(
            f"""
            select row_to_json(t)
            from (
              select id, routing_mode, client_route_pools
              from aggregate_groups
              where name = {shell_quote_sql(self.args.group)}
            ) t
            """,
            {},
        )
        if not group_row:
            raise RuntimeError(f"aggregate group not found: {self.args.group}")
        targets = self.psql_json(
            f"""
            select coalesce(json_agg(row_to_json(t) order by order_index), '[]'::json)
            from (
              select real_group, order_index, weight
              from aggregate_group_targets
              where aggregate_group_id = {int(group_row["id"])}
              order by order_index
            ) t
            """,
            [],
        )
        self.snapshot = {"group": group_row, "targets": targets}

    def apply_scenario_config(self, routing_mode):
        group_id = int(self.snapshot["group"]["id"])
        client_route_pools = {
            "enabled": True,
            "claude_code_cli": {
                "enabled": True,
                "fallback_to_default": self.args.fallback_to_default,
                "targets": [
                    {"real_group": self.args.cli_route, "weight": self.args.cli_weight}
                ],
            },
        }
        client_route_pools_sql = shell_quote_sql(json.dumps(client_route_pools, ensure_ascii=False))
        self.psql(
            f"""
            update aggregate_groups
            set routing_mode = {shell_quote_sql(routing_mode)},
                client_route_pools = {client_route_pools_sql}
            where id = {group_id}
            """
        )
        self.psql(f"delete from aggregate_group_targets where aggregate_group_id = {group_id}")
        self.psql(
            f"""
            insert into aggregate_group_targets (aggregate_group_id, real_group, order_index, weight)
            values ({group_id}, {shell_quote_sql(self.args.default_route)}, 0, {int(self.args.default_weight)})
            """
        )

    def restore_state(self):
        if not self.snapshot:
            return
        group = self.snapshot["group"]
        group_id = int(group["id"])
        routing_mode = shell_quote_sql(group.get("routing_mode") or "failover")
        client_route_pools = group.get("client_route_pools") or ""
        self.psql(
            f"""
            update aggregate_groups
            set routing_mode = {routing_mode},
                client_route_pools = {shell_quote_sql(client_route_pools)}
            where id = {group_id}
            """
        )
        self.psql(f"delete from aggregate_group_targets where aggregate_group_id = {group_id}")
        for target in self.snapshot["targets"]:
            self.psql(
                f"""
                insert into aggregate_group_targets (aggregate_group_id, real_group, order_index, weight)
                values (
                  {group_id},
                  {shell_quote_sql(target["real_group"])},
                  {int(target["order_index"])},
                  {int(target["weight"] if target["weight"] is not None else 100)}
                )
                """
            )

    def call_messages(self, cli):
        headers = {
            "Authorization": f"Bearer {self.args.token}",
            "Content-Type": "application/json",
            "Anthropic-Version": "2023-06-01",
        }
        if cli:
            headers.update(
                {
                    "User-Agent": "claude-cli/2.1.116 (external, sdk-cli)",
                    "X-App": "cli",
                    "Anthropic-Beta": "claude-code-20990101,interleaved-thinking-2025-05-14",
                }
            )
        else:
            headers["User-Agent"] = "normal-client/1.0"
        body = {
            "model": self.args.model,
            "max_tokens": self.args.max_tokens,
            "messages": [{"role": "user", "content": self.args.message}],
            "metadata": {"user_id": "client-pool-scenario-user"},
        }
        req = urllib.request.Request(
            self.base_url + self.args.endpoint,
            data=json.dumps(body).encode("utf-8"),
            headers=headers,
            method="POST",
        )
        started = time.time()
        try:
            with NO_PROXY_OPENER.open(req, timeout=self.args.timeout) as resp:
                data, text = decode_body(resp.read())
                return {
                    "ok": True,
                    "status": resp.status,
                    "elapsed_ms": int((time.time() - started) * 1000),
                    "body": data if data is not None else text[:500],
                }
        except urllib.error.HTTPError as exc:
            raw = exc.read()
            data, text = decode_body(raw)
            return {
                "ok": False,
                "status": exc.code,
                "elapsed_ms": int((time.time() - started) * 1000),
                "body": data if data is not None else text[:500],
            }
        except Exception as exc:
            return {
                "ok": False,
                "status": 0,
                "elapsed_ms": int((time.time() - started) * 1000),
                "body": str(exc),
            }

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

    def logs_after(self, after_id):
        return self.psql_json(
            f"""
            select coalesce(json_agg(row_to_json(t)), '[]'::json)
            from (
              select id,
                     request_id,
                     channel_id,
                     model_name,
                     "group" as log_group,
                     other::json -> 'admin_info' as admin_info
              from logs
              where id > {int(after_id)}
                and "group" = {shell_quote_sql(self.args.group)}
                and model_name = {shell_quote_sql(self.args.model)}
              order by id desc
              limit 8
            ) t
            """,
            [],
        )

    def run(self):
        self.snapshot_state()
        report = {"group": self.args.group, "model": self.args.model, "scenarios": []}
        modes = ["cluster", "failover"] if self.args.routing_mode == "both" else [self.args.routing_mode]
        try:
            for mode in modes:
                self.clear_affinity()
                self.apply_scenario_config(mode)
                time.sleep(self.args.config_wait)
                for name, cli in [("claude_cli", True), ("normal_claude", False)]:
                    scenario_name = f"{mode}_{name}"
                    for attempt in range(1, self.args.attempts_per_scenario + 1):
                        after_id = self.last_log_id()
                        result = self.call_messages(cli)
                        time.sleep(self.args.log_wait)
                        logs = self.logs_after(after_id)
                        item = {
                            "mode": mode,
                            "scenario": scenario_name,
                            "attempt": attempt,
                            "request": result,
                            "recent_logs": logs,
                        }
                        report["scenarios"].append(item)
                        print(f"== {scenario_name} #{attempt} ==")
                        print(json.dumps(item, ensure_ascii=False, indent=2))
            if self.args.report_json:
                with open(self.args.report_json, "w", encoding="utf-8") as f:
                    json.dump(report, f, ensure_ascii=False, indent=2)
            if self.args.strict:
                self.assert_report(report)
        finally:
            if not self.args.no_restore:
                self.restore_state()
                self.clear_affinity()

    def assert_report(self, report):
        scenarios_by_name = {}
        for scenario in report["scenarios"]:
            scenarios_by_name[scenario["scenario"]] = scenario
        modes = ["cluster", "failover"] if self.args.routing_mode == "both" else [self.args.routing_mode]
        for mode in modes:
            cli_logs = scenarios_by_name.get(f"{mode}_claude_cli", {}).get("recent_logs") or []
            cli_admin_logs = [item.get("admin_info") or {} for item in cli_logs]
            cli_attempted_dedicated = any(
                admin.get("client_type") == "claude_code_cli"
                and admin.get("client_route_pool") == "claude_code_cli"
                and admin.get("aggregate_route_pool") == "claude_code_cli"
                and admin.get("route_group") == self.args.cli_route
                for admin in cli_admin_logs
            )
            if not cli_attempted_dedicated:
                raise RuntimeError(f"strict check failed: {mode} cli dedicated route attempt missing in {cli_admin_logs}")
            normal_admin = (scenarios_by_name.get(f"{mode}_normal_claude", {}).get("recent_logs") or [{}])[0].get("admin_info") or {}
            if normal_admin.get("route_group") != self.args.default_route:
                raise RuntimeError(f"strict check failed: {mode} normal route_group expected {self.args.default_route}, got {normal_admin}")
            if normal_admin.get("client_type"):
                raise RuntimeError(f"strict check failed: {mode} normal request should not have client_type, got {normal_admin}")


def parse_args():
    parser = argparse.ArgumentParser(description="Simulate aggregate group Claude CLI client route pool scenarios.")
    parser.add_argument("--base-url", default="http://localhost:3001")
    parser.add_argument("--token", default=DEFAULT_TOKEN)
    parser.add_argument("--group", default="codex_cluster_dev_20260428")
    parser.add_argument("--model", default="claude-sonnet-4-6")
    parser.add_argument("--endpoint", default="/v1/messages?beta=true")
    parser.add_argument("--default-route", default="claude-re-kiro")
    parser.add_argument("--cli-route", default="claude-re-kiro-003")
    parser.add_argument("--default-weight", type=int, default=100)
    parser.add_argument("--cli-weight", type=int, default=100)
    parser.add_argument("--fallback-to-default", action=argparse.BooleanOptionalAction, default=True)
    parser.add_argument("--routing-mode", choices=["cluster", "failover", "both"], default="both")
    parser.add_argument("--max-tokens", type=int, default=2)
    parser.add_argument("--message", default="Reply OK only.")
    parser.add_argument("--attempts-per-scenario", type=int, default=1)
    parser.add_argument("--timeout", type=int, default=90)
    parser.add_argument("--config-wait", type=float, default=0.5)
    parser.add_argument("--log-wait", type=float, default=1.0)
    parser.add_argument("--postgres-container", default="postgres-dev")
    parser.add_argument("--postgres-user", default="root")
    parser.add_argument("--postgres-db", default="new-api")
    parser.add_argument("--redis-container", default="redis-dev")
    parser.add_argument("--report-json", default="")
    parser.add_argument("--strict", action="store_true")
    parser.add_argument("--no-restore", action="store_true")
    return parser.parse_args()


def main():
    scenario = ClientPoolScenario(parse_args())
    scenario.run()


if __name__ == "__main__":
    main()
