# Contract: Terraform root `infra/terraform/aws`

- Inputs and outputs as in [data-model.md](../data-model.md); every output consumed by the
  CLI is stable (renames need a CLI change in the same PR).
- Provider `default_tags`: `project=aibox`, `run-id`, `owner`, `profile`.
- Backend: `s3` with partial config (`bucket`, `key`, `region` from the CLI), `use_lockfile = true`.
- Commands the CLI runs (engine = `tofu` or `terraform`), all with `-chdir=infra/terraform/aws`:
  1. `init -input=false -reconfigure -backend-config=bucket=… -backend-config=key=… -backend-config=region=…`
  2. `plan -input=false -var-file=<abs>/.aibox/<p>/terraform.tfvars.json -out=<abs>/.aibox/<p>/plan.tfplan`
  3. `apply -input=false <plan>` (after `--yes` or interactive confirmation)
  4. `output -json` → `.aibox/<p>/outputs.json`
  5. `down`: `destroy -input=false -auto-approve -var-file=…` (after `--yes` or confirmation)
