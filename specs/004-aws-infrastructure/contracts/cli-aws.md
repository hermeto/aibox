# Contract: `aibox up|down --profile aws-*`

| Flag | Default | Meaning |
| --- | --- | --- |
| `--yes` | false | apply/destroy without the interactive confirmation |
| `--dry-run` | false | print every command; no AWS call beyond doctor's read-only checks |
| `--admin-cidr` | operator egress IP /32 | CIDR allowed to reach the EKS public endpoint (repeatable) |
| `--owner` | profile `ownerEmail` → `git config user.email` | `owner` tag |
| `--skip-doctor` | false | skip preflight checks |

`up`: doctor → state bucket check → render inputs → init → plan → confirm → apply → outputs →
kubeconfig context `aibox-<profile>` → Argo CD + root Application → wait. Exit 1 on any failure
(nothing applied if doctor fails). `down`: confirm → destroy → remove run state.
