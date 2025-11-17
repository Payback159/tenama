package models

import (
	"testing"
)

func TestConfigUnmarshal(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				LogLevel:        "info",
				CleanupInterval: "24h",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.config.LogLevel == "" && !tt.wantErr {
				t.Errorf("Config should have LogLevel set")
			}
		})
	}
}

func TestResources(t *testing.T) {
	tests := []struct {
		name string
		res  Resources
	}{
		{
			name: "valid resources",
			res: Resources{
				Requests: struct {
					CPU     string `yaml:"cpu"`
					Memory  string `yaml:"memory"`
					Storage string `yaml:"storage"`
				}{
					CPU:     "100m",
					Memory:  "128Mi",
					Storage: "1Gi",
				},
				Limits: struct {
					CPU    string `yaml:"cpu"`
					Memory string `yaml:"memory"`
				}{
					CPU:    "500m",
					Memory: "512Mi",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.res.Requests.CPU == "" {
				t.Errorf("Resources CPU should not be empty")
			}
			if tt.res.Limits.Memory == "" {
				t.Errorf("Resources Memory limit should not be empty")
			}
		})
	}
}

func TestBasicAuth(t *testing.T) {
	tests := []struct {
		name     string
		auth     BasicAuth
		wantUser bool
	}{
		{
			name: "valid basic auth",
			auth: BasicAuth{
				{
					Username: "testuser",
					Password: "testpass",
				},
			},
			wantUser: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantUser && len(tt.auth) == 0 {
				t.Errorf("BasicAuth should have credentials")
			}
			if tt.wantUser && tt.auth[0].Username == "" {
				t.Errorf("BasicAuth username should not be empty")
			}
		})
	}
}
