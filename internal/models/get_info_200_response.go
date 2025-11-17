package models

type GetInfo200Response struct {
	BuildDate string `json:"buildDate,omitempty"`

	Commit string `json:"commit,omitempty"`

	Version string `json:"version,omitempty"`

	GlobalLimits *GlobalLimitsStatus `json:"globalLimits,omitempty"`
}

type GlobalLimitsStatus struct {
	Enabled      bool              `json:"enabled"`
	CurrentUsage map[string]string `json:"currentUsage"`
	Limits       map[string]string `json:"limits"`
}
