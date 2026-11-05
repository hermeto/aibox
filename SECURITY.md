# Security policy

## Supported versions

AIBox is pre-1.0. Security fixes are made on the default branch and released in the next
version. Only the latest release receives fixes.

| Version | Supported |
| --- | --- |
| latest release / default branch | ✅ |
| older releases | ❌ |

## Reporting a vulnerability

**Please do not open a public issue for security problems.**

Report privately through GitHub's
[private vulnerability reporting](https://github.com/hermeto/aibox/security/advisories/new)
("Security" tab → "Report a vulnerability"). Include:

- the affected component (CLI, Terraform, platform manifests, workflows) and version/commit;
- steps to reproduce or a proof of concept;
- the impact you expect (e.g. credential exposure, cloud cost escalation, cluster takeover).

## Response targets

| Step | Target |
| --- | --- |
| Acknowledge the report | within 3 business days |
| Triage and severity assessment | within 7 days |
| Fix or mitigation for critical/high issues | within 30 days |
| Public advisory | after a fix is released, crediting the reporter unless they prefer otherwise |

AIBox is maintained by a single maintainer; these are best-effort targets.

## Scope notes

AIBox provisions cloud resources. Reports about ways to bypass the cost guardrails (budget,
TTL teardown, orphan detection) or to obtain cloud credentials from CI are in scope.
