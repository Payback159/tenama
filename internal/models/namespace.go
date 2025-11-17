package models

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type Namespace struct {
	Infix string `json:"infix,omitempty"`

	Suffix string `json:"suffix,omitempty"`

	// How long should the namespace be preserved until it becomes obsolete and is automatically cleaned up.
	Duration string `json:"duration,omitempty"`

	// A list of users to be authorized as editors in this namespace.
	Users []string `json:"users,omitempty"`

	// Optional: Resource requests for this namespace (cpu, memory, storage)
	Resources *ResourceRequest `json:"resources,omitempty"`
}

// ResourceRequest defines requested and limited resources for a namespace
type ResourceRequest struct {
	CPU     string `json:"cpu,omitempty"`
	Memory  string `json:"memory,omitempty"`
	Storage string `json:"storage,omitempty"`
}

// MarshalToResourceList converts ResourceRequest to v1.ResourceList
func (r *ResourceRequest) MarshalToResourceList() (v1.ResourceList, error) {
	if r == nil {
		return v1.ResourceList{}, nil
	}
	rl := v1.ResourceList{}

	if r.CPU != "" {
		quantity, err := resource.ParseQuantity(r.CPU)
		if err != nil {
			return nil, err
		}
		rl[v1.ResourceCPU] = quantity
	}

	if r.Memory != "" {
		quantity, err := resource.ParseQuantity(r.Memory)
		if err != nil {
			return nil, err
		}
		rl[v1.ResourceMemory] = quantity
	}

	if r.Storage != "" {
		quantity, err := resource.ParseQuantity(r.Storage)
		if err != nil {
			return nil, err
		}
		rl[v1.ResourceStorage] = quantity
	}

	return rl, nil
}
