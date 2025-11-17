package handlers

import (
	"testing"
)

func TestContainerInitialization(t *testing.T) {
	tests := []struct {
		name string
		want *Container
	}{
		{
			name: "empty container",
			want: &Container{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container := &Container{}
			if container == nil {
				t.Errorf("Container should not be nil")
			}
		})
	}
}
