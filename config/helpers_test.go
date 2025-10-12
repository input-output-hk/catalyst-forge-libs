package config

import (
	"testing"

	"github.com/input-output-hk/catalyst-forge-libs/schema/common"
	"github.com/input-output-hk/catalyst-forge-libs/schema/phases"
	"github.com/input-output-hk/catalyst-forge-libs/schema/publishers"
)

// TestFilterPhasesByGroup tests filtering phases by group number.
func TestFilterPhasesByGroup(t *testing.T) {
	phases := map[string]phases.PhaseDefinition{
		"build": {Group: 1},
		"test":  {Group: 2},
		"lint":  {Group: 1},
		"deploy": {Group: 3},
	}

	tests := []struct {
		name  string
		group int
		want  []string
	}{
		{
			name:  "group 1",
			group: 1,
			want:  []string{"build", "lint"},
		},
		{
			name:  "group 2",
			group: 2,
			want:  []string{"test"},
		},
		{
			name:  "group 3",
			group: 3,
			want:  []string{"deploy"},
		},
		{
			name:  "non-existent group",
			group: 99,
			want:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterPhasesByGroup(phases, tt.group)
			
			if len(got) != len(tt.want) {
				t.Errorf("filterPhasesByGroup() got %d items, want %d", len(got), len(tt.want))
				return
			}
			
			// Check that all expected items are present
			for i, want := range tt.want {
				if got[i] != want {
					t.Errorf("filterPhasesByGroup()[%d] = %v, want %v", i, got[i], want)
				}
			}
		})
	}
}

// TestFilterPhasesByGroup_Empty tests filtering with empty input.
func TestFilterPhasesByGroup_Empty(t *testing.T) {
	phases := map[string]phases.PhaseDefinition{}
	got := filterPhasesByGroup(phases, 1)
	
	if len(got) != 0 {
		t.Errorf("filterPhasesByGroup() with empty map got %d items, want 0", len(got))
	}
}

// TestSortPhasesByGroup tests sorting phases by group.
func TestSortPhasesByGroup(t *testing.T) {
	tests := []struct {
		name   string
		phases map[string]phases.PhaseDefinition
		want   [][]string
	}{
		{
			name: "multiple groups",
			phases: map[string]phases.PhaseDefinition{
				"build":  {Group: 1},
				"test":   {Group: 2},
				"lint":   {Group: 1},
				"deploy": {Group: 3},
			},
			want: [][]string{
				{"build", "lint"},
				{"test"},
				{"deploy"},
			},
		},
		{
			name: "single group",
			phases: map[string]phases.PhaseDefinition{
				"build": {Group: 1},
				"test":  {Group: 1},
			},
			want: [][]string{
				{"build", "test"},
			},
		},
		{
			name:   "empty phases",
			phases: map[string]phases.PhaseDefinition{},
			want:   [][]string{},
		},
		{
			name: "non-sequential groups",
			phases: map[string]phases.PhaseDefinition{
				"build":  {Group: 1},
				"deploy": {Group: 5},
				"test":   {Group: 3},
			},
			want: [][]string{
				{"build"},
				{"test"},
				{"deploy"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sortPhasesByGroup(tt.phases)
			
			if len(got) != len(tt.want) {
				t.Errorf("sortPhasesByGroup() got %d groups, want %d", len(got), len(tt.want))
				return
			}
			
			for i := range tt.want {
				if len(got[i]) != len(tt.want[i]) {
					t.Errorf("sortPhasesByGroup() group %d got %d items, want %d", i, len(got[i]), len(tt.want[i]))
					continue
				}
				
				for j := range tt.want[i] {
					if got[i][j] != tt.want[i][j] {
						t.Errorf("sortPhasesByGroup() group %d item %d = %v, want %v", i, j, got[i][j], tt.want[i][j])
					}
				}
			}
		})
	}
}

// TestPhaseDefinitionToString tests phase definition formatting.
func TestPhaseDefinitionToString(t *testing.T) {
	tests := []struct {
		name  string
		phase phases.PhaseDefinition
		want  string
	}{
		{
			name:  "minimal phase",
			phase: phases.PhaseDefinition{Group: 1},
			want:  "PhaseDefinition{group=1}",
		},
		{
			name: "phase with description",
			phase: phases.PhaseDefinition{
				Group:       1,
				Description: "Build phase",
			},
			want: `PhaseDefinition{group=1, description="Build phase"}`,
		},
		{
			name: "phase with timeout",
			phase: phases.PhaseDefinition{
				Group:   1,
				Timeout: "30m",
			},
			want: "PhaseDefinition{group=1, timeout=30m}",
		},
		{
			name: "phase with required",
			phase: phases.PhaseDefinition{
				Group:    1,
				Required: true,
			},
			want: "PhaseDefinition{group=1, required=true}",
		},
		{
			name: "phase with all fields",
			phase: phases.PhaseDefinition{
				Group:       2,
				Description: "Test phase",
				Timeout:     "1h",
				Required:    true,
			},
			want: `PhaseDefinition{group=2, description="Test phase", timeout=1h, required=true}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := phaseDefinitionToString(tt.phase)
			if got != tt.want {
				t.Errorf("phaseDefinitionToString() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestPublisherConfigToString tests publisher config formatting.
func TestPublisherConfigToString(t *testing.T) {
	tests := []struct {
		name string
		pub  publishers.PublisherConfig
		want string
	}{
		{
			name: "docker publisher",
			pub: publishers.PublisherConfig{
				"type":      "docker",
				"registry":  "ghcr.io",
				"namespace": "myorg",
			},
			want: "PublisherConfig{type=docker, registry=ghcr.io, namespace=myorg}",
		},
		{
			name: "docker publisher with credentials",
			pub: publishers.PublisherConfig{
				"type":      "docker",
				"registry":  "ghcr.io",
				"namespace": "myorg",
				"credentials": common.SecretRef{
					"provider": "aws",
					"name":     "docker-creds",
				},
			},
			want: "PublisherConfig{type=docker, registry=ghcr.io, namespace=myorg, credentials=<present>}",
		},
		{
			name: "github publisher",
			pub: publishers.PublisherConfig{
				"type":       "github",
				"repository": "owner/repo",
			},
			want: "PublisherConfig{type=github, repository=owner/repo}",
		},
		{
			name: "s3 publisher",
			pub: publishers.PublisherConfig{
				"type":   "s3",
				"bucket": "my-bucket",
			},
			want: "PublisherConfig{type=s3, bucket=my-bucket}",
		},
		{
			name: "s3 publisher with region",
			pub: publishers.PublisherConfig{
				"type":   "s3",
				"bucket": "my-bucket",
				"region": "us-west-2",
			},
			want: "PublisherConfig{type=s3, bucket=my-bucket, region=us-west-2}",
		},
		{
			name: "missing type field",
			pub:  publishers.PublisherConfig{},
			want: "PublisherConfig{invalid: missing type field}",
		},
		{
			name: "type field not string",
			pub: publishers.PublisherConfig{
				"type": 123,
			},
			want: "PublisherConfig{invalid: type field is not a string}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := publisherConfigToString(tt.pub)
			if got != tt.want {
				t.Errorf("publisherConfigToString() = %v, want %v", got, tt.want)
			}
		})
	}
}
