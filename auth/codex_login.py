#!/usr/bin/env python3

import asyncio
import base64
import hashlib
import json
import os
import secrets
import sys
from http.server import HTTPServer, BaseHTTPRequestHandler
from threading import Thread
from urllib.parse import urlencode, parse_qs, urlparse

import aiohttp

CODEX_CLIENT_ID = "app_EMoamEEZ73f0CkXaXp7hrann"
CODEX_AUTHORIZE_URL = "https://auth.openai.com/oauth/authorize"
CODEX_TOKEN_URL = "https://auth.openai.com/oauth/token"
CODEX_USAGE_URL = "https://chatgpt.com/backend-api/wham/usage"
CALLBACK_PORT = 1455
REDIRECT_URI = f"http://localhost:{CALLBACK_PORT}/auth/callback"
SCOPE = "openid profile email offline_access"


def emit(data: dict):
    try:
        print(json.dumps(data), flush=True)
    except BrokenPipeError:
        pass


def generate_pkce_pair():
    verifier = secrets.token_urlsafe(64)
    digest = hashlib.sha256(verifier.encode("ascii")).digest()
    challenge = base64.urlsafe_b64encode(digest).rstrip(b"=").decode("ascii")
    return verifier, challenge


class CallbackHandler(BaseHTTPRequestHandler):
    result = None

    def do_GET(self):
        if not self.path.startswith("/auth/callback"):
            self.send_response(404)
            self.end_headers()
            return

        params = parse_qs(urlparse(self.path).query)
        code = params.get("code", [None])[0]
        error = params.get("error", [None])[0]

        if error:
            CallbackHandler.result = {"error": f"{error}: {params.get('error_description', [''])[0]}"}
        elif code:
            CallbackHandler.result = {"code": code}
        else:
            CallbackHandler.result = {"error": "no code received"}

        self.send_response(200)
        self.send_header("Content-Type", "text/html")
        self.end_headers()

        ok = code is not None
        color = "#10b981" if ok else "#ef4444"
        icon = "&#10004;" if ok else "&#10060;"
        msg = "Login successful! You can close this tab." if ok else f"Login failed: {CallbackHandler.result.get('error', '')}"
        html = f"""<!DOCTYPE html>
<html><head><title>enowx-ai</title>
<style>body{{font-family:system-ui;display:flex;align-items:center;justify-content:center;min-height:100vh;margin:0;background:#0a0a0a;color:#e5e5e5}}
.c{{text-align:center;padding:3rem;border-radius:1rem;border:1px solid #262626;background:#171717;max-width:400px}}
.i{{font-size:3rem;margin-bottom:1rem}}.m{{font-size:1.1rem;color:{color}}}.h{{margin-top:1rem;font-size:.85rem;color:#737373}}</style></head>
<body><div class="c"><div class="i">{icon}</div><p class="m">{msg}</p><p class="h">You can close this tab now.</p></div></body></html>"""
        self.wfile.write(html.encode())

    def log_message(self, format, *args):
        pass


async def exchange_code(code: str, verifier: str) -> dict:
    form = {
        "grant_type": "authorization_code",
        "code": code,
        "redirect_uri": REDIRECT_URI,
        "client_id": CODEX_CLIENT_ID,
        "code_verifier": verifier,
    }
    async with aiohttp.ClientSession() as session:
        async with session.post(CODEX_TOKEN_URL, data=form) as resp:
            body = await resp.text()
            if resp.status != 200:
                return {"error": f"token exchange failed ({resp.status}): {body[:200]}"}
            return json.loads(body)


async def fetch_usage(access_token: str) -> dict | None:
    try:
        async with aiohttp.ClientSession() as session:
            headers = {
                "Authorization": f"Bearer {access_token}",
                "User-Agent": "Mozilla/5.0",
            }
            async with session.get(CODEX_USAGE_URL, headers=headers, timeout=aiohttp.ClientTimeout(total=15)) as resp:
                if resp.status != 200:
                    return None
                return await resp.json()
    except Exception:
        return None


