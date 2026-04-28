#!/usr/bin/env python3
import argparse
import json
import sys
import time
import uuid


def generate(cookies_json: str, prompt: str, timeout: int = 60, mode: str = "image") -> dict:
    from curl_cffi import requests

    cookies = json.loads(cookies_json)
    caz = cookies.get("caz", "")
    cb = cookies.get("cb", "")
    cau = cookies.get("cau", "")
    user_id = cookies.get("user_id", "")

    if not caz:
        return {"ok": False, "error": "missing caz cookie"}

    s = requests.Session(impersonate="chrome131")

    all_cookies = json.loads(cookies.get("all_cookies", "{}"))
    for name, value in all_cookies.items():
        s.cookies.set(name, value, domain=".canva.com")
    s.cookies.set("CAZ", caz, domain=".canva.com")
    s.cookies.set("CB", cb, domain=".canva.com")
    s.cookies.set("CAU", cau, domain=".canva.com")

    headers = {
        "Origin": "https://www.canva.com",
        "Referer": "https://www.canva.com/ai",
        "Content-Type": "application/json;charset=UTF-8",
        "x-canva-brand": cb,
        "x-canva-locale": "id-ID",
        "x-canva-accept-prefix": "no-prefix",
        "x-canva-active-user": cau,
        "x-canva-authz": caz,
        "x-canva-user": user_id,
        "x-canva-request": "createthread",
        "x-canva-app": "home",
    }

    actual_prompt = prompt.strip()
    if mode == "image":
        if not any(w in actual_prompt.lower() for w in ["generate", "create", "draw", "make", "gambar", "buat"]):
            actual_prompt = f"Generate an image of {actual_prompt}"
    elif mode == "design":
        if not any(w in actual_prompt.lower() for w in ["design", "poster", "banner", "konten", "template", "buat"]):
            actual_prompt = f"Buat konten media sosial yang menampilkan {actual_prompt}"

    keyword = actual_prompt[:50]

    body = {
        "A": uuid.uuid4().hex[:26].upper(),
        "B": [{"A?": "A", "A": actual_prompt, "L": keyword}],
        "C": str(uuid.uuid4()),
        "D": {"D": "D", "G": {"A?": "E"}, "H": "C", "J": "UTC"},
        "A?": "G",
    }

    resp = s.post("https://www.canva.com/_ajax/assistant/threads", headers=headers, json=body)
    if resp.status_code == 403:
        return {"ok": False, "error": "forbidden — cookies expired or invalid"}
    if resp.status_code != 200:
        return {"ok": False, "error": f"create thread failed: {resp.status_code} {resp.text[:200]}"}

    data = resp.json()
    thread_id = data.get("A", "")
    if not thread_id:
        return {"ok": False, "error": "no thread_id in response"}

    headers["x-canva-request"] = "getthread"
    msg_seq = 0
    frag_offset = 0
    images = []
    code_parts = []
    text_parts = []
    codelet_ids = set()
    seen_urls = set()
    deadline = time.time() + timeout

    while time.time() < deadline:
        time.sleep(2)
        r = s.get(
            f"https://www.canva.com/_ajax/assistant/threads/{thread_id}"
            f"?afterMessageSeq={msg_seq}&updateFragmentsOffset={frag_offset}&withThumbnail=true",
            headers=headers,
        )
        if r.status_code != 200:
            continue

        d = r.json()
        state = d.get("e", {}).get("A", "")

        for f in d.get("f", []):
            ftype = f.get("A?")
            if ftype == "B":
                text_parts.append(f.get("V", ""))
            elif ftype == "D":
                tool = f.get("P", "")
                u = f.get("U", {})
                if tool == "generateCodelet":
                    chunk = u.get("A", "")
                    if chunk and u.get("A?") == "L":
                        code_parts.append(chunk)
                    elif u.get("A?") == "A":
                        text_parts.append(chunk)
            elif ftype == "L":
                v = f.get("U", {}).get("V", {})
                if isinstance(v, dict):
                    for _, val in v.items():
                        if isinstance(val, dict):
                            url = val.get("A", "")
                            if url and url not in seen_urls:
                                seen_urls.add(url)
                                images.append(url)

        frag_offset += len(d.get("f", []))

        for m in d.get("A", []):
            msg_seq = max(msg_seq, m.get("B", 0))
            q = m.get("A", {}).get("Q", {})
            if q.get("A?") == "C":
                final_images = []
                for img in q.get("Q", []):
                    full = img.get("K", "")
                    if full and "image-resize" in full and full not in seen_urls:
                        seen_urls.add(full)
                        final_images.append(full)
                    thumb = img.get("L", "")
                    if not full and thumb and "image-resize" in thumb and thumb not in seen_urls:
                        seen_urls.add(thumb)
                        final_images.append(thumb)
                if final_images:
                    images = final_images

        if d.get("F", 0) > msg_seq:
            msg_seq = d["F"]

        if state == "E" or d.get("A?") == "C":
            break

    codelet_urls = []
    if code_parts:
        try:
            embed_headers = dict(headers)
            embed_headers["x-canva-request"] = "getcodelet"
            r = s.get(
                f"https://www.canva.com/_ajax/assistant/threads/{thread_id}?afterMessageSeq=0&updateFragmentsOffset=0&withThumbnail=true",
                headers=embed_headers,
            )
            if r.status_code == 200:
                full_data = r.json()
                for m in full_data.get("A", []):
                    q = m.get("A", {}).get("Q", {})
                    text = q.get("Q", {})
                    if isinstance(text, dict):
                        a_val = text.get("A", "")
                        if "features.json" in a_val:
                            import re
                            ids = re.findall(r'```features\.json', a_val)
                            for cid_match in re.finditer(r'/embed/codelets/([a-z0-9]+)', str(full_data)):
                                codelet_ids.add(cid_match.group(1))
        except Exception:
            pass

        for cid in codelet_ids:
            try:
                r = s.get(
                    f"https://www.canva.com/_ajax/embed/codelets/{cid}?includeEmbed&includeFileUrls",
                    headers=headers,
                )
                if r.status_code == 200:
                    cd = r.json()
                    hosted_url = cd.get("url", "")
                    if hosted_url:
                        codelet_urls.append(hosted_url)
                    for f in cd.get("files", []):
                        if f.get("indexDocument"):
                            codelet_urls.append(f.get("url", ""))
            except Exception:
                pass

    quota_used = None
    quota_limit = None
    try:
        headers["x-canva-request"] = "getquota"
        qr = s.post(
            "https://www.canva.com/_ajax/quota/quota/get",
            headers=headers,
            json={"A": "C", "B": cb, "C": user_id},
        )
        if qr.status_code == 200:
            q = qr.json().get("A", {})
            raw_used = q.get("C")
            raw_limit = q.get("D")
            if isinstance(raw_used, (int, float)) and isinstance(raw_limit, (int, float)) and raw_limit > 0:
                quota_used = int(raw_used)
                quota_limit = int(raw_limit)
    except Exception:
        pass

    return {
        "ok": True,
        "images": images,
        "code": "".join(code_parts) if code_parts else None,
        "codelet_urls": codelet_urls if codelet_urls else None,
        "text": "".join(text_parts),
        "quota_used": quota_used,
        "quota_limit": quota_limit,
    }


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--cookies", required=True)
    parser.add_argument("--prompt", required=True)
    parser.add_argument("--timeout", type=int, default=60)
    parser.add_argument("--mode", default="image", choices=["image", "code", "design"])
    args = parser.parse_args()

    result = generate(args.cookies, args.prompt, args.timeout, args.mode)
    print(json.dumps(result))
    sys.exit(0 if result.get("ok") else 1)
