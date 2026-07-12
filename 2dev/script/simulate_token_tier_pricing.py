#!/usr/bin/env python3
"""Validate token tier pricing against Docker dev and the real upstream."""

import argparse
import json
import secrets
import subprocess
import time
import urllib.error
import urllib.request
from datetime import datetime
from decimal import Decimal, ROUND_HALF_UP
from pathlib import Path


NO_PROXY_OPENER = urllib.request.build_opener(urllib.request.ProxyHandler({}))
REQUEST_ID_HEADER = "X-Oneapi-Request-Id"
OPTION_KEY = "TokenTierPricingRules"
MODEL = "gpt-5.6-luna"


def require(condition, message):
    if not condition:
        raise AssertionError(message)


def sql_str(value):
    return "'" + str(value).replace("'", "''") + "'"


def run_cmd(args, check=True):
    result = subprocess.run(args, text=True, capture_output=True, check=False)
    if check and result.returncode != 0:
        raise RuntimeError(result.stderr.strip() or "command failed")
    return result.stdout.strip()


class Validator:
    def __init__(self, args):
        self.args = args
        self.base_url = args.base_url.rstrip("/")
        self.report = {
            "started_at": datetime.now().isoformat(),
            "base_url": self.base_url,
            "model": MODEL,
            "scenarios": [],
            "configuration_restored": False,
        }
        self.token = None
        self.admin = None
        self.original_option_exists = False
        self.original_option_value = "{}"
        self.temporary_admin_token = False

    def log(self, message):
        print(message, flush=True)

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

    def request(self, method, path, body=None, headers=None, timeout=None):
        request_headers = {"Content-Type": "application/json"}
        if headers:
            request_headers.update(headers)
        data = None if body is None else json.dumps(body).encode("utf-8")
        req = urllib.request.Request(
            self.base_url + path,
            data=data,
            method=method,
            headers=request_headers,
        )
        try:
            with NO_PROXY_OPENER.open(
                req, timeout=timeout or self.args.request_timeout
            ) as response:
                raw = response.read()
                request_id = response.headers.get(REQUEST_ID_HEADER, "")
                return response.status, raw, request_id
        except urllib.error.HTTPError as exc:
            raw = exc.read()
            request_id = exc.headers.get(REQUEST_ID_HEADER, "")
            return exc.code, raw, request_id

    def admin_headers(self):
        return {
            "Authorization": f"Bearer {self.admin['access_token']}",
            "New-Api-User": str(self.admin["id"]),
        }

    def ensure_ready(self):
        for container in [
            self.args.app_container,
            self.args.postgres_container,
            self.args.redis_container,
            self.args.upstream_container,
        ]:
            state = run_cmd(
                [
                    "docker",
                    "inspect",
                    "-f",
                    "{{.State.Status}}|{{if .State.Health}}{{.State.Health.Status}}{{end}}",
                    container,
                ],
                check=False,
            )
            status, _, health = state.partition("|")
            require(status == "running", f"container {container} is not running: {state}")
            require(not health or health == "healthy", f"container {container} is not healthy: {state}")
        require(self.psql("select 1") == "1", "Postgres readiness probe failed")
        require(
            run_cmd(["docker", "exec", self.args.redis_container, "redis-cli", "ping"]) == "PONG",
            "Redis readiness probe failed",
        )
        status, raw, _ = self.request("GET", "/api/status")
        require(status == 200, f"gateway status failed: HTTP {status} {raw[:500]!r}")

    def load_secrets_and_snapshot(self):
        self.token = self.psql_json(
            f"""
            select json_build_object(
              'id', id, 'key', key, 'user_id', user_id, 'group', "group"
            ) from tokens where name = {sql_str(self.args.token_name)} limit 1
            """,
            {},
        )
        require(self.token and self.token.get("key"), "named test token was not found")
        self.admin = self.psql_json(
            """
            select json_build_object('id', id, 'access_token', access_token)
            from users where role >= 100 and deleted_at is null order by id limit 1
            """,
            {},
        )
        require(self.admin, "administrator was not found")
        if not self.admin.get("access_token"):
            self.admin["access_token"] = secrets.token_hex(16)
            self.psql(
                f"update users set access_token = {sql_str(self.admin['access_token'])} "
                f"where id = {int(self.admin['id'])}"
            )
            self.temporary_admin_token = True

        option = self.psql_json(
            f"""
            select json_build_object('value', value)
            from options where key = {sql_str(OPTION_KEY)} limit 1
            """,
            {},
        )
        self.original_option_exists = bool(option)
        self.original_option_value = option.get("value", "{}") if option else "{}"

    def update_rules(self, rules):
        status, raw, _ = self.request(
            "PUT",
            "/api/option/",
            body={"key": OPTION_KEY, "value": json.dumps(rules, separators=(",", ":"))},
            headers=self.admin_headers(),
        )
        payload = json.loads(raw.decode("utf-8")) if raw else {}
        require(status == 200 and payload.get("success") is True, f"option update failed: {payload}")

    def quota_snapshot(self):
        return self.psql_json(
            f"""
            select json_build_object(
              'user', (select used_quota from users where id = {int(self.token['user_id'])}),
              'token', (select used_quota from tokens where id = {int(self.token['id'])}),
              'channels', (select coalesce(json_object_agg(id, used_quota), '{{}}'::json)
                           from channels where status = 1)
            )
            """,
            {},
        )

    def wait_log(self, request_id):
        deadline = time.time() + self.args.log_wait_timeout
        while time.time() < deadline:
            row = self.psql_json(
                f"""
                select row_to_json(t) from (
                  select id, quota, prompt_tokens, completion_tokens, channel_id,
                         token_id, user_id, model_name, request_id, is_stream, content,
                         case when coalesce(other, '') = '' then '{{}}'::json else other::json end as other
                  from logs where request_id = {sql_str(request_id)} and type = 2
                  order by id desc limit 1
                ) t
                """,
                {},
            )
            if row:
                return row
            time.sleep(0.25)
        raise AssertionError(f"consume log not found for request {request_id}")

    def wait_quota_delta(self, before, log):
        expected = int(log["quota"])
        channel_key = str(log["channel_id"])
        deadline = time.time() + self.args.quota_wait_timeout
        latest = before
        while time.time() < deadline:
            latest = self.quota_snapshot()
            deltas = {
                "user": int(latest["user"]) - int(before["user"]),
                "token": int(latest["token"]) - int(before["token"]),
                "channel": int((latest["channels"] or {}).get(channel_key, 0))
                - int((before["channels"] or {}).get(channel_key, 0)),
            }
            if all(value == expected for value in deltas.values()):
                return deltas
            time.sleep(0.5)
        raise AssertionError(
            f"quota deltas did not settle to {expected}: before={before}, after={latest}"
        )

    @staticmethod
    def rounded_quota(value):
        return int(value.quantize(Decimal("1"), rounding=ROUND_HALF_UP))

    def recompute_tier(self, log):
        audit = log["other"].get("token_tier_pricing")
        require(audit, f"tier audit is missing: log={log['id']}")
        prices = {key: Decimal(str(value)) for key, value in audit["prices"].items()}
        components = {
            "uncached_input": Decimal(audit["billed_uncached_tokens"])
            * prices["input"],
            "cached_input": Decimal(audit["billed_cached_tokens"])
            * prices["cached_input"],
            "cache_write": Decimal(audit["billed_cache_write_tokens"])
            * prices["cache_write"],
            "output": Decimal(audit["billed_output_tokens"]) * prices["output"],
        }
        quota_per_unit = Decimal(self.psql("select value from options where key = 'QuotaPerUnit' limit 1") or "500000")
        component_quota = {
            key: value * quota_per_unit / Decimal(1_000_000)
            for key, value in components.items()
        }
        subtotal = sum(component_quota.values(), Decimal(0))
        final = subtotal * Decimal(str(audit["group_ratio"]))
        expected = self.rounded_quota(final)
        require(expected == int(log["quota"]), f"tier quota mismatch: expected={expected}, log={log['quota']}")
        require("整次请求换档" in (log.get("content") or ""), "human tier calculation is missing")
        return {
            "formula": component_quota,
            "subtotal_quota": subtotal,
            "final_decimal": final,
            "expected_quota": expected,
            "tier": audit,
        }

    def recompute_legacy(self, log):
        other = log["other"]
        prompt = int(log["prompt_tokens"])
        completion = int(log["completion_tokens"])
        cached = int(other.get("cache_tokens") or 0)
        cache_write = int(other.get("cache_creation_tokens") or 0)
        ordinary = max(0, prompt - cached - cache_write)
        weighted = (
            Decimal(ordinary)
            + Decimal(cached) * Decimal(str(other.get("cache_ratio") or 0))
            + Decimal(cache_write) * Decimal(str(other.get("cache_creation_ratio") or 0))
            + Decimal(completion) * Decimal(str(other.get("completion_ratio") or 0))
        )
        final = weighted * Decimal(str(other.get("model_ratio") or 0)) * Decimal(str(other.get("group_ratio") or 0))
        expected = self.rounded_quota(final)
        require(expected == int(log["quota"]), f"legacy quota mismatch: expected={expected}, log={log['quota']}")
        require("token_tier_pricing" not in other, "disabled tier unexpectedly wrote an audit")
        return {"formula": weighted, "final_decimal": final, "expected_quota": expected}

    def call_model(self, name, prompt, stream=False, expected_tier=None, expect_legacy=False):
        before = self.quota_snapshot()
        body = {
            "model": MODEL,
            "input": prompt,
            "instructions": "Reply with OK only.",
            "max_output_tokens": 32,
            "stream": stream,
        }
        status, raw, request_id = self.request(
            "POST",
            "/v1/responses",
            body=body,
            headers={"Authorization": f"Bearer sk-{self.token['key']}"},
            timeout=self.args.long_request_timeout if len(prompt) > 1_000_000 else None,
        )
        require(status == 200, f"{name} failed HTTP {status}: {raw[:1000]!r}")
        require(request_id, f"{name} response did not include a request ID")
        log = self.wait_log(request_id)
        deltas = self.wait_quota_delta(before, log)
        calculation = self.recompute_legacy(log) if expect_legacy else self.recompute_tier(log)
        if expected_tier is not None:
            require(
                int(calculation["tier"]["tier_index"]) == expected_tier,
                f"{name} selected tier {calculation['tier']['tier_index']}, expected {expected_tier}",
            )
        require(bool(log["is_stream"]) == stream, f"{name} stream flag mismatch")
        scenario = {
            "name": name,
            "request_id": request_id,
            "model": log["model_name"],
            "log_id": log["id"],
            "channel_id": log["channel_id"],
            "usage": {
                "prompt_tokens": log["prompt_tokens"],
                "completion_tokens": log["completion_tokens"],
            },
            "expected_quota": calculation["expected_quota"],
            "actual_quota": log["quota"],
            "quota_deltas": deltas,
            "tier_index": calculation.get("tier", {}).get("tier_index"),
            "meter_source": calculation.get("tier", {}).get("meter_source"),
            "status": "passed",
        }
        self.report["scenarios"].append(scenario)
        self.log(
            f"PASS {name}: request_id={request_id} usage={log['prompt_tokens']}/{log['completion_tokens']} "
            f"tier={scenario['tier_index'] or '-'} quota={log['quota']} channel={log['channel_id']}"
        )
        return log

    def run(self):
        self.ensure_ready()
        self.load_secrets_and_snapshot()
        failure = None
        try:
            self.update_rules({MODEL: {"enabled": False}})
            legacy_log = self.call_model(
                "A-disabled-legacy", "small baseline one", expect_legacy=True
            )

            self.update_rules({})
            short_log = self.call_model(
                "B-official-short", "small baseline two", expected_tier=1
            )
            require(
                int(legacy_log["prompt_tokens"]) == int(short_log["prompt_tokens"]),
                "equivalent short prompts produced different input usage",
            )
            require(
                int(legacy_log["quota"]) == int(short_log["quota"]),
                "legacy and official first-tier charges differ for equivalent usage",
            )

            test_rules = {
                MODEL: {
                    "id": "docker-three-tier-test",
                    "enabled": True,
                    "service_tier": "standard",
                    "meter": "input_tokens_total",
                    "billing_mode": "whole_request",
                    "tiers": [
                        {"up_to_inclusive": 20, "use_base_price": True},
                        {
                            "up_to_inclusive": 100,
                            "prices": {"input": 3, "cached_input": 0.3, "cache_write": 3.75, "output": 12},
                        },
                        {
                            "up_to_inclusive": None,
                            "prices": {"input": 4, "cached_input": 0.4, "cache_write": 5, "output": 18},
                        },
                    ],
                }
            }
            self.update_rules(test_rules)
            self.call_model("C1-three-tier-first", "hi", expected_tier=1)
            self.call_model("C2-three-tier-second", "hello " * 40, expected_tier=2)
            self.call_model("C3-three-tier-third", "hello " * 160, expected_tier=3)

            self.update_rules({})
            if self.args.allow_real_long_context:
                long_log = self.call_model(
                    "D-official-real-long-context",
                    "hello " * self.args.long_context_repetitions,
                    expected_tier=2,
                )
                require(
                    int(long_log["other"]["token_tier_pricing"]["total_input_tokens"]) > 272000,
                    "real long-context request did not exceed 272K actual input tokens",
                )
            else:
                self.report["scenarios"].append(
                    {"name": "D-official-real-long-context", "status": "skipped"}
                )
            self.call_model("E-official-stream", "streaming check", stream=True, expected_tier=1)
        except Exception as exc:
            failure = exc
            self.report["failure"] = f"{type(exc).__name__}: {exc}"
        finally:
            try:
                self.update_rules(json.loads(self.original_option_value or "{}"))
                if not self.original_option_exists:
                    self.psql(f"delete from options where key = {sql_str(OPTION_KEY)}")
                self.report["configuration_restored"] = True
            finally:
                if self.temporary_admin_token:
                    self.psql(f"update users set access_token = null where id = {int(self.admin['id'])}")

        self.ensure_ready()
        self.report["finished_at"] = datetime.now().isoformat()
        output = Path(self.args.report_dir) / f"token-tier-pricing-report-{int(time.time())}.json"
        output.parent.mkdir(parents=True, exist_ok=True)
        output.write_text(
            json.dumps(self.report, ensure_ascii=False, indent=2, default=str),
            encoding="utf-8",
        )
        self.log(f"Report: {output}")
        if failure is not None:
            raise failure


def parse_args():
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", default="http://localhost:3001")
    parser.add_argument("--app-container", default="new-api-dev")
    parser.add_argument("--postgres-container", default="postgres-dev")
    parser.add_argument("--redis-container", default="redis-dev")
    parser.add_argument("--upstream-container", default="sub2api-dev")
    parser.add_argument("--postgres-user", default="root")
    parser.add_argument("--postgres-db", default="new-api")
    parser.add_argument("--token-name", default="test-gpt")
    parser.add_argument("--report-dir", default="tmp")
    parser.add_argument("--request-timeout", type=int, default=180)
    parser.add_argument("--long-request-timeout", type=int, default=900)
    parser.add_argument("--log-wait-timeout", type=int, default=60)
    parser.add_argument("--quota-wait-timeout", type=int, default=60)
    parser.add_argument("--long-context-repetitions", type=int, default=285000)
    parser.add_argument("--allow-real-long-context", action="store_true")
    return parser.parse_args()


if __name__ == "__main__":
    Validator(parse_args()).run()
