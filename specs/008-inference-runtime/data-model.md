# Data model: 008

- **Catalog additions** (`schemas/catalog.schema.json`): optional `shutdownGracePeriodSeconds`
  (30–3600, default 120) and `startupTimeoutSeconds` (60–7200, default 1800).
- **Rendered LLMInferenceService** (`aibox-serving`): name = model id (dots → dashes), labels
  `aibox.dev/model`, `aibox.dev/runtime`; `spec.model.{uri,name}`, `storageInitializer.enabled:
  false`, `replicas: 1`, `parallelism.tensor`, `template` (container `main`: image, command,
  args, env, resources, probes, `postStart`; node selector; tolerations; grace period).
- **`aibox-run` ConfigMap** (`aibox-serving`): `runId`, `weightsBucket`, `region`.
- **Metric mapping** (`metrics/runtime-metrics.yaml`): `metrics[]` of `{name, type, help,
  vllm, sglang}`.
