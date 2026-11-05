// Package profile loads and validates AIBox profiles (profiles/<name>.yaml) against the
// JSON Schema in schemas/profile.schema.json.
package profile

import "time"

// Provider identifies where a profile runs.
type Provider string

// Supported providers.
const (
	ProviderKind Provider = "kind"
	ProviderAWS  Provider = "aws"
)

// Capacity types for GPU classes.
const (
	CapacitySpot          = "spot"
	CapacityOnDemand      = "on-demand"
	CapacityCapacityBlock = "capacity-block"
)

// Profile is the typed form of a profile file.
type Profile struct {
	APIVersion string   `json:"apiVersion"`
	Kind       string   `json:"kind"`
	Metadata   Metadata `json:"metadata"`
	Spec       Spec     `json:"spec"`

	// Path is the file the profile was loaded from (not part of the file).
	Path string `json:"-"`
}

// Metadata identifies a profile.
type Metadata struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// Spec is the environment definition.
type Spec struct {
	Provider   Provider   `json:"provider"`
	Cluster    Cluster    `json:"cluster"`
	Local      *Local     `json:"local,omitempty"`
	AWS        *AWS       `json:"aws,omitempty"`
	Guardrails Guardrails `json:"guardrails"`
}

// Cluster holds Kubernetes cluster settings.
type Cluster struct {
	Name              string `json:"name"`
	KubernetesVersion string `json:"kubernetesVersion"`
}

// Local holds resource expectations for the kind provider.
type Local struct {
	MinCPUs              int     `json:"minCPUs"`
	MinMemoryGiB         float64 `json:"minMemoryGiB"`
	RecommendedMemoryGiB float64 `json:"recommendedMemoryGiB"`
}

// AWS holds settings for the aws provider.
type AWS struct {
	Region     string     `json:"region"`
	GPUClasses []GPUClass `json:"gpuClasses"`
}

// GPUClass is one GPU node pool definition.
type GPUClass struct {
	Name                  string   `json:"name"`
	InstanceTypes         []string `json:"instanceTypes"`
	CapacityType          string   `json:"capacityType"`
	MaxNodes              int      `json:"maxNodes"`
	CapacityReservationID string   `json:"capacityReservationId,omitempty"`
}

// Guardrails holds cost-safety defaults.
type Guardrails struct {
	TTL        string `json:"ttl"`
	OwnerEmail string `json:"ownerEmail,omitempty"`
}

// TTLDuration parses Guardrails.TTL.
func (g Guardrails) TTLDuration() (time.Duration, error) { return time.ParseDuration(g.TTL) }

// IsCloud reports whether the profile provisions cloud resources.
func (p *Profile) IsCloud() bool { return p.Spec.Provider == ProviderAWS }
