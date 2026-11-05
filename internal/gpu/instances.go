// Package gpu renders Karpenter EC2NodeClasses and NodePools from a profile's GPU classes.
package gpu

// InstanceSpec is the capacity of an instance type used to compute pool limits.
type InstanceSpec struct {
	GPUs  int
	VCPUs int
}

// Instances lists the GPU instance types AIBox profiles may use (AWS instance type specs).
var Instances = map[string]InstanceSpec{
	"g6.xlarge":   {GPUs: 1, VCPUs: 4},
	"g6.2xlarge":  {GPUs: 1, VCPUs: 8},
	"g6.4xlarge":  {GPUs: 1, VCPUs: 16},
	"g6.12xlarge": {GPUs: 4, VCPUs: 48},
	"g5.xlarge":   {GPUs: 1, VCPUs: 4},
	"g5.2xlarge":  {GPUs: 1, VCPUs: 8},
	"g5.12xlarge": {GPUs: 4, VCPUs: 48},
	"p5.48xlarge": {GPUs: 8, VCPUs: 192},
}

// Pinned node settings (see specs/006-gpu-node-provisioning/research.md).
const (
	AMIAlias                = "bottlerocket@v1.68.1"
	DefaultConsolidateAfter = "10m"
	ExpireAfter             = "168h"
	ClassLabel              = "aibox.dev/gpu-class"
	GPUTaintKey             = "nvidia.com/gpu"
	GPUTaintValue           = "present"
)
