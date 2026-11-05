# Contract: `aibox` CLI

## Global flags

| Flag | Default | Meaning |
| --- | --- | --- |
| `--profile` | `local` | profile name (`profiles/<name>.yaml`) or path to a YAML file |
| `--output`, `-o` | `text` | `text` or `json`; JSON goes to stdout only |
| `--verbose`, `-v` | false | debug logs (slog) on stderr |
| `--dry-run` | false | print external commands instead of running them |
| `--yes`, `-y` | false | assume "yes" for confirmations |

## Commands (feature 002 state)

| Command | State | Delivered by |
| --- | --- | --- |
| `up` | not implemented (exit 2) | 003 (local), 004 (aws) |
| `down` | not implemented (exit 2) | 003, 004 |
| `models` | not implemented (exit 2) | 007 |
| `smoke` | not implemented (exit 2) | 003 |
| `bench` | not implemented (exit 2) | 013 |
| `report` | not implemented (exit 2) | 014 |
| `cost` | not implemented (exit 2) | 005 |
| `doctor` | implemented | 002 |
| `version` | implemented | 002 |

Not-implemented message (stderr; JSON mode adds `{"error":…,"code":2}` on stdout):
`aibox <cmd>: not implemented yet (planned in feature NNN) — see https://github.com/hermeto/aibox/issues?q="v1.0 roadmap"`

## Exit codes

| Code | Meaning |
| --- | --- |
| 0 | success |
| 1 | failure (doctor fail, invalid profile, runtime error) |
| 2 | command not implemented yet |
| 64 | usage error (unknown command/flag, bad flag value) |
