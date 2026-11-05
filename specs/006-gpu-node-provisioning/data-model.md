# Data model: 006 gpu-node-provisioning

`GPUClass` (profile) → `EC2NodeClass` `aibox-<class>` (alias, role `<cluster>-karpenter-node`,
discovery tag `karpenter.sh/discovery=<cluster>`, RAID0, IMDSv2, optional reservation) +
`NodePool` `<class>` (labels `aibox.dev/gpu-class`, taint `nvidia.com/gpu=present:NoSchedule`,
requirements: instance types, capacity type, arch amd64; limits; disruption WhenEmpty/10m;
`expireAfter: 168h`).
