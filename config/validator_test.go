package config

import (
	"strings"
	"testing"

	"github.com/input-output-hk/catalyst-forge-libs/schema"
	"github.com/input-output-hk/catalyst-forge-libs/schema/artifacts"
	"github.com/input-output-hk/catalyst-forge-libs/schema/phases"
	"github.com/input-output-hk/catalyst-forge-libs/schema/publishers"
)

// TestValidateProjectConfig tests the main validateProjectConfig function.
//
//nolint:funlen // Comprehensive table-driven test with many test cases
func TestValidateProjectConfig(t *testing.T) {
	tests := []struct {
		name    string
		project *ProjectConfig
		repo    *RepoConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid configuration with matching phases and publishers",
			project: &ProjectConfig{
				ProjectConfig: &schema.ProjectConfig{
					Name: "test-project",
					Phases: map[string]schema.PhaseParticipation{
						"build": {Steps: []schema.Step{{Name: "compile", Action: "earthly", Target: "+build"}}},
						"test":  {Steps: []schema.Step{{Name: "unit-tests", Action: "earthly", Target: "+test"}}},
					},
					Artifacts: map[string]artifacts.ArtifactSpec{
						"api": {
							"type":       "container",
							"publishers": []interface{}{"docker-hub", "ghcr"},
						},
					},
				},
			},
			repo: &RepoConfig{
				RepoConfig: &schema.RepoConfig{
					Phases: map[string]phases.PhaseDefinition{
						"build": {Group: 1, Description: "Build phase"},
						"test":  {Group: 2, Description: "Test phase"},
					},
					Publishers: map[string]publishers.PublisherConfig{
						"docker-hub": {"type": "docker", "registry": "docker.io"},
						"ghcr":       {"type": "docker", "registry": "ghcr.io"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid phase reference",
			project: &ProjectConfig{
				ProjectConfig: &schema.ProjectConfig{
					Name: "test-project",
					Phases: map[string]schema.PhaseParticipation{
						"build":  {Steps: []schema.Step{}},
						"deploy": {Steps: []schema.Step{}}, // Invalid - not in repo
					},
					Artifacts: map[string]artifacts.ArtifactSpec{},
				},
			},
			repo: &RepoConfig{
				RepoConfig: &schema.RepoConfig{
					Phases: map[string]phases.PhaseDefinition{
						"build": {Group: 1},
						"test":  {Group: 2},
					},
					Publishers: map[string]publishers.PublisherConfig{},
				},
			},
			wantErr: true,
			errMsg:  "unknown phase(s): deploy",
		},
		{
			name: "invalid publisher reference",
			project: &ProjectConfig{
				ProjectConfig: &schema.ProjectConfig{
					Name: "test-project",
					Phases: map[string]schema.PhaseParticipation{
						"build": {Steps: []schema.Step{}},
					},
					Artifacts: map[string]artifacts.ArtifactSpec{
						"api": {
							"type":       "container",
							"publishers": []interface{}{"invalid-publisher"},
						},
					},
				},
			},
			repo: &RepoConfig{
				RepoConfig: &schema.RepoConfig{
					Phases: map[string]phases.PhaseDefinition{
						"build": {Group: 1},
					},
					Publishers: map[string]publishers.PublisherConfig{
						"docker-hub": {"type": "docker"},
					},
				},
			},
			wantErr: true,
			errMsg:  "unknown publisher \"invalid-publisher\"",
		},
		{
			name: "multiple validation errors",
			project: &ProjectConfig{
				ProjectConfig: &schema.ProjectConfig{
					Name: "test-project",
					Phases: map[string]schema.PhaseParticipation{
						"invalid-phase": {Steps: []schema.Step{}},
					},
					Artifacts: map[string]artifacts.ArtifactSpec{
						"api": {
							"type":       "container",
							"publishers": []interface{}{"invalid-publisher"},
						},
					},
				},
			},
			repo: &RepoConfig{
				RepoConfig: &schema.RepoConfig{
					Phases: map[string]phases.PhaseDefinition{
						"build": {Group: 1},
					},
					Publishers: map[string]publishers.PublisherConfig{
						"docker-hub": {"type": "docker"},
					},
				},
			},
			wantErr: true,
			errMsg:  "validation failed",
		},
		{
			name:    "nil project",
			project: nil,
			repo: &RepoConfig{
				RepoConfig: &schema.RepoConfig{},
			},
			wantErr: true,
			errMsg:  "project configuration is nil",
		},
		{
			name: "nil repo",
			project: &ProjectConfig{
				ProjectConfig: &schema.ProjectConfig{},
			},
			repo:    nil,
			wantErr: true,
			errMsg:  "repository configuration is nil",
		},
		{
			name: "empty phases and artifacts",
			project: &ProjectConfig{
				ProjectConfig: &schema.ProjectConfig{
					Name:      "test-project",
					Phases:    map[string]schema.PhaseParticipation{},
					Artifacts: map[string]artifacts.ArtifactSpec{},
				},
			},
			repo: &RepoConfig{
				RepoConfig: &schema.RepoConfig{
					Phases:     map[string]phases.PhaseDefinition{},
					Publishers: map[string]publishers.PublisherConfig{},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProjectConfig(tt.project, tt.repo)

			if (err != nil) != tt.wantErr {
				t.Errorf("validateProjectConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errMsg != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("validateProjectConfig() error message = %v, want substring %q", err, tt.errMsg)
				}
			}
		})
	}
}

// TestValidateProjectPhaseReferences tests phase reference validation.
//
//nolint:funlen // Comprehensive table-driven test with many test cases
func TestValidateProjectPhaseReferences(t *testing.T) {
	tests := []struct {
		name    string
		project *ProjectConfig
		repo    *RepoConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "all phases valid",
			project: &ProjectConfig{
				ProjectConfig: &schema.ProjectConfig{
					Phases: map[string]schema.PhaseParticipation{
						"build": {Steps: []schema.Step{}},
						"test":  {Steps: []schema.Step{}},
					},
				},
			},
			repo: &RepoConfig{
				RepoConfig: &schema.RepoConfig{
					Phases: map[string]phases.PhaseDefinition{
						"build": {Group: 1},
						"test":  {Group: 2},
						"lint":  {Group: 1},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "single invalid phase",
			project: &ProjectConfig{
				ProjectConfig: &schema.ProjectConfig{
					Phases: map[string]schema.PhaseParticipation{
						"build":   {Steps: []schema.Step{}},
						"invalid": {Steps: []schema.Step{}},
					},
				},
			},
			repo: &RepoConfig{
				RepoConfig: &schema.RepoConfig{
					Phases: map[string]phases.PhaseDefinition{
						"build": {Group: 1},
						"test":  {Group: 2},
					},
				},
			},
			wantErr: true,
			errMsg:  "unknown phase(s): invalid",
		},
		{
			name: "multiple invalid phases",
			project: &ProjectConfig{
				ProjectConfig: &schema.ProjectConfig{
					Phases: map[string]schema.PhaseParticipation{
						"invalid1": {Steps: []schema.Step{}},
						"invalid2": {Steps: []schema.Step{}},
					},
				},
			},
			repo: &RepoConfig{
				RepoConfig: &schema.RepoConfig{
					Phases: map[string]phases.PhaseDefinition{
						"build": {Group: 1},
					},
				},
			},
			wantErr: true,
			errMsg:  "unknown phase(s)",
		},
		{
			name: "nil project phases",
			project: &ProjectConfig{
				ProjectConfig: &schema.ProjectConfig{
					Phases: nil,
				},
			},
			repo: &RepoConfig{
				RepoConfig: &schema.RepoConfig{
					Phases: map[string]phases.PhaseDefinition{
						"build": {Group: 1},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty project phases",
			project: &ProjectConfig{
				ProjectConfig: &schema.ProjectConfig{
					Phases: map[string]schema.PhaseParticipation{},
				},
			},
			repo: &RepoConfig{
				RepoConfig: &schema.RepoConfig{
					Phases: map[string]phases.PhaseDefinition{
						"build": {Group: 1},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "nil project config",
			project: &ProjectConfig{
				ProjectConfig: nil,
			},
			repo: &RepoConfig{
				RepoConfig: &schema.RepoConfig{
					Phases: map[string]phases.PhaseDefinition{
						"build": {Group: 1},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProjectPhaseReferences(tt.project, tt.repo)

			if (err != nil) != tt.wantErr {
				t.Errorf("validateProjectPhaseReferences() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errMsg != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("validateProjectPhaseReferences() error message = %v, want substring %q", err, tt.errMsg)
				}
			}
		})
	}
}

// TestValidateArtifactPublisherReferences tests artifact publisher reference validation.
//
//nolint:funlen // Comprehensive table-driven test with many test cases
func TestValidateArtifactPublisherReferences(t *testing.T) {
	tests := []struct {
		name    string
		project *ProjectConfig
		repo    *RepoConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "all publishers valid",
			project: &ProjectConfig{
				ProjectConfig: &schema.ProjectConfig{
					Artifacts: map[string]artifacts.ArtifactSpec{
						"api": {
							"type":       "container",
							"publishers": []interface{}{"docker-hub", "ghcr"},
						},
						"web": {
							"type":       "container",
							"publishers": []interface{}{"docker-hub"},
						},
					},
				},
			},
			repo: &RepoConfig{
				RepoConfig: &schema.RepoConfig{
					Publishers: map[string]publishers.PublisherConfig{
						"docker-hub": {"type": "docker"},
						"ghcr":       {"type": "docker"},
						"s3":         {"type": "s3"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "single invalid publisher",
			project: &ProjectConfig{
				ProjectConfig: &schema.ProjectConfig{
					Artifacts: map[string]artifacts.ArtifactSpec{
						"api": {
							"type":       "container",
							"publishers": []interface{}{"invalid-publisher"},
						},
					},
				},
			},
			repo: &RepoConfig{
				RepoConfig: &schema.RepoConfig{
					Publishers: map[string]publishers.PublisherConfig{
						"docker-hub": {"type": "docker"},
					},
				},
			},
			wantErr: true,
			errMsg:  "unknown publisher \"invalid-publisher\"",
		},
		{
			name: "multiple invalid publishers in one artifact",
			project: &ProjectConfig{
				ProjectConfig: &schema.ProjectConfig{
					Artifacts: map[string]artifacts.ArtifactSpec{
						"api": {
							"type":       "container",
							"publishers": []interface{}{"invalid1", "invalid2"},
						},
					},
				},
			},
			repo: &RepoConfig{
				RepoConfig: &schema.RepoConfig{
					Publishers: map[string]publishers.PublisherConfig{
						"docker-hub": {"type": "docker"},
					},
				},
			},
			wantErr: true,
			errMsg:  "unknown publisher",
		},
		{
			name: "invalid publishers across multiple artifacts",
			project: &ProjectConfig{
				ProjectConfig: &schema.ProjectConfig{
					Artifacts: map[string]artifacts.ArtifactSpec{
						"api": {
							"type":       "container",
							"publishers": []interface{}{"invalid1"},
						},
						"web": {
							"type":       "container",
							"publishers": []interface{}{"invalid2"},
						},
					},
				},
			},
			repo: &RepoConfig{
				RepoConfig: &schema.RepoConfig{
					Publishers: map[string]publishers.PublisherConfig{
						"docker-hub": {"type": "docker"},
					},
				},
			},
			wantErr: true,
			errMsg:  "unknown publisher",
		},
		{
			name: "artifact with no publishers field",
			project: &ProjectConfig{
				ProjectConfig: &schema.ProjectConfig{
					Artifacts: map[string]artifacts.ArtifactSpec{
						"api": {
							"type": "container",
						},
					},
				},
			},
			repo: &RepoConfig{
				RepoConfig: &schema.RepoConfig{
					Publishers: map[string]publishers.PublisherConfig{
						"docker-hub": {"type": "docker"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "artifact with empty publishers list",
			project: &ProjectConfig{
				ProjectConfig: &schema.ProjectConfig{
					Artifacts: map[string]artifacts.ArtifactSpec{
						"api": {
							"type":       "container",
							"publishers": []interface{}{},
						},
					},
				},
			},
			repo: &RepoConfig{
				RepoConfig: &schema.RepoConfig{
					Publishers: map[string]publishers.PublisherConfig{
						"docker-hub": {"type": "docker"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "nil artifacts",
			project: &ProjectConfig{
				ProjectConfig: &schema.ProjectConfig{
					Artifacts: nil,
				},
			},
			repo: &RepoConfig{
				RepoConfig: &schema.RepoConfig{
					Publishers: map[string]publishers.PublisherConfig{
						"docker-hub": {"type": "docker"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty artifacts map",
			project: &ProjectConfig{
				ProjectConfig: &schema.ProjectConfig{
					Artifacts: map[string]artifacts.ArtifactSpec{},
				},
			},
			repo: &RepoConfig{
				RepoConfig: &schema.RepoConfig{
					Publishers: map[string]publishers.PublisherConfig{
						"docker-hub": {"type": "docker"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "mixed valid and invalid publishers",
			project: &ProjectConfig{
				ProjectConfig: &schema.ProjectConfig{
					Artifacts: map[string]artifacts.ArtifactSpec{
						"api": {
							"type":       "container",
							"publishers": []interface{}{"docker-hub", "invalid"},
						},
					},
				},
			},
			repo: &RepoConfig{
				RepoConfig: &schema.RepoConfig{
					Publishers: map[string]publishers.PublisherConfig{
						"docker-hub": {"type": "docker"},
					},
				},
			},
			wantErr: true,
			errMsg:  "unknown publisher \"invalid\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateArtifactPublisherReferences(tt.project, tt.repo)

			if (err != nil) != tt.wantErr {
				t.Errorf("validateArtifactPublisherReferences() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errMsg != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("validateArtifactPublisherReferences() error message = %v, want substring %q", err, tt.errMsg)
				}
			}
		})
	}
}

// TestValidateProjectConfig_ErrorMessageClarity tests that error messages include helpful details.
func TestValidateProjectConfig_ErrorMessageClarity(t *testing.T) {
	tests := []struct {
		name             string
		project          *ProjectConfig
		repo             *RepoConfig
		wantErrSubstring string
	}{
		{
			name: "phase error includes available phases",
			project: &ProjectConfig{
				ProjectConfig: &schema.ProjectConfig{
					Phases: map[string]schema.PhaseParticipation{
						"invalid": {Steps: []schema.Step{}},
					},
				},
			},
			repo: &RepoConfig{
				RepoConfig: &schema.RepoConfig{
					Phases: map[string]phases.PhaseDefinition{
						"build": {Group: 1},
						"test":  {Group: 2},
					},
				},
			},
			wantErrSubstring: "available phases:",
		},
		{
			name: "publisher error includes artifact name",
			project: &ProjectConfig{
				ProjectConfig: &schema.ProjectConfig{
					Artifacts: map[string]artifacts.ArtifactSpec{
						"my-api": {
							"type":       "container",
							"publishers": []interface{}{"invalid"},
						},
					},
				},
			},
			repo: &RepoConfig{
				RepoConfig: &schema.RepoConfig{
					Publishers: map[string]publishers.PublisherConfig{
						"docker-hub": {"type": "docker"},
					},
				},
			},
			wantErrSubstring: "artifact \"my-api\"",
		},
		{
			name: "publisher error includes available publishers",
			project: &ProjectConfig{
				ProjectConfig: &schema.ProjectConfig{
					Artifacts: map[string]artifacts.ArtifactSpec{
						"api": {
							"type":       "container",
							"publishers": []interface{}{"invalid"},
						},
					},
				},
			},
			repo: &RepoConfig{
				RepoConfig: &schema.RepoConfig{
					Publishers: map[string]publishers.PublisherConfig{
						"docker-hub": {"type": "docker"},
						"ghcr":       {"type": "docker"},
					},
				},
			},
			wantErrSubstring: "available publishers:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateProjectConfig(tt.project, tt.repo)

			if err == nil {
				t.Error("validateProjectConfig() expected error, got nil")
				return
			}

			if !strings.Contains(err.Error(), tt.wantErrSubstring) {
				t.Errorf("validateProjectConfig() error = %v, want substring %q", err, tt.wantErrSubstring)
			}
		})
	}
}
