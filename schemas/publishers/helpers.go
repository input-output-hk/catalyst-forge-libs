// Package publishers provides helper methods for working with publisher discriminated unions.
package publishers

import (
	"encoding/json"
	"fmt"
)

// Type returns the discriminator value for this PublisherConfig.
// Returns an empty string if the type field is missing or not a string.
func (pc PublisherConfig) Type() string {
	if t, ok := pc["type"].(string); ok {
		return t
	}
	return ""
}

// AsDocker attempts to convert the PublisherConfig to a DockerPublisher.
// Returns the publisher and true if successful, nil and false otherwise.
func (pc PublisherConfig) AsDocker() (*DockerPublisher, bool) {
	if pc.Type() != "docker" {
		return nil, false
	}

	// Re-marshal and unmarshal to convert map[string]any to DockerPublisher
	data, err := json.Marshal(pc)
	if err != nil {
		return nil, false
	}

	var docker DockerPublisher
	if err := json.Unmarshal(data, &docker); err != nil {
		return nil, false
	}

	return &docker, true
}

// AsGitHub attempts to convert the PublisherConfig to a GitHubPublisher.
// Returns the publisher and true if successful, nil and false otherwise.
func (pc PublisherConfig) AsGitHub() (*GitHubPublisher, bool) {
	if pc.Type() != "github" {
		return nil, false
	}

	data, err := json.Marshal(pc)
	if err != nil {
		return nil, false
	}

	var github GitHubPublisher
	if err := json.Unmarshal(data, &github); err != nil {
		return nil, false
	}

	return &github, true
}

// AsS3 attempts to convert the PublisherConfig to an S3Publisher.
// Returns the publisher and true if successful, nil and false otherwise.
func (pc PublisherConfig) AsS3() (*S3Publisher, bool) {
	if pc.Type() != "s3" {
		return nil, false
	}

	data, err := json.Marshal(pc)
	if err != nil {
		return nil, false
	}

	var s3 S3Publisher
	if err := json.Unmarshal(data, &s3); err != nil {
		return nil, false
	}

	return &s3, true
}

// Validate checks if the PublisherConfig has a valid type discriminator.
// Returns an error if the type is missing or unknown.
func (pc PublisherConfig) Validate() error {
	typ := pc.Type()
	if typ == "" {
		return fmt.Errorf("publisher config missing 'type' field")
	}

	switch typ {
	case "docker", "github", "s3":
		return nil
	default:
		return fmt.Errorf("unknown publisher type: %q", typ)
	}
}
