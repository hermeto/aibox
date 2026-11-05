# Requirements Quality Checklist: 008

- [x] CHK001 Every catalog runtime field maps to a documented runtime flag for vLLM and SGLang? [FR-001, FR-006, research]
- [x] CHK002 Readiness semantics (load + warm-up) and the startup budget are explicit? [FR-003, Clarifications]
- [x] CHK003 Drain behaviour and the grace period knob are defined? [FR-004]
- [x] CHK004 Placement rules and the TP > GPUs failure are defined? [FR-002, Edge Cases]
- [x] CHK005 Each loading method has a defined path to the runtime? [Clarifications, 007 FR]
- [x] CHK006 Metric contract names are tied to pinned versions and a mapping file? [FR-007, SC-002]
- [x] CHK007 Local testability through the same templates? [FR-008, SC-001]
- [x] CHK008 No account data in committed manifests? [Clarifications, constitution II]