async def main():
    headless = os.getenv("CODEX_HEADLESS", "false").lower() == "true"

    emit({"type": "progress", "step": "starting", "message": "Starting Codex login..."})

    verifier, challenge = generate_pkce_pair()
    state = secrets.token_urlsafe(32)

    authorize_url = CODEX_AUTHORIZE_URL + "?" + urlencode({
        "client_id": CODEX_CLIENT_ID,
        "redirect_uri": REDIRECT_URI,
        "response_type": "code",
        "scope": SCOPE,
        "state": state,
        "code_challenge": challenge,
        "code_challenge_method": "S256",
        "id_token_add_organizations": "true",
        "codex_cli_simplified_flow": "true",
        "originator": "codex_cli_rs",
    })

    CallbackHandler.result = None
    server = HTTPServer(("127.0.0.1", CALLBACK_PORT), CallbackHandler)
    server_thread = Thread(target=server.serve_forever, daemon=True)
    server_thread.start()

    emit({"type": "progress", "step": "browser_launch", "message": "Launching browser..."})

    try:
        from browserforge.fingerprints import Screen
        from camoufox.async_api import AsyncCamoufox

        camoufox_kwargs = {
            "headless": headless,
            "os": "windows",
            "block_webrtc": True,
            "humanize": False,
            "screen": Screen(max_width=1920, max_height=1080),
        }

        proxy_url = os.getenv("CODEX_PROXY_URL", "")
        if proxy_url:
            parsed = urlparse(proxy_url)
            proxy_cfg = {"server": f"{parsed.scheme}://{parsed.hostname}:{parsed.port}"}
            if parsed.username:
                proxy_cfg["username"] = parsed.username
            if parsed.password:
                proxy_cfg["password"] = parsed.password
            camoufox_kwargs["proxy"] = proxy_cfg
            camoufox_kwargs["geoip"] = True

        async with AsyncCamoufox(**camoufox_kwargs) as browser:
            page = await browser.new_page()
            page.set_default_timeout(120000)

            browser_closed = False

            def on_disconnected():
                nonlocal browser_closed
                browser_closed = True

            browser.on("disconnected", on_disconnected)

            await page.goto(authorize_url, wait_until="domcontentloaded", timeout=30000)

            emit({"type": "progress", "step": "waiting_login", "message": "Waiting for login in browser..."})

            for _ in range(300):
                if CallbackHandler.result is not None:
                    break
                if browser_closed:
                    break
                try:
                    if page.is_closed():
                        break
                except Exception:
                    break
                await asyncio.sleep(1)

            if browser_closed or (CallbackHandler.result is None and page.is_closed()):
                emit({"type": "error", "error": "cancelled (browser closed)"})
                server.shutdown()
                sys.exit(1)

            if CallbackHandler.result is None:
                emit({"type": "error", "error": "timeout waiting for login (5 minutes)"})
                server.shutdown()
                sys.exit(1)

    except Exception as exc:
        if "browser closed" in str(exc).lower() or "target closed" in str(exc).lower():
            emit({"type": "error", "error": "cancelled (browser closed)"})
        else:
            emit({"type": "error", "error": f"browser failed: {exc}"})
        server.shutdown()
        sys.exit(1)

    server.shutdown()

    result = CallbackHandler.result
    if "error" in result:
        emit({"type": "error", "error": result["error"]})
        sys.exit(1)

    emit({"type": "progress", "step": "exchanging", "message": "Exchanging authorization code..."})

    token_result = await exchange_code(result["code"], verifier)
    if "error" in token_result:
        emit({"type": "error", "error": token_result["error"]})
        sys.exit(1)

    access_token = token_result.get("access_token", "")
    refresh_token = token_result.get("refresh_token", "")
    id_token = token_result.get("id_token", "")

    email = ""
    used_percent = 0.0

    if id_token:
        try:
            parts = id_token.split(".")
            payload = json.loads(base64.urlsafe_b64decode(parts[1] + "=="))
            email = payload.get("email", "")
        except Exception:
            pass

    emit({"type": "progress", "step": "fetching_usage", "message": "Fetching usage info..."})

    usage = await fetch_usage(access_token)
    if usage:
        if not email:
            email = usage.get("email", "")
        rate_limit = usage.get("rate_limit", {})
        primary = rate_limit.get("primary_window", {})
        used_percent = primary.get("used_percent", 0.0)

    emit({
        "type": "result",
        "success": True,
        "email": email or "codex-user",
        "access_token": access_token,
        "refresh_token": refresh_token,
        "id_token": id_token,
        "used_percent": used_percent,
    })


if __name__ == "__main__":
    asyncio.run(main())
