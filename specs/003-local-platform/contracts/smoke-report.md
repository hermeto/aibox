# Contract: `aibox smoke` report

```json
{
  "profile": "local",
  "endpoint": "http://127.0.0.1:30080/v1",
  "checks": [
    { "name": "models/list", "status": "pass", "message": "sim-llama-8b listed" },
    { "name": "sim-llama-8b/chat-completion", "status": "pass", "message": "200, 1 choice, 12 completion tokens" },
    { "name": "metrics/required-series", "status": "pass", "message": "4/4 series present" }
  ],
  "ok": true
}
```

- Every check has `status` `pass` or `fail`; `ok` is false iff any check fails; exit code 1
  when not ok.
- Later features add checks (gateway auth, traces, dashboards) without changing the shape.
- `--endpoint` overrides the base URL; `--model` restricts to one model.
