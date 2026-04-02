#!/usr/bin/env python3
import argparse
import json
import subprocess
import sys
import time
import urllib.error
import urllib.request


def http_request(method, url, headers=None, body=None, timeout=60, retries=4):
    last_exc = None
    for attempt in range(1, retries + 1):
        data = None
        if body is not None:
            data = json.dumps(body).encode("utf-8")
        req = urllib.request.Request(url, data=data, method=method)
        for key, value in (headers or {}).items():
            req.add_header(key, value)
        try:
            with urllib.request.urlopen(req, timeout=timeout) as resp:
                payload = resp.read()
                return resp.status, json.loads(payload.decode("utf-8"))
        except urllib.error.HTTPError as exc:
            last_exc = exc
            if exc.code not in (429, 500, 502, 503, 504) or attempt == retries:
                raise
            time.sleep(attempt)
        except Exception as exc:
            last_exc = exc
            if attempt == retries:
                raise
            time.sleep(attempt)
    raise last_exc


def token_headers(token):
    return {
        "Authorization": f"Bearer sk-{token}",
        "Content-Type": "application/json",
    }


def get_token_usage(base_url, token):
    _, payload = http_request(
        "GET",
        f"{base_url}/api/usage/token/",
        headers={"Authorization": f"Bearer sk-{token}"},
    )
    return payload["data"]


def get_token_logs(base_url, token):
    _, payload = http_request(
        "GET",
        f"{base_url}/api/log/token",
        headers={"Authorization": f"Bearer sk-{token}"},
    )
    if not payload.get("success"):
        raise RuntimeError(f"failed to fetch token logs: {payload.get('message')}")
    return payload["data"]


def wait_for_token_usage_change(base_url, token, used_before, retries=5, interval=1):
    try:
        last_usage = get_token_usage(base_url, token)
    except Exception:
        return None
    for _ in range(retries):
        if last_usage and last_usage["total_used"] > used_before:
            return last_usage
        time.sleep(interval)
        try:
            last_usage = get_token_usage(base_url, token)
        except Exception:
            return None
    return last_usage


def chat_once(base_url, token, model, message, retries=3):
    payload = {
        "model": model,
        "messages": [{"role": "user", "content": message}],
        "max_tokens": 32,
        "stream": False,
    }
    last_error = None
    for attempt in range(1, retries + 1):
        try:
            status, body = http_request(
                "POST",
                f"{base_url}/v1/chat/completions",
                headers=token_headers(token),
                body=payload,
                timeout=120,
            )
            if status != 200:
                raise RuntimeError(f"chat completion status={status} payload={body}")
            if "choices" not in body or not body["choices"]:
                raise RuntimeError(f"missing choices in response: {body}")
            return body
        except urllib.error.HTTPError as exc:
            body = exc.read().decode("utf-8", errors="replace")
            last_error = RuntimeError(
                f"chat completion http error attempt={attempt} status={exc.code} body={body}"
            )
            if exc.code < 500 or attempt == retries:
                raise last_error
            time.sleep(1)
        except Exception as exc:
            last_error = exc
            if attempt == retries:
                raise
            time.sleep(1)
    raise last_error


def postgres_query(container, sql):
    result = subprocess.run(
        [
            "docker",
            "exec",
            container,
            "psql",
            "-U",
            "root",
            "-d",
            "new-api",
            "-t",
            "-A",
            "-F",
            "\t",
            "-c",
            sql,
        ],
        check=True,
        capture_output=True,
        text=True,
    )
    return [line for line in result.stdout.splitlines() if line.strip()]


def get_token_db_meta(container, token):
    rows = postgres_query(
        container,
        f"select id, name, \"group\", used_quota, remain_quota, unlimited_quota from tokens where key = '{token}' limit 1;",
    )
    if not rows:
        raise RuntimeError(f"token not found in postgres for key prefix={token[:6]}")
    token_id, token_name, logical_group, used_quota, remain_quota, unlimited_quota = rows[0].split("\t")
    return {
        "token_id": int(token_id),
        "token_name": token_name,
        "logical_group": logical_group,
        "used_quota": int(used_quota),
        "remain_quota": int(remain_quota),
        "unlimited_quota": unlimited_quota == "t",
    }


def get_latest_raw_log(container, token_id, min_log_id=0):
    rows = postgres_query(
        container,
        "select id, type, model_name, token_name, \"group\", quota, other "
        f"from logs where token_id = {token_id} and id > {min_log_id} "
        "order by id desc limit 1;",
    )
    if not rows:
        return None
    log_id, log_type, model_name, token_name, logical_group, quota, other = rows[0].split(
        "\t", 6
    )
    return {
        "id": int(log_id),
        "type": int(log_type),
        "model_name": model_name,
        "token_name": token_name,
        "logical_group": logical_group,
        "quota": int(quota),
        "other": json.loads(other) if other else {},
    }


