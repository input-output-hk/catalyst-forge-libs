package config

import (
	"testing"

	"github.com/input-output-hk/catalyst-forge-libs/schema"
	"github.com/input-output-hk/catalyst-forge-libs/schema/phases"
	"github.com/input-output-hk/catalyst-forge-libs/schema/publishers"
)

// TestGetPhase tests the GetPhase method.
func TestGetPhase(t *testing.T) {
	tests := []struct {
		name      string
		phases    map[string]phases.PhaseDefinition
		phaseName string
		wantFound bool
		wantGroup int64
	}{
		{
			name: "existing phase",
			phases: map[string]phases.PhaseDefinition{
				"test": {Group: 1, Description: "Test phase", Required: true},
			},
			phaseName: "test",
			wantFound: true,
			wantGroup: 1,
		},
		{
			name: "non-existing phase",
			phases: map[string]phases.PhaseDefinition{
				"test": {Group: 1, Description: "Test phase"},
			},
			phaseName: "build",
			wantFound: false,
		},
		{
			name:      "empty phases map",
			phases:    map[string]phases.PhaseDefinition{},
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
			repo := &RepoConfig{
				RepoConfig: &schema.RepoConfig{
					Phases: tt.phases,
				},
			}

			phase, found := repo.GetPhase(tt.phaseName)

			if found != tt.wantFound {
				t.Errorf("GetPhase() found = %v, want %v", found, tt.wantFound)
			}

			if tt.wantFound {
				if phase == nil {
					t.Errorf("GetPhase() returned nil phase when found=true")
				} else if phase.Group != tt.wantGroup {
					t.Errorf("GetPhase() group = %v, want %v", phase.Group, tt.wantGroup)
				}
			} else if phase != nil {
				t.Errorf("GetPhase() returned non-nil phase when found=false")
			}
		})
	}
}

// TestListPhases tests the ListPhases method.
func TestListPhases(t *testing.T) {
	tests := []struct {
		name   string
		phases map[string]phases.PhaseDefinition
		want   []string
	}{
		{
			name: "multiple phases",
			phases: map[string]phases.PhaseDefinition{
				"test":    {Group: 1},
				"build":   {Group: 1},
				"release": {Group: 2},
			},
			want: []string{"build", "release", "test"},
		},
		{
			name: "single phase",
			phases: map[string]phases.PhaseDefinition{
				"test": {Group: 1},
			},
			want: []string{"test"},
		},
		{
			name:   "empty phases",
			phases: map[string]phases.PhaseDefinition{},
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
			repo := &RepoConfig{
				RepoConfig: &schema.RepoConfig{
					Phases: tt.phases,
				},
			}

			got := repo.ListPhases()

			if len(got) != len(tt.want) {
				t.Errorf("ListPhases() length = %v, want %v", len(got), len(tt.want))
				return
			}

			for i, name := range got {
				if name != tt.want[i] {
					t.Errorf("ListPhases()[%d] = %v, want %v", i, name, tt.want[i])
				}
			}
		})
	}
}

