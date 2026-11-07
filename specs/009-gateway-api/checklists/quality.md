# Requirements Quality Checklist: 009

- [x] CHK001 Every endpoint and error status (401, 404, 413, 429, 503) is specified? [FR-001…FR-004, Edge Cases, contracts]
- [x] CHK002 Key lifecycle defined without secrets in git? [FR-003, Clarifications]
- [x] CHK003 Both limit dimensions (requests, tokens) and their source are defined? [FR-004, Clarifications]
- [x] CHK004 Baseline routing switchable without redeploying servers? [FR-005]
- [x] CHK005 TLS and exposure rules per profile? [FR-006]
- [x] CHK006 Streaming and client disconnect behaviour? [FR-007, Edge Cases]
- [x] CHK007 Local testability of every story in CI? [SC-001]
