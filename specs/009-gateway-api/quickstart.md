# Quickstart: 009

```bash
go run ./cmd/aibox up --profile local
KEY=$(cat .aibox/local/api-key)
curl -s http://127.0.0.1:30080/v1/chat/completions -H "Authorization: Bearer $KEY" \
  -H 'Content-Type: application/json' -d '{"model":"sim-llama-8b","messages":[{"role":"user","content":"Hi"}]}'
go run ./cmd/aibox smoke --profile local --output json | jq '.checks[] | select(.status!="pass")'
```