// TestHasPhase tests the HasPhase method.
func TestHasPhase(t *testing.T) {
	tests := []struct {
		name      string
		phases    map[string]phases.PhaseDefinition
		phaseName string
		want      bool
	}{
		{
			name: "existing phase",
			phases: map[string]phases.PhaseDefinition{
				"test": {Group: 1},
			},
			phaseName: "test",
			want:      true,
		},
		{
			name: "non-existing phase",
			phases: map[string]phases.PhaseDefinition{
				"test": {Group: 1},
			},
			phaseName: "build",
			want:      false,
		},
		{
			name:      "empty phases",
			phases:    map[string]phases.PhaseDefinition{},
			phaseName: "test",
			want:      false,
		},
		{
			name:      "nil phases",
			phases:    nil,
			phaseName: "test",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &RepoConfig{
				RepoConfig: &schema.RepoConfig{
					Phases: tt.phases,
				},
			}

			got := repo.HasPhase(tt.phaseName)

			if got != tt.want {
				t.Errorf("HasPhase() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestGetPublisher tests the GetPublisher method.
func TestGetPublisher(t *testing.T) {
	tests := []struct {
		name          string
		publishers    map[string]publishers.PublisherConfig
		publisherName string
		wantFound     bool
		wantType      string
	}{
		{
			name: "existing docker publisher",
			publishers: map[string]publishers.PublisherConfig{
				"docker-hub": {
					"type":      "docker",
					"registry":  "docker.io",
					"namespace": "myorg",
				},
			},
			publisherName: "docker-hub",
			wantFound:     true,
			wantType:      "docker",
		},
		{
			name: "non-existing publisher",
			publishers: map[string]publishers.PublisherConfig{
				"docker-hub": {"type": "docker"},
			},
			publisherName: "github",
			wantFound:     false,
		},
		{
			name:          "empty publishers",
			publishers:    map[string]publishers.PublisherConfig{},
			publisherName: "docker-hub",
			wantFound:     false,
		},
		{
			name:          "nil publishers",
			publishers:    nil,
			publisherName: "docker-hub",
			wantFound:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &RepoConfig{
				RepoConfig: &schema.RepoConfig{
					Publishers: tt.publishers,
				},
			}

			pub, found := repo.GetPublisher(tt.publisherName)

			if found != tt.wantFound {
				t.Errorf("GetPublisher() found = %v, want %v", found, tt.wantFound)
			}

			if tt.wantFound {
				if pub == nil {
					t.Errorf("GetPublisher() returned nil publisher when found=true")
				} else if pubType, ok := pub["type"].(string); ok && pubType != tt.wantType {
					t.Errorf("GetPublisher() type = %v, want %v", pubType, tt.wantType)
				}
			} else if pub != nil {
				t.Errorf("GetPublisher() returned non-nil publisher when found=false")
			}
		})
	}
}

// TestListPublishers tests the ListPublishers method.
func TestListPublishers(t *testing.T) {
	tests := []struct {
		name       string
		publishers map[string]publishers.PublisherConfig
		want       []string
	}{
		{
			name: "multiple publishers",
			publishers: map[string]publishers.PublisherConfig{
				"docker-hub": {"type": "docker"},
				"github":     {"type": "github"},
				"s3":         {"type": "s3"},
			},
			want: []string{"docker-hub", "github", "s3"},
		},
		{
			name: "single publisher",
			publishers: map[string]publishers.PublisherConfig{
				"docker-hub": {"type": "docker"},
			},
			want: []string{"docker-hub"},
		},
		{
			name:       "empty publishers",
			publishers: map[string]publishers.PublisherConfig{},
			want:       []string{},
		},
		{
			name:       "nil publishers",
			publishers: nil,
			want:       []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &RepoConfig{
				RepoConfig: &schema.RepoConfig{
					Publishers: tt.publishers,
				},
			}

			got := repo.ListPublishers()

			if len(got) != len(tt.want) {
				t.Errorf("ListPublishers() length = %v, want %v", len(got), len(tt.want))
				return
			}

			for i, name := range got {
				if name != tt.want[i] {
					t.Errorf("ListPublishers()[%d] = %v, want %v", i, name, tt.want[i])
				}
			}
		})
	}
}

// TestHasPublisher tests the HasPublisher method.
func TestHasPublisher(t *testing.T) {
	tests := []struct {
		name          string
		publishers    map[string]publishers.PublisherConfig
		publisherName string
		want          bool
	}{
		{
			name: "existing publisher",
			publishers: map[string]publishers.PublisherConfig{
				"docker-hub": {"type": "docker"},
			},
			publisherName: "docker-hub",
			want:          true,
		},
		{
			name: "non-existing publisher",
			publishers: map[string]publishers.PublisherConfig{
				"docker-hub": {"type": "docker"},
			},
			publisherName: "github",
			want:          false,
		},
		{
			name:          "empty publishers",
			publishers:    map[string]publishers.PublisherConfig{},
			publisherName: "docker-hub",
			want:          false,
		},
		{
			name:          "nil publishers",
			publishers:    nil,
			publisherName: "docker-hub",
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &RepoConfig{
				RepoConfig: &schema.RepoConfig{
					Publishers: tt.publishers,
				},
			}

			got := repo.HasPublisher(tt.publisherName)

			if got != tt.want {
				t.Errorf("HasPublisher() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsMonorepo tests the IsMonorepo method.
func TestIsMonorepo(t *testing.T) {
	tests := []struct {
		name     string
		strategy string
		nilRepo  bool
		want     bool
	}{
		{
			name:     "monorepo strategy",
			strategy: "monorepo",
			want:     true,
		},
		{
			name:     "tag-all strategy",
			strategy: "tag-all",
			want:     false,
		},
		{
			name:     "empty strategy",
			strategy: "",
			want:     false,
		},
		{
			name:    "nil repo config",
			nilRepo: true,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var repo *RepoConfig
			if tt.nilRepo {
				repo = &RepoConfig{}
			} else {
				repo = &RepoConfig{
					RepoConfig: &schema.RepoConfig{
						Tagging: schema.TaggingStrategy{
							Strategy: tt.strategy,
						},
					},
				}
			}

			got := repo.IsMonorepo()

			if got != tt.want {
				t.Errorf("IsMonorepo() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsTagAll tests the IsTagAll method.
func TestIsTagAll(t *testing.T) {
	tests := []struct {
		name     string
		strategy string
		nilRepo  bool
		want     bool
	}{
		{
			name:     "tag-all strategy",
			strategy: "tag-all",
			want:     true,
		},
		{
			name:     "monorepo strategy",
			strategy: "monorepo",
			want:     false,
		},
		{
			name:     "empty strategy",
			strategy: "",
			want:     false,
		},
		{
			name:    "nil repo config",
			nilRepo: true,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var repo *RepoConfig
			if tt.nilRepo {
				repo = &RepoConfig{}
			} else {
				repo = &RepoConfig{
					RepoConfig: &schema.RepoConfig{
						Tagging: schema.TaggingStrategy{
							Strategy: tt.strategy,
						},
					},
				}
			}

			got := repo.IsTagAll()

			if got != tt.want {
				t.Errorf("IsTagAll() = %v, want %v", got, tt.want)
			}
		})
	}
}
