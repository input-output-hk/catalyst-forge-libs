package config

import (
	"sort"

	"github.com/input-output-hk/catalyst-forge-libs/schema"
	"github.com/input-output-hk/catalyst-forge-libs/schema/phases"
	"github.com/input-output-hk/catalyst-forge-libs/schema/publishers"
)

// RepoConfig wraps schema.RepoConfig with helper methods for convenient access
// to repository configuration data. All methods are read-only; configurations
// are immutable after loading.
type RepoConfig struct {
	*schema.RepoConfig // Embedded for direct access to all schema fields
}

// Phase helper methods

// GetPhase retrieves a phase definition by name.
// Returns the phase definition and true if found, or nil and false if not found.
func (r *RepoConfig) GetPhase(name string) (*phases.PhaseDefinition, bool) {
	if r.Phases == nil {
		return nil, false
	}
	phase, ok := r.Phases[name]
	if !ok {
		return nil, false
	}
	return &phase, true
}

// ListPhases returns a sorted list of all phase names defined in the repository.
// The list is sorted alphabetically for deterministic output.
func (r *RepoConfig) ListPhases() []string {
	if r.Phases == nil {
		return []string{}
	}
	names := make([]string, 0, len(r.Phases))
	for name := range r.Phases {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// HasPhase checks if a phase with the given name exists.
func (r *RepoConfig) HasPhase(name string) bool {
	if r.Phases == nil {
		return false
	}
	_, ok := r.Phases[name]
	return ok
}

// Publisher helper methods

// GetPublisher retrieves a publisher configuration by name.
// Returns the publisher configuration and true if found, or an empty map and false if not found.
func (r *RepoConfig) GetPublisher(name string) (publishers.PublisherConfig, bool) {
	if r.Publishers == nil {
		return nil, false
	}
	pub, ok := r.Publishers[name]
	return pub, ok
}

// ListPublishers returns a sorted list of all publisher names defined in the repository.
// The list is sorted alphabetically for deterministic output.
func (r *RepoConfig) ListPublishers() []string {
	if r.Publishers == nil {
		return []string{}
	}
	names := make([]string, 0, len(r.Publishers))
	for name := range r.Publishers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// HasPublisher checks if a publisher with the given name exists.
func (r *RepoConfig) HasPublisher(name string) bool {
	if r.Publishers == nil {
		return false
	}
	_, ok := r.Publishers[name]
	return ok
}

// Tagging strategy helper methods

// IsMonorepo checks if the repository uses monorepo tagging strategy.
// Monorepo strategy creates individual tags per project.
func (r *RepoConfig) IsMonorepo() bool {
	if r.RepoConfig == nil {
		return false
	}
	return r.Tagging.Strategy == "monorepo"
}

// IsTagAll checks if the repository uses tag-all tagging strategy.
// Tag-all strategy creates a single tag for the entire repository.
func (r *RepoConfig) IsTagAll() bool {
	if r.RepoConfig == nil {
		return false
	}
	return r.Tagging.Strategy == "tag-all"
}