def run_case(args, label, token, model, expected_group, expected_route_group=None):
    print(f"\n=== {label} ===")
    token_meta = get_token_db_meta(args.postgres_container, token)
    try:
        usage_before = get_token_usage(args.base_url, token)
    except Exception:
        usage_before = None
    try:
        token_logs_before = get_token_logs(args.base_url, token)
    except Exception:
        token_logs_before = []
    raw_before = get_latest_raw_log(args.postgres_container, token_meta["token_id"]) or {"id": 0}

    print(
        json.dumps(
            {
                "token_name": token_meta["token_name"],
                "model": model,
                "usage_before": {
                    "total_used": None if usage_before is None else usage_before["total_used"],
                    "total_available": None if usage_before is None else usage_before["total_available"],
                    "db_used_quota": token_meta["used_quota"],
                    "db_remain_quota": token_meta["remain_quota"],
                },
                "expected_group": expected_group,
                "expected_route_group": expected_route_group,
            },
            ensure_ascii=False,
            indent=2,
        )
    )

    response = chat_once(base_url=args.base_url, token=token, model=model, message=args.message)
    content = response["choices"][0]["message"]["content"]
    print(f"response={content!r}")

    usage_after = None
    if usage_before is not None:
        usage_after = wait_for_token_usage_change(
            args.base_url, token, usage_before["total_used"]
        )
    time.sleep(1)
    try:
        token_logs_after = get_token_logs(args.base_url, token)
    except Exception:
        token_logs_after = []
    raw_after = get_latest_raw_log(
        args.postgres_container, token_meta["token_id"], min_log_id=raw_before["id"]
    )

    if raw_after is None:
        raise RuntimeError(f"{label}: no raw log found in postgres")

    if raw_after["logical_group"] != expected_group:
        raise RuntimeError(
            f"{label}: raw log logical group mismatch expected={expected_group} actual={raw_after['logical_group']}"
        )

    if raw_after["quota"] <= 0:
        raise RuntimeError(f"{label}: raw log quota is not positive: {raw_after['quota']}")

    token_meta_after = get_token_db_meta(args.postgres_container, token)
    if token_meta_after["used_quota"] <= token_meta["used_quota"]:
        raise RuntimeError(
            f"{label}: db used_quota did not increase: before={token_meta['used_quota']} after={token_meta_after['used_quota']}"
        )

    latest_user_log_group = None
    if token_logs_after:
        latest_user_log = token_logs_after[0]
        latest_user_log_group = latest_user_log.get("group")
        if latest_user_log_group != expected_group:
            raise RuntimeError(
                f"{label}: user log group mismatch expected={expected_group} actual={latest_user_log_group}"
            )

    admin_info = (raw_after["other"] or {}).get("admin_info", {})
    actual_route_group = admin_info.get("route_group")
    if expected_route_group is not None and actual_route_group != expected_route_group:
        raise RuntimeError(
            f"{label}: route_group mismatch expected={expected_route_group} actual={actual_route_group}"
        )

    summary = {
        "token_name": token_meta["token_name"],
        "db_used_quota_before": token_meta["used_quota"],
        "db_used_quota_after": token_meta_after["used_quota"],
        "usage_before": None if usage_before is None else usage_before["total_used"],
        "usage_after": None if usage_after is None else usage_after["total_used"],
        "usage_changed": False
        if usage_after is None
        else usage_after["total_used"] > usage_before["total_used"],
        "user_log_group": latest_user_log_group,
        "raw_log_group": raw_after["logical_group"],
        "route_group": actual_route_group,
        "quota": raw_after["quota"],
        "request_path": raw_after["other"].get("request_path"),
    }
    print(json.dumps(summary, ensure_ascii=False, indent=2))


def main():
    parser = argparse.ArgumentParser(
        description="Test ordinary-group and aggregate-group tokens against local new-api."
    )
    parser.add_argument("--base-url", default="http://localhost:3001")
    parser.add_argument("--postgres-container", default="postgres-dev")
    parser.add_argument("--message", default="Reply with OK only.")
    parser.add_argument("--normal-token", required=True)
    parser.add_argument("--normal-model", required=True)
    parser.add_argument("--normal-expected-group", required=True)
    parser.add_argument("--normal-expected-route-group")
    parser.add_argument("--aggregate-token", required=True)
    parser.add_argument("--aggregate-model", required=True)
    parser.add_argument("--aggregate-expected-group", required=True)
    parser.add_argument("--aggregate-expected-route-group")
    args = parser.parse_args()

    try:
        run_case(
            args,
            label="ordinary-group",
            token=args.normal_token,
            model=args.normal_model,
            expected_group=args.normal_expected_group,
            expected_route_group=args.normal_expected_route_group,
        )
        run_case(
            args,
            label="aggregate-group",
            token=args.aggregate_token,
            model=args.aggregate_model,
            expected_group=args.aggregate_expected_group,
            expected_route_group=args.aggregate_expected_route_group,
        )
    except urllib.error.HTTPError as exc:
        body = exc.read().decode("utf-8", errors="replace")
        print(f"HTTPError: status={exc.code} body={body}", file=sys.stderr)
        sys.exit(1)
    except Exception as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
