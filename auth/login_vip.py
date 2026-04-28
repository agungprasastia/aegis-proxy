#!/usr/bin/env python3
import argparse
import asyncio
import json
import sys
import os

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from app.providers.kiro import KiroProviderAdapter
from app.providers.codebuddy_vip import CodeBuddyVipProviderAdapter
from app.providers.base import NormalizedAccount
from app.errors.exceptions import BatcherError, RetryableBatcherError

MAX_RETRIES = 3
BASE_DELAY = 2.0
MAX_DELAY = 15.0


def emit(data: dict):
    try:
        print(json.dumps(data), flush=True)
    except BrokenPipeError:
        pass


def retry_delay(attempt: int) -> float:
    return min(BASE_DELAY * (2**attempt), MAX_DELAY)


async def _run_provider_once(adapter, account: NormalizedAccount) -> dict:
    provider_name = adapter.name
    session = None
    try:
        session = await adapter.bootstrap_session(account)
        emit({"type": "progress", "provider": provider_name, "step": "browser_launch", "message": "Browser session ready"})
        auth_state = await adapter.authenticate(account, session)
        emit({"type": "progress", "provider": provider_name, "step": "authenticated", "message": "Authenticated"})
        tokens = await adapter.fetch_tokens(account, auth_state, session)
        emit({"type": "progress", "provider": provider_name, "step": "tokens", "message": "Tokens obtained"})
        quota = None
        try:
            quota = await adapter.fetch_quota(account, tokens, session)
            emit({"type": "progress", "provider": provider_name, "step": "quota", "message": "Quota fetched"})
        except Exception as e:
            emit({"type": "progress", "provider": provider_name, "step": "quota_skip", "message": f"Quota fetch skipped: {e}"})
        return {"success": True, "provider": provider_name, "credentials": tokens, "quota": quota}
    finally:
        if session is not None:
            try:
                await adapter.cleanup_session(session)
            except Exception:
                pass


async def run_provider(adapter, account: NormalizedAccount) -> dict:
    provider_name = adapter.name
    last_error = None
    emit({"type": "progress", "provider": provider_name, "step": "init", "message": "Initializing..."})
    for attempt in range(MAX_RETRIES):
        try:
            return await _run_provider_once(adapter, account)
        except RetryableBatcherError as e:
            last_error = e
            if attempt < MAX_RETRIES - 1:
                delay = retry_delay(attempt)
                emit({"type": "progress", "provider": provider_name, "step": "retry", "message": f"Retryable error: {e.message} -- retrying in {delay:.0f}s (attempt {attempt + 2}/{MAX_RETRIES})"})
                await asyncio.sleep(delay)
            else:
                emit({"type": "error", "provider": provider_name, "error": e.message, "code": e.code.value})
                return {"success": False, "provider": provider_name, "error": e.message}
        except BatcherError as e:
            emit({"type": "error", "provider": provider_name, "error": e.message, "code": e.code.value})
            return {"success": False, "provider": provider_name, "error": e.message}
        except Exception as e:
            last_error = e
            if attempt < MAX_RETRIES - 1:
                delay = retry_delay(attempt)
                emit({"type": "progress", "provider": provider_name, "step": "retry", "message": f"Error: {e} -- retrying in {delay:.0f}s (attempt {attempt + 2}/{MAX_RETRIES})"})
                await asyncio.sleep(delay)
            else:
                emit({"type": "error", "provider": provider_name, "error": str(e)})
                return {"success": False, "provider": provider_name, "error": str(e)}
    emit({"type": "error", "provider": provider_name, "error": str(last_error)})
    return {"success": False, "provider": provider_name, "error": str(last_error)}


async def main(email: str, password: str):
    emit({"type": "progress", "provider": "all", "step": "start", "message": f"Starting VIP login for {email}..."})
    concurrent = int(os.getenv("BATCHER_CONCURRENT", "2"))
    priority = os.getenv("BATCHER_PRIORITY", "standard").lower()
    kiro_account = NormalizedAccount(provider="kiro", identifier=email, secret=password)
    cb_account = NormalizedAccount(provider="codebuddy", identifier=email, secret=password)
    kiro_adapter = KiroProviderAdapter()
    cb_adapter = CodeBuddyVipProviderAdapter()
    if concurrent == 1:
        if priority == "max":
            cb_result = await run_provider(cb_adapter, cb_account)
            if isinstance(cb_result, BaseException):
                cb_result = {"success": False, "provider": "codebuddy", "error": str(cb_result)}
            result = {"type": "result", "kiro": {"success": False, "provider": "kiro", "error": "skipped (priority=max)"}, "codebuddy": cb_result}
        else:
            kiro_result = await run_provider(kiro_adapter, kiro_account)
            if isinstance(kiro_result, BaseException):
                kiro_result = {"success": False, "provider": "kiro", "error": str(kiro_result)}
            result = {"type": "result", "kiro": kiro_result, "codebuddy": {"success": False, "provider": "codebuddy", "error": "skipped (priority=standard)"}}
        emit(result)
        return
    kiro_result, cb_result = await asyncio.gather(
        run_provider(kiro_adapter, kiro_account),
        run_provider(cb_adapter, cb_account),
        return_exceptions=True,
    )
    if isinstance(kiro_result, BaseException):
        kiro_result = {"success": False, "provider": "kiro", "error": str(kiro_result)}
    if isinstance(cb_result, BaseException):
        cb_result = {"success": False, "provider": "codebuddy", "error": str(cb_result)}
    emit({"type": "result", "kiro": kiro_result, "codebuddy": cb_result})


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--email", required=True)
    parser.add_argument("--password", required=True)
    args = parser.parse_args()
    asyncio.run(main(args.email, args.password))
