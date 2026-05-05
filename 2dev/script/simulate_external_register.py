#!/usr/bin/env python3
import argparse
import hashlib
import json
import subprocess
import sys
import time
import urllib.error
import urllib.request


NO_PROXY_OPENER = urllib.request.build_opener(urllib.request.ProxyHandler({}))


def shell_quote_sql(value):
    return "'" + str(value).replace("'", "''") + "'"


def run_cmd(args, check=True):
    result = subprocess.run(args, text=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if check and result.returncode != 0:
        detail = (result.stderr or result.stdout or "").strip()
        raise RuntimeError(f"command failed ({result.returncode}): {' '.join(args)}\n{detail}")
    return result.stdout.strip()


def decode_response(raw):
    text = raw.decode("utf-8", errors="replace")
    try:
        return json.loads(text)
    except json.JSONDecodeError:
        return {"success": False, "message": text}


def http_json(method, url, headers=None, body=None, timeout=30):
    data = None
    if body is not None:
        data = json.dumps(body).encode("utf-8")
    req = urllib.request.Request(url, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    for key, value in (headers or {}).items():
        req.add_header(key, value)
    try:
        with NO_PROXY_OPENER.open(req, timeout=timeout) as resp:
            return resp.status, decode_response(resp.read())
    except urllib.error.HTTPError as exc:
        return exc.code, decode_response(exc.read())


class ExternalRegisterScenario:
    def __init__(self, args):
        self.args = args
        self.base_url = args.base_url.rstrip("/")
        namespace = hashlib.sha1(args.prefix.encode()).hexdigest()[:8]
        self.owner_username = f"er{namespace}o"
        self.invite_code = f"ER{namespace.upper()}"
        self.first_user = f"er{namespace}a"
        self.second_user = f"er{namespace}b"
        self.third_user = f"er{namespace}c"
        self.fourth_user = f"er{namespace}d"
        self.root_access_token = hashlib.sha256(f"{args.prefix}:root".encode()).hexdigest()[:32]
        self.root_user_id = None

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
                "-F",
                "\t",
                "-v",
                "ON_ERROR_STOP=1",
                "-c",
                sql,
            ]
        )

    def restart_app(self):
        run_cmd(["docker", "restart", self.args.app_container])
        self.wait_status()

    def wait_status(self):
        last = None
        for _ in range(30):
            try:
                _, payload = http_json("GET", f"{self.base_url}/api/status", timeout=5)
                last = payload
                if payload.get("success"):
                    return
            except Exception as exc:
                last = exc
            time.sleep(1)
        raise RuntimeError(f"dev app not healthy: {last}")

    def setup(self):
        print("=== setup dev data ===", flush=True)
        self.psql(
            f"""
            update users set access_token = null where access_token = {shell_quote_sql(self.root_access_token)};
            delete from tokens where user_id in (select id from users where username in (
              {shell_quote_sql(self.owner_username)},
              {shell_quote_sql(self.first_user)},
              {shell_quote_sql(self.second_user)},
              {shell_quote_sql(self.third_user)},
              {shell_quote_sql(self.fourth_user)}
            ));
            delete from logs where user_id in (select id from users where username in (
              {shell_quote_sql(self.owner_username)},
              {shell_quote_sql(self.first_user)},
              {shell_quote_sql(self.second_user)},
              {shell_quote_sql(self.third_user)},
              {shell_quote_sql(self.fourth_user)}
            ));
            delete from users where username in (
              {shell_quote_sql(self.owner_username)},
              {shell_quote_sql(self.first_user)},
              {shell_quote_sql(self.second_user)},
              {shell_quote_sql(self.third_user)},
              {shell_quote_sql(self.fourth_user)}
            );
            delete from invite_codes where code = {shell_quote_sql(self.invite_code)};
            insert into users (username, password, display_name, role, status, "group", aff_code, quota, used_quota, request_count)
            values ({shell_quote_sql(self.owner_username)}, 'external-register-test-owner', {shell_quote_sql(self.owner_username)}, 1, 1, 'default', {shell_quote_sql(self.args.prefix + "_aff")}, 0, 0, 0);
            insert into invite_codes (code, prefix, owner_user_id, target_group, reward_quota_per_use, reward_total_uses, reward_used_uses, status, created_time, updated_time)
            values (
              {shell_quote_sql(self.invite_code)},
              {shell_quote_sql(self.args.prefix.upper())},
              (select id from users where username = {shell_quote_sql(self.owner_username)}),
              'vip',
              3000,
              5,
              0,
              1,
              extract(epoch from now())::bigint,
              extract(epoch from now())::bigint
            );
            update users
            set access_token = {shell_quote_sql(self.root_access_token)}
            where id = (select id from users where role = 100 order by id asc limit 1);
            update options set value = 'false' where key = 'ExternalRegisterEnabled';
            update options set value = '' where key = 'ExternalRegisterAuthKey';
            """
        )
        root_user_id = self.psql(
            f"select id from users where access_token = {shell_quote_sql(self.root_access_token)} limit 1"
        )
        if not root_user_id:
            raise RuntimeError("root user with role=100 not found in dev database")
        self.root_user_id = int(root_user_id)
        self.restart_app()

    def cleanup(self):
        print("=== cleanup dev data ===", flush=True)
        self.psql(
            f"""
            delete from tokens where user_id in (select id from users where username in (
              {shell_quote_sql(self.owner_username)},
              {shell_quote_sql(self.first_user)},
              {shell_quote_sql(self.second_user)},
              {shell_quote_sql(self.third_user)},
              {shell_quote_sql(self.fourth_user)}
            ));
            delete from logs where user_id in (select id from users where username in (
              {shell_quote_sql(self.owner_username)},
              {shell_quote_sql(self.first_user)},
              {shell_quote_sql(self.second_user)},
              {shell_quote_sql(self.third_user)},
              {shell_quote_sql(self.fourth_user)}
            ));
            delete from users where username in (
              {shell_quote_sql(self.owner_username)},
              {shell_quote_sql(self.first_user)},
              {shell_quote_sql(self.second_user)},
              {shell_quote_sql(self.third_user)},
              {shell_quote_sql(self.fourth_user)}
            );
            delete from invite_codes where code = {shell_quote_sql(self.invite_code)};
            delete from options where key in ('ExternalRegisterEnabled', 'ExternalRegisterAuthKey');
            update users set access_token = null where access_token = {shell_quote_sql(self.root_access_token)};
            """
        )
        self.restart_app()

    def root_headers(self):
        if self.root_user_id is None:
            raise RuntimeError("root user id not initialized")
        return {
            "Authorization": f"Bearer {self.root_access_token}",
            "New-Api-User": str(self.root_user_id),
        }

    def root_auth_code(self, method):
        status_code, payload = http_json(
            method,
            f"{self.base_url}/api/option/external_register_auth_code",
            headers=self.root_headers(),
        )
        if status_code >= 400 or not payload.get("success"):
            raise RuntimeError(f"{method} auth code failed: status={status_code}, payload={payload}")
        return payload.get("data") or {}

    def generate_auth_key(self):
        data = self.root_auth_code("POST")
        auth_key = data.get("auth_key")
        auth_keys = data.get("auth_keys") or []
        if not data.get("enabled") or not data.get("configured") or not auth_key or auth_key not in auth_keys:
            raise AssertionError(f"generated auth code response invalid: {data}")
        return auth_key

    def delete_auth_key(self, auth_key):
        status_code, payload = http_json(
            "DELETE",
            f"{self.base_url}/api/option/external_register_auth_code",
            headers=self.root_headers(),
            body={"auth_key": auth_key},
        )
        if status_code >= 400 or not payload.get("success"):
            raise RuntimeError(f"DELETE one auth code failed: status={status_code}, payload={payload}")
        return payload.get("data") or {}

    def register(self, username, auth_key, invite_code=None):
        headers = {}
        if auth_key is not None:
            headers["Authorization"] = f"Bearer {auth_key}"
        body = {
            "username": username,
            "password": "password123",
        }
        if invite_code is not None:
            body["invite_code"] = invite_code
        return http_json(
            "POST",
            f"{self.base_url}/api/user/external_register",
            headers=headers,
            body=body,
        )[1]

    def fetch_user(self, username):
        rows = self.psql(
            f"""
            select u.username, u."group", u.inviter_id, u.invite_code_id, u.invite_code_owner_id, u.quota, coalesce(ic.code, '-')
            from users u
            left join invite_codes ic on ic.id = u.invite_code_id
            where u.username = {shell_quote_sql(username)}
            """
        )
        if not rows:
            return None
        username, group, inviter_id, invite_code_id, owner_id, quota, code = rows.split("\t")
        return {
            "username": username,
            "group": group,
            "inviter_id": int(inviter_id),
            "invite_code_id": int(invite_code_id),
            "invite_code_owner_id": int(owner_id),
            "quota": int(quota),
            "code": code,
        }

    def run(self):
        self.setup()
        try:
            print("=== generate auth key via root API ===", flush=True)
            initial_state = self.root_auth_code("GET")
            if initial_state.get("configured") or initial_state.get("enabled"):
                raise AssertionError(f"expected clean auth state: {initial_state}")
            old_key = self.generate_auth_key()

            print("=== auth rejection checks ===", flush=True)
            missing = self.register(self.first_user, None, self.invite_code)
            if missing.get("success"):
                raise AssertionError(f"missing auth unexpectedly succeeded: {missing}")
            wrong = self.register(self.first_user, "wrong-secret", self.invite_code)
            if wrong.get("success"):
                raise AssertionError(f"wrong auth unexpectedly succeeded: {wrong}")

            print("=== successful external register with invite code ===", flush=True)
            success = self.register(self.first_user, old_key, self.invite_code)
            if not success.get("success"):
                raise AssertionError(f"expected register success: {success}")
            user = self.fetch_user(self.first_user)
            if not user:
                raise AssertionError("registered user not found")
            if user["group"] != "vip" or user["code"] != self.invite_code:
                raise AssertionError(f"unexpected user binding: {user}")
            if user["inviter_id"] != user["invite_code_owner_id"]:
                raise AssertionError(f"inviter/owner mismatch: {user}")
            if user["quota"] < 3000:
                raise AssertionError(f"invite reward missing: {user}")

            print("=== multiple auth keys remain valid ===", flush=True)
            new_key = self.generate_auth_key()
            if new_key == old_key:
                raise AssertionError("regenerated auth key did not change")
            old_key_result = self.register(self.second_user, old_key)
            if not old_key_result.get("success"):
                raise AssertionError(f"old key should still succeed: {old_key_result}")
            old_key_user = self.fetch_user(self.second_user)
            if old_key_user["invite_code_id"] != 0 or old_key_user["invite_code_owner_id"] != 0:
                raise AssertionError(f"optional invite code should not bind invite fields: {old_key_user}")

            new_key_result = self.register(self.third_user, new_key, self.invite_code)
            if not new_key_result.get("success"):
                raise AssertionError(f"new key should succeed: {new_key_result}")

            print("=== delete one auth key keeps others valid ===", flush=True)
            delete_one_state = self.delete_auth_key(old_key)
            remaining_keys = delete_one_state.get("auth_keys") or []
            if old_key in remaining_keys or new_key not in remaining_keys:
                raise AssertionError(f"unexpected delete-one response: {delete_one_state}")
            deleted_key_result = self.register(self.fourth_user, old_key)
            if deleted_key_result.get("success"):
                raise AssertionError(f"deleted key unexpectedly succeeded: {deleted_key_result}")

            print("=== delete all auth keys disables endpoint ===", flush=True)
            deleted_state = self.root_auth_code("DELETE")
            if deleted_state.get("enabled") or deleted_state.get("configured") or deleted_state.get("auth_key"):
                raise AssertionError(f"unexpected delete response: {deleted_state}")
            disabled = self.register(self.fourth_user, new_key)
            if disabled.get("success"):
                raise AssertionError(f"disabled endpoint unexpectedly succeeded: {disabled}")

            print("external register simulation passed", flush=True)
        finally:
            if not self.args.keep_state:
                self.cleanup()


def parse_args():
    parser = argparse.ArgumentParser(description="Simulate external register against local docker dev.")
    parser.add_argument("--base-url", default="http://127.0.0.1:3001")
    parser.add_argument("--prefix", default=f"codex_extreg_{int(time.time())}")
    parser.add_argument("--app-container", default="new-api-dev")
    parser.add_argument("--postgres-container", default="postgres-dev")
    parser.add_argument("--postgres-user", default="root")
    parser.add_argument("--postgres-db", default="new-api")
    parser.add_argument("--keep-state", action="store_true")
    return parser.parse_args()


def main():
    args = parse_args()
    scenario = ExternalRegisterScenario(args)
    scenario.run()


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        sys.exit(1)
