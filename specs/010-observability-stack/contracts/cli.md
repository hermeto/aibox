# Contract: `aibox render rules`

- `aibox render rules --profile <p>` prints the PrometheusRule with the canonical runtime
  series, cost rules for the profile's region and SLO alerts for its catalog models;
  `make generate` writes `platform/observability/overlays/<p>/generated-rules.yaml`.
