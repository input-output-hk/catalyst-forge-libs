package config

import (
	"testing"

	"github.com/input-output-hk/catalyst-forge-libs/schema"
	"github.com/input-output-hk/catalyst-forge-libs/schema/artifacts"
	"github.com/input-output-hk/catalyst-forge-libs/schema/phases"
	"github.com/input-output-hk/catalyst-forge-libs/schema/publishers"
)

// TestParticipatesIn tests the ParticipatesIn method.
func TestParticipatesIn(t *testing.T) {
	tests := []struct {
		name      string
		phases    map[string]schema.PhaseParticipation
		phaseName string
		want      bool
	}{
		{
			name: "participates in phase",
			phases: map[string]schema.PhaseParticipation{
				"test": {
					Steps: []schema.Step{
						{Name: "unit-tests", Action: "earthly", Target: "+test"},
					},
				},
			},
			phaseName: "test",
			want:      true,
		},
		{
			name: "does not participate in phase",
			phases: map[string]schema.PhaseParticipation{
				"test": {Steps: []schema.Step{}},
			},
			phaseName: "build",
			want:      false,
		},
		{
			name:      "empty phases map",
			phases:    map[string]schema.PhaseParticipation{},
			phaseName: "test",
			want:      false,
		},
		{
			name:      "nil phases map",
			phases:    nil,
			phaseName: "test",
			want:      false,
		},
		{
			name:      "nil project config",
			phases:    nil,
			phaseName: "test",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := &ProjectConfig{
				ProjectConfig: &schema.ProjectConfig{
					Phases: tt.phases,
				},
			}

			got := project.ParticipatesIn(tt.phaseName)

			if got != tt.want {
				t.Errorf("ParticipatesIn() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestGetPhaseSteps tests the GetPhaseSteps method.
func TestGetPhaseSteps(t *testing.T) {
	tests := []struct {
		name      string
		phases    map[string]schema.PhaseParticipation
		phaseName string
		wantFound bool
		wantCount int
	}{
		{
			name: "existing phase with steps",
			phases: map[string]schema.PhaseParticipation{
				"test": {
					Steps: []schema.Step{
						{Name: "unit-tests", Action: "earthly", Target: "+test"},
						{Name: "integration-tests", Action: "earthly", Target: "+integration"},
					},
				},
			},
			phaseName: "test",
			wantFound: true,
			wantCount: 2,
		},
		{
			name: "existing phase with no steps",
			phases: map[string]schema.PhaseParticipation{
				"test": {Steps: []schema.Step{}},
			},
			phaseName: "test",
			wantFound: true,
			wantCount: 0,
		},
		{
			name: "non-existing phase",
			phases: map[string]schema.PhaseParticipation{
				"test": {Steps: []schema.Step{}},
			},
			phaseName: "build",
			wantFound: false,
		},
		{
			name:      "empty phases map",
			phases:    map[string]schema.PhaseParticipation{},
			phaseName: "test",
			wantFound: false,
		},
		{
			name:      "nil phases map",
			phases:    nil,
			phaseName: "test",
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := &ProjectConfig{
				ProjectConfig: &schema.ProjectConfig{
					Phases: tt.phases,
				},
			}

			steps, found := project.GetPhaseSteps(tt.phaseName)

			if found != tt.wantFound {
				t.Errorf("GetPhaseSteps() found = %v, want %v", found, tt.wantFound)
			}

			if tt.wantFound {
				if steps == nil {
					t.Errorf("GetPhaseSteps() returned nil steps when found=true")
				} else if len(steps) != tt.wantCount {
					t.Errorf("GetPhaseSteps() step count = %v, want %v", len(steps), tt.wantCount)
				}
			} else if steps != nil {
				t.Errorf("GetPhaseSteps() returned non-nil steps when found=false")
			}
		})
	}
}

// TestListParticipatingPhases tests the ListParticipatingPhases method.
func TestListParticipatingPhases(t *testing.T) {
	tests := []struct {
		name   string
		phases map[string]schema.PhaseParticipation
		want   []string
	}{
		{
			name: "multiple phases",
			phases: map[string]schema.PhaseParticipation{
				"test":    {Steps: []schema.Step{}},
				"build":   {Steps: []schema.Step{}},
				"release": {Steps: []schema.Step{}},
			},
			want: []string{"build", "release", "test"},
		},
		{
			name: "single phase",
			phases: map[string]schema.PhaseParticipation{
				"test": {Steps: []schema.Step{}},
			},
			want: []string{"test"},
		},
		{
			name:   "empty phases",
			phases: map[string]schema.PhaseParticipation{},
			want:   []string{},
		},
		{
			name:   "nil phases",
			phases: nil,
			want:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := &ProjectConfig{
				ProjectConfig: &schema.ProjectConfig{
					Phases: tt.phases,
				},
			}

			got := project.ListParticipatingPhases()

			if len(got) != len(tt.want) {
				t.Errorf("ListParticipatingPhases() length = %v, want %v", len(got), len(tt.want))
				return
			}

			for i, name := range got {
				if name != tt.want[i] {
					t.Errorf("ListParticipatingPhases()[%d] = %v, want %v", i, name, tt.want[i])
				}
			}
		})
	}
}

// TestGetArtifact tests the GetArtifact method.
func TestGetArtifact(t *testing.T) {
	tests := []struct {
		name         string
		artifacts    map[string]artifacts.ArtifactSpec
		artifactName string
		wantFound    bool
		wantType     string
	}{
		{
			name: "existing container artifact",
			artifacts: map[string]artifacts.ArtifactSpec{
				"api": {
					"type": "container",
					"ref":  "myapp:latest",
				},
			},
			artifactName: "api",
			wantFound:    true,
			wantType:     "container",
		},
		{
			name: "non-existing artifact",
			artifacts: map[string]artifacts.ArtifactSpec{
				"api": {"type": "container"},
			},
			artifactName: "web",
			wantFound:    false,
		},
		{
			name:         "empty artifacts",
			artifacts:    map[string]artifacts.ArtifactSpec{},
			artifactName: "api",
			wantFound:    false,
		},
		{
			name:         "nil artifacts",
			artifacts:    nil,
			artifactName: "api",
			wantFound:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := &ProjectConfig{
				ProjectConfig: &schema.ProjectConfig{
					Artifacts: tt.artifacts,
				},
			}

			artifact, found := project.GetArtifact(tt.artifactName)

			if found != tt.wantFound {
				t.Errorf("GetArtifact() found = %v, want %v", found, tt.wantFound)
			}

			if tt.wantFound {
				if artifact == nil {
					t.Errorf("GetArtifact() returned nil artifact when found=true")
				} else if artType, ok := artifact["type"].(string); ok && artType != tt.wantType {
					t.Errorf("GetArtifact() type = %v, want %v", artType, tt.wantType)
				}
			} else if artifact != nil {
				t.Errorf("GetArtifact() returned non-nil artifact when found=false")
			}
		})
	}
}

// TestListArtifacts tests the ListArtifacts method.
func TestListArtifacts(t *testing.T) {
	tests := []struct {
		name      string
		artifacts map[string]artifacts.ArtifactSpec
		want      []string
	}{
		{
			name: "multiple artifacts",
			artifacts: map[string]artifacts.ArtifactSpec{
				"api":     {"type": "container"},
				"web":     {"type": "container"},
				"cli":     {"type": "binary"},
				"package": {"type": "archive"},
			},
			want: []string{"api", "cli", "package", "web"},
		},
		{
			name: "single artifact",
			artifacts: map[string]artifacts.ArtifactSpec{
				"api": {"type": "container"},
			},
			want: []string{"api"},
		},
		{
			name:      "empty artifacts",
			artifacts: map[string]artifacts.ArtifactSpec{},
			want:      []string{},
		},
		{
			name:      "nil artifacts",
			artifacts: nil,
			want:      []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := &ProjectConfig{
				ProjectConfig: &schema.ProjectConfig{
					Artifacts: tt.artifacts,
				},
			}

			got := project.ListArtifacts()

			if len(got) != len(tt.want) {
				t.Errorf("ListArtifacts() length = %v, want %v", len(got), len(tt.want))
				return
			}

			for i, name := range got {
				if name != tt.want[i] {
					t.Errorf("ListArtifacts()[%d] = %v, want %v", i, name, tt.want[i])
				}
			}
		})
	}
}

// TestHasArtifact tests the HasArtifact method.
func TestHasArtifact(t *testing.T) {
	tests := []struct {
		name         string
		artifacts    map[string]artifacts.ArtifactSpec
		artifactName string
		want         bool
	}{
		{
			name: "existing artifact",
			artifacts: map[string]artifacts.ArtifactSpec{
				"api": {"type": "container"},
			},
			artifactName: "api",
			want:         true,
		},
		{
			name: "non-existing artifact",
			artifacts: map[string]artifacts.ArtifactSpec{
				"api": {"type": "container"},
			},
			artifactName: "web",
			want:         false,
		},
		{
			name:         "empty artifacts",
			artifacts:    map[string]artifacts.ArtifactSpec{},
			artifactName: "api",
			want:         false,
		},
		{
			name:         "nil artifacts",
			artifacts:    nil,
			artifactName: "api",
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := &ProjectConfig{
				ProjectConfig: &schema.ProjectConfig{
					Artifacts: tt.artifacts,
				},
			}

			got := project.HasArtifact(tt.artifactName)

			if got != tt.want {
				t.Errorf("HasArtifact() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestGetArtifactPublishers tests the GetArtifactPublishers method.
func TestGetArtifactPublishers(t *testing.T) {
	tests := []struct {
		name         string
		artifacts    map[string]artifacts.ArtifactSpec
		artifactName string
		wantPubs     []string
		wantErr      bool
	}{
		{
			name: "artifact with publishers",
			artifacts: map[string]artifacts.ArtifactSpec{
				"api": {
					"type":       "container",
					"publishers": []interface{}{"docker-hub", "ghcr"},
				},
			},
			artifactName: "api",
			wantPubs:     []string{"docker-hub", "ghcr"},
			wantErr:      false,
		},
		{
			name: "artifact with no publishers field",
			artifacts: map[string]artifacts.ArtifactSpec{
				"api": {
					"type": "container",
				},
			},
			artifactName: "api",
			wantPubs:     []string{},
			wantErr:      false,
		},
		{
			name: "artifact with empty publishers",
			artifacts: map[string]artifacts.ArtifactSpec{
				"api": {
					"type":       "container",
					"publishers": []interface{}{},
				},
			},
			artifactName: "api",
			wantPubs:     []string{},
			wantErr:      false,
		},
		{
			name: "non-existing artifact",
			artifacts: map[string]artifacts.ArtifactSpec{
				"api": {"type": "container"},
			},
			artifactName: "web",
			wantErr:      true,
		},
		{
			name: "artifact with invalid publishers type",
			artifacts: map[string]artifacts.ArtifactSpec{
				"api": {
					"type":       "container",
					"publishers": "not-a-slice",
				},
			},
			artifactName: "api",
			wantErr:      true,
		},
		{
			name: "artifact with non-string publisher",
			artifacts: map[string]artifacts.ArtifactSpec{
				"api": {
					"type":       "container",
					"publishers": []interface{}{"docker-hub", 123},
				},
			},
			artifactName: "api",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := &ProjectConfig{
				ProjectConfig: &schema.ProjectConfig{
					Artifacts: tt.artifacts,
				},
			}

			pubs, err := project.GetArtifactPublishers(tt.artifactName)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetArtifactPublishers() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(pubs) != len(tt.wantPubs) {
					t.Errorf("GetArtifactPublishers() length = %v, want %v", len(pubs), len(tt.wantPubs))
					return
				}

				for i, pub := range pubs {
					if pub != tt.wantPubs[i] {
						t.Errorf("GetArtifactPublishers()[%d] = %v, want %v", i, pub, tt.wantPubs[i])
					}
				}
			}
		})
	}
}

// TestHasRelease tests the HasRelease method.
func TestHasRelease(t *testing.T) {
	tests := []struct {
		name       string
		release    *schema.ReleaseConfig
		nilProject bool
		want       bool
	}{
		{
			name: "has release config",
			release: &schema.ReleaseConfig{
				On: []schema.ReleaseTrigger{
					{Branch: "main"},
				},
			},
			want: true,
		},
		{
			name:    "no release config",
			release: nil,
			want:    false,
		},
		{
			name:       "nil project config",
			nilProject: true,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var project *ProjectConfig
			if tt.nilProject {
				project = &ProjectConfig{}
			} else {
				project = &ProjectConfig{
					ProjectConfig: &schema.ProjectConfig{
						Release: tt.release,
					},
				}
			}

			got := project.HasRelease()

			if got != tt.want {
				t.Errorf("HasRelease() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestGetReleaseTriggers tests the GetReleaseTriggers method.
func TestGetReleaseTriggers(t *testing.T) {
	tests := []struct {
		name      string
		release   *schema.ReleaseConfig
		wantFound bool
		wantCount int
	}{
		{
			name: "has release triggers",
			release: &schema.ReleaseConfig{
				On: []schema.ReleaseTrigger{
					{Branch: "main"},
					{Tag: true},
				},
			},
			wantFound: true,
			wantCount: 2,
		},
		{
			name: "has release with no triggers",
			release: &schema.ReleaseConfig{
				On: []schema.ReleaseTrigger{},
			},
			wantFound: true,
			wantCount: 0,
		},
		{
			name:      "no release config",
			release:   nil,
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := &ProjectConfig{
				ProjectConfig: &schema.ProjectConfig{
					Release: tt.release,
				},
			}

			config, found := project.GetReleaseTriggers()

			if found != tt.wantFound {
				t.Errorf("GetReleaseTriggers() found = %v, want %v", found, tt.wantFound)
			}

			if tt.wantFound {
				if config == nil {
					t.Errorf("GetReleaseTriggers() returned nil config when found=true")
				} else if len(config.On) != tt.wantCount {
					t.Errorf("GetReleaseTriggers() trigger count = %v, want %v", len(config.On), tt.wantCount)
				}
			} else if config != nil {
				t.Errorf("GetReleaseTriggers() returned non-nil config when found=false")
			}
		})
	}
}

// TestHasDeployment tests the HasDeployment method.
func TestHasDeployment(t *testing.T) {
	tests := []struct {
		name       string
		deploy     *schema.DeploymentConfig
		nilProject bool
		want       bool
	}{
		{
			name: "has deployment config",
			deploy: &schema.DeploymentConfig{
				Resources: []schema.K8sResource{
					{
						ApiVersion: "apps/v1",
						Kind:       "Deployment",
					},
				},
			},
			want: true,
		},
		{
			name:   "no deployment config",
			deploy: nil,
			want:   false,
		},
		{
			name:       "nil project config",
			nilProject: true,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var project *ProjectConfig
			if tt.nilProject {
				project = &ProjectConfig{}
			} else {
				project = &ProjectConfig{
					ProjectConfig: &schema.ProjectConfig{
						Deploy: tt.deploy,
					},
				}
			}

			got := project.HasDeployment()

			if got != tt.want {
				t.Errorf("HasDeployment() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestGetDeploymentConfig tests the GetDeploymentConfig method.
func TestGetDeploymentConfig(t *testing.T) {
	tests := []struct {
		name      string
		deploy    *schema.DeploymentConfig
		wantFound bool
		wantCount int
	}{
		{
			name: "has deployment resources",
			deploy: &schema.DeploymentConfig{
				Resources: []schema.K8sResource{
					{
						ApiVersion: "apps/v1",
						Kind:       "Deployment",
					},
					{
						ApiVersion: "v1",
						Kind:       "Service",
					},
				},
			},
			wantFound: true,
			wantCount: 2,
		},
		{
			name: "has deployment with no resources",
			deploy: &schema.DeploymentConfig{
				Resources: []schema.K8sResource{},
			},
			wantFound: true,
			wantCount: 0,
		},
		{
			name:      "no deployment config",
			deploy:    nil,
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := &ProjectConfig{
				ProjectConfig: &schema.ProjectConfig{
					Deploy: tt.deploy,
				},
			}

			config, found := project.GetDeploymentConfig()

			if found != tt.wantFound {
				t.Errorf("GetDeploymentConfig() found = %v, want %v", found, tt.wantFound)
			}

			if tt.wantFound {
				if config == nil {
					t.Errorf("GetDeploymentConfig() returned nil config when found=true")
				} else if len(config.Resources) != tt.wantCount {
					t.Errorf("GetDeploymentConfig() resource count = %v, want %v", len(config.Resources), tt.wantCount)
				}
			} else if config != nil {
				t.Errorf("GetDeploymentConfig() returned non-nil config when found=false")
			}
		})
	}
}

// TestProjectValidate tests the Validate method on ProjectConfig.
func TestProjectValidate(t *testing.T) {
	tests := []struct {
		name    string
		project *ProjectConfig
		repo    *RepoConfig
		wantErr bool
	}{
		{
			name: "valid config",
			project: &ProjectConfig{
				ProjectConfig: &schema.ProjectConfig{
					Name: "test-project",
					Phases: map[string]schema.PhaseParticipation{
						"test": {Steps: []schema.Step{}},
					},
					Artifacts: map[string]artifacts.ArtifactSpec{},
				},
			},
			repo: &RepoConfig{
				RepoConfig: &schema.RepoConfig{
					Phases: map[string]phases.PhaseDefinition{
						"test": {Group: 1, Description: "Test phase"},
					},
					Publishers: map[string]publishers.PublisherConfig{},
				},
			},
			wantErr: false, // Should pass now that validator is implemented
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.project.Validate(tt.repo)

			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
