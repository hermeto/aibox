# Data model: 010

- **Canonical series** (recording rules): `aibox:<canonical name>` with labels `model`,
  `namespace`; histograms keep `le`.
- **Cost series**: `aibox:instance_price_usd_per_hour{instance_type,capacity_type}`,
  `aibox:pool_cost_usd_per_hour{pool}`, `aibox:usd_per_million_output_tokens{model}`.
- **Alerts**: `AIBoxTTFTSLOViolation{model}`, `AIBoxKVCacheSaturated`, `AIBoxPodsPending`,
  `AIBoxGPUNodeIdle`, `AIBoxPriceMissing`.
