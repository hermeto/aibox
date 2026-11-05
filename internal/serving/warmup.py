# postStart hook of AIBox model servers: wait until the server is healthy, then send one
# 1-token completion. Kubernetes starts no probes before the hook returns, so the server
# becomes ready only after its weights are loaded and a request succeeded.
import json, os, sys, time, urllib.request  # noqa: E401

base = "http://127.0.0.1:8000"
deadline = time.time() + float(os.environ.get("AIBOX_STARTUP_TIMEOUT", "1800"))
while time.time() < deadline:
    try:
        with urllib.request.urlopen(base + "/health", timeout=5) as r:
            if r.status == 200:
                break
    except Exception:
        pass
    time.sleep(5)
else:
    sys.exit("model server not healthy before the startup timeout")
body = json.dumps({"model": os.environ["AIBOX_MODEL_ID"], "prompt": "Hello", "max_tokens": 1}).encode()
req = urllib.request.Request(base + "/v1/completions", data=body, headers={"Content-Type": "application/json"})
with urllib.request.urlopen(req, timeout=300) as r:
    if r.status != 200:
        sys.exit(f"warm-up request failed: HTTP {r.status}")
