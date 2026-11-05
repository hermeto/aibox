# Contract: cost guardrail commands

| Command | Behavior | Exit |
| --- | --- | --- |
| `aibox up --profile aws-* --budget 50 [--notify-email x] [--ttl 4h]` | refuses without budget/e-mail; applies guardrails, then main | 1 if missing inputs |
| `aibox ttl extend 2h --profile aws-*` | re-applies guardrails with expiry + 2 h (≤ start + 24 h) | 1 if no active run or over 24 h |
| `aibox ttl show --profile aws-*` | prints start, expiry, remaining | 1 if no active run |
| `aibox down --profile aws-*` | destroy main → destroy guardrails → orphan check | 1 if orphans remain |
| `aibox down --profile aws-* --orphans-only` | only the orphan check | 1 if orphans |
| `aibox cost --profile aws-*` | current run cost (text/JSON) | 1 if no active run |
